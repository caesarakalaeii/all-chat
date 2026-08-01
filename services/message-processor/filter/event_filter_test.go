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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestMapEventTypeToColumn_Twitch(t *testing.T) {
	tests := []struct {
		eventType string
		expected  string
	}{
		{"subscription", "enable_twitch_subs"},
		{"resubscription", "enable_twitch_resubs"},
		{"gift_subscription", "enable_twitch_gift_subs"},
		{"mystery_gift", "enable_twitch_gift_subs"},
		{"bits", "enable_twitch_bits"},
		{"raid", "enable_twitch_raids"},
		{"unraid", "enable_twitch_raids"},
		{"channel_points", "enable_twitch_channel_points"},
		{"follow", "enable_twitch_follows"},
		// Chat notices (ADR-0046): watch streaks get their own toggle; the sub-adjacent
		// conversions ride the gift-sub toggle; the rest are chat content, never toggleable.
		{"watch_streak", "enable_twitch_watch_streaks"},
		{"bits_badge_tier", "enable_twitch_bits"},
		{"gift_paid_upgrade", "enable_twitch_gift_subs"},
		{"prime_paid_upgrade", "enable_twitch_gift_subs"},
		{"pay_it_forward", "enable_twitch_gift_subs"},
		{"announcement", columnAlwaysEnabled},
		{"charity_donation", columnAlwaysEnabled},
		{"modiversary", columnAlwaysEnabled},
		{"twitch_notice", columnAlwaysEnabled},
		{"unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.eventType, func(t *testing.T) {
			result := mapEventTypeToColumn("twitch", tt.eventType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMapEventTypeToColumn_YouTube(t *testing.T) {
	tests := []struct {
		eventType string
		expected  string
	}{
		{"super_chat", "enable_youtube_super_chat"},
		{"super_sticker", "enable_youtube_super_sticker"},
		{"new_sponsor", "enable_youtube_members"},
		{"member_milestone", "enable_youtube_member_milestones"},
		{"membership_gift", "enable_youtube_member_gifts"},
		{"gift_received", "enable_youtube_member_gifts"},
		{"unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.eventType, func(t *testing.T) {
			result := mapEventTypeToColumn("youtube", tt.eventType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMapEventTypeToColumn_TikTok(t *testing.T) {
	tests := []struct {
		eventType string
		expected  string
	}{
		{"like_aggregate", "enable_tiktok_likes"},
		{"gift", "enable_tiktok_gifts"},
		{"follow", "enable_tiktok_follows"},
		{"share", "enable_tiktok_shares"},
		{"unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.eventType, func(t *testing.T) {
			result := mapEventTypeToColumn("tiktok", tt.eventType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMapEventTypeToColumn_UnknownPlatform(t *testing.T) {
	result := mapEventTypeToColumn("unknown_platform", "subscription")
	assert.Equal(t, "", result)
}

// CarriesChatterMessage decides whether a DISABLED per-overlay toggle may discard the message or
// must only suppress its decoration. Getting this wrong means a settings toggle deletes a viewer's
// chat message — the exact failure ADR-0046 set out to fix.
func TestCarriesChatterMessage(t *testing.T) {
	tests := []struct {
		platform  string
		eventType string
		want      bool
		why       string
	}{
		{"twitch", "watch_streak", true, "the notice payload IS the viewer's chat message"},
		{"twitch", "announcement", true, "the notice payload IS the announcement body"},
		{"twitch", "subscription", false, "system description of a sub, no chatter message"},
		{"twitch", "raid", false, "system description of a raid"},
		{"twitch", "follow", false, "system description of a follow"},
		{"twitch", "bits_badge_tier", false, "system description of a badge unlock"},
		{"twitch", "resubscription", false, "optional message attached to a sub event; long-standing behaviour is to drop the whole notice"},
		{"twitch", "", false, "plain chat never reaches the event filter"},
		{"youtube", "super_chat", false, "scoped to Twitch chat notices only"},
		{"kick", "announcement", false, "scoped to Twitch chat notices only"},
	}

	for _, tt := range tests {
		t.Run(tt.platform+"/"+tt.eventType, func(t *testing.T) {
			assert.Equal(t, tt.want, CarriesChatterMessage(tt.platform, tt.eventType), tt.why)
		})
	}
}

// The always-enabled sentinel must never reach the SQL query, which interpolates the column name.
func TestIsEventEnabled_AlwaysEnabledSentinelNeverQueries(t *testing.T) {
	// A nil pool would panic if the sentinel fell through to the query, so this both asserts the
	// short-circuit and proves no SQL is built for these types.
	f := NewEventFilter(nil, zap.NewNop())
	for _, eventType := range []string{"announcement", "charity_donation", "modiversary", "twitch_notice"} {
		t.Run(eventType, func(t *testing.T) {
			enabled, err := f.IsEventEnabled(context.Background(), "overlay-1", "twitch", eventType)
			require.NoError(t, err)
			assert.True(t, enabled)
		})
	}
}
