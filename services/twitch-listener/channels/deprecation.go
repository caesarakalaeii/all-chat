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

package channels

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// DeprecationMode controls the Twitch IRC listener shutdown behaviour. The IRC
// listener is being retired in favour of the EventSub listener (ADR-0017); this
// gate drives a two-phase rollout:
//
//   - warn:    join channels as usual, but publish an in-overlay migration
//     notice to every connected source on a fixed interval.
//   - enforce: refuse to join any Twitch channel, forcing users to re-add their
//     source (which routes them to the EventSub listener).
//
// The phases are sequenced deliberately: run "warn" for a grace period so users
// see the notice and migrate, then flip to "enforce" to stop serving IRC chat.
type DeprecationMode int

const (
	// DeprecationOff is the default: the listener behaves normally.
	DeprecationOff DeprecationMode = iota
	// DeprecationWarn keeps serving chat but nudges connected sources to migrate.
	DeprecationWarn
	// DeprecationEnforce stops the listener from joining any channel.
	DeprecationEnforce
)

func (m DeprecationMode) String() string {
	switch m {
	case DeprecationWarn:
		return "warn"
	case DeprecationEnforce:
		return "enforce"
	default:
		return "off"
	}
}

// ParseDeprecationMode maps the TWITCH_IRC_DEPRECATION_MODE env value to a mode.
// Unknown or empty values resolve to "off" — fail-safe, so a typo can never
// silently stop the listener from serving chat.
func ParseDeprecationMode(s string) DeprecationMode {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "warn", "soft":
		return DeprecationWarn
	case "enforce", "hard", "block":
		return DeprecationEnforce
	default:
		return DeprecationOff
	}
}

// DefaultDeprecationNoticeInterval is how often connected sources are reminded
// to migrate during the warn phase.
const DefaultDeprecationNoticeInterval = 5 * time.Minute

// DeprecationConfig is the resolved deprecation gate for the channel manager.
type DeprecationConfig struct {
	Mode           DeprecationMode
	NoticeInterval time.Duration
}

// DeprecationNoticePublisher publishes a single migration notice for an overlay
// subscribed to the given (now-deprecated) Twitch channel.
type DeprecationNoticePublisher interface {
	PublishDeprecationNotice(ctx context.Context, overlayID, channel string) error
}

// deprecationNoticeText is the human-readable description rendered in the overlay
// activity feed and dashboard. Kept terse — the frontend supplies the styling and
// the call-to-action link.
const deprecationNoticeText = "The legacy Twitch chat connection is being retired. Re-add your Twitch source to keep chat working — it switches to the new EventSub connection automatically."

// deprecationStreamKey is the Redis Stream the message-processor consumes. Mirrors
// publisher.StreamKey; duplicated here (rather than imported) to avoid a package
// dependency from channels → publisher.
const deprecationStreamKey = "chat:raw"

// RedisNoticePublisher publishes deprecation notices onto the chat:raw Redis
// Stream as "system" events, reusing the existing system-message pipeline
// (message-processor SystemNormalizer → overlay pub/sub). This mirrors the
// token-refresh-service token_expiration_warning publish path exactly, so the
// notice reaches the overlay activity feed with no new plumbing.
type RedisNoticePublisher struct {
	redis *redis.Client
}

// NewRedisNoticePublisher builds a publisher backed by the given Redis client.
func NewRedisNoticePublisher(rc *redis.Client) *RedisNoticePublisher {
	return &RedisNoticePublisher{redis: rc}
}

// PublishDeprecationNotice publishes a listener_deprecation_notice system event
// targeted at a single overlay.
func (p *RedisNoticePublisher) PublishDeprecationNotice(ctx context.Context, overlayID, channel string) error {
	raw := map[string]interface{}{
		"message_id":   uuid.New().String(),
		"platform":     "system",
		"overlay_id":   overlayID,
		"channel_id":   "system",
		"channel_name": "All-Chat System",
		"user_id":      "system",
		"username":     "system",
		"text":         "",
		"timestamp":    time.Now().UTC().Format(time.RFC3339),
		"event_type":   "listener_deprecation_notice",
		"event_data": map[string]interface{}{
			"platform":    "twitch",
			"channel_id":  channel,
			"description": deprecationNoticeText,
			"action_url":  "/dashboard",
		},
	}

	payload, err := json.Marshal(raw)
	if err != nil {
		return fmt.Errorf("marshal deprecation notice: %w", err)
	}

	return p.redis.XAdd(ctx, &redis.XAddArgs{
		Stream: deprecationStreamKey,
		Values: map[string]interface{}{"data": string(payload)},
	}).Err()
}
