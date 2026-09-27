// This file is part of All-Chat.
// Copyright (C) 2026 caesarakalaeii
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published
// by the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package subscription

import (
	"testing"

	"github.com/caesar/all-chat/services/api-gateway/models"
)

// Pentest F2a: a status frame belongs on an overlay only when the overlay has
// that exact platform+channel configured. Platform-only matching broadcast
// every other streamer's statuses cross-tenant; this table pins the conjunct.
func TestOverlayHasSource(t *testing.T) {
	sources := []OverlaySource{
		{Platform: "twitch", ChannelID: "111", ChannelName: "mine"},
		{Platform: "youtube", ChannelID: "UC-mine", ChannelName: "mine-yt"},
	}

	cases := []struct {
		name     string
		platform string
		channel  string
		want     bool
	}{
		{"exact platform+channel match", "twitch", "111", true},
		{"second source matches", "youtube", "UC-mine", true},
		{"same platform, different channel must not match", "twitch", "999", false},
		{"different platform, same channel id must not match", "kick", "111", false},
		{"no sources configured", "", "", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := overlayHasSource(sources, models.PlatformStatusData{Platform: tc.platform, ChannelID: tc.channel})
			if got != tc.want {
				t.Errorf("overlayHasSource(platform=%q, channel=%q) = %v, want %v",
					tc.platform, tc.channel, got, tc.want)
			}
		})
	}
}

// The negative case is the finding: an overlay sharing only the platform with
// the status frame must not receive it — that was the cross-tenant leak.
func TestOverlayHasSource_EmptySourcesNeverMatches(t *testing.T) {
	if overlayHasSource(nil, models.PlatformStatusData{Platform: "twitch", ChannelID: "111"}) {
		t.Error("overlayHasSource(nil, …) = true; an overlay with no sources must never match")
	}
}
