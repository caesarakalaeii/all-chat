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

package sendall

import "testing"

func TestNormalizeText(t *testing.T) {
	cases := map[string]string{
		"  Hello   World  ": "hello world",
		"HELLO":             "hello",
		"a\tb\nc":           "a b c",
		"":                  "",
		"Already normal":    "already normal",
	}
	for in, want := range cases {
		if got := NormalizeText(in); got != want {
			t.Errorf("NormalizeText(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestKeyStableUnderNormalization(t *testing.T) {
	// The writer (auth-service) and reader (message-processor) must derive the SAME key
	// from differently-whitespaced/cased text, or the echoes never match.
	if Key("twitch", "123", "Hello  World") != Key("twitch", "123", "hello world") {
		t.Error("expected normalization to make keys equal")
	}
}

func TestKeyDistinctByDimension(t *testing.T) {
	base := Key("twitch", "123", "hi")
	if base == Key("kick", "123", "hi") {
		t.Error("platform should differentiate key")
	}
	if base == Key("twitch", "456", "hi") {
		t.Error("sender id should differentiate key")
	}
	if base == Key("twitch", "123", "bye") {
		t.Error("text should differentiate key")
	}
}

func TestPublishedKeyDistinct(t *testing.T) {
	if PublishedKey("ov1", "g1") == PublishedKey("ov2", "g1") {
		t.Error("overlay id should differentiate published key")
	}
	if PublishedKey("ov1", "g1") == PublishedKey("ov1", "g2") {
		t.Error("group id should differentiate published key")
	}
}
