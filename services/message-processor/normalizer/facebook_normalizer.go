// This file is part of All-Chat.
// Copyright (C) 2026 caesarakalaeii
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package normalizer

import (
	"fmt"
	"time"

	"github.com/caesar/all-chat/services/message-processor/models"
)

// FacebookNormalizer normalizes Facebook live-video comments to unified format.
//
// Facebook comments carry no per-user colors, badges or emote tokens (ADR-0060
// excludes emotes; Graph gives no badge surface on comments), so the user block
// is just id/name and the metadata records the moderation-relevant ids: the
// native comment id (for delete/hide) and the commenter id (for ban).
type FacebookNormalizer struct{}

// NewFacebookNormalizer creates a new Facebook message normalizer.
func NewFacebookNormalizer() *FacebookNormalizer {
	return &FacebookNormalizer{}
}

// Normalize converts a RawChatMessage to UnifiedChatMessage.
func (n *FacebookNormalizer) Normalize(raw *models.RawChatMessage, overlayID string) (*models.UnifiedChatMessage, error) {
	if raw.Platform != "facebook" {
		return nil, fmt.Errorf("unsupported platform: %s", raw.Platform)
	}

	if err := validateChannelID(raw.ChannelID); err != nil {
		return nil, fmt.Errorf("invalid channel ID: %w", err)
	}

	timestamp := raw.Timestamp
	if timestamp.IsZero() {
		timestamp = time.Now()
	}

	return &models.UnifiedChatMessage{
		ID:          raw.MessageID,
		OverlayID:   overlayID,
		Platform:    "facebook",
		ChannelID:   raw.ChannelID,
		ChannelName: firstNonEmpty(raw.ChannelName, raw.ChannelID),
		User: models.UserInfo{
			ID:          raw.UserID,
			Username:    raw.Username,
			DisplayName: raw.Username,
			// Graph comments expose no avatar URL, colors or badges.
		},
		Message: models.MessageInfo{
			Text: raw.Text,
			// Emotes are out of scope for Facebook (ADR-0060).
		},
		Timestamp: timestamp,
		Metadata:  n.extractMetadata(raw),
	}, nil
}

// extractMetadata records the ids the moderation write-path acts on.
func (n *FacebookNormalizer) extractMetadata(raw *models.RawChatMessage) map[string]interface{} {
	metadata := make(map[string]interface{})

	if v := raw.Tags["live_video_id"]; v != "" {
		metadata["live_video_id"] = v
	}
	if v := raw.Tags["parent_comment_id"]; v != "" {
		metadata["parent_comment_id"] = v
	}
	// The native comment id doubles as the moderation target; mirrors
	// youtube's metadata["youtube_message_id"].
	metadata["facebook_comment_id"] = raw.MessageID

	metadata["bits"] = 0
	metadata["is_subscriber"] = false
	metadata["is_turbo"] = false

	return metadata
}
