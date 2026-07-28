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

package seventv

import (
	"encoding/json"
	"testing"
)

func TestUserResponse_ActiveEmoteSetID(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			// Post-2026-08-03 shape: emote_set is null, only emote_set_id is set.
			name: "null emote_set with top-level emote_set_id",
			body: `{"id": "u1", "platform": "TWITCH", "username": "xqc", "emote_set_id": "01FE9DRF000009TR6M9N941CYW", "emote_set": null}`,
			want: "01FE9DRF000009TR6M9N941CYW",
		},
		{
			// Legacy shape: embedded emote_set only.
			name: "embedded emote_set without top-level id",
			body: `{"id": "u1", "platform": "TWITCH", "username": "xqc", "emote_set": {"id": "legacy-set", "name": "Emotes"}}`,
			want: "legacy-set",
		},
		{
			name: "top-level id preferred over embedded set",
			body: `{"emote_set_id": "top-level", "emote_set": {"id": "embedded"}}`,
			want: "top-level",
		},
		{
			name: "no emote set at all",
			body: `{"id": "u1", "platform": "TWITCH", "emote_set_id": null, "emote_set": null}`,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var user UserResponse
			if err := json.Unmarshal([]byte(tt.body), &user); err != nil {
				t.Fatalf("failed to unmarshal: %v", err)
			}
			if got := user.ActiveEmoteSetID(); got != tt.want {
				t.Errorf("ActiveEmoteSetID() = %q, want %q", got, tt.want)
			}
		})
	}
}
