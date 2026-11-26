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

package config

import (
	"slices"
	"testing"
)

// STICKY_CHANNEL_IDS is hand-edited in a Kubernetes manifest, so stray spaces
// and trailing commas have to parse into a clean list.
func TestLoadParsesStickyChannelIDs(t *testing.T) {
	cases := map[string]struct {
		raw  string
		want []string
	}{
		"empty":            {"", nil},
		"only separators":  {" , , ", nil},
		"single":           {"123", []string{"123"}},
		"spaced and empty": {" 123 , ,456,", []string{"123", "456"}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Setenv("STICKY_CHANNEL_IDS", tc.raw)
			got := Load().StickyChannelIDs
			if !slices.Equal(got, tc.want) {
				t.Fatalf("StickyChannelIDs = %#v, want %#v", got, tc.want)
			}
		})
	}
}

// The sticky text is taken verbatim; only surrounding whitespace from the YAML
// is dropped, so the approved wording is posted as written.
func TestLoadKeepsStickyMessageVerbatim(t *testing.T) {
	t.Setenv("STICKY_MESSAGE", "  🏖️ ping {bot} for help  ")
	if got, want := Load().StickyMessage, "🏖️ ping {bot} for help"; got != want {
		t.Fatalf("StickyMessage = %q, want %q", got, want)
	}
}

// Both vars default to empty, which is how the feature stays off.
func TestLoadDefaultsStickySettingsToEmpty(t *testing.T) {
	cfg := Load()
	if len(cfg.StickyChannelIDs) != 0 || cfg.StickyMessage != "" {
		t.Fatalf("sticky defaults non-empty: ids=%#v message=%q", cfg.StickyChannelIDs, cfg.StickyMessage)
	}
}
