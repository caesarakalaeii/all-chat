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

package clients

import "testing"

func TestBuildTwitchEmoteURLHandlesTemplates(t *testing.T) {
	emote := twitchChatEmote{
		ID:        "305954156",
		Format:    []string{"static"},
		ThemeMode: []string{"dark"},
		Scale:     []string{"3.0"},
	}

	tests := []struct {
		name     string
		template string
		expected string
	}{
		{
			name:     "single braces",
			template: "https://static-cdn.jtvnw.net/emoticons/v2/{id}/{format}/{theme_mode}/{scale}",
			expected: "https://static-cdn.jtvnw.net/emoticons/v2/305954156/static/dark/3.0",
		},
		{
			name:     "double braces",
			template: "https://static-cdn.jtvnw.net/emoticons/v2/{{id}}/{{format}}/{{theme_mode}}/{{scale}}",
			expected: "https://static-cdn.jtvnw.net/emoticons/v2/305954156/static/dark/3.0",
		},
	}

	for _, tt := range tests {
		got := buildTwitchEmoteURL(tt.template, emote)
		if got != tt.expected {
			t.Fatalf("%s: expected %s, got %s", tt.name, tt.expected, got)
		}
	}
}
