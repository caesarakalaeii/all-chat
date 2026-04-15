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

package middleware

import "testing"

func TestAnonymizeIP(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"IPv4 basic", "192.168.1.42", "192.168.1.0"},
		{"IPv4 already zero", "10.0.0.0", "10.0.0.0"},
		{"IPv4 high octet", "255.255.255.255", "255.255.255.0"},
		{"IPv6 full", "2001:0db8:0000:0000:0000:0000:0000:0001", "2001:db8::"},
		{"IPv6 short", "2001:db8::1", "2001:db8::"},
		{"IPv6 loopback", "::1", "::"},
		{"invalid string", "not-an-ip", "not-an-ip"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AnonymizeIP(tt.raw)
			if got != tt.want {
				t.Errorf("AnonymizeIP(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}
