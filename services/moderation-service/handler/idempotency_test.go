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

func TestIdempotency_DuplicateKeyIsNoOp(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	gin.SetMode(gin.TestMode)
	calls := 0
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("user_id", "u1"); c.Next() })
	r.Use(IdempotencyMiddleware(rdb, time.Minute))
	r.POST("/x", func(c *gin.Context) { calls++; c.JSON(http.StatusOK, gin.H{"status": "ok"}) })

	post := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/x", nil)
		req.Header.Set("Idempotency-Key", "abc")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	first := post()
	second := post()

	assert.Equal(t, 1, calls, "the handler must run once for a repeated idempotency key")
	assert.Equal(t, http.StatusOK, first.Code)
	assert.Contains(t, second.Body.String(), "duplicate_ignored")
}

func TestIdempotency_DistinctKeysBothProceed(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	gin.SetMode(gin.TestMode)
	calls := 0
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("user_id", "u1"); c.Next() })
	r.Use(IdempotencyMiddleware(rdb, time.Minute))
	r.POST("/x", func(c *gin.Context) { calls++; c.Status(http.StatusOK) })

	for _, k := range []string{"a", "b", ""} { // distinct keys + a no-key request all proceed
		req := httptest.NewRequest(http.MethodPost, "/x", nil)
		if k != "" {
			req.Header.Set("Idempotency-Key", k)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
	assert.Equal(t, 3, calls)
}
