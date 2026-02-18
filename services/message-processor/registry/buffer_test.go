package registry

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/caesar/all-chat/services/message-processor/models"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeletionBuffer_AddAndGet(t *testing.T) {
	// Setup miniredis
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	defer client.Close()

	buffer := NewRedisDeletionBuffer(client, 60*time.Second)
	ctx := context.Background()

	// Create test deletion event
	event := &models.RawChatMessage{
		MessageID: "del-123",
		Platform:  "twitch",
		ChannelID: "channel1",
		EventType: "message_deletion",
		EventData: map[string]interface{}{
			"deletion_type": "single",
			"target_msg_id": "msg-456",
		},
	}

	// Add deletion to buffer
	err = buffer.Add(ctx, "twitch", "channel1", "msg-456", event)
	require.NoError(t, err)

	// Retrieve deletion from buffer
	retrieved, err := buffer.Get(ctx, "twitch", "channel1", "msg-456")
	require.NoError(t, err)
	require.NotNil(t, retrieved)
	assert.Equal(t, "del-123", retrieved.MessageID)
	assert.Equal(t, "message_deletion", retrieved.EventType)
	assert.Equal(t, "single", retrieved.EventData["deletion_type"])
	assert.Equal(t, "msg-456", retrieved.EventData["target_msg_id"])
}

func TestDeletionBuffer_GetNonExistent(t *testing.T) {
	// Setup miniredis
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	defer client.Close()

	buffer := NewRedisDeletionBuffer(client, 60*time.Second)
	ctx := context.Background()

	// Try to get non-existent deletion
	retrieved, err := buffer.Get(ctx, "twitch", "channel1", "nonexistent")
	require.NoError(t, err)
	assert.Nil(t, retrieved, "Should return nil for non-existent deletion")
}

func TestDeletionBuffer_TTLExpiration(t *testing.T) {
	// Setup miniredis
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	defer client.Close()

	buffer := NewRedisDeletionBuffer(client, 2*time.Second)
	ctx := context.Background()

	// Create test deletion event
	event := &models.RawChatMessage{
		MessageID: "del-123",
		Platform:  "twitch",
		ChannelID: "channel1",
		EventType: "message_deletion",
		EventData: map[string]interface{}{
			"deletion_type": "single",
			"target_msg_id": "msg-456",
		},
	}

	// Add deletion to buffer
	err = buffer.Add(ctx, "twitch", "channel1", "msg-456", event)
	require.NoError(t, err)

	// Verify it exists
	retrieved, err := buffer.Get(ctx, "twitch", "channel1", "msg-456")
	require.NoError(t, err)
	require.NotNil(t, retrieved)

	// Fast forward time by 3 seconds
	mr.FastForward(3 * time.Second)

	// Should be expired now
	retrieved, err = buffer.Get(ctx, "twitch", "channel1", "msg-456")
	require.NoError(t, err)
	assert.Nil(t, retrieved, "Deletion should be expired after TTL")
}

func TestDeletionBuffer_Remove(t *testing.T) {
	// Setup miniredis
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	defer client.Close()

	buffer := NewRedisDeletionBuffer(client, 60*time.Second)
	ctx := context.Background()

	// Create test deletion event
	event := &models.RawChatMessage{
		MessageID: "del-123",
		Platform:  "twitch",
		ChannelID: "channel1",
		EventType: "message_deletion",
		EventData: map[string]interface{}{
			"deletion_type": "single",
			"target_msg_id": "msg-456",
		},
	}

	// Add deletion to buffer
	err = buffer.Add(ctx, "twitch", "channel1", "msg-456", event)
	require.NoError(t, err)

	// Remove deletion
	err = buffer.Remove(ctx, "twitch", "channel1", "msg-456")
	require.NoError(t, err)

	// Should no longer exist
	retrieved, err := buffer.Get(ctx, "twitch", "channel1", "msg-456")
	require.NoError(t, err)
	assert.Nil(t, retrieved, "Deletion should be removed")
}

func TestDeletionBuffer_MultipleDeletionsNoConflict(t *testing.T) {
	// Setup miniredis
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	defer client.Close()

	buffer := NewRedisDeletionBuffer(client, 60*time.Second)
	ctx := context.Background()

	// Add deletion for platform1/channel1/msg1
	event1 := &models.RawChatMessage{
		MessageID: "del-1",
		Platform:  "twitch",
		ChannelID: "channel1",
		EventType: "message_deletion",
		EventData: map[string]interface{}{
			"deletion_type": "single",
			"target_msg_id": "msg-1",
		},
	}
	err = buffer.Add(ctx, "twitch", "channel1", "msg-1", event1)
	require.NoError(t, err)

	// Add deletion for platform2/channel2/msg2
	event2 := &models.RawChatMessage{
		MessageID: "del-2",
		Platform:  "youtube",
		ChannelID: "channel2",
		EventType: "message_deletion",
		EventData: map[string]interface{}{
			"deletion_type": "single",
			"target_msg_id": "msg-2",
		},
	}
	err = buffer.Add(ctx, "youtube", "channel2", "msg-2", event2)
	require.NoError(t, err)

	// Add deletion for same platform/channel but different message
	event3 := &models.RawChatMessage{
		MessageID: "del-3",
		Platform:  "twitch",
		ChannelID: "channel1",
		EventType: "message_deletion",
		EventData: map[string]interface{}{
			"deletion_type": "single",
			"target_msg_id": "msg-3",
		},
	}
	err = buffer.Add(ctx, "twitch", "channel1", "msg-3", event3)
	require.NoError(t, err)

	// Verify each deletion is stored independently
	retrieved1, err := buffer.Get(ctx, "twitch", "channel1", "msg-1")
	require.NoError(t, err)
	require.NotNil(t, retrieved1)
	assert.Equal(t, "del-1", retrieved1.MessageID)

	retrieved2, err := buffer.Get(ctx, "youtube", "channel2", "msg-2")
	require.NoError(t, err)
	require.NotNil(t, retrieved2)
	assert.Equal(t, "del-2", retrieved2.MessageID)

	retrieved3, err := buffer.Get(ctx, "twitch", "channel1", "msg-3")
	require.NoError(t, err)
	require.NotNil(t, retrieved3)
	assert.Equal(t, "del-3", retrieved3.MessageID)
}
