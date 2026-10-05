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

package models

import (
	"encoding/json"
	"testing"
	"time"

	mpmodels "github.com/caesar/all-chat/services/message-processor/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAlertJSONSchema pins the wire contract of the alert envelope published to
// overlay:{id}:alerts and stored in alert_events.event_data. The alerts overlay
// frontend parses this JSON; renaming a key silently breaks every alert render,
// so the keys are asserted exactly, not loosely.
func TestAlertJSONSchema(t *testing.T) {
	occurred := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	alert := Alert{
		AlertID:   "alert-uuid",
		OverlayID: "overlay-uuid",
		Platform:  "twitch",
		EventType: "bits",
		EventData: &mpmodels.EventInfo{
			Type:  "bits",
			Tier:  "high",
			Value: &mpmodels.EventValue{Amount: 100, Currency: "bits", DisplayText: "100 bits"},
		},
		User: AlertUser{
			ID:        "user-1",
			Name:      "SomeChatter",
			AvatarURL: "https://cdn.example/avatar.png",
		},
		OccurredAt: occurred,
	}

	raw, err := json.Marshal(alert)
	require.NoError(t, err)

	var envelope map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &envelope))

	wantKeys := []string{"alert_id", "overlay_id", "platform", "event_type", "event_data", "user", "occurred_at"}
	gotKeys := make([]string, 0, len(envelope))
	for k := range envelope {
		gotKeys = append(gotKeys, k)
	}
	assert.ElementsMatch(t, wantKeys, gotKeys, "alert envelope keys must match the documented schema exactly")

	var user map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(envelope["user"], &user))
	assert.ElementsMatch(t, []string{"id", "name", "avatar_url"}, keysOf(user), "alert user keys must match the documented schema exactly")

	// The envelope must round-trip: the publisher serializes the same value the
	// repository persists, and a lossy round-trip would make them disagree.
	var back Alert
	require.NoError(t, json.Unmarshal(raw, &back))
	assert.Equal(t, alert, back, "alert must survive a JSON round-trip unchanged")
}

func keysOf(m map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// TestNewAlertID_DeterministicAndOverlayScoped pins the idempotency contract of
// alert_events.id: the same stream message re-delivered to the same overlay must
// produce the same id (ON CONFLICT DO NOTHING then dedupes), while different
// overlays or different messages must get different ids (one row per delivery).
func TestNewAlertID_DeterministicAndOverlayScoped(t *testing.T) {
	first := NewAlertID("twitch", "12345", "msg-1", "overlay-a")
	again := NewAlertID("twitch", "12345", "msg-1", "overlay-a")
	assert.Equal(t, first, again, "re-delivery of the same message must derive the same alert id")

	assert.NotEqual(t, first, NewAlertID("twitch", "12345", "msg-1", "overlay-b"),
		"each target overlay needs its own alert row")
	assert.NotEqual(t, first, NewAlertID("twitch", "12345", "msg-2", "overlay-a"),
		"different messages must not collide")
	assert.NotEqual(t, first, NewAlertID("youtube", "12345", "msg-1", "overlay-a"),
		"different platforms must not collide")

	_, err := uuid.Parse(first)
	assert.NoError(t, err, "derived alert id must be a valid UUID")
}
