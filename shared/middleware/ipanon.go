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

import "net"

// AnonymizeIP truncates the last octet of an IPv4 address or the last 80 bits
// of an IPv6 address. This preserves enough information for abuse detection and
// geographic attribution while satisfying DSGVO data-minimisation requirements.
//
// Examples:
//
//	"192.168.1.42"   → "192.168.1.0"
//	"2001:db8::1"    → "2001:db8::"
//	"invalid"        → "invalid"
func AnonymizeIP(raw string) string {
	ip := net.ParseIP(raw)
	if ip == nil {
		return raw
	}

	if v4 := ip.To4(); v4 != nil {
		v4[3] = 0
		return v4.String()
	}

	// IPv6: zero out the last 80 bits (bytes 6–15), keeping a /48 prefix.
	v6 := ip.To16()
	for i := 6; i < 16; i++ {
		v6[i] = 0
	}
	return v6.String()
}
