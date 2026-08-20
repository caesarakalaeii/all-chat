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

package handlers

// Pins the loopback redirect rule (ADR-0049, RFC 8252 §7.3). This validator decides
// where a device-token authorization code may be delivered, so every case in the rule
// gets an assertion rather than a spot check.
//
// TestLoopbackRedirect_LocalhostIsRejected is the single most important test in the
// device-linking feature and carries a failure message explaining why, in the style of
// services/engagement-service/handler/api_token_gate_test.go for the premium gate.

import (
	"errors"
	"strings"
	"testing"
)

func TestLoopbackRedirect_AcceptsTheOneLegitimateShape(t *testing.T) {
	cases := []struct {
		name string
		uri  string
		want string
	}{
		{
			name: "IPv4 loopback with an ephemeral port",
			uri:  "http://127.0.0.1:51234" + LoopbackPath,
			want: "http://127.0.0.1:51234" + LoopbackPath,
		},
		{
			name: "IPv6 loopback with an ephemeral port",
			uri:  "http://[::1]:51234" + LoopbackPath,
			want: "http://[::1]:51234" + LoopbackPath,
		},
		{
			// Any port, because the plugin cannot reserve one in advance and a fixed
			// port collides. RFC 8252 expects the authorization server to allow this,
			// and it is only safe because the host is pinned to a literal.
			name: "a different port is equally acceptable",
			uri:  "http://127.0.0.1:8081" + LoopbackPath,
			want: "http://127.0.0.1:8081" + LoopbackPath,
		},
		{
			name: "no port at all",
			uri:  "http://127.0.0.1" + LoopbackPath,
			want: "http://127.0.0.1" + LoopbackPath,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ValidateLoopbackRedirect(tc.uri)
			if err != nil {
				t.Fatalf("ValidateLoopbackRedirect(%q) = error %v, want it accepted", tc.uri, err)
			}
			if got != tc.want {
				t.Errorf("normalised to %q, want %q", got, tc.want)
			}
		})
	}
}

func TestLoopbackRedirect_LocalhostIsRejected(t *testing.T) {
	// THE assertion. `localhost` must never be accepted, at any port, in any casing.
	for _, uri := range []string{
		"http://localhost:1234" + LoopbackPath,
		"http://localhost" + LoopbackPath,
		"http://LOCALHOST:1234" + LoopbackPath,
		"http://localhost.:1234" + LoopbackPath,
		"http://localhost.localdomain:1234" + LoopbackPath,
		"http://subdomain.localhost:1234" + LoopbackPath,
	} {
		got, err := ValidateLoopbackRedirect(uri)
		if err == nil {
			t.Fatalf("ValidateLoopbackRedirect(%q) was ACCEPTED (returned %q), and it must never be.\n\n"+
				"`localhost` is a NAME, not an address: it is resolved through DNS, /etc/hosts, a\n"+
				"search domain or whatever resolver the machine is configured with, so it can be\n"+
				"pointed at a host that is not this machine — and the resolution can change between\n"+
				"the moment this URI is validated and the moment the browser follows it. The literal\n"+
				"addresses 127.0.0.1 and [::1] cannot be redirected that way, which is the entire\n"+
				"reason ADR-0049 pins them. What travels over this redirect is a one-time code that\n"+
				"is exchanged for a long-lived device token, so accepting `localhost` hands that code\n"+
				"to whoever controls name resolution on the streamer's machine or network.\n\n"+
				"If this test is failing because the rule was widened: it must not be. Fix the caller.",
				uri, got)
		}
		if !errors.Is(err, ErrLoopbackHost) && !errors.Is(err, ErrLoopbackScheme) {
			t.Errorf("ValidateLoopbackRedirect(%q) rejected with %v; expected the host rule to fire", uri, err)
		}
	}
}

func TestLoopbackRedirect_RejectsNonLoopbackHosts(t *testing.T) {
	// A host that merely resembles a loopback literal is still not one. The last two
	// are the classic decimal/octal encodings of 127.0.0.1 that some parsers accept.
	for _, uri := range []string{
		"http://evil.example.com" + LoopbackPath,
		"http://127.0.0.2:1234" + LoopbackPath,
		"http://127.1:1234" + LoopbackPath,
		"http://2130706433:1234" + LoopbackPath,
		"http://0177.0.0.1:1234" + LoopbackPath,
		"http://[::ffff:127.0.0.1]:1234" + LoopbackPath,
		"http://127.0.0.1.evil.example.com:1234" + LoopbackPath,
		"http://[::2]:1234" + LoopbackPath,
	} {
		if got, err := ValidateLoopbackRedirect(uri); err == nil {
			t.Errorf("ValidateLoopbackRedirect(%q) accepted a non-loopback host (returned %q)", uri, got)
		}
	}
}

func TestLoopbackRedirect_RejectsUserinfoInTheAuthority(t *testing.T) {
	// Userinfo is the oldest authority-confusion trick there is: a reader (and some
	// parsers) sees 127.0.0.1 where the real host is evil.example.com.
	for _, uri := range []string{
		"http://127.0.0.1@evil.example.com:1234" + LoopbackPath,
		"http://user@127.0.0.1:1234" + LoopbackPath,
		"http://user:pass@127.0.0.1:1234" + LoopbackPath,
	} {
		if got, err := ValidateLoopbackRedirect(uri); err == nil {
			t.Errorf("ValidateLoopbackRedirect(%q) accepted userinfo (returned %q)", uri, got)
		}
	}
}

func TestLoopbackRedirect_RejectsWrongScheme(t *testing.T) {
	for _, uri := range []string{
		"https://127.0.0.1:1234" + LoopbackPath,
		"HTTP://127.0.0.1:1234" + LoopbackPath,
		"file:///etc/passwd",
		"javascript:alert(1)",
		"//127.0.0.1:1234" + LoopbackPath,
		"127.0.0.1:1234" + LoopbackPath,
		"http:127.0.0.1:1234" + LoopbackPath,
	} {
		if got, err := ValidateLoopbackRedirect(uri); err == nil {
			t.Errorf("ValidateLoopbackRedirect(%q) accepted a non-http scheme (returned %q)", uri, got)
		}
	}
}

func TestLoopbackRedirect_RejectsQueryAndFragment(t *testing.T) {
	// We append `code` and `state` ourselves. A client-supplied query or fragment could
	// shadow either, and a fragment is invisible to the server that receives the
	// redirect anyway.
	for _, uri := range []string{
		"http://127.0.0.1:1234" + LoopbackPath + "?code=stolen",
		"http://127.0.0.1:1234" + LoopbackPath + "?",
		"http://127.0.0.1:1234" + LoopbackPath + "#frag",
		"http://127.0.0.1:1234" + LoopbackPath + "?a=b#c",
	} {
		if got, err := ValidateLoopbackRedirect(uri); err == nil {
			t.Errorf("ValidateLoopbackRedirect(%q) accepted a query or fragment (returned %q)", uri, got)
		}
	}
}

func TestLoopbackRedirect_RejectsAnyOtherPath(t *testing.T) {
	for _, uri := range []string{
		"http://127.0.0.1:1234/",
		"http://127.0.0.1:1234",
		"http://127.0.0.1:1234/callback",
		"http://127.0.0.1:1234" + LoopbackPath + "/extra",
		"http://127.0.0.1:1234" + LoopbackPath + "x",
		"http://127.0.0.1:1234" + strings.ToUpper(LoopbackPath),
	} {
		if got, err := ValidateLoopbackRedirect(uri); err == nil {
			t.Errorf("ValidateLoopbackRedirect(%q) accepted a path other than %q (returned %q)",
				uri, LoopbackPath, got)
		}
	}
}

func TestLoopbackRedirect_RejectsAnythingNeedingNormalisation(t *testing.T) {
	// The rule is "already in the one shape we accept", not "normalises to it". Each of
	// these decodes or normalises onto something acceptable, and each is refused.
	for _, uri := range []string{
		"http://127.0.0.1:1234/allchat/../allchat/device-callback",
		"http://127.0.0.1:1234//allchat/device-callback",
		"http://127.0.0.1:1234/allchat%2Fdevice-callback",
		"http://127.0.0.1:1234%2f@evil.example.com" + LoopbackPath,
		`http://127.0.0.1:1234\@evil.example.com` + LoopbackPath,
		"http://127.0.0.1:1234\n" + LoopbackPath,
		"http://127.0.0.1:1234\t" + LoopbackPath,
		" http://127.0.0.1:1234" + LoopbackPath,
		"http://127.0.0.1:1234" + LoopbackPath + " ",
		"http://%31%32%37.0.0.1:1234" + LoopbackPath,
	} {
		if got, err := ValidateLoopbackRedirect(uri); err == nil {
			t.Errorf("ValidateLoopbackRedirect(%q) accepted a value that only looks safe after "+
				"normalisation (returned %q)", uri, got)
		}
	}
}

func TestLoopbackRedirect_RejectsBadPorts(t *testing.T) {
	for _, uri := range []string{
		"http://127.0.0.1:0" + LoopbackPath,
		"http://127.0.0.1:99999" + LoopbackPath,
		"http://127.0.0.1:port" + LoopbackPath,
		"http://127.0.0.1:-1" + LoopbackPath,
		"http://127.0.0.1:" + LoopbackPath,
	} {
		if got, err := ValidateLoopbackRedirect(uri); err == nil {
			t.Errorf("ValidateLoopbackRedirect(%q) accepted an invalid port (returned %q)", uri, got)
		}
	}
}

func TestLoopbackRedirect_RejectsEmpty(t *testing.T) {
	for _, uri := range []string{"", "   "} {
		if _, err := ValidateLoopbackRedirect(uri); !errors.Is(err, ErrLoopbackEmpty) {
			t.Errorf("ValidateLoopbackRedirect(%q) = %v, want ErrLoopbackEmpty", uri, err)
		}
	}
}

func TestBuildLoopbackRedirect_AppendsCodeAndState(t *testing.T) {
	got, err := BuildLoopbackRedirect("http://127.0.0.1:51234"+LoopbackPath, "one-time-code", "plugin-state")
	if err != nil {
		t.Fatalf("BuildLoopbackRedirect: %v", err)
	}
	want := "http://127.0.0.1:51234" + LoopbackPath + "?code=one-time-code&state=plugin-state"
	if got != want {
		t.Errorf("BuildLoopbackRedirect = %q, want %q", got, want)
	}
}

func TestBuildLoopbackRedirect_RevalidatesTheStoredValue(t *testing.T) {
	// The stored value came from a validated string, so this should never fire in
	// production — but a credential delivery is not something to bet on "should never".
	if _, err := BuildLoopbackRedirect("http://localhost:1234"+LoopbackPath, "code", "state"); err == nil {
		t.Fatal("BuildLoopbackRedirect built a Location header for a host that fails the rule")
	}
}

func TestBuildLoopbackRedirect_OmitsEmptyState(t *testing.T) {
	got, err := BuildLoopbackRedirect("http://127.0.0.1:51234"+LoopbackPath, "code", "")
	if err != nil {
		t.Fatalf("BuildLoopbackRedirect: %v", err)
	}
	if strings.Contains(got, "state=") {
		t.Errorf("BuildLoopbackRedirect = %q, want no state parameter when the plugin sent none", got)
	}
}
