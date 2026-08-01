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

package filter

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// EventFilter checks if specific event types are enabled for overlays
type EventFilter struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

// NewEventFilter creates a new event filter
func NewEventFilter(db *pgxpool.Pool, logger *zap.Logger) *EventFilter {
	return &EventFilter{
		db:     db,
		logger: logger,
	}
}

// CarriesChatterMessage reports whether an event type's text IS the chatter's own chat message,
// rather than a system description of something that happened.
//
// These events are chat first and decoration second: a Twitch watch streak is a returning viewer's
// ordinary message that Twitch happens to tag with a milestone, and an announcement is a message the
// broadcaster chose to highlight (ADR-0046). That distinction decides what a disabled per-overlay
// toggle may do. For an ordinary event (a follow, a raid) "disabled" means drop it. For these, the
// caller suppresses only the decoration and still delivers the message down the chat path — a
// settings toggle must never delete a viewer's message, which would silently re-create the
// dropped-message bug that ADR-0046 fixed.
//
// Deliberately NOT included: resubscription. Its text is an optional message attached to a
// subscription event, so a streamer disabling "Resubscriptions" means the whole notice, and
// including it here would change long-standing behaviour rather than fix a regression.
func CarriesChatterMessage(platform, eventType string) bool {
	if platform != "twitch" {
		return false
	}
	switch eventType {
	case "watch_streak", "announcement":
		return true
	default:
		return false
	}
}

// columnAlwaysEnabled marks event types that are deliberately not toggleable, so they are enabled
// without a database round-trip and without the unknown-event warning. Used for notices that are
// chat content rather than alerts (an announcement is a message the broadcaster chose to highlight —
// hiding it would hide chat) and for rare informational notices with no natural settings home.
const columnAlwaysEnabled = "-"

// IsEventEnabled checks if an event type is enabled for a specific overlay
// Returns true if enabled, false if disabled, and error on database issues
func (f *EventFilter) IsEventEnabled(ctx context.Context, overlayID, platform, eventType string) (bool, error) {
	// Map event type to database column name
	columnName := mapEventTypeToColumn(platform, eventType)
	if columnName == columnAlwaysEnabled {
		return true, nil
	}
	if columnName == "" {
		// Unknown event type - default to enabled
		f.logger.Warn("Unknown event type, defaulting to enabled",
			zap.String("platform", platform),
			zap.String("event_type", eventType),
		)
		return true, nil
	}

	// Query the overlay_event_settings table
	query := fmt.Sprintf(`
		SELECT %s
		FROM overlay_event_settings
		WHERE overlay_id = $1
	`, columnName)

	var enabled bool
	err := f.db.QueryRow(ctx, query, overlayID).Scan(&enabled)
	if err != nil {
		// If no settings row exists, default to enabled
		// (migration creates settings for all overlays, but be defensive)
		f.logger.Debug("No event settings found, defaulting to enabled",
			zap.String("overlay_id", overlayID),
			zap.Error(err),
		)
		return true, nil
	}

	return enabled, nil
}

// mapEventTypeToColumn converts a platform + event type to database column name
func mapEventTypeToColumn(platform, eventType string) string {
	// Normalize event type (remove underscores, lowercase)
	normalized := strings.ReplaceAll(eventType, "_", "")

	switch platform {
	case "twitch":
		switch eventType {
		case "subscription":
			return "enable_twitch_subs"
		case "resubscription":
			return "enable_twitch_resubs"
		case "gift_subscription", "mystery_gift", "gift_paid_upgrade", "anon_gift_paid_upgrade",
			"prime_paid_upgrade", "pay_it_forward":
			return "enable_twitch_gift_subs"
		case "bits", "bits_badge_tier":
			return "enable_twitch_bits"
		case "raid", "unraid":
			return "enable_twitch_raids"
		case "channel_points":
			return "enable_twitch_channel_points"
		case "follow":
			return "enable_twitch_follows"
		case "watch_streak":
			return "enable_twitch_watch_streaks"
		case "announcement", "charity_donation", "modiversary", "twitch_notice":
			// Chat notices that are not toggleable — see columnAlwaysEnabled.
			return columnAlwaysEnabled
		default:
			// Unknown Twitch event, group under subs for now
			if strings.Contains(normalized, "sub") || strings.Contains(normalized, "gift") {
				return "enable_twitch_gift_subs"
			}
			return ""
		}

	case "youtube":
		switch eventType {
		case "super_chat":
			return "enable_youtube_super_chat"
		case "super_sticker":
			return "enable_youtube_super_sticker"
		case "new_sponsor":
			return "enable_youtube_members"
		case "member_milestone":
			return "enable_youtube_member_milestones"
		case "membership_gift", "gift_received":
			return "enable_youtube_member_gifts"
		default:
			return ""
		}

	case "kick":
		switch eventType {
		case "subscription":
			return "enable_kick_subs"
		case "gift_subscription", "donation":
			return "enable_kick_gifts"
		default:
			return ""
		}

	case "tiktok":
		switch eventType {
		case "like_aggregate":
			return "enable_tiktok_likes"
		case "gift":
			return "enable_tiktok_gifts"
		case "follow":
			return "enable_tiktok_follows"
		case "share":
			return "enable_tiktok_shares"
		default:
			return ""
		}

	case "system":
		switch eventType {
		case "token_expiration_warning", "source_permission_error":
			return "enable_token_warnings"
		default:
			return ""
		}

	default:
		return ""
	}
}
