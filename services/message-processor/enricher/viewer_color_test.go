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

package enricher

import (
	"regexp"
	"strconv"
	"testing"
)

var hexColorRe = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

func TestAutoColor_Deterministic(t *testing.T) {
	const key = "twitch:12345"
	first := AutoColor(key)
	for i := 0; i < 100; i++ {
		if got := AutoColor(key); got != first {
			t.Fatalf("AutoColor(%q) not deterministic: iteration %d returned %q, want %q", key, i, got, first)
		}
	}
}

func TestAutoColor_ReturnsPaletteColor(t *testing.T) {
	inPalette := make(map[string]bool, len(autoColorPalette))
	for _, c := range autoColorPalette {
		inPalette[c] = true
	}
	for _, k := range []string{"", "twitch:1", "youtube:abc", "uuid-1", "kick:999", "tiktok:xyz", "discord:42"} {
		if got := AutoColor(k); !inPalette[got] {
			t.Errorf("AutoColor(%q) = %q, not in palette", k, got)
		}
	}
}

func TestAutoColorPalette_Valid(t *testing.T) {
	if len(autoColorPalette) != 16 {
		t.Errorf("expected a 16-color palette, got %d", len(autoColorPalette))
	}
	seen := make(map[string]bool, len(autoColorPalette))
	for i, c := range autoColorPalette {
		if !hexColorRe.MatchString(c) {
			t.Errorf("palette[%d] = %q is not a valid #RRGGBB hex color", i, c)
		}
		if seen[c] {
			t.Errorf("palette[%d] = %q is a duplicate", i, c)
		}
		seen[c] = true
	}
}

func TestAutoColor_DistributesAcrossPalette(t *testing.T) {
	// Hashing many distinct keys should exercise most of the palette. With 1000
	// keys spread over 16 buckets, hitting at least half is overwhelmingly likely
	// for a sane hash; this guards against a degenerate mapping (e.g. always [0]).
	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		seen[AutoColor("viewer-"+strconv.Itoa(i))] = true
	}
	if len(seen) < len(autoColorPalette)/2 {
		t.Errorf("poor distribution: only %d of %d palette colors used", len(seen), len(autoColorPalette))
	}
}
