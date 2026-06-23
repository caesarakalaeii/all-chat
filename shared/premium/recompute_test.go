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

package premium

import "testing"

func boolPtr(b bool) *bool { return &b }

// TestEffective is the exhaustive truth table for the entitlement rule:
// is_premium = (override IS TRUE) OR (override IS NULL AND hasActiveSub).
func TestEffective(t *testing.T) {
	cases := []struct {
		name     string
		override *bool
		active   bool
		want     bool
	}{
		// No admin opinion: premium follows the subscription.
		{"nil override, active sub", nil, true, true},
		{"nil override, no sub", nil, false, false},
		// Force-grant (comp/staff): premium regardless of subscription.
		{"force-grant, active sub", boolPtr(true), true, true},
		{"force-grant, no sub", boolPtr(true), false, true},
		// Force-deny (reserved): never premium, even when paying.
		{"force-deny, active sub", boolPtr(false), true, false},
		{"force-deny, no sub", boolPtr(false), false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Effective(tc.override, tc.active); got != tc.want {
				t.Fatalf("Effective(%v, %v) = %v, want %v", tc.override, tc.active, got, tc.want)
			}
		})
	}
}
