package deletion

import (
	"container/ring"
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

// RawMessage interface abstracts message types to avoid circular dependency
// innertube.RawChatMessage implements this via GetMessageID() and GetChannelID() methods
type RawMessage interface {
	GetMessageID() string
	GetChannelID() string
}

// Publisher interface for publishing deletion events to Redis Streams
type Publisher interface {
	Publish(ctx context.Context, msg RawMessage) error
}

// DeletionBuffer manages per-channel deletion event buffering with 500ms delay
type DeletionBuffer struct {
	bufferDuration time.Duration
	maxSize        int
	flushInterval  time.Duration
	channels       map[string]*channelBuffer
	publisher      Publisher
	mu             sync.RWMutex
	logger         *zap.Logger
	ctx            context.Context
	cancel         context.CancelFunc
	wg             sync.WaitGroup
}

type channelBuffer struct {
	ring     *ring.Ring
	mu       sync.Mutex
	ticker   *time.Ticker
	stopChan chan struct{}
	count    int
}

type bufferedEvent struct {
	message RawMessage
	addedAt time.Time
}

func NewDeletionBuffer(publisher Publisher, logger *zap.Logger) *DeletionBuffer {
	ctx, cancel := context.WithCancel(context.Background())
	return &DeletionBuffer{
		bufferDuration: 500 * time.Millisecond,
		maxSize:        1000,
		flushInterval:  100 * time.Millisecond,
		channels:       make(map[string]*channelBuffer),
		publisher:      publisher,
		logger:         logger,
		ctx:            ctx,
		cancel:         cancel,
	}
}

func (db *DeletionBuffer) Add(channelID string, deletionEvent RawMessage) error {
	db.mu.Lock()
	cb, exists := db.channels[channelID]
	if !exists {
		cb = &channelBuffer{
			ring:     ring.New(db.maxSize),
			stopChan: make(chan struct{}),
			count:    0,
		}
		db.channels[channelID] = cb
		db.startFlusher(channelID, cb)
	}
	db.mu.Unlock()

	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.ring.Value != nil {
		oldEvent := cb.ring.Value.(*bufferedEvent)
		db.logger.Warn("Deletion buffer overflow",
			zap.String("channel_id", channelID),
			zap.String("dropped_message_id", oldEvent.message.GetMessageID()),
		)
	} else {
		cb.count++
	}

	cb.ring.Value = &bufferedEvent{
		message: deletionEvent,
		addedAt: time.Now(),
	}
	cb.ring = cb.ring.Next()

	return nil
}

func (db *DeletionBuffer) startFlusher(channelID string, cb *channelBuffer) {
	cb.ticker = time.NewTicker(db.flushInterval)
	db.wg.Add(1)
	go func() {
		defer db.wg.Done()
		defer cb.ticker.Stop()
		for {
			select {
			case <-cb.stopChan:
				db.flushExpired(channelID, cb)
				return
			case <-db.ctx.Done():
				db.flushExpired(channelID, cb)
				return
			case <-cb.ticker.C:
				db.flushExpired(channelID, cb)
			}
		}
	}()
}

func (db *DeletionBuffer) flushExpired(channelID string, cb *channelBuffer) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := time.Now()
	var toClear []*ring.Ring
	r := cb.ring
	for i := 0; i < cb.ring.Len(); i++ {
		if r.Value != nil {
			event := r.Value.(*bufferedEvent)
			if now.Sub(event.addedAt) >= db.bufferDuration {
				toClear = append(toClear, r)
			}
		}
		r = r.Next()
	}

	for _, ringPos := range toClear {
		if ringPos.Value == nil {
			continue
		}
		event := ringPos.Value.(*bufferedEvent)
		ctx := context.Background()
		if err := db.publisher.Publish(ctx, event.message); err != nil {
			db.logger.Error("Failed to publish deletion event",
				zap.String("channel_id", channelID),
				zap.String("message_id", event.message.GetMessageID()),
				zap.Error(err),
			)
		}
		ringPos.Value = nil
		cb.count--
	}
}

func (db *DeletionBuffer) Cleanup(channelID string) {
	db.mu.Lock()
	cb, exists := db.channels[channelID]
	if !exists {
		db.mu.Unlock()
		return
	}
	delete(db.channels, channelID)
	db.mu.Unlock()

	close(cb.stopChan)
	db.flushAll(channelID, cb)
}

func (db *DeletionBuffer) flushAll(channelID string, cb *channelBuffer) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	ctx := context.Background()
	cb.ring.Do(func(v interface{}) {
		if v != nil {
			event := v.(*bufferedEvent)
			db.publisher.Publish(ctx, event.message)
		}
	})
}

func (db *DeletionBuffer) Shutdown() {
	db.cancel()
	db.mu.RLock()
	channelIDs := make([]string, 0, len(db.channels))
	for channelID := range db.channels {
		channelIDs = append(channelIDs, channelID)
	}
	db.mu.RUnlock()

	for _, channelID := range channelIDs {
		db.Cleanup(channelID)
	}
	db.wg.Wait()
}
