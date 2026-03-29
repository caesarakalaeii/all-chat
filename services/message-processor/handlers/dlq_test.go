package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupTestRouter(t *testing.T, mr *miniredis.Miniredis) (*gin.Engine, *redis.Client) {
	t.Helper()
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { client.Close() })
	logger := zaptest.NewLogger(t)
	r := gin.New()
	r.POST("/admin/dlq/replay", HandleDLQReplay(client, logger))
	return r, client
}

func TestHandleDLQReplay_ReplaysDLQMessages(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	r, client := setupTestRouter(t, mr)
	ctx := context.Background()

	// Add 3 entries to chat:dlq
	for i := 0; i < 3; i++ {
		_, err := client.XAdd(ctx, &redis.XAddArgs{
			Stream: "chat:dlq",
			ID:     "*",
			Values: map[string]interface{}{
				"original_stream_id": "100-0",
				"source_service":     "twitch-listener",
				"failure_reason":     "parse_error",
				"retry_count":        "3",
				"original_data":      `{"message_id":"test","platform":"twitch"}`,
				"dlq_timestamp":      "2026-01-01T00:00:00Z",
			},
		}).Result()
		require.NoError(t, err)
	}

	// Call the replay endpoint
	req := httptest.NewRequest(http.MethodPost, "/admin/dlq/replay", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp map[string]int
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 3, resp["replayed"])
	assert.Equal(t, 0, resp["failed"])

	// DLQ should be empty after replay
	dlqLen, err := client.XLen(ctx, "chat:dlq").Result()
	require.NoError(t, err)
	assert.Equal(t, int64(0), dlqLen)

	// chat:raw should have 3 new entries
	rawLen, err := client.XLen(ctx, "chat:raw").Result()
	require.NoError(t, err)
	assert.Equal(t, int64(3), rawLen)

	// Verify replayed entries have "replayed":"true" field
	rawEntries, err := client.XRange(ctx, "chat:raw", "-", "+").Result()
	require.NoError(t, err)
	for _, entry := range rawEntries {
		assert.Equal(t, "true", entry.Values["replayed"])
		assert.Equal(t, `{"message_id":"test","platform":"twitch"}`, entry.Values["data"])
	}
}

func TestHandleDLQReplay_EmptyDLQ(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	r, _ := setupTestRouter(t, mr)

	req := httptest.NewRequest(http.MethodPost, "/admin/dlq/replay", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp map[string]int
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 0, resp["replayed"])
	assert.Equal(t, 0, resp["failed"])
}

func TestHandleDLQReplay_LimitsTo100Messages(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	r, client := setupTestRouter(t, mr)
	ctx := context.Background()

	// Add 150 entries to chat:dlq
	for i := 0; i < 150; i++ {
		_, err := client.XAdd(ctx, &redis.XAddArgs{
			Stream: "chat:dlq",
			ID:     "*",
			Values: map[string]interface{}{
				"original_data": `{}`,
				"replayed":      "false",
			},
		}).Result()
		require.NoError(t, err)
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/dlq/replay", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp map[string]int
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	// Should only replay up to 100 messages at a time
	assert.Equal(t, 100, resp["replayed"])

	// 50 should remain in DLQ
	dlqLen, err := client.XLen(ctx, "chat:dlq").Result()
	require.NoError(t, err)
	assert.Equal(t, int64(50), dlqLen)
}
