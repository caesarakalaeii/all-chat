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

// Package models defines the alert envelope: the normalized event the
// alert-processor publishes to overlay:{id}:alerts and persists to alert_events.
package models

import (
	"strings"
	"time"

	mpmodels "github.com/caesar/all-chat/services/message-processor/models"
	"github.com/google/uuid"
)

// Alert is one platform event delivered to one alert-capable overlay. The JSON
// shape is the wire contract with the alerts-overlay frontend — see
// TestAlertJSONSchema, which pins the keys exactly.
type Alert struct {
	AlertID    string              `json:"alert_id"`
	OverlayID  string              `json:"overlay_id"`
	Platform   string              `json:"platform"`
	EventType  string              `json:"event_type"`
	EventData  *mpmodels.EventInfo `json:"event_data"`
	User       AlertUser           `json:"user"`
	OccurredAt time.Time           `json:"occurred_at"`
}

// AlertUser is the event sender. ID is the platform-native id (most senders
// never resolve to an All-Chat account), Name the display name as normalized.
type AlertUser struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
}

// NewAlertID derives the alert_events primary key from the identity of the
// delivery: which raw stream message, sent to which overlay.
//
// It is deliberately NOT uuid.New(): the consumer group is at-least-once, so a
// half-finished batch is re-delivered and re-persisted. A random id would then
// insert a second, identical row; this derivation makes the retry land on the
// same primary key and ON CONFLICT DO NOTHING absorb it. The per-overlay
// component keeps one routed message to N overlays from collapsing onto one
// row. See migration 101.
func NewAlertID(platform, channelID, messageID, overlayID string) string {
	identity := strings.Join([]string{"all-chat:alert", platform, channelID, messageID, overlayID}, "\x00")
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte(identity)).String()
}

// AlertUserFrom maps the message-processor's normalized sender onto the alert
// envelope's user. The display name is what the viewer recognizes; platforms
// that supply none (or normalize to the raw handle) fall back to the username.
func AlertUserFrom(user mpmodels.UserInfo) AlertUser {
	name := user.DisplayName
	if name == "" {
		name = user.Username
	}
	return AlertUser{
		ID:        user.ID,
		Name:      name,
		AvatarURL: user.AvatarURL,
	}
}
