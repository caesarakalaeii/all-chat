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

package subscribers

import (
	"time"

	"github.com/google/uuid"
)

// RawChatMessage mirrors innertube.RawChatMessage's wire fields for event
// publication. The poller builds the subscriber event in the exact chat:raw
// schema so the message-processor consumes it like any other YouTube event.
type RawChatMessage struct {
	MessageID string            `json:"message_id"`
	Platform  string            `json:"platform"`
	ChannelID string            `json:"channel_id"`
	StreamID  string            `json:"stream_id"`
	UserID    string            `json:"user_id"`
	Username  string            `json:"username"`
	Text      string            `json:"text"`
	Timestamp time.Time         `json:"timestamp"`
	Tags      map[string]string `json:"tags"`

	EventType string                 `json:"event_type,omitempty"`
	EventData map[string]interface{} `json:"event_data,omitempty"`
}

// BuildEventMessage converts a poller Event into the chat:raw subscriber
// event. Contract (message-processor normalizer):
//
//	event_type: "subscriber"
//	event_data: subscriber_title, subscriber_channel_id, subscriber_avatar_url, published_at
//	tags: display_name, profile_image (standard youtube tags)
func BuildEventMessage(ev Event, at time.Time) *RawChatMessage {
	title := ev.Title
	if title == "" {
		title = "Someone"
	}
	return &RawChatMessage{
		MessageID: uuid.New().String(),
		Platform:  "youtube",
		ChannelID: ev.ChannelID,
		StreamID:  ev.StreamID,
		UserID:    ev.UserID,
		Username:  title,
		Text:      title + " just subscribed",
		Timestamp: at.UTC(),
		Tags: map[string]string{
			"display_name":  title,
			"profile_image": ev.AvatarURL,
			"is_sponsor":    "false",
			"is_moderator":  "false",
			"is_owner":      "false",
			"is_verified":   "false",
		},
		EventType: "subscriber",
		EventData: map[string]interface{}{
			"subscriber_title":      title,
			"subscriber_channel_id": ev.UserID,
			"subscriber_avatar_url": ev.AvatarURL,
			"published_at":          ev.SubAt.UTC().Format(time.RFC3339),
		},
	}
}
