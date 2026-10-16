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

// An unseeded key fails closed on both dimensions, so a typo between the constant and the
// migration would keep the language picker beta-only even after the admin graduates the row.
// The seeded values are pinned too: is_premium=TRUE would put a language behind a paywall.
func TestUILanguagesGateSeed(t *testing.T) {
	raw, err := os.ReadFile("../../migrations/103_ui_languages_gate.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	sql := regexp.MustCompile(`(?m)--.*$`).ReplaceAllString(string(raw), "")

	rows := regexp.MustCompile(`(?is)INSERT\s+INTO\s+feature_gates\s*\(\s*feature_key\s*,\s*is_premium\s*,\s*early_access\s*,\s*description\s*\)\s*VALUES\s*\(\s*'([^']+)'\s*,\s*(TRUE|FALSE)\s*,\s*(TRUE|FALSE)\s*,`).FindAllStringSubmatch(sql, -1)
	if len(rows) != 1 {
		t.Fatalf("want exactly one feature_gates seed row, got %d", len(rows))
	}
	if got := rows[0][1]; got != GateUILanguages {
		t.Fatalf("seeded key = %q, want %q", got, GateUILanguages)
	}
	if got := rows[0][2]; !strings.EqualFold(got, "FALSE") {
		t.Fatalf("seeded is_premium = %s, want FALSE", got)
	}
	if got := rows[0][3]; !strings.EqualFold(got, "TRUE") {
		t.Fatalf("seeded early_access = %s, want TRUE", got)
	}
	if !regexp.MustCompile(`(?i)ON\s+CONFLICT\s*\(\s*feature_key\s*\)\s*DO\s+NOTHING`).MatchString(sql) {
		t.Fatal("seed must be idempotent (ON CONFLICT (feature_key) DO NOTHING): the runner re-applies every migration on pod start")
	}
}
