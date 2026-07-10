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

package access

import "testing"

func TestModeFor(t *testing.T) {
	p := NewPolicy([]string{"111", "222", "", "  "})
	cases := []struct {
		uid  string
		want Mode
	}{
		{"111", ModeAdmin},
		{"222", ModeAdmin},
		{"333", ModeSupport},
		{"", ModeSupport},
		{"  ", ModeSupport},
	}
	for _, c := range cases {
		if got := p.ModeFor(c.uid); got != c.want {
			t.Errorf("ModeFor(%q) = %v, want %v", c.uid, got, c.want)
		}
	}
}

func TestZeroValueIsSupport(t *testing.T) {
	var m Mode // zero value
	if m != ModeSupport {
		t.Fatalf("zero-value Mode = %v, want ModeSupport (fail-closed default)", m)
	}
	if m == ModeAdmin {
		t.Fatal("zero-value Mode must never equal ModeAdmin")
	}
}

func TestBlankAdminNotAdmin(t *testing.T) {
	// A blank entry must never be treated as admin.
	p := NewPolicy([]string{""})
	if p.IsAdmin("") {
		t.Fatal("empty UID must not be admin")
	}
	if p.AdminCount() != 0 {
		t.Fatalf("AdminCount = %d, want 0 (blank dropped)", p.AdminCount())
	}
}
