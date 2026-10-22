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

package safehttp

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"
)

func TestPublic(t *testing.T) {
	for addr, want := range map[string]bool{
		"8.8.8.8":              true,
		"159.69.119.147":       true,
		"2606:4700::1111":      true,
		"127.0.0.1":            false,
		"10.43.0.10":           false, // k3s service network
		"172.16.5.4":           false,
		"192.168.1.1":          false,
		"169.254.169.254":      false, // cloud metadata
		"100.64.0.1":           false, // CGNAT
		"0.0.0.0":              false,
		"0.1.2.3":              false,
		"255.255.255.255":      false,
		"224.0.0.1":            false,
		"::1":                  false,
		"fe80::1":              false,
		"fd00::1":              false,
		"::ffff:10.0.0.1":      false, // IPv4-mapped private
		"64:ff9b::a00:1":       false, // NAT64 of 10.0.0.1
		"::ffff:93.184.216.34": true,
	} {
		if got := Public(netip.MustParseAddr(addr)); got != want {
			t.Errorf("Public(%s) = %v, want %v", addr, got, want)
		}
	}
}

func TestClientRefusesLoopback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("request reached a loopback server")
	}))
	defer srv.Close()

	_, err := Client(2 * time.Second).Get(srv.URL)
	if !errors.Is(err, ErrNotPublic) {
		t.Fatalf("Get(%s) error = %v, want ErrNotPublic", srv.URL, err)
	}
}

func TestDialContextRefusesPrivate(t *testing.T) {
	_, err := DialContext(context.Background(), "tcp", "10.0.0.1:80")
	if !errors.Is(err, ErrNotPublic) {
		t.Fatalf("DialContext error = %v, want ErrNotPublic", err)
	}
}
