package main

import (
	"testing"

	"github.com/caesar/all-chat/services/api-gateway/models"
)

func TestShouldBufferForReplay(t *testing.T) {
	const testOverlayID = "test-ov"

	tests := []struct {
		name          string
		msgType       models.WSMessageType
		overlayID     string
		liveConnCount int
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldBufferForReplay(tt.msgType, tt.overlayID, testOverlayID, tt.liveConnCount)
			if got != tt.want {
				t.Errorf("shouldBufferForReplay(%q, %q, %q, %d) = %v, want %v",
					tt.msgType, tt.overlayID, testOverlayID, tt.liveConnCount, got, tt.want)
			}
		})
	}
}
