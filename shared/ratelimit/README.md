# Rate Limiting Package

Distributed rate limiting for All-Chat microservices using Redis.

## Features

- ✅ Redis-based distributed rate limiting
- ✅ Per-IP and per-user rate limits
- ✅ Automatic rate limit headers (X-RateLimit-*)
- ✅ Configurable limits and burst sizes
- ✅ Fail-open on Redis errors
- ✅ Admin functions for limit management

## Quick Start

### 1. Initialize Rate Limiter

```go
import (
    "github.com/caesar/all-chat/shared/ratelimit"
    "github.com/redis/go-redis/v9"
)

func main() {
    redisClient := redis.NewClient(&redis.Options{
        Addr: "localhost:6379",
    })

    rl := ratelimit.NewRateLimiter(ratelimit.Config{
        RequestsPerMinute: 60,      // 60 requests per minute
        BurstSize:         10,       // Allow burst of 10
        KeyPrefix:         "api",    // Redis key prefix
        RedisClient:       redisClient,
        Logger:            log,
    })

    router := gin.New()
    router.Use(rl.Middleware())

    // Your routes...
}
```

### 2. Apply to Specific Routes

```go
// Rate limit only specific routes
publicAPI := router.Group("/api/v1")
publicAPI.Use(rl.Middleware())
{
    publicAPI.GET("/data", handleData)
    publicAPI.POST("/submit", handleSubmit)
}

// No rate limit on health checks
router.GET("/health", handleHealth)
```

### 3. Different Limits for Different Routes

```go
// Strict rate limit for expensive operations
strictRL := ratelimit.NewRateLimiter(ratelimit.Config{
    RequestsPerMinute: 10,
    RedisClient:       redisClient,
    KeyPrefix:         "api:expensive",
    Logger:            log,
})

expensiveAPI := router.Group("/api/v1/expensive")
expensiveAPI.Use(strictRL.Middleware())

// Relaxed rate limit for cheap operations
relaxedRL := ratelimit.NewRateLimiter(ratelimit.Config{
    RequestsPerMinute: 100,
    RedisClient:       redisClient,
    KeyPrefix:         "api:cheap",
    Logger:            log,
})

cheapAPI := router.Group("/api/v1/cheap")
cheapAPI.Use(relaxedRL.Middleware())
```

## Configuration

| Field | Description | Default |
|-------|-------------|---------|
| `RequestsPerMinute` | Max requests per minute per client | Required |
| `BurstSize` | Additional burst allowance | `RequestsPerMinute` |
| `KeyPrefix` | Redis key prefix | `"ratelimit"` |
| `RedisClient` | Redis client instance | Required |
| `Logger` | Zap logger for warnings | `zap.NewNop()` |

## Rate Limit Keys

The rate limiter uses different keys based on authentication:

1. **Authenticated users**: `{prefix}:user:{user_id}`
   - Limits per user across all IPs
   - User ID extracted from JWT context

2. **Anonymous users**: `{prefix}:ip:{ip_address}`
   - Limits per IP address
   - Falls back to IP if no auth

## Response Headers

The middleware adds standard rate limit headers:

```
X-RateLimit-Limit: 60
X-RateLimit-Remaining: 42
X-RateLimit-Reset: 1640000000
```

When rate limited (429 status):
```
Retry-After: 45
```

## Custom Rate Limit Checks

For custom logic outside HTTP requests:

```go
// Check limit for custom key
allowed, remaining, resetTime, err := rl.CheckLimitForKey(ctx, "operation:user-123")
if !allowed {
    return errors.New("rate limit exceeded")
}

// Proceed with operation
doExpensiveOperation()
```

## Admin Functions

```go
// Reset rate limit for a specific client
err := rl.ResetLimit(ctx, "ratelimit:user:user-123")

// Or reset IP-based limit
err := rl.ResetLimit(ctx, "ratelimit:ip:192.168.1.1")
```

## Error Handling

The rate limiter **fails open** on Redis errors:
- If Redis is unavailable, requests are allowed
- Errors are logged for monitoring
- Prevents Redis outage from blocking all traffic

## Testing

Run tests with Redis available:

```bash
# Start Redis
docker run -d -p 6379:6379 redis:latest

# Run tests
go test ./shared/ratelimit/...
```

## Example: API Gateway

```go
package main

import (
    "github.com/caesar/all-chat/shared/ratelimit"
    "github.com/gin-gonic/gin"
)

func main() {
    router := gin.New()

    // Global rate limit (relaxed)
    globalRL := ratelimit.NewRateLimiter(ratelimit.Config{
        RequestsPerMinute: 120,
        KeyPrefix:         "api:global",
        RedisClient:       redisClient,
        Logger:            log,
    })
    router.Use(globalRL.Middleware())

    // Public API (moderate limits)
    publicRL := ratelimit.NewRateLimiter(ratelimit.Config{
        RequestsPerMinute: 60,
        KeyPrefix:         "api:public",
        RedisClient:       redisClient,
        Logger:            log,
    })

    publicAPI := router.Group("/api/v1")
    publicAPI.Use(publicRL.Middleware())

    // Auth endpoints (strict limits to prevent brute force)
    authRL := ratelimit.NewRateLimiter(ratelimit.Config{
        RequestsPerMinute: 10,
        KeyPrefix:         "api:auth",
        RedisClient:       redisClient,
        Logger:            log,
    })

    router.POST("/auth/login", authRL.Middleware(), handleLogin)
    router.POST("/auth/register", authRL.Middleware(), handleRegister)

    router.Run(":8080")
}
```

## Best Practices

1. **Different limits for different endpoints**
   - Auth endpoints: 5-10 req/min (prevent brute force)
   - Public API: 60 req/min (normal usage)
   - Authenticated API: 120 req/min (higher for logged-in users)

2. **Monitor rate limit hits**
   - Check logs for `Rate limit exceeded` warnings
   - Alert on high rate limit hit rates

3. **Whitelist internal services**
   - Don't rate limit internal service-to-service calls
   - Use separate routes or skip middleware for internal APIs

4. **Graceful degradation**
   - Fail open on Redis errors
   - Don't let rate limiter take down the service

5. **Clear error messages**
   - Return helpful 429 responses with Retry-After header
   - Document rate limits in API documentation

## Monitoring

Key metrics to track:
- Rate limit hits (429 responses)
- Redis errors in rate limiter
- Per-endpoint rate limit usage
- Top rate-limited IPs/users

## Redis Key Pattern

```
{prefix}:ip:{ip_address}      # IP-based limits
{prefix}:user:{user_id}       # User-based limits
{prefix}:custom:{custom_key}  # Custom limits

# Keys expire after 1 minute (sliding window)
```

## Troubleshooting

### Rate limits not working

1. Check Redis connection: `redis-cli PING`
2. Verify middleware is registered: Check router setup
3. Check Redis keys: `redis-cli KEYS ratelimit:*`

### False positives

- Check if proxy/load balancer is preserving client IPs
- Verify `X-Forwarded-For` header handling
- Consider using user-based limits for authenticated users

### Redis memory usage

- Keys auto-expire after 1 minute
- Monitor Redis memory: `redis-cli INFO memory`
- Consider using separate Redis instance for rate limiting

## Resources

- [Rate Limiting Patterns](https://redis.io/docs/reference/patterns/rate-limiter/)
- [HTTP Rate Limit Headers](https://tools.ietf.org/id/draft-polli-ratelimit-headers-00.html)
