// This file is part of All-Chat.
// Copyright (C) 2026 caesarakalaeii
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package listener

import (
	"context"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

// PublishFunc is the underlying publish operation the ring buffer wraps.
type PublishFunc func(ctx context.Context, payload []byte) error

// bufferedMsg holds a payload enqueued for retry.
type bufferedMsg struct {
	payload    []byte
	enqueuedAt time.Time
}

// RingBufferPublisher wraps a publish function with an in-memory ring buffer
// that retries failed publishes. Opt-in per D-07.
//
// When publishFn fails, the message is buffered instead of being dropped.
// A background goroutine retries buffered messages every 500ms using
// context.Background() so retries are not tied to the caller's context.
//
// When the buffer reaches capacity, the oldest message is dropped and
// ring_buffer_drops_total is incremented.
//
// Call Stop() during service shutdown to cleanly drain the retry goroutine.
type RingBufferPublisher struct {
	publishFn   PublishFunc
	logger      *zap.Logger
	serviceName string // used in overflow log and Prometheus labels

	mu       sync.Mutex
	buf      []bufferedMsg
	head     int
	tail     int
	size     int
	capacity int

	stopCh  chan struct{}
	stopped bool
	wg      sync.WaitGroup

	// Metrics
	depth      prometheus.Gauge
	dropsTotal prometheus.Counter

	// dropsCount mirrors dropsTotal for test assertions (prometheus counters are not readable)
	dropsCount int64
}

// NewRingBufferPublisher creates a RingBufferPublisher and starts its retry goroutine.
//
// capacity is the maximum number of messages to hold; 1000 is the recommended default.
// publishFn is the underlying publish operation (e.g. XADD to Redis).
// logger may be nil; a no-op logger is used in that case.
// serviceName is used as the Prometheus label value for ring_buffer_depth and ring_buffer_drops_total.
//
// Metrics are registered with prometheus.DefaultRegisterer. Use NewRingBufferPublisherWithRegisterer
// for tests or scenarios requiring a custom registry.
func NewRingBufferPublisher(capacity int, publishFn PublishFunc, logger *zap.Logger, serviceName string) *RingBufferPublisher {
	return NewRingBufferPublisherWithRegisterer(capacity, publishFn, logger, serviceName, prometheus.DefaultRegisterer)
}

// NewRingBufferPublisherWithRegisterer is like NewRingBufferPublisher but registers
// metrics with the provided prometheus.Registerer. Use prometheus.NewRegistry() in tests
// to avoid duplicate registration panics.
func NewRingBufferPublisherWithRegisterer(capacity int, publishFn PublishFunc, logger *zap.Logger, serviceName string, reg prometheus.Registerer) *RingBufferPublisher {
	if logger == nil {
		logger = zap.NewNop()
	}

	depth := prometheus.NewGauge(prometheus.GaugeOpts{
		Name:        "ring_buffer_depth",
		Help:        "Current number of messages in the publish retry ring buffer",
		ConstLabels: prometheus.Labels{"service": serviceName},
	})
	dropsTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Name:        "ring_buffer_drops_total",
		Help:        "Total messages dropped from ring buffer due to capacity overflow",
		ConstLabels: prometheus.Labels{"service": serviceName},
	})

	// Best-effort registration: if already registered (e.g. duplicate service name), ignore.
	_ = reg.Register(depth)
	_ = reg.Register(dropsTotal)

	rb := &RingBufferPublisher{
		publishFn:   publishFn,
		logger:      logger,
		serviceName: serviceName,
		buf:         make([]bufferedMsg, capacity),
		capacity:    capacity,
		stopCh:      make(chan struct{}),
		depth:       depth,
		dropsTotal:  dropsTotal,
	}

	rb.wg.Add(1)
	go rb.retryLoop()

	return rb
}

// Publish attempts to call publishFn immediately. If publishFn returns an error,
// the payload is buffered for retry and Publish returns nil — the caller is not
// made aware of the transient failure.
//
// Returns an error only if the ring buffer has been stopped.
func (rb *RingBufferPublisher) Publish(ctx context.Context, payload []byte) error {
	rb.mu.Lock()
	if rb.stopped {
		rb.mu.Unlock()
		return context.Canceled
	}
	rb.mu.Unlock()

	if err := rb.publishFn(ctx, payload); err != nil {
		rb.enqueue(payload)
		rb.logger.Warn("publish failed — buffered for retry",
			zap.Int("buffer_depth", rb.getSize()),
			zap.Error(err),
		)
		return nil // Caller does not see the error — message is buffered
	}
	return nil
}

// Stop signals the retry goroutine to exit and waits for it to finish.
// It is safe to call Stop from any goroutine. After Stop returns, no further
// retries will occur.
func (rb *RingBufferPublisher) Stop() {
	rb.mu.Lock()
	if rb.stopped {
		rb.mu.Unlock()
		return
	}
	rb.stopped = true
	rb.mu.Unlock()
	close(rb.stopCh)
	rb.wg.Wait()
}

// enqueue adds payload to the ring buffer. If the buffer is full, the oldest
// message is overwritten (ring semantics) and dropsTotal is incremented.
// Caller must NOT hold rb.mu — enqueue acquires it internally.
func (rb *RingBufferPublisher) enqueue(payload []byte) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.size == rb.capacity {
		// Buffer full: drop oldest message (advance head)
		rb.head = (rb.head + 1) % rb.capacity
		rb.size--
		rb.dropsTotal.Inc()
		rb.dropsCount++
		// Error-level sentinel log matched by Grafana Loki alert rule.
		rb.logger.Error("ring_buffer_overflow_drop",
			zap.String("service", rb.serviceName),
			zap.Int("capacity", rb.capacity),
			zap.Int("current_depth", rb.size),
		)
	}

	rb.buf[rb.tail] = bufferedMsg{
		payload:    payload,
		enqueuedAt: time.Now(),
	}
	rb.tail = (rb.tail + 1) % rb.capacity
	rb.size++
	rb.depth.Set(float64(rb.size))
}

// dequeueHead removes and returns the head item. Returns (nil, false) if empty.
// Caller MUST hold rb.mu.
func (rb *RingBufferPublisher) dequeueHead() (bufferedMsg, bool) {
	if rb.size == 0 {
		return bufferedMsg{}, false
	}
	item := rb.buf[rb.head]
	rb.head = (rb.head + 1) % rb.capacity
	rb.size--
	rb.depth.Set(float64(rb.size))
	return item, true
}

// retryLoop runs in a background goroutine, attempting to re-publish buffered
// messages every 500ms using context.Background().
func (rb *RingBufferPublisher) retryLoop() {
	defer rb.wg.Done()
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-rb.stopCh:
			return
		case <-ticker.C:
			rb.drainOneTick()
		}
	}
}

// drainOneTick attempts to re-publish all buffered messages in one tick.
// On failure, the message is re-enqueued at the front (left in place) and we
// stop trying for this tick to avoid thundering-herd retries.
func (rb *RingBufferPublisher) drainOneTick() {
	for {
		rb.mu.Lock()
		item, ok := rb.dequeueHead()
		rb.mu.Unlock()

		if !ok {
			return
		}

		if err := rb.publishFn(context.Background(), item.payload); err != nil {
			// Re-enqueue at front by temporarily re-adding (enqueue appends at tail,
			// so we put it back and stop processing this tick).
			rb.requeue(item)
			return
		}
	}
}

// requeue puts item back at the head of the buffer without incrementing drops.
// Used when a retry attempt fails — the message stays in the buffer for the next tick.
func (rb *RingBufferPublisher) requeue(item bufferedMsg) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	if rb.size == rb.capacity {
		// Buffer is full — drop this requeue (we already removed it; it's lost)
		rb.dropsTotal.Inc()
		rb.dropsCount++
		return
	}

	// Move head back one slot (ring semantics)
	rb.head = (rb.head - 1 + rb.capacity) % rb.capacity
	rb.buf[rb.head] = item
	rb.size++
	rb.depth.Set(float64(rb.size))
}

// getSize returns the current number of buffered messages. Thread-safe.
func (rb *RingBufferPublisher) getSize() int {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	return rb.size
}

// getDropsTotal returns the total number of messages dropped due to overflow. Thread-safe.
func (rb *RingBufferPublisher) getDropsTotal() int64 {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	return rb.dropsCount
}
