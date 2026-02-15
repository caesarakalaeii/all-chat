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
