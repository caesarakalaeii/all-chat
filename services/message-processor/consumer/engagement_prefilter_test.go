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

package consumer

import (
	"strings"
	"testing"
)

// TestLooksLikeCommand covers the hot-path pre-filter, including the P2-5 fix: a leading
// '+'/'-' ("+1" chat-agreement idiom) must NOT be treated as a bare-number vote candidate
// even though strconv.Atoi would accept it.
func TestLooksLikeCommand(t *testing.T) {
	cases := []struct {
		text string
		want bool
	}{
		{"!vote 2", true},
		{"!bet 1 100", true},
		{"1", true},
		{"42", true},
		{" 7 ", true}, // trimmed
		{"hello", false},
		{"", false},
		{"   ", false},
		{"123", false},                          // 3 digits: too long for the bare shortcut
		{"+1", false},                           // P2-5: chat-agreement idiom, not a vote
		{"-1", false},                           // P2-5
		{"+9", false},                           // P2-5
		{"9x", false},                           // leading digit but not a number
		{"!" + strings.Repeat("x", 600), false}, // P3-6: over the byte cap, not a real command
	}
	for _, tc := range cases {
		if got := looksLikeCommand(tc.text); got != tc.want {
			t.Errorf("looksLikeCommand(%q) = %v, want %v", tc.text, got, tc.want)
		}
	}
}
