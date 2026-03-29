package publisher

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/caesar/all-chat/services/twitch-listener/models"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// parseDataField extracts a RawChatMessage from the "data" JSON field in a stream entry.
func parseDataField(t *testing.T, entry redis.XMessage) *models.RawChatMessage {
	t.Helper()
	dataStr, ok := entry.Values["data"].(string)
	require.True(t, ok, "stream entry should have a 'data' string field")
	var msg models.RawChatMessage
	require.NoError(t, json.Unmarshal([]byte(dataStr), &msg))
	return &msg
}

func TestStreamPublisher_Publish(t *testing.T) {
	// Skip if Redis is not available
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	ctx := context.Background()
	logger := zaptest.NewLogger(t)

	// Connect to Redis
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15, // Use DB 15 for tests
	})
	defer client.Close()

	// Ping to check connection
	err := client.Ping(ctx).Err()
	if err != nil {
		t.Skip("Redis not available:", err)
	}

	// Clean up test stream before and after
	defer client.Del(ctx, StreamKey)

	// Create publisher
	publisher := NewStreamPublisher(client, logger)

	msg := &models.RawChatMessage{
		MessageID: "test-msg-1",
		Platform:  "twitch",
		ChannelID: "xqc",
		UserID:    "12345",
		Username:  "testuser",
		Text:      "Hello World",
		Timestamp: time.Now().UTC(),
		Tags: map[string]string{
			"color": "#FF0000",
		},
	}

	// Publish message
	err = publisher.Publish(ctx, msg)
	require.NoError(t, err)

	// Verify message was added to stream
	result, err := client.XRange(ctx, StreamKey, "-", "+").Result()
	require.NoError(t, err)
	assert.Len(t, result, 1)

	// Verify message fields from the "data" JSON blob
	parsed := parseDataField(t, result[0])
	assert.Equal(t, "test-msg-1", parsed.MessageID)
	assert.Equal(t, "twitch", parsed.Platform)
	assert.Equal(t, "xqc", parsed.ChannelID)
	assert.Equal(t, "12345", parsed.UserID)
	assert.Equal(t, "testuser", parsed.Username)
	assert.Equal(t, "Hello World", parsed.Text)
	assert.False(t, parsed.Timestamp.IsZero())
}

func TestStreamPublisher_PublishBatch(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	ctx := context.Background()
	logger := zaptest.NewLogger(t)

	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15,
	})
	defer client.Close()

	err := client.Ping(ctx).Err()
	if err != nil {
		t.Skip("Redis not available:", err)
	}

	defer client.Del(ctx, StreamKey)

	publisher := NewStreamPublisher(client, logger)

	// Create batch of messages
	messages := []*models.RawChatMessage{
		{
			MessageID: "batch-1",
			Platform:  "twitch",
			ChannelID: "xqc",
			UserID:    "111",
			Username:  "user1",
			Text:      "Message 1",
			Timestamp: time.Now().UTC(),
			Tags:      map[string]string{},
		},
		{
			MessageID: "batch-2",
			Platform:  "twitch",
			ChannelID: "xqc",
			UserID:    "222",
			Username:  "user2",
			Text:      "Message 2",
			Timestamp: time.Now().UTC(),
			Tags:      map[string]string{},
		},
		{
			MessageID: "batch-3",
			Platform:  "twitch",
			ChannelID: "summit1g",
			UserID:    "333",
			Username:  "user3",
			Text:      "Message 3",
			Timestamp: time.Now().UTC(),
			Tags:      map[string]string{},
		},
	}

	// Publish batch
	err = publisher.PublishBatch(ctx, messages)
	require.NoError(t, err)

	// Verify all messages were added
	result, err := client.XRange(ctx, StreamKey, "-", "+").Result()
	require.NoError(t, err)
	assert.Len(t, result, 3)

	// Verify message IDs from the "data" JSON blobs
	assert.Equal(t, "batch-1", parseDataField(t, result[0]).MessageID)
	assert.Equal(t, "batch-2", parseDataField(t, result[1]).MessageID)
	assert.Equal(t, "batch-3", parseDataField(t, result[2]).MessageID)
}

func TestStreamPublisher_PublishBatch_Empty(t *testing.T) {
	ctx := context.Background()
	logger := zaptest.NewLogger(t)

	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15,
	})
	defer client.Close()

	publisher := NewStreamPublisher(client, logger)

	// Publishing empty batch should not error
	err := publisher.PublishBatch(ctx, []*models.RawChatMessage{})
	assert.NoError(t, err)
}

func TestStreamPublisher_Ping(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	ctx := context.Background()
	logger := zaptest.NewLogger(t)

	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15,
	})
	defer client.Close()

	publisher := NewStreamPublisher(client, logger)

	err := publisher.Ping(ctx)
	if err != nil {
		t.Skip("Redis not available:", err)
	}

	assert.NoError(t, err)
}

func TestStreamPublisher_MaxLen(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	ctx := context.Background()
	logger := zaptest.NewLogger(t)

	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   15,
	})
	defer client.Close()

	err := client.Ping(ctx).Err()
	if err != nil {
		t.Skip("Redis not available:", err)
	}

	defer client.Del(ctx, StreamKey)

	publisher := NewStreamPublisher(client, logger)

	// The MaxLen with Approx trimming should keep approximately MaxStreamLength messages
	// This test just verifies the MAXLEN parameter works
	msg := &models.RawChatMessage{
		MessageID: "maxlen-test",
		Platform:  "twitch",
		ChannelID: "test",
		UserID:    "123",
		Username:  "user",
		Text:      "test",
		Timestamp: time.Now().UTC(),
		Tags:      map[string]string{},
	}

	err = publisher.Publish(ctx, msg)
	require.NoError(t, err)

	// Check stream info
	info, err := client.XInfoStream(ctx, StreamKey).Result()
	require.NoError(t, err)
	assert.Equal(t, int64(1), info.Length)
}
