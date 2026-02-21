package streams

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// setupTestRedis creates a Redis client for testing
// Uses Redis database 15 to avoid conflicts with production data
func setupTestRedis(t *testing.T) *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       15, // Use separate DB for tests
	})

	// Verify connection
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skipf("Redis not available: %v", err)
	}

	// Clean up test database before tests
	if err := client.FlushDB(ctx).Err(); err != nil {
		t.Fatalf("failed to flush test database: %v", err)
	}

	return client
}

func TestRepository_SetChannelVideoMapping(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()

	logger := zap.NewNop()
	repo := NewRepository(client, logger)

	ctx := context.Background()
	channelID := "UC_test_channel_123"
	videoID := "test_video_456"

	// Set mapping
	err := repo.SetChannelVideoMapping(ctx, channelID, videoID)
	if err != nil {
		t.Fatalf("SetChannelVideoMapping failed: %v", err)
	}

	// Verify Redis key exists
	key := "innertube:channel_video:" + channelID
	storedValue, err := client.Get(ctx, key).Result()
	if err != nil {
		t.Fatalf("failed to get key from Redis: %v", err)
	}

	if storedValue != videoID {
		t.Errorf("expected video ID %q, got %q", videoID, storedValue)
	}

	// Verify TTL is set (should be ~24 hours)
	ttl, err := client.TTL(ctx, key).Result()
	if err != nil {
		t.Fatalf("failed to get TTL: %v", err)
	}

	expectedTTL := 24 * time.Hour
	// Allow 10 second margin for test execution
	if ttl < expectedTTL-10*time.Second || ttl > expectedTTL {
		t.Errorf("expected TTL ~%v, got %v", expectedTTL, ttl)
	}
}

func TestRepository_GetChannelVideoMapping(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()

	logger := zap.NewNop()
	repo := NewRepository(client, logger)

	ctx := context.Background()
	channelID := "UC_test_channel_get"
	videoID := "test_video_get"

	// Set up test data
	key := "innertube:channel_video:" + channelID
	if err := client.Set(ctx, key, videoID, 24*time.Hour).Err(); err != nil {
		t.Fatalf("failed to set test data: %v", err)
	}

	// Get mapping
	result, err := repo.GetChannelVideoMapping(ctx, channelID)
	if err != nil {
		t.Fatalf("GetChannelVideoMapping failed: %v", err)
	}

	if result != videoID {
		t.Errorf("expected video ID %q, got %q", videoID, result)
	}
}

func TestRepository_GetChannelVideoMapping_NotFound(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()

	logger := zap.NewNop()
	repo := NewRepository(client, logger)

	ctx := context.Background()
	channelID := "UC_nonexistent_channel"

	// Get mapping for non-existent channel
	result, err := repo.GetChannelVideoMapping(ctx, channelID)

	// Should return empty string and redis.Nil error
	if err != redis.Nil {
		t.Errorf("expected redis.Nil error, got %v", err)
	}

	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestRepository_DeleteChannelVideoMapping(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()

	logger := zap.NewNop()
	repo := NewRepository(client, logger)

	ctx := context.Background()
	channelID := "UC_test_channel_delete"
	videoID := "test_video_delete"

	// Set up test data
	key := "innertube:channel_video:" + channelID
	if err := client.Set(ctx, key, videoID, 24*time.Hour).Err(); err != nil {
		t.Fatalf("failed to set test data: %v", err)
	}

	// Verify key exists
	exists, err := client.Exists(ctx, key).Result()
	if err != nil {
		t.Fatalf("failed to check key existence: %v", err)
	}
	if exists != 1 {
		t.Fatal("test data not set up correctly")
	}

	// Delete mapping
	err = repo.DeleteChannelVideoMapping(ctx, channelID)
	if err != nil {
		t.Fatalf("DeleteChannelVideoMapping failed: %v", err)
	}

	// Verify key is deleted
	exists, err = client.Exists(ctx, key).Result()
	if err != nil {
		t.Fatalf("failed to check key existence after deletion: %v", err)
	}
	if exists != 0 {
		t.Error("key still exists after deletion")
	}
}

func TestRepository_SetChannelVideoMapping_UpdateExisting(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()

	logger := zap.NewNop()
	repo := NewRepository(client, logger)

	ctx := context.Background()
	channelID := "UC_test_channel_update"
	videoID1 := "test_video_old"
	videoID2 := "test_video_new"

	// Set initial mapping
	err := repo.SetChannelVideoMapping(ctx, channelID, videoID1)
	if err != nil {
		t.Fatalf("SetChannelVideoMapping failed: %v", err)
	}

	// Update with new video ID
	err = repo.SetChannelVideoMapping(ctx, channelID, videoID2)
	if err != nil {
		t.Fatalf("SetChannelVideoMapping (update) failed: %v", err)
	}

	// Verify updated value
	result, err := repo.GetChannelVideoMapping(ctx, channelID)
	if err != nil {
		t.Fatalf("GetChannelVideoMapping failed: %v", err)
	}

	if result != videoID2 {
		t.Errorf("expected updated video ID %q, got %q", videoID2, result)
	}
}
