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

// InstagramNormalizer normalizes Instagram live_comments to unified format.
//
// IG live comments carry no per-user colors, badges or emote tokens (ADR-0062
// excludes emotes; live_comments gives only id, text, timestamp, username and
// user{id}), and the ingest is read-only: there is no moderation write path,
// so the metadata records just the native comment id.
type InstagramNormalizer struct{}

// NewInstagramNormalizer creates a new Instagram message normalizer.
func NewInstagramNormalizer() *InstagramNormalizer {
	return &InstagramNormalizer{}
}

// Normalize converts a RawChatMessage to UnifiedChatMessage.
func (n *InstagramNormalizer) Normalize(raw *models.RawChatMessage, overlayID string) (*models.UnifiedChatMessage, error) {
	if raw.Platform != "instagram" {
		return nil, fmt.Errorf("unsupported platform: %s", raw.Platform)
	}

	if err := validateChannelID(raw.ChannelID); err != nil {
		return nil, fmt.Errorf("invalid channel ID: %w", err)
	}

	timestamp := raw.Timestamp
	if timestamp.IsZero() {
		timestamp = time.Now()
	}

	// live_comments exposes only the @handle as a name; when even that is
	// absent fall back to the commenter id so the overlay never renders blank.
	username := firstNonEmpty(raw.Username, raw.UserID)

	return &models.UnifiedChatMessage{
		ID:          raw.MessageID,
		OverlayID:   overlayID,
		Platform:    "instagram",
		ChannelID:   raw.ChannelID,
		ChannelName: firstNonEmpty(raw.ChannelName, raw.ChannelID),
		User: models.UserInfo{
			ID:          raw.UserID,
			Username:    username,
			DisplayName: username,
			// live_comments expose no avatar URL, colors or badges.
		},
		Message: models.MessageInfo{
			Text: raw.Text,
			// Emotes are out of scope for Instagram (ADR-0062).
		},
		Timestamp: timestamp,
		Metadata:  n.extractMetadata(raw),
	}, nil
}

// extractMetadata records the native comment id; ingest is read-only (no
// moderation write path for Instagram, ADR-0062).
func (n *InstagramNormalizer) extractMetadata(raw *models.RawChatMessage) map[string]interface{} {
	metadata := make(map[string]interface{})

	metadata["instagram_comment_id"] = raw.MessageID

	metadata["bits"] = 0
	metadata["is_subscriber"] = false
	metadata["is_turbo"] = false

	return metadata
}
