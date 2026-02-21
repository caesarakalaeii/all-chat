package shared

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeMessage_IDNormalization(t *testing.T) {
	original := &RawChatMessage{
		MessageID: "some-unique-id-12345",
		Platform:  "youtube",
		ChannelID: "UC123",
		UserID:    "user456",
		Username:  "TestUser",
		Text:      "Hello world",
		Timestamp: time.Now(),
		Tags:      map[string]string{"badge": "moderator"},
	}

	normalized := NormalizeMessage(original)

	assert.Equal(t, "<normalized>", normalized.MessageID, "MessageID should be normalized to '<normalized>'")
	assert.Equal(t, original.MessageID, "some-unique-id-12345", "Original message should not be mutated")
}

func TestNormalizeMessage_TimestampTruncation(t *testing.T) {
	// Create timestamp with microsecond precision
	originalTime := time.Date(2024, 1, 15, 10, 30, 45, 123456789, time.UTC)

	original := &RawChatMessage{
		MessageID: "msg-123",
		Platform:  "youtube",
		ChannelID: "UC123",
		UserID:    "user456",
		Username:  "TestUser",
		Text:      "Hello world",
		Timestamp: originalTime,
	}

	normalized := NormalizeMessage(original)

	expectedTime := time.Date(2024, 1, 15, 10, 30, 45, 0, time.UTC)
	assert.Equal(t, expectedTime, normalized.Timestamp, "Timestamp should be truncated to 1-second precision")
	assert.Equal(t, originalTime, original.Timestamp, "Original timestamp should not be mutated")
}

func TestNormalizeMessage_OtherFieldsPreserved(t *testing.T) {
	original := &RawChatMessage{
		MessageID:   "msg-123",
		Platform:    "youtube",
		ChannelID:   "UC123",
		ChannelName: "Test Channel",
		UserID:      "user789",
		Username:    "TestUser",
		Text:        "Hello world",
		Timestamp:   time.Now(),
		Tags: map[string]string{
			"badge":       "moderator",
			"user_badges": "verified",
		},
		EventType: "super_chat",
		EventData: map[string]interface{}{
			"amount_micros": 5000000,
			"currency":      "USD",
		},
	}

	normalized := NormalizeMessage(original)

	// Verify all fields preserved (except MessageID and Timestamp)
	assert.Equal(t, original.Platform, normalized.Platform)
	assert.Equal(t, original.ChannelID, normalized.ChannelID)
	assert.Equal(t, original.ChannelName, normalized.ChannelName)
	assert.Equal(t, original.UserID, normalized.UserID)
	assert.Equal(t, original.Username, normalized.Username)
	assert.Equal(t, original.Text, normalized.Text)
	assert.Equal(t, original.EventType, normalized.EventType)

	// Verify Tags deep copied
	assert.Equal(t, original.Tags, normalized.Tags)
	assert.NotSame(t, original.Tags, normalized.Tags, "Tags should be deep copied")

	// Verify EventData deep copied
	assert.Equal(t, original.EventData, normalized.EventData)
	assert.NotSame(t, original.EventData, normalized.EventData, "EventData should be deep copied")
}

func TestNormalizeMessage_NilHandling(t *testing.T) {
	normalized := NormalizeMessage(nil)
	assert.Nil(t, normalized, "Normalizing nil should return nil")
}

func TestNormalizeMessage_DeepCopyMutation(t *testing.T) {
	original := &RawChatMessage{
		MessageID: "msg-123",
		Platform:  "youtube",
		ChannelID: "UC123",
		UserID:    "user456",
		Username:  "TestUser",
		Text:      "Hello world",
		Timestamp: time.Now(),
		Tags: map[string]string{
			"badge": "moderator",
		},
	}

	normalized := NormalizeMessage(original)

	// Mutate normalized Tags
	normalized.Tags["badge"] = "vip"
	normalized.Tags["new_key"] = "new_value"

	// Verify original unchanged
	assert.Equal(t, "moderator", original.Tags["badge"], "Original Tags should not be mutated")
	assert.NotContains(t, original.Tags, "new_key", "Original Tags should not contain new keys")
}

func TestCompareMessages_Identical(t *testing.T) {
	timestamp := time.Date(2024, 1, 15, 10, 30, 45, 0, time.UTC)

	msg1 := &RawChatMessage{
		MessageID: "official-id-123",
		Platform:  "youtube",
		ChannelID: "UC123",
		UserID:    "user456",
		Username:  "TestUser",
		Text:      "Hello world",
		Timestamp: timestamp,
		Tags:      map[string]string{"badge": "moderator"},
	}

	msg2 := &RawChatMessage{
		MessageID: "innertube-id-789", // Different ID
		Platform:  "youtube",
		ChannelID: "UC123",
		UserID:    "user456",
		Username:  "TestUser",
		Text:      "Hello world",
		Timestamp: timestamp.Add(500 * time.Millisecond), // Within 1-second precision
		Tags:      map[string]string{"badge": "moderator"},
	}

	match, diff := CompareMessages(msg1, msg2)

	assert.True(t, match, "Messages should match after normalization")
	assert.Empty(t, diff, "Diff should be empty for matching messages")
}

func TestCompareMessages_DifferentContent(t *testing.T) {
	timestamp := time.Now()

	msg1 := &RawChatMessage{
		MessageID: "msg-1",
		Platform:  "youtube",
		ChannelID: "UC123",
		UserID:    "user456",
		Username:  "TestUser",
		Text:      "Hello world",
		Timestamp: timestamp,
	}

	msg2 := &RawChatMessage{
		MessageID: "msg-2",
		Platform:  "youtube",
		ChannelID: "UC123",
		UserID:    "user456",
		Username:  "DifferentUser", // Different username
		Text:      "Hello world",
		Timestamp: timestamp,
	}

	match, diff := CompareMessages(msg1, msg2)

	assert.False(t, match, "Messages with different usernames should not match")
	assert.NotEmpty(t, diff, "Diff should be present for non-matching messages")
	assert.Contains(t, diff, "username", "Diff should mention username field")
}

func TestCompareMessages_AllowedDifferences(t *testing.T) {
	timestamp := time.Date(2024, 1, 15, 10, 30, 45, 0, time.UTC)

	msg1 := &RawChatMessage{
		MessageID: "official-id-unique-123",
		Platform:  "youtube",
		ChannelID: "UC123",
		UserID:    "user456",
		Username:  "TestUser",
		Text:      "Hello world",
		Timestamp: timestamp,
	}

	msg2 := &RawChatMessage{
		MessageID: "innertube-id-different-789",
		Platform:  "youtube",
		ChannelID: "UC123",
		UserID:    "user456",
		Username:  "TestUser",
		Text:      "Hello world",
		Timestamp: timestamp.Add(999 * time.Millisecond), // Just under 1 second
	}

	match, diff := CompareMessages(msg1, msg2)

	assert.True(t, match, "Messages with only ID/timestamp differences should match")
	assert.Empty(t, diff, "No diff for allowed differences")
}

func TestCompareMessages_EventData(t *testing.T) {
	timestamp := time.Now()

	msg1 := &RawChatMessage{
		MessageID: "msg-1",
		Platform:  "youtube",
		ChannelID: "UC123",
		UserID:    "user456",
		Username:  "TestUser",
		Text:      "Super Chat",
		Timestamp: timestamp,
		EventType: "super_chat",
		EventData: map[string]interface{}{
			"amount_micros": 5000000,
			"currency":      "USD",
		},
	}

	msg2 := &RawChatMessage{
		MessageID: "msg-2",
		Platform:  "youtube",
		ChannelID: "UC123",
		UserID:    "user456",
		Username:  "TestUser",
		Text:      "Super Chat",
		Timestamp: timestamp,
		EventType: "super_chat",
		EventData: map[string]interface{}{
			"amount_micros": 10000000, // Different amount
			"currency":      "USD",
		},
	}

	match, diff := CompareMessages(msg1, msg2)

	assert.False(t, match, "Messages with different EventData should not match")
	assert.NotEmpty(t, diff, "Diff should be present")
	assert.Contains(t, diff, "amount_micros", "Diff should mention amount_micros field")
}

func TestAllowedFieldDifferences(t *testing.T) {
	allowed := AllowedFieldDifferences()

	require.Len(t, allowed, 2, "Should have 2 allowed differences")
	assert.Contains(t, allowed, "message_id")
	assert.Contains(t, allowed, "timestamp")
}

func TestParseMessageFromJSON(t *testing.T) {
	jsonData := []byte(`{
		"message_id": "msg-123",
		"platform": "youtube",
		"channel_id": "UC123",
		"channel_name": "Test Channel",
		"user_id": "user789",
		"username": "TestUser",
		"text": "Hello world",
		"timestamp": "2024-01-15T10:30:45Z",
		"tags": {
			"badge": "moderator"
		}
	}`)

	msg, err := ParseMessageFromJSON(jsonData)

	require.NoError(t, err)
	assert.Equal(t, "msg-123", msg.MessageID)
	assert.Equal(t, "youtube", msg.Platform)
	assert.Equal(t, "TestUser", msg.Username)
	assert.Equal(t, "Hello world", msg.Text)
	assert.Equal(t, "moderator", msg.Tags["badge"])
}

func TestParseMessageFromJSON_InvalidJSON(t *testing.T) {
	jsonData := []byte(`{invalid json}`)

	msg, err := ParseMessageFromJSON(jsonData)

	assert.Error(t, err)
	assert.Nil(t, msg)
}
