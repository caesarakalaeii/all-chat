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
	"testing"

	"github.com/stretchr/testify/assert"
)

// EngagementActiveKey must be case-insensitive on both inputs so the writer
// (engagement-service, which keys with the DB-stored channel casing) and the
// reader (message-processor, which keys with the listener-lowercased casing)
// never miss on a mixed-case channel and silently drop a vote/wager.
func TestEngagementActiveKey_CaseNormalized(t *testing.T) {
	tests := []struct {
		name      string
		platform  string
		channelID string
		want      string
	}{
		{
			name:      "mixed-case channel matches lowercased channel",
			platform:  "twitch",
			channelID: "CaesarLP",
			want:      "engagement:active:twitch:caesarlp",
		},
		{
			name:      "already-lowercase channel",
			platform:  "twitch",
			channelID: "caesarlp",
			want:      "engagement:active:twitch:caesarlp",
		},
		{
			name:      "mixed-case platform and channel",
			platform:  "Twitch",
			channelID: "X",
			want:      "engagement:active:twitch:x",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, EngagementActiveKey(tt.platform, tt.channelID))
		})
	}

	// Writer casing and reader casing must resolve to the same key.
	assert.Equal(t,
		EngagementActiveKey("twitch", "CaesarLP"),
		EngagementActiveKey("twitch", "caesarlp"),
		"writer (stored casing) and reader (lowercased casing) must agree on the key",
	)
}
