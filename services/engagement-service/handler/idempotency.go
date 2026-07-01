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

// IdempotencyMiddleware dedupes repeated writes carrying an Idempotency-Key header
// (double-clicks, client retries) via a short-lived per-identity Redis marker. The
// key is scoped to the authenticated user_id or viewer_id, so it must run after
// JWTAuth. Fails open on Redis errors (availability over dedup). Requests without
// the header always proceed — the durable dedup (poll_votes PK, points dedup_key)
// is the real guard; this only smooths client-side retries.
func IdempotencyMiddleware(rdb *redis.Client, ttl time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
		if key == "" {
			c.Next()
			return
		}
		identity := c.GetString("user_id")
		if identity == "" {
			identity = c.GetString("viewer_id")
		}
		redisKey := fmt.Sprintf("engidem:%s:%s", identity, key)
		acquired, err := rdb.SetNX(c.Request.Context(), redisKey, "1", ttl).Result()
		if err != nil {
			c.Next() // fail open
			return
		}
		if !acquired {
			c.AbortWithStatusJSON(http.StatusOK, gin.H{"status": "duplicate_ignored"})
			return
		}
		c.Next()
	}
}
