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

package channels

import (
	"strings"
	"testing"
	"time"
)

func TestFreshnessClause_PositiveThreshold_RendersInterval(t *testing.T) {
	r := &Repository{idleThreshold: 7 * 24 * time.Hour}

	clause := r.freshnessClause()

	if !strings.Contains(clause, "AND o.last_connected_at > NOW() - INTERVAL") {
		t.Errorf("expected freshness clause to include the SQL predicate, got %q", clause)
	}
	if !strings.Contains(clause, "604800 seconds") {
		t.Errorf("expected 7d to render as 604800 seconds, got %q", clause)
	}
}

func TestFreshnessClause_ZeroThreshold_ReturnsEmpty(t *testing.T) {
	r := &Repository{idleThreshold: 0}

	if got := r.freshnessClause(); got != "" {
		t.Errorf("zero threshold must disable filter; got %q", got)
	}
}

func TestFreshnessClause_NegativeThreshold_ReturnsEmpty(t *testing.T) {
	r := &Repository{idleThreshold: -time.Second}

	if got := r.freshnessClause(); got != "" {
		t.Errorf("negative threshold must disable filter; got %q", got)
	}
}

func TestDefaultIdleOverlayThreshold_IsSevenDays(t *testing.T) {
	if DefaultIdleOverlayThreshold != 7*24*time.Hour {
		t.Errorf("expected default threshold to be 7d, got %s", DefaultIdleOverlayThreshold)
	}
}

func TestNewRepository_StoresThreshold(t *testing.T) {
	threshold := 36 * time.Hour
	r := NewRepository(nil, threshold)

	if r.idleThreshold != threshold {
		t.Errorf("expected idleThreshold=%s, got %s", threshold, r.idleThreshold)
	}
}
