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

package irc

import "testing"

func TestIsValidTwitchLogin(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want bool
	}{
		// Valid logins per https://help.twitch.tv/s/article/managing-your-account
		// (4–25 chars; lowercase a-z, digits, underscore).
		{"plain ascii", "caedrel", true},
		{"with digits", "shahin200x", true},
		{"with underscore", "sephi_tv", true},
		{"min length 3 (legacy logins like xqc)", "xqc", true},
		{"length 4", "abcd", true},
		{"max length 25", "abcdefghij0123456789klmno", true},
		{"mixed case is normalised", "CaesarLP", true},

		// Invalid — Twitch silently drops these JOINs (no SELFJOIN, no NOTICE),
		// which used to drive the joinAckWatchdog into a reconnect storm.
		{"arabic", "شوشو", false},
		{"chinese", "一代鹹魚", false},
		{"cyrillic mixed", "fooшоу", false},
		{"hyphen", "hello-world", false},
		{"space", "foo bar", false},
		{"too short 2", "ab", false},
		{"too long 26", "abcdefghij0123456789klmnop", false},
		{"empty", "", false},
		{"leading hash like an irc channel", "#caedrel", false},
		{"only digits", "1234", true}, // Twitch allows numeric-only logins (legacy).
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := IsValidTwitchLogin(tc.in)
			if got != tc.want {
				t.Errorf("IsValidTwitchLogin(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}
