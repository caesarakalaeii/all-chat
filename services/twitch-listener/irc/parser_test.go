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
