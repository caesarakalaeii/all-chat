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

	"github.com/caesar/all-chat/services/message-processor/models"
	"github.com/stretchr/testify/assert"
)

func TestNormalizeDeletion_BatchTimeout_BanDurationFloat64(t *testing.T) {
	// After a JSON round-trip over Redis Streams, numeric EventData values are
	// float64. This is the real production shape — a timeout must keep its duration.
	raw := &models.RawChatMessage{
		MessageID: "del-1",
		Platform:  "twitch",
		EventType: "message_deletion",
		EventData: map[string]interface{}{
			"deletion_type":   "batch",
			"target_user_id":  "u123",
			"target_username": "spammer",
			"ban_duration":    float64(600),
		},
	}

	unified := NormalizeDeletion(raw)

	assert.Equal(t, "message_deletion", unified.Event.Type)
	assert.Equal(t, "batch", unified.Event.Metadata["deletion_type"])
	assert.Equal(t, "u123", unified.Event.Metadata["target_user_id"])
	assert.Equal(t, "spammer", unified.Event.Metadata["target_username"])
	// Forwarded as int and NOT dropped — distinguishes a timeout from a permanent ban.
	assert.Equal(t, 600, unified.Event.Metadata["ban_duration"])
}

func TestNormalizeDeletion_BatchTimeout_BanDurationInt(t *testing.T) {
	// Defensive: an in-process int (no JSON round-trip) must still work.
	raw := &models.RawChatMessage{
		EventType: "message_deletion",
		EventData: map[string]interface{}{
			"deletion_type":  "batch",
			"target_user_id": "u123",
			"ban_duration":   300,
		},
	}

	unified := NormalizeDeletion(raw)

	assert.Equal(t, 300, unified.Event.Metadata["ban_duration"])
}

func TestNormalizeDeletion_BatchBan_NoDuration(t *testing.T) {
	// Permanent ban: no ban_duration present -> key omitted (frontend reads this as a ban).
	raw := &models.RawChatMessage{
		EventType: "message_deletion",
		EventData: map[string]interface{}{
			"deletion_type":  "batch",
			"target_user_id": "u123",
		},
	}

	unified := NormalizeDeletion(raw)

	_, hasDuration := unified.Event.Metadata["ban_duration"]
	assert.False(t, hasDuration, "permanent ban must not carry a ban_duration")
	assert.Equal(t, "u123", unified.Event.Metadata["target_user_id"])
}

func TestNormalizeDeletion_Clear(t *testing.T) {
	raw := &models.RawChatMessage{
		EventType: "message_deletion",
		EventData: map[string]interface{}{"deletion_type": "clear"},
	}

	unified := NormalizeDeletion(raw)

	assert.Equal(t, "clear", unified.Event.Metadata["deletion_type"])
}

func TestNormalizeDeletion_Single(t *testing.T) {
	raw := &models.RawChatMessage{
		EventType: "message_deletion",
		EventData: map[string]interface{}{
			"deletion_type": "single",
			"target_uuid":   "msg-uuid-1",
		},
	}

	unified := NormalizeDeletion(raw)

	assert.Equal(t, "single", unified.Event.Metadata["deletion_type"])
	assert.Equal(t, "msg-uuid-1", unified.Event.Metadata["target_uuid"])
}
