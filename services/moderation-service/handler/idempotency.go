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

package handler

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// IdempotencyMiddleware dedupes repeated commands carrying an Idempotency-Key header
// (double-clicks, client retries) using a short-lived, per-user Redis marker. The
// first request proceeds; a duplicate within the TTL gets a 200 no-op so the platform
// action is not performed twice. Fails open on Redis errors (availability over dedup).
//
// The marker is set on receipt, so a retry after a *failed* action within the TTL is
// also deduped — keep the TTL short. Requests without the header always proceed.
// Must run after JWTAuth so the key is scoped to the authenticated user.
func IdempotencyMiddleware(rdb *redis.Client, ttl time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
		if key == "" {
			c.Next()
			return
		}
		redisKey := fmt.Sprintf("modidem:%s:%s", c.GetString("user_id"), key)
		acquired, err := rdb.SetNX(c.Request.Context(), redisKey, "1", ttl).Result()
		if err != nil {
			c.Next() // fail open — don't block moderation on a Redis blip
			return
		}
		if !acquired {
			c.AbortWithStatusJSON(http.StatusOK, gin.H{"status": "duplicate_ignored"})
			return
		}
		c.Next()
	}
}
