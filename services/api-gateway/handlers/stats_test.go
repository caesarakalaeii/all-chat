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

package handlers

import (
	"encoding/json"
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

func setupStatsTestRedis(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	return client, mr
}

func TestGetActiveOverlays_Empty(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client, _ := setupStatsTestRedis(t)
	handler := NewStatsHandler(client)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/overlays/active", nil)

	handler.GetActiveOverlays(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var ids []string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &ids))
	assert.Empty(t, ids)
}

func TestGetActiveOverlays_ReturnsConnectedIDs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	client, mr := setupStatsTestRedis(t)
	handler := NewStatsHandler(client)

	// Simulate active overlays in Redis
	mr.Set("overlay:connected:abc-123", "1")
	mr.SetTTL("overlay:connected:abc-123", 10*time.Minute)
	mr.Set("overlay:connected:def-456", "1")
	mr.SetTTL("overlay:connected:def-456", 10*time.Minute)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/overlays/active", nil)

	handler.GetActiveOverlays(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var ids []string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &ids))
	assert.Len(t, ids, 2)
	assert.ElementsMatch(t, []string{"abc-123", "def-456"}, ids)
}
