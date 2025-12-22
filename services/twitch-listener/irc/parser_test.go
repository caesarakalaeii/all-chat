package irc

import (
	"testing"
	"time"

	"github.com/caesar/all-chat/services/twitch-listener/models"
	"github.com/gempir/go-twitch-irc/v4"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParser_ParsePrivateMessage(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name    string
		msg     twitch.PrivateMessage
		wantErr bool
		check   func(*testing.T, *models.RawChatMessage)
	}{
		{
			name: "basic message",
			msg: twitch.PrivateMessage{
				Channel: "#xqc",
				User: twitch.User{
					ID:          "12345678",
					Name:        "viewer123",
					DisplayName: "Viewer123",
					Color:       "#FF0000",
				},
				Message: "Hello World",
				Time:    time.Date(2025, 11, 13, 10, 0, 0, 0, time.UTC),
			},
			wantErr: false,
			check: func(t *testing.T, msg *models.RawChatMessage) {
				assert.Equal(t, "twitch", msg.Platform)
				assert.Equal(t, "xqc", msg.ChannelID)
				assert.Equal(t, "12345678", msg.UserID)
				assert.Equal(t, "viewer123", msg.Username)
				assert.Equal(t, "Hello World", msg.Text)
				assert.Equal(t, "Viewer123", msg.Tags["display-name"])
				assert.Equal(t, "#FF0000", msg.Tags["color"])
				assert.NotEmpty(t, msg.MessageID) // UUID should be generated
			},
		},
		{
			name: "message with badges",
			msg: twitch.PrivateMessage{
				Channel: "#summit1g",
				User: twitch.User{
					ID:          "99999",
					Name:        "subber",
					DisplayName: "Subber",
					Badges: map[string]int{
						"subscriber": 12,
						"moderator":  1,
					},
				},
				Message: "Subscribed for 12 months!",
				Time:    time.Now(),
			},
			wantErr: false,
			check: func(t *testing.T, msg *models.RawChatMessage) {
				assert.Equal(t, "summit1g", msg.ChannelID)
				assert.Contains(t, msg.Tags["badges"], "subscriber/12")
				assert.Contains(t, msg.Tags["badges"], "moderator/1")
				assert.Equal(t, "1", msg.Tags["subscriber"])
				assert.Equal(t, "1", msg.Tags["mod"])
			},
		},
		{
			name: "message with emotes",
			msg: twitch.PrivateMessage{
				Channel: "#test",
				User: twitch.User{
					ID:   "111",
					Name: "emoteuser",
				},
				Message: "Kappa test Kappa",
				Emotes: []*twitch.Emote{
					{
						ID:   "25",
						Name: "Kappa",
						Positions: []twitch.EmotePosition{
							{Start: 0, End: 4},
							{Start: 11, End: 15},
						},
					},
				},
				Time: time.Now(),
			},
			wantErr: false,
			check: func(t *testing.T, msg *models.RawChatMessage) {
				assert.Equal(t, "test", msg.ChannelID)
				emotes := msg.Tags["emotes"]
				assert.NotEmpty(t, emotes)
				assert.Contains(t, emotes, "25:")
				assert.Contains(t, emotes, "0-4")
				assert.Contains(t, emotes, "11-15")
			},
		},
		{
			name: "missing channel",
			msg: twitch.PrivateMessage{
				User: twitch.User{
					Name: "user",
				},
				Message: "test",
			},
			wantErr: true,
		},
		{
			name: "missing username",
			msg: twitch.PrivateMessage{
				Channel: "#test",
				Message: "test",
			},
			wantErr: true,
		},
		{
			name: "missing message text",
			msg: twitch.PrivateMessage{
				Channel: "#test",
				User: twitch.User{
					Name: "user",
				},
			},
			wantErr: true,
		},
		{
			name: "channel with # prefix",
			msg: twitch.PrivateMessage{
				Channel: "#TestChannel",
				User: twitch.User{
					ID:   "123",
					Name: "User",
				},
				Message: "test",
				Time:    time.Now(),
			},
			wantErr: false,
			check: func(t *testing.T, msg *models.RawChatMessage) {
				// Should be lowercase and without #
				assert.Equal(t, "testchannel", msg.ChannelID)
			},
		},
		{
			name: "username normalization",
			msg: twitch.PrivateMessage{
				Channel: "#test",
				User: twitch.User{
					ID:   "456",
					Name: "UserName",
				},
				Message: "test",
				Time:    time.Now(),
			},
			wantErr: false,
			check: func(t *testing.T, msg *models.RawChatMessage) {
				// Should be lowercase
				assert.Equal(t, "username", msg.Username)
			},
		},
		{
			name: "shared chat - all tags present",
			msg: twitch.PrivateMessage{
				Channel: "#hostchannel",
				User: twitch.User{
					ID:          "123456",
					Name:        "guestuser",
					DisplayName: "GuestUser",
					Color:       "#00FF00",
					Badges: map[string]int{
						"subscriber": 6,
					},
				},
				Message: "Hello from shared chat!",
				Time:    time.Now(),
				Tags: map[string]string{
					"source-room-id":    "987654321",
					"source-id":         "123456",
					"source-badges":     "subscriber/6,moderator/1",
					"source-badge-info": "subscriber/6",
				},
			},
			wantErr: false,
			check: func(t *testing.T, msg *models.RawChatMessage) {
				assert.Equal(t, "hostchannel", msg.ChannelID)
				assert.Equal(t, "guestuser", msg.Username)
				assert.Equal(t, "Hello from shared chat!", msg.Text)
				// Verify shared chat tags are extracted
				assert.Equal(t, "987654321", msg.Tags["source-room-id"])
				assert.Equal(t, "123456", msg.Tags["source-id"])
				assert.Equal(t, "subscriber/6,moderator/1", msg.Tags["source-badges"])
				assert.Equal(t, "subscriber/6", msg.Tags["source-badge-info"])
			},
		},
		{
			name: "shared chat - partial tags (only source-room-id)",
			msg: twitch.PrivateMessage{
				Channel: "#hostchannel",
				User: twitch.User{
					ID:          "789",
					Name:        "partialuser",
					DisplayName: "PartialUser",
				},
				Message: "Partial shared chat tags",
				Time:    time.Now(),
				Tags: map[string]string{
					"source-room-id": "111222333",
				},
			},
			wantErr: false,
			check: func(t *testing.T, msg *models.RawChatMessage) {
				assert.Equal(t, "hostchannel", msg.ChannelID)
				// Only source-room-id should be present
				assert.Equal(t, "111222333", msg.Tags["source-room-id"])
				// Other shared chat tags should not be present
				assert.Equal(t, "", msg.Tags["source-id"])
				assert.Equal(t, "", msg.Tags["source-badges"])
				assert.Equal(t, "", msg.Tags["source-badge-info"])
			},
		},
		{
			name: "shared chat - no shared tags (backward compatibility)",
			msg: twitch.PrivateMessage{
				Channel: "#regularchannel",
				User: twitch.User{
					ID:          "999",
					Name:        "regularuser",
					DisplayName: "RegularUser",
					Badges: map[string]int{
						"subscriber": 3,
					},
				},
				Message: "Regular message without shared chat",
				Time:    time.Now(),
				Tags:    map[string]string{}, // Empty tags map
			},
			wantErr: false,
			check: func(t *testing.T, msg *models.RawChatMessage) {
				assert.Equal(t, "regularchannel", msg.ChannelID)
				assert.Equal(t, "regularuser", msg.Username)
				// Verify no shared chat tags are present
				assert.Equal(t, "", msg.Tags["source-room-id"])
				assert.Equal(t, "", msg.Tags["source-id"])
				assert.Equal(t, "", msg.Tags["source-badges"])
				assert.Equal(t, "", msg.Tags["source-badge-info"])
				// Regular badges should still be parsed
				assert.Contains(t, msg.Tags["badges"], "subscriber/3")
			},
		},
		{
			name: "shared chat - with emotes and badges",
			msg: twitch.PrivateMessage{
				Channel: "#collab",
				User: twitch.User{
					ID:          "555",
					Name:        "collaborator",
					DisplayName: "Collaborator",
					Color:       "#FF00FF",
					Badges: map[string]int{
						"subscriber": 12,
						"moderator":  1,
					},
				},
				Message: "Kappa shared message",
				Emotes: []*twitch.Emote{
					{
						ID:   "25",
						Name: "Kappa",
						Positions: []twitch.EmotePosition{
							{Start: 0, End: 4},
						},
					},
				},
				Time: time.Now(),
				Tags: map[string]string{
					"source-room-id":    "444555666",
					"source-id":         "555",
					"source-badges":     "subscriber/12,moderator/1,vip/1",
					"source-badge-info": "subscriber/12",
				},
			},
			wantErr: false,
			check: func(t *testing.T, msg *models.RawChatMessage) {
				assert.Equal(t, "collab", msg.ChannelID)
				// Verify emotes are parsed
				assert.Contains(t, msg.Tags["emotes"], "25:")
				assert.Contains(t, msg.Tags["emotes"], "0-4")
				// Verify regular badges are parsed
				assert.Contains(t, msg.Tags["badges"], "subscriber/12")
				assert.Contains(t, msg.Tags["badges"], "moderator/1")
				// Verify shared chat tags are extracted
				assert.Equal(t, "444555666", msg.Tags["source-room-id"])
				assert.Equal(t, "555", msg.Tags["source-id"])
				assert.Equal(t, "subscriber/12,moderator/1,vip/1", msg.Tags["source-badges"])
				assert.Equal(t, "subscriber/12", msg.Tags["source-badge-info"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.ParsePrivateMessage(tt.msg)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)

				// Verify UUID format for message ID
				_, uuidErr := uuid.Parse(result.MessageID)
				assert.NoError(t, uuidErr, "MessageID should be a valid UUID")

				// Verify timestamp is set
				assert.False(t, result.Timestamp.IsZero())

				if tt.check != nil {
					tt.check(t, result)
				}
			}
		})
	}
}

func TestBoolToString(t *testing.T) {
	assert.Equal(t, "1", boolToString(true))
	assert.Equal(t, "0", boolToString(false))
}
