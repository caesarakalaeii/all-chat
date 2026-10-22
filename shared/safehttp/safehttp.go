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

// Package safehttp connects to user-chosen URLs without reaching anything that is
// not on the public internet (ADR-0080). The cluster leaves egress open, so this is
// the control that keeps a user's URL away from pods, services, the nodes' private
// networks and the cloud metadata endpoint.
//
// The check runs on the IP each connection actually dials, after DNS resolution, so
// a hostname that resolves (or re-resolves, DNS rebinding) to a private address is
// refused, and so is every redirect hop.
package safehttp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"syscall"
	"time"
)

// ErrNotPublic is returned for a connection to a non-public address.
var ErrNotPublic = errors.New("destination is not a public internet address")

// nonPublic are the non-public ranges netip's predicates do not cover.
var nonPublic = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),     // "this network"
	netip.MustParsePrefix("100.64.0.0/10"), // carrier-grade NAT
	netip.MustParsePrefix("192.0.0.0/24"),  // IETF protocol assignments
	netip.MustParsePrefix("198.18.0.0/15"), // benchmarking
	netip.MustParsePrefix("64:ff9b::/96"),  // NAT64, which can map to a private IPv4
}

// Public reports whether ip is a public unicast address.
func Public(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsGlobalUnicast() || ip.IsPrivate() {
		return false
	}
	for _, p := range nonPublic {
		if p.Contains(ip) {
			return false
		}
	}
	return true
}

var dialer = &net.Dialer{
	Timeout: 5 * time.Second,
	Control: func(_, address string, _ syscall.RawConn) error {
		ap, err := netip.ParseAddrPort(address)
		if err != nil {
			return err
		}
		if !Public(ap.Addr()) {
			return fmt.Errorf("%w: %s", ErrNotPublic, ap.Addr())
		}
		return nil
	},
}

// DialContext dials like net.Dialer but refuses non-public addresses. Use it as a
// websocket dialer's NetDialContext.
func DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return dialer.DialContext(ctx, network, address)
}

// maxRedirects bounds a redirect chain; each hop is dialed through DialContext.
const maxRedirects = 3

// Client returns an http.Client for user-chosen URLs. It never uses a proxy: through
// a proxy the dial check would see the proxy's address instead of the target's.
func Client(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy:           nil,
			DialContext:     DialContext,
			IdleConnTimeout: 30 * time.Second, // idle pools to arbitrary hosts do not linger
		},
		CheckRedirect: func(_ *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return fmt.Errorf("stopped after %d redirects", maxRedirects)
			}
			return nil
		},
	}
}
