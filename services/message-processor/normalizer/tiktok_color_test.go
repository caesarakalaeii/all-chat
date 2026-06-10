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
	"testing"
	"time"

	"github.com/caesar/all-chat/services/message-processor/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TikTok no longer hardcodes the brand pink (#FE2C55) as every viewer's color.
// The color is left empty so the viewer-badge enricher can assign each viewer a
// deterministic per-viewer auto-color downstream.
func TestNormalize_TikTokLeavesColorEmpty(t *testing.T) {
	n := NewTikTokNormalizer()

	raw := &models.RawChatMessage{
		MessageID: "tt-msg-color",
		Platform:  "tiktok",
		ChannelID: "creator123",
		UserID:    "user456",
		Username:  "TTViewer",
		Text:      "hello",
		Timestamp: time.Now(),
		Tags:      map[string]string{"user_unique_id": "ttviewer"},
	}

	unified, err := n.Normalize(raw, "overlay-tt-color")
	require.NoError(t, err)
	require.NotNil(t, unified)

	assert.Empty(t, unified.User.Color, "TikTok normalizer should leave color empty for the auto-color enricher")
	assert.NotEqual(t, "#FE2C55", unified.User.Color, "hardcoded TikTok brand color must not be set")
}
