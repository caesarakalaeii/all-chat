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

package featuregates

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// The Data API listener and overlay-manager both resolve GateYouTubeOfficialAPI; an unseeded
// key reads as premium (fail closed), so a typo between constant and migration would silently
// lock the feature even after the admin flips the row open.
func TestYouTubeOfficialAPIGateSeed(t *testing.T) {
	raw, err := os.ReadFile("../../migrations/102_youtube_official_api_gate.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	sql := regexp.MustCompile(`(?m)--.*$`).ReplaceAllString(string(raw), "")

	rows := regexp.MustCompile(`(?is)INSERT\s+INTO\s+feature_gates\s*\(\s*feature_key\s*,\s*is_premium\s*,\s*description\s*\)\s*VALUES\s*\(\s*'([^']+)'\s*,\s*(TRUE|FALSE)\s*,`).FindAllStringSubmatch(sql, -1)
	if len(rows) != 1 {
		t.Fatalf("want exactly one feature_gates seed row, got %d", len(rows))
	}
	if got := rows[0][1]; got != GateYouTubeOfficialAPI {
		t.Fatalf("seeded key = %q, want %q", got, GateYouTubeOfficialAPI)
	}
	if got := rows[0][2]; !strings.EqualFold(got, "TRUE") {
		t.Fatalf("seeded is_premium = %s, want TRUE", got)
	}
	if !regexp.MustCompile(`(?i)ON\s+CONFLICT\s*\(\s*feature_key\s*\)\s*DO\s+NOTHING`).MatchString(sql) {
		t.Fatal("seed must be idempotent (ON CONFLICT (feature_key) DO NOTHING): the runner re-applies every migration on pod start")
	}
}
