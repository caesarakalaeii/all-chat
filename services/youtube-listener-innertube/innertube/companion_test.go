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

package innertube

import "testing"

func TestLooksVertical(t *testing.T) {
	// Titles taken from production discovery logs, plus their landscape twins.
	for title, want := range map[string]bool{
		"Fixing The 2nd Zoo! - Planet Zoo 2 (EP5) (Portrait)":          true,
		"Alchemy Factory! - Vertical":                                  true,
		"GTA 5: Getting Ready for The Last Ride | Vertical Live":       true,
		"🔴 LIVE - DZISIAJ JUŻ JAKOŚ FUNKCJONUJĘ...  [PION]":            true,
		"CAN I FIND THE PIRATE SHOP TOKEN 👀 #fortnite #shortslive #on": true,
		"Minecraft im Hochformat":                                      true,
		"Fixing The 2nd Zoo! - Planet Zoo 2 (EP5)":                     false,
		"Alchemy Factory!":     false,
		"Verticality speedrun": false,
		"Pionier Gameplay":     false,
		"#shorts compilation":  false,
	} {
		if got := LooksVertical(title); got != want {
			t.Errorf("LooksVertical(%q) = %v, want %v", title, got, want)
		}
	}
}

func TestSelectStreamPrefersLandscape(t *testing.T) {
	// The vertical broadcast often has more concurrent viewers (Shorts feed); "most
	// viewers" used to pick it and drop the landscape chat.
	candidates := []LiveStreamCandidate{
		{VideoID: "vert", Title: "Planet Zoo (Portrait)", ViewerCount: 40},
		{VideoID: "land", Title: "Planet Zoo", ViewerCount: 5},
	}
	for _, strategy := range []string{"", StrategyMostViewers, StrategyFirstFound, StrategyFewestViewers} {
		got, err := SelectStream(candidates, strategy, "")
		if err != nil || got.VideoID != "land" {
			t.Errorf("strategy %q picked %q (err %v), want land", strategy, got.VideoID, err)
		}
	}
	got, _ := SelectStream(candidates, StrategyTitleMatch, "portrait")
	if got.VideoID != "vert" {
		t.Errorf("an explicit title match must still reach the vertical stream, got %q", got.VideoID)
	}
	onlyVertical := candidates[:1]
	if got, _ := SelectStream(onlyVertical, "", ""); got.VideoID != "vert" {
		t.Errorf("a lone vertical stream must still be selected, got %q", got.VideoID)
	}
}

func TestSameBroadcastTitle(t *testing.T) {
	for _, tt := range []struct {
		a, b string
		want bool
	}{
		{"Proviamo Sniper Elite 4!", "Proviamo Sniper Elite 4! - Vertical", true}, // live sample 2026-11-14
		{"Planet Zoo 2 (EP5)", "Planet Zoo 2 (EP5) (Portrait)", true},
		{"Just chatting 🔴", "just chatting", true},
		{"Fortnite with viewers", "Fortnite with viewers", true},
		{"PIRATE TOKEN DEV HOUR LIVE SOON #fortnite", "PIRATE SHOP DEV HOUR SOON #fortnite", false},
		{"Lofi hip hop radio", "Synthwave radio", false},
		{"🔴", "🔴", false}, // nothing left to compare
	} {
		if got := SameBroadcastTitle(tt.a, tt.b); got != tt.want {
			t.Errorf("SameBroadcastTitle(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestSelectCompanion(t *testing.T) {
	main := LiveStreamCandidate{VideoID: "land", Title: "Planet Zoo", ViewerCount: 50}
	vertical := LiveStreamCandidate{VideoID: "vert", Title: "Planet Zoo (Portrait)", ViewerCount: 5}
	unmarked := LiveStreamCandidate{VideoID: "other", Title: "Lofi beats 24/7", ViewerCount: 900}
	twin := LiveStreamCandidate{VideoID: "twin", Title: "Planet Zoo", ViewerCount: 3}

	tests := []struct {
		name       string
		candidates []LiveStreamCandidate
		mode       string
		want       string
	}{
		{"auto merges a vertical-looking stream", []LiveStreamCandidate{main, vertical}, CompanionAuto, "vert"},
		{"auto leaves an unmarked second stream alone", []LiveStreamCandidate{main, unmarked}, CompanionAuto, ""},
		{"auto prefers the vertical one among several", []LiveStreamCandidate{main, unmarked, vertical}, CompanionAuto, "vert"},
		{"on merges an unmarked second stream", []LiveStreamCandidate{main, unmarked}, CompanionOn, "other"},
		{"on still prefers a vertical-looking stream", []LiveStreamCandidate{main, unmarked, vertical}, CompanionOn, "vert"},
		{"auto merges a same-title stream without any marker", []LiveStreamCandidate{main, unmarked, twin}, CompanionAuto, "twin"},
		{"a same-title stream beats a merely vertical-looking one", []LiveStreamCandidate{main, twin, {VideoID: "v2", Title: "Other game (Portrait)", ViewerCount: 99}}, CompanionAuto, "twin"},
		{"off never merges", []LiveStreamCandidate{main, vertical}, CompanionOff, ""},
		{"no second stream", []LiveStreamCandidate{main}, CompanionOn, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := SelectCompanion(tt.candidates, main, tt.mode)
			if tt.want == "" {
				if ok {
					t.Errorf("got companion %q, want none", got.VideoID)
				}
				return
			}
			if !ok || got.VideoID != tt.want {
				t.Errorf("got %q (ok=%v), want %q", got.VideoID, ok, tt.want)
			}
		})
	}
}
