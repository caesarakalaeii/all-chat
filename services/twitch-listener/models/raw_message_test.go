package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRawChatMessage_ToJSON(t *testing.T) {
	msg := &RawChatMessage{
		MessageID: "123e4567-e89b-12d3-a456-426614174000",
		Platform:  "twitch",
		ChannelID: "xqc",
		UserID:    "12345678",
		Username:  "viewer123",
		Text:      "Hello Kappa",
		Timestamp: time.Date(2025, 11, 13, 10, 0, 0, 0, time.UTC),
		Tags: map[string]string{
			"display-name": "Viewer123",
			"color":        "#FF0000",
			"badges":       "subscriber/12",
		},
	}

	jsonBytes, err := msg.ToJSON()
	require.NoError(t, err)
	assert.NotEmpty(t, jsonBytes)
	assert.Contains(t, string(jsonBytes), "viewer123")
	assert.Contains(t, string(jsonBytes), "Hello Kappa")
}

func TestFromJSON(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		wantErr bool
		check   func(*testing.T, *RawChatMessage)
	}{
		{
			name: "valid message",
			json: `{
				"message_id": "123e4567-e89b-12d3-a456-426614174000",
				"platform": "twitch",
				"channel_id": "xqc",
				"user_id": "12345678",
				"username": "viewer123",
				"text": "Hello World",
				"timestamp": "2025-11-13T10:00:00Z",
				"tags": {
					"display-name": "Viewer123",
					"color": "#FF0000"
				}
			}`,
			wantErr: false,
			check: func(t *testing.T, msg *RawChatMessage) {
				assert.Equal(t, "123e4567-e89b-12d3-a456-426614174000", msg.MessageID)
				assert.Equal(t, "twitch", msg.Platform)
				assert.Equal(t, "xqc", msg.ChannelID)
				assert.Equal(t, "12345678", msg.UserID)
				assert.Equal(t, "viewer123", msg.Username)
				assert.Equal(t, "Hello World", msg.Text)
				assert.Equal(t, "Viewer123", msg.Tags["display-name"])
				assert.Equal(t, "#FF0000", msg.Tags["color"])
			},
		},
		{
			name:    "invalid json",
			json:    `{invalid}`,
			wantErr: true,
		},
		{
			name: "empty tags",
			json: `{
				"message_id": "123",
				"platform": "twitch",
				"channel_id": "test",
				"user_id": "456",
				"username": "user",
				"text": "hi",
				"timestamp": "2025-11-13T10:00:00Z",
				"tags": {}
			}`,
			wantErr: false,
			check: func(t *testing.T, msg *RawChatMessage) {
				assert.Equal(t, "test", msg.ChannelID)
				assert.NotNil(t, msg.Tags)
				assert.Empty(t, msg.Tags)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := FromJSON([]byte(tt.json))

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, msg)
			} else {
				require.NoError(t, err)
				require.NotNil(t, msg)
				if tt.check != nil {
					tt.check(t, msg)
				}
			}
		})
	}
}

func TestRawChatMessage_RoundTrip(t *testing.T) {
	original := &RawChatMessage{
		MessageID: "test-id",
		Platform:  "twitch",
		ChannelID: "test_channel",
		UserID:    "user123",
		Username:  "testuser",
		Text:      "Test message",
		Timestamp: time.Now().UTC().Truncate(time.Second),
		Tags: map[string]string{
			"key1": "value1",
			"key2": "value2",
		},
	}

	// Convert to JSON
	jsonBytes, err := original.ToJSON()
	require.NoError(t, err)

	// Convert back from JSON
	parsed, err := FromJSON(jsonBytes)
	require.NoError(t, err)

	// Verify all fields match
	assert.Equal(t, original.MessageID, parsed.MessageID)
	assert.Equal(t, original.Platform, parsed.Platform)
	assert.Equal(t, original.ChannelID, parsed.ChannelID)
	assert.Equal(t, original.UserID, parsed.UserID)
	assert.Equal(t, original.Username, parsed.Username)
	assert.Equal(t, original.Text, parsed.Text)
	assert.Equal(t, original.Timestamp.Unix(), parsed.Timestamp.Unix())
	assert.Equal(t, original.Tags, parsed.Tags)
}
