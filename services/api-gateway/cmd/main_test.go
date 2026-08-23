package main

import (
	"testing"

	"github.com/caesar/all-chat/services/api-gateway/models"
	wsconn "github.com/caesar/all-chat/services/api-gateway/websocket"
)

// overlayBroadcastFilter decides, for every frame leaving the Redis pub/sub
// callback, who may receive it. It is tested as one unit rather than as two
// independent booleans because the classification and the routing are only
// useful together: a caller that classifies a mod_action frame correctly and
// then forwards it with OwnerOnly unset leaks held AutoMod text to every
// anonymous OBS browser source, and no test of the classification alone would
// notice.
func TestOverlayBroadcastFilter(t *testing.T) {
	tests := []struct {
		name      string
		eventType string
		msgType   models.WSMessageType
		want      wsconn.BroadcastFilter
	}{
		{
			name:      "a mod_action frame is owner-only (held AutoMod text must not reach anonymous sockets)",
			eventType: "mod_action",
			msgType:   models.WSMessageTypeChatMessage,
			want:      wsconn.BroadcastFilter{OwnerOnly: true},
		},
		{
			name:      "an ordinary chat frame reaches every non-engagement socket",
			eventType: "chat_message",
			msgType:   models.WSMessageTypeChatMessage,
			want:      wsconn.BroadcastFilter{},
		},
		{
			name:      "a poll update is an engagement frame and stays public",
			eventType: "",
			msgType:   models.WSMessageTypePollUpdate,
			want:      wsconn.BroadcastFilter{EngagementFrame: true},
		},
		{
			name:      "a prediction update is an engagement frame and stays public",
			eventType: "",
			msgType:   models.WSMessageTypePredictionUpdate,
			want:      wsconn.BroadcastFilter{EngagementFrame: true},
		},
		{
			name:      "message_deletion is public and stays broadcastable",
			eventType: "message_deletion",
			msgType:   models.WSMessageTypeChatMessage,
			want:      wsconn.BroadcastFilter{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := overlayBroadcastFilter(tt.eventType, tt.msgType); got != tt.want {
				t.Errorf("overlayBroadcastFilter(%q, %q) = %+v, want %+v", tt.eventType, tt.msgType, got, tt.want)
			}
		})
	}
}

func TestIsModerationFrame(t *testing.T) {
	tests := []struct {
		name      string
		eventType string
		want      bool
	}{
		{
			name:      "mod_action is the moderation frame type the message-processor publishes",
			eventType: "mod_action",
			want:      true,
		},
		{
			name:      "an ordinary chat message is not a moderation frame",
			eventType: "chat_message",
			want:      false,
		},
		{
			name:      "message_deletion is public and stays broadcastable",
			eventType: "message_deletion",
			want:      false,
		},
		{
			name:      "an empty event type is not a moderation frame",
			eventType: "",
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isModerationFrame(tt.eventType); got != tt.want {
				t.Errorf("isModerationFrame(%q) = %v, want %v", tt.eventType, got, tt.want)
			}
		})
	}
}

func TestShouldBufferForReplay(t *testing.T) {
	const testOverlayID = "test-ov"

	tests := []struct {
		name          string
		msgType       models.WSMessageType
		overlayID     string
		liveConnCount int
		isModFrame    bool
		want          bool
	}{
		{
			name:          "chat message with no live connections buffers",
			msgType:       models.WSMessageTypeChatMessage,
			overlayID:     "ov",
			liveConnCount: 0,
			want:          true,
		},
		{
			name:          "message update with no live connections buffers",
			msgType:       models.WSMessageTypeMessageUpdate,
			overlayID:     "ov",
			liveConnCount: 0,
			want:          true,
		},
		{
			name:          "poll update never buffers",
			msgType:       models.WSMessageTypePollUpdate,
			overlayID:     "ov",
			liveConnCount: 0,
			want:          false,
		},
		{
			name:          "prediction update never buffers",
			msgType:       models.WSMessageTypePredictionUpdate,
			overlayID:     "ov",
			liveConnCount: 0,
			want:          false,
		},
		{
			name:          "chat message with live connections does not buffer",
			msgType:       models.WSMessageTypeChatMessage,
			overlayID:     "ov",
			liveConnCount: 1,
			want:          false,
		},
		{
			name:          "test-stream overlay does not buffer",
			msgType:       models.WSMessageTypeChatMessage,
			overlayID:     testOverlayID,
			liveConnCount: 0,
			want:          false,
		},
		{
			name:          "mod frames are never replay-buffered (held AutoMod text must not reach anonymous sockets on connect)",
			msgType:       models.WSMessageTypeChatMessage,
			overlayID:     "ov",
			liveConnCount: 0,
			isModFrame:    true,
			want:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldBufferForReplay(tt.msgType, tt.overlayID, testOverlayID, tt.liveConnCount, tt.isModFrame)
			if got != tt.want {
				t.Errorf("shouldBufferForReplay(%q, %q, %q, %d, %v) = %v, want %v",
					tt.msgType, tt.overlayID, testOverlayID, tt.liveConnCount, tt.isModFrame, got, tt.want)
			}
		})
	}
}
