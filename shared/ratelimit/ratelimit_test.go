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

package ratelimit

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func setupTestRedis(t *testing.T) *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15, // Use separate DB for tests
	})

	// Ping to check connection
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available for testing")
	}

	// Clean up test keys
	client.FlushDB(ctx)

	return client
}

func TestRateLimiter_AllowsWithinLimit(t *testing.T) {
	redisClient := setupTestRedis(t)
	defer redisClient.Close()

	gin.SetMode(gin.TestMode)

	rl := NewRateLimiter(Config{
		RequestsPerMinute: 10,
		KeyPrefix:         "test",
		RedisClient:       redisClient,
		Logger:            zap.NewNop(),
	})

	router := gin.New()
	router.Use(rl.Middleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	// Make requests within limit
	for i := 0; i < 10; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, 200, w.Code, "Request %d should succeed", i+1)
		assert.Contains(t, w.Header().Get("X-RateLimit-Limit"), "10")
	}
}

func TestRateLimiter_BlocksOverLimit(t *testing.T) {
	redisClient := setupTestRedis(t)
	defer redisClient.Close()

	gin.SetMode(gin.TestMode)

	rl := NewRateLimiter(Config{
		RequestsPerMinute: 5,
		KeyPrefix:         "test",
		RedisClient:       redisClient,
		Logger:            zap.NewNop(),
	})

	router := gin.New()
	router.Use(rl.Middleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	// Make requests up to limit
	for i := 0; i < 5; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)
	}

	// Next request should be blocked
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 429, w.Code)
	assert.NotEmpty(t, w.Header().Get("Retry-After"))
}

func TestRateLimiter_RemainingHeader(t *testing.T) {
	redisClient := setupTestRedis(t)
	defer redisClient.Close()

	gin.SetMode(gin.TestMode)

	rl := NewRateLimiter(Config{
		RequestsPerMinute: 10,
		KeyPrefix:         "test",
		RedisClient:       redisClient,
		Logger:            zap.NewNop(),
	})

	router := gin.New()
	router.Use(rl.Middleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	// First request
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, "10", w.Header().Get("X-RateLimit-Limit"))
	assert.Equal(t, "9", w.Header().Get("X-RateLimit-Remaining"))
	assert.NotEmpty(t, w.Header().Get("X-RateLimit-Reset"))
}

func TestRateLimiter_UserIDOverIP(t *testing.T) {
	redisClient := setupTestRedis(t)
	defer redisClient.Close()

	gin.SetMode(gin.TestMode)

	rl := NewRateLimiter(Config{
		RequestsPerMinute: 5,
		KeyPrefix:         "test",
		RedisClient:       redisClient,
		Logger:            zap.NewNop(),
	})

	router := gin.New()
	router.Use(func(c *gin.Context) {
		// Simulate authenticated request
		c.Set("user_id", "user-123")
		c.Next()
	})
	router.Use(rl.Middleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	// Make requests - should be limited per user, not IP
	for i := 0; i < 5; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, 200, w.Code)
	}

	// 6th request should be blocked
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, 429, w.Code)
}

func TestCheckLimitForKey(t *testing.T) {
	redisClient := setupTestRedis(t)
	defer redisClient.Close()

	rl := NewRateLimiter(Config{
		RequestsPerMinute: 3,
		KeyPrefix:         "test",
		RedisClient:       redisClient,
		Logger:            zap.NewNop(),
	})

	ctx := context.Background()

	// Check limit multiple times
	for i := 1; i <= 3; i++ {
		allowed, remaining, _, err := rl.CheckLimitForKey(ctx, "test-key")
		assert.NoError(t, err)
		assert.True(t, allowed)
		assert.Equal(t, 3-i, remaining)
	}

	// 4th check should fail
	allowed, remaining, _, err := rl.CheckLimitForKey(ctx, "test-key")
	assert.NoError(t, err)
	assert.False(t, allowed)
	assert.Equal(t, 0, remaining)
}

func TestResetLimit(t *testing.T) {
	redisClient := setupTestRedis(t)
	defer redisClient.Close()

	rl := NewRateLimiter(Config{
		RequestsPerMinute: 2,
		KeyPrefix:         "test",
		RedisClient:       redisClient,
		Logger:            zap.NewNop(),
	})

	ctx := context.Background()
	key := "test:custom:test-key"

	// Use up limit
	rl.CheckLimitForKey(ctx, "test-key")
	rl.CheckLimitForKey(ctx, "test-key")

	allowed, _, _, _ := rl.CheckLimitForKey(ctx, "test-key")
	assert.False(t, allowed, "Should be rate limited")

	// Reset limit
	err := rl.ResetLimit(ctx, key)
	assert.NoError(t, err)

	// Should work again
	allowed, _, _, err = rl.CheckLimitForKey(ctx, "test-key")
	assert.NoError(t, err)
	assert.True(t, allowed, "Should be allowed after reset")
}
