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

package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
)

// Client captures the subset of redis.Client behavior used by shared packages.
type Client interface {
	Incr(ctx context.Context, key string) *redis.IntCmd
	Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd
	TTL(ctx context.Context, key string) *redis.DurationCmd
}

// NewClientWithTracing creates a Redis client with optional OpenTelemetry instrumentation
func NewClientWithTracing(addr, password string, tracingEnabled bool) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password, // Support password auth
		DB:           0,        // Default DB
		PoolSize:     50,       // Max connections per client
		MinIdleConns: 10,       // Keep warm connections
		MaxRetries:   3,        // Retry failed commands
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolTimeout:  4 * time.Second, // Wait for connection from pool
	})

	// Test connection BEFORE instrumentation
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	// Add OpenTelemetry instrumentation if enabled
	if tracingEnabled {
		// Add distributed tracing (creates spans for each Redis command)
		if err := redisotel.InstrumentTracing(client); err != nil {
			client.Close()
			return nil, fmt.Errorf("failed to instrument Redis tracing: %w", err)
		}

		// Add metrics (connection pool stats, command counts)
		if err := redisotel.InstrumentMetrics(client); err != nil {
			// Metrics are optional - don't fail if this errors
			// Continue with tracing only
		}
	}

	return client, nil
}

// NewClient creates a Redis client without tracing (backward compatibility)
func NewClient(addr, password string) (*redis.Client, error) {
	return NewClientWithTracing(addr, password, false)
}

// RetryOptions configures startup connection retry with exponential backoff.
type RetryOptions struct {
	// MaxAttempts is the maximum number of connection attempts. A value <= 0
	// means retry until the supplied context is cancelled.
	MaxAttempts int
	// InitialBackoff is the wait before the second attempt. Defaults to 1s.
	InitialBackoff time.Duration
	// MaxBackoff caps the backoff between attempts. Defaults to 30s.
	MaxBackoff time.Duration
}

// DefaultRetryOptions returns sensible startup-retry defaults: retry indefinitely
// (bounded only by the caller's context) with backoff growing from 1s to 30s.
// This keeps a service alive — reporting not-ready via its readiness probe —
// while Redis is briefly unavailable (e.g. rescheduled onto another node),
// instead of crash-looping on a fatal connection error.
func DefaultRetryOptions() RetryOptions {
	return RetryOptions{
		MaxAttempts:    0,
		InitialBackoff: 1 * time.Second,
		MaxBackoff:     30 * time.Second,
	}
}

// NewClientWithRetry behaves like NewClientWithTracing but retries the initial
// connection with exponential backoff instead of returning on the first failure.
// It stops and returns an error when MaxAttempts is exhausted or ctx is cancelled.
// onRetry, if non-nil, is invoked after each failed attempt (before the backoff
// sleep) so callers can log progress.
func NewClientWithRetry(ctx context.Context, addr, password string, tracingEnabled bool, opts RetryOptions, onRetry func(attempt int, err error, backoff time.Duration)) (*redis.Client, error) {
	if opts.InitialBackoff <= 0 {
		opts.InitialBackoff = 1 * time.Second
	}
	if opts.MaxBackoff <= 0 {
		opts.MaxBackoff = 30 * time.Second
	}

	backoff := opts.InitialBackoff
	var lastErr error
	for attempt := 1; ; attempt++ {
		client, err := NewClientWithTracing(addr, password, tracingEnabled)
		if err == nil {
			return client, nil
		}
		lastErr = err

		if opts.MaxAttempts > 0 && attempt >= opts.MaxAttempts {
			return nil, fmt.Errorf("redis connection failed after %d attempts: %w", attempt, lastErr)
		}

		if onRetry != nil {
			onRetry(attempt, err, backoff)
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("redis connection aborted after %d attempts (%v): %w", attempt, ctx.Err(), lastErr)
		case <-time.After(backoff):
		}

		backoff *= 2
		if backoff > opts.MaxBackoff {
			backoff = opts.MaxBackoff
		}
	}
}

// NewRedisClient is an alias for backwards compatibility (no error handling, no tracing)
func NewRedisClient(addr string) *redis.Client {
	client, _ := NewClientWithTracing(addr, "", false)
	return client
}

// HealthCheck verifies the Redis connection is healthy
func HealthCheck(client *redis.Client) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	return client.Ping(ctx).Err()
}

// BuildDSN builds a Redis connection string from environment variables
func BuildDSN(host, port string) string {
	return fmt.Sprintf("%s:%s", host, port)
}
