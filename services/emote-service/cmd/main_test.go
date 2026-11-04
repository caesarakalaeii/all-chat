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

package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// TestRateLimitMiddleware_BypassesProbeEndpoints documents the contract that
// kubelet liveness/readiness probes and Prometheus scrapes must never be
// rate-limited. Without the bypass, a single node-gateway IP saturating one
// rate-limit bucket trips the readiness probe with 429 and kubelet kills the
// pod in a loop (observed in production: ~327 restarts in 22h).
func TestRateLimitMiddleware_BypassesProbeEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	// Limit of 1 per minute: any non-bypassed second hit must return 429.
	rl := newRedisRateLimiter(rdb, 1, time.Minute)
	mw := rateLimitMiddleware(rl, zap.NewNop())

	router := gin.New()
	router.Use(mw)
	router.GET("/health/live", func(c *gin.Context) { c.Status(http.StatusOK) })
	router.GET("/health/ready", func(c *gin.Context) { c.Status(http.StatusOK) })
	router.GET("/metrics", func(c *gin.Context) { c.Status(http.StatusOK) })
	router.GET("/emotes/twitch/global", func(c *gin.Context) { c.Status(http.StatusOK) })

	hit := func(path string) int {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.RemoteAddr = "10.42.3.1:54321" // Same source IP as kubelet would use.
		router.ServeHTTP(w, req)
		return w.Code
	}

	// Probe endpoints must always pass, even when hammered far past the limit.
	for _, path := range []string{"/health/live", "/health/ready", "/metrics"} {
		for i := 0; i < 5; i++ {
			require.Equal(t, http.StatusOK, hit(path),
				"probe endpoint %s must bypass rate limit (iteration %d)", path, i)
		}
	}

	// Sanity check: a non-probe endpoint from the same source IP still gets
	// rate-limited. First request through, second must be 429.
	assert.Equal(t, http.StatusOK, hit("/emotes/twitch/global"))
	assert.Equal(t, http.StatusTooManyRequests, hit("/emotes/twitch/global"))
}

// The bucket key holds the raw client IP or the raw API key. Redis drops it
// after the window; the 429 warning goes to Loki and may carry neither.
func TestRateLimitMiddleware_LogsNoRawIPOrAPIKey(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for name, apiKey := range map[string]string{"by IP": "", "by API key": "secret-api-key"} {
		t.Run(name, func(t *testing.T) {
			mr := miniredis.RunT(t)
			rl := newRedisRateLimiter(redis.NewClient(&redis.Options{Addr: mr.Addr()}), 1, time.Minute)
			core, logs := observer.New(zap.DebugLevel)

			router := gin.New()
			router.Use(rateLimitMiddleware(rl, zap.New(core)))
			router.GET("/emotes/twitch/global", func(c *gin.Context) { c.Status(http.StatusOK) })
			for i := 0; i < 2; i++ {
				req := httptest.NewRequest(http.MethodGet, "/emotes/twitch/global", nil)
				req.RemoteAddr = "203.0.113.42:54321"
				if apiKey != "" {
					req.Header.Set(apiKeyHeader, apiKey)
				}
				router.ServeHTTP(httptest.NewRecorder(), req)
			}

			require.Equal(t, 1, logs.FilterMessage("Rate limit exceeded").Len())
			logged := fmt.Sprint(logs.All()[0].ContextMap())
			assert.NotContains(t, logged, "203.0.113.42")
			assert.NotContains(t, logged, "secret-api-key")
		})
	}
}
