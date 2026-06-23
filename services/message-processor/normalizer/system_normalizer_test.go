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

func TestSystemNormalizer_ListenerDeprecationNotice(t *testing.T) {
	n := NewSystemNormalizer()
	raw := &models.RawChatMessage{
		MessageID: "msg-1",
		Platform:  "system",
		EventType: "listener_deprecation_notice",
		Timestamp: time.Now(),
		EventData: map[string]interface{}{
			"platform":    "twitch",
			"channel_id":  "xqc",
			"description": "Re-add your Twitch source to keep chat working.",
			"action_url":  "/dashboard",
		},
	}

	unified, err := n.Normalize(raw, "overlay-123")
	require.NoError(t, err)

	assert.Equal(t, "overlay-123", unified.OverlayID)
	assert.Equal(t, "system", unified.Platform)
	require.NotNil(t, unified.Event)
	assert.Equal(t, "listener_deprecation_notice", unified.Event.Type)
	assert.Equal(t, "high", unified.Event.Tier)
	assert.Equal(t, "Re-add your Twitch source to keep chat working.", unified.Event.Metadata["description"])
	assert.Equal(t, "/dashboard", unified.Event.Metadata["action_url"])
}

// A notice without a supplied description must still carry a sensible default so
// the overlay never renders an empty migration banner.
func TestSystemNormalizer_ListenerDeprecationNotice_DefaultDescription(t *testing.T) {
	n := NewSystemNormalizer()
	raw := &models.RawChatMessage{
		MessageID: "msg-2",
		Platform:  "system",
		EventType: "listener_deprecation_notice",
		Timestamp: time.Now(),
		EventData: map[string]interface{}{"channel_id": "xqc"},
	}

	unified, err := n.Normalize(raw, "overlay-123")
	require.NoError(t, err)
	require.NotNil(t, unified.Event)
	assert.NotEmpty(t, unified.Event.Metadata["description"])
}

func TestSystemNormalizer_RejectsUnknownEventType(t *testing.T) {
	n := NewSystemNormalizer()
	raw := &models.RawChatMessage{
		Platform:  "system",
		EventType: "totally_unknown",
		Timestamp: time.Now(),
	}
	_, err := n.Normalize(raw, "overlay-123")
	assert.Error(t, err)
}
