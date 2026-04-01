package ratelimit

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"net"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Config holds rate limiter configuration
type Config struct {
	RequestsPerMinute int           // Max requests per minute per IP/user
	BurstSize         int           // Burst allowance (tokens)
	KeyPrefix         string        // Redis key prefix
	RedisClient       *redis.Client // Redis client for distributed rate limiting
	Logger            *zap.Logger
}

// RateLimiter provides rate limiting functionality
type RateLimiter struct {
	cfg Config
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(cfg Config) *RateLimiter {
	if cfg.KeyPrefix == "" {
		cfg.KeyPrefix = "ratelimit"
	}
	if cfg.BurstSize == 0 {
		cfg.BurstSize = cfg.RequestsPerMinute
	}
	if cfg.Logger == nil {
		cfg.Logger = zap.NewNop()
	}
	return &RateLimiter{cfg: cfg}
}

// Middleware returns a Gin middleware that enforces rate limits
func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Determine the rate limit key (IP address or user ID if authenticated)
		key := rl.getClientKey(c)

		// Check rate limit
		allowed, remaining, resetTime, err := rl.checkLimit(c.Request.Context(), key)
		if err != nil {
			rl.cfg.Logger.Error("Rate limit check failed",
				zap.Error(err),
				zap.String("key", key))
			// On error, allow the request (fail open)
			c.Next()
			return
		}

		// Set rate limit headers
		c.Header("X-RateLimit-Limit", strconv.Itoa(rl.cfg.RequestsPerMinute))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(remaining))
		c.Header("X-RateLimit-Reset", strconv.FormatInt(resetTime.Unix(), 10))

		if !allowed {
			rl.cfg.Logger.Warn("Rate limit exceeded",
				zap.String("key", key),
				zap.String("ip", anonymizeIP(c.ClientIP())),
				zap.String("path", c.Request.URL.Path))

			c.Header("Retry-After", strconv.FormatInt(int64(time.Until(resetTime).Seconds()), 10))
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "rate limit exceeded",
				"retry_after": resetTime.Unix(),
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// getClientKey determines the rate limit key for the client
func (rl *RateLimiter) getClientKey(c *gin.Context) string {
	// Try to get user ID from JWT context (if authenticated)
	if userID, exists := c.Get("user_id"); exists {
		return fmt.Sprintf("%s:user:%s", rl.cfg.KeyPrefix, userID)
	}

	// Fall back to IP address
	return fmt.Sprintf("%s:ip:%s", rl.cfg.KeyPrefix, c.ClientIP())
}

// checkLimit checks if the request is within rate limits using Redis
// Returns: allowed, remaining, resetTime, error
func (rl *RateLimiter) checkLimit(ctx context.Context, key string) (bool, int, time.Time, error) {
	now := time.Now()
	windowStart := now.Truncate(time.Minute)
	resetTime := windowStart.Add(time.Minute)

	// Use Redis INCR with expiry for distributed rate limiting
	pipe := rl.cfg.RedisClient.Pipeline()

	// Increment counter
	incrCmd := pipe.Incr(ctx, key)

	// Set expiry if key is new (INCR returns 1)
	pipe.ExpireAt(ctx, key, resetTime)

	// Execute pipeline
	_, err := pipe.Exec(ctx)
	if err != nil {
		return false, 0, resetTime, fmt.Errorf("redis pipeline: %w", err)
	}

	// Get current count
	count := int(incrCmd.Val())

	// Check if limit exceeded
	allowed := count <= rl.cfg.RequestsPerMinute
	remaining := rl.cfg.RequestsPerMinute - count
	if remaining < 0 {
		remaining = 0
	}

	return allowed, remaining, resetTime, nil
}

// CheckLimitForKey checks rate limit for a specific key (for custom use cases)
func (rl *RateLimiter) CheckLimitForKey(ctx context.Context, key string) (bool, int, time.Time, error) {
	fullKey := fmt.Sprintf("%s:custom:%s", rl.cfg.KeyPrefix, key)
	return rl.checkLimit(ctx, fullKey)
}

// ResetLimit resets the rate limit for a specific client (admin function)
func (rl *RateLimiter) ResetLimit(ctx context.Context, key string) error {
	return rl.cfg.RedisClient.Del(ctx, key).Err()
}

// anonymizeIP truncates the last octet (IPv4) or last 80 bits (IPv6) for
// DSGVO-compliant log output. Kept local to avoid a circular dependency
// on the shared/middleware package.
func anonymizeIP(raw string) string {
	ip := net.ParseIP(raw)
	if ip == nil {
		return raw
	}
	if v4 := ip.To4(); v4 != nil {
		v4[3] = 0
		return v4.String()
	}
	v6 := ip.To16()
	for i := 6; i < 16; i++ {
		v6[i] = 0
	}
	return v6.String()
}
