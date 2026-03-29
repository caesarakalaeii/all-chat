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
