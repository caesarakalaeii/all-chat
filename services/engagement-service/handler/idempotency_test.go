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
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIdempotencyMiddleware covers M7a: a failed handler releases its marker so a
// retry can proceed, a successful one suppresses a duplicate, and requests without
// the header always run.
func TestIdempotencyMiddleware(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	gin.SetMode(gin.TestMode)

	var calls int
	status := http.StatusOK
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("viewer_id", "v1") }) // simulate an authenticated identity
	r.Use(IdempotencyMiddleware(rdb, time.Minute))
	r.POST("/x", func(c *gin.Context) { calls++; c.JSON(status, gin.H{"ok": true}) })

	do := func(key string) int {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/x", nil)
		if key != "" {
			req.Header.Set("Idempotency-Key", key)
		}
		r.ServeHTTP(w, req)
		return w.Code
	}

	// A failed request (500) must release the marker so the retry reaches the handler.
	status = http.StatusInternalServerError
	assert.Equal(t, 500, do("k1"))
	assert.Equal(t, 1, calls)
	status = http.StatusOK
	assert.Equal(t, 200, do("k1"), "retry after a failure must not be swallowed")
	assert.Equal(t, 2, calls)

	// A successful request's marker suppresses a duplicate (handler not re-run).
	status = http.StatusOK
	assert.Equal(t, 200, do("k2"))
	assert.Equal(t, 3, calls)
	assert.Equal(t, 200, do("k2"))
	assert.Equal(t, 3, calls, "a duplicate of a succeeded request must be short-circuited")

	// No Idempotency-Key → always runs.
	assert.Equal(t, 200, do(""))
	assert.Equal(t, 4, calls)
	assert.Equal(t, 200, do(""))
	assert.Equal(t, 5, calls)
}
