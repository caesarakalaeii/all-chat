package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// NewClient creates a new Redis client with optimized settings
func NewClient(addr, password string) (*redis.Client, error) {
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

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return client, nil
}

// NewRedisClient is an alias for backwards compatibility
func NewRedisClient(addr string) *redis.Client {
	client, _ := NewClient(addr, "")
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
