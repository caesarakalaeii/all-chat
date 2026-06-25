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

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNotifyUser(t *testing.T) {
	t.Skip("Wave 0 stub - implement in Wave 2")
}

func TestExtractWSAuthToken_ReadsAccessCookie(t *testing.T) {
	req := httptest.NewRequest("GET", "/ws/overlay/abc", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: "cookie-jwt"})
	tok, echo := extractWSAuthToken(req)
	if tok != "cookie-jwt" {
		t.Errorf("token=%q want cookie-jwt", tok)
	}
	if echo != nil {
		t.Errorf("echo header should be nil for cookie path, got %v", echo)
	}
}

func TestExtractWSAuthToken_SubprotocolBeatsCookie(t *testing.T) {
	req := httptest.NewRequest("GET", "/ws/overlay/abc", nil)
	req.Header.Set("Sec-WebSocket-Protocol", "bearer.subproto-jwt")
	req.AddCookie(&http.Cookie{Name: "access_token", Value: "cookie-jwt"})
	tok, echo := extractWSAuthToken(req)
	if tok != "subproto-jwt" {
		t.Errorf("token=%q want subproto-jwt (subprotocol precedence)", tok)
	}
	if echo == nil || echo.Get("Sec-WebSocket-Protocol") != "bearer.subproto-jwt" {
		t.Errorf("echo header should be set for subprotocol path")
	}
}

func TestExtractWSAuthToken_NoTokenNoCookie(t *testing.T) {
	req := httptest.NewRequest("GET", "/ws/overlay/abc", nil)
	tok, echo := extractWSAuthToken(req)
	if tok != "" {
		t.Errorf("token=%q want empty", tok)
	}
	if echo != nil {
		t.Errorf("echo should be nil")
	}
}

// TestOriginAllowedForWS_CookieWithEmptyOriginRejected (audit I1) verifies that
// a request carrying the access_token cookie with NO Origin header is rejected.
// A browser always sends Origin on a WS handshake; a missing Origin + cookie
// signals a CSRF attempt (attacker page suppressing Origin with victim's cookie).
func TestOriginAllowedForWS_CookieWithEmptyOriginRejected(t *testing.T) {
	allowed := []string{"https://allch.at"}
	firstParty := []string{"https://allch.at"}
	req := httptest.NewRequest("GET", "/ws/overlay/abc", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: "cookie-jwt"})
	// No Origin header set.
	if originAllowedForWS(allowed, firstParty, req) {
		t.Error("want false (cookie + empty origin), got true")
	}
}

// TestOriginAllowedForWS_CookieWithAllowedOriginAccepted (audit I1) verifies
// that a browser request with the access_token cookie AND an allowed Origin
// passes the check (legitimate streamer monitor view).
func TestOriginAllowedForWS_CookieWithAllowedOriginAccepted(t *testing.T) {
	allowed := []string{"https://allch.at"}
	firstParty := []string{"https://allch.at"}
	req := httptest.NewRequest("GET", "/ws/overlay/abc", nil)
	req.Header.Set("Origin", "https://allch.at")
	req.AddCookie(&http.Cookie{Name: "access_token", Value: "cookie-jwt"})
	if !originAllowedForWS(allowed, firstParty, req) {
		t.Error("want true (cookie + allowed origin), got false")
	}
}

// TestOriginAllowedForWS_NoCookieEmptyOriginAllowed (audit I1) verifies that a
// non-browser client (no access cookie, no Origin) is allowed — e.g. OBS or a
// service authenticating via the subprotocol or ?token= query param.
func TestOriginAllowedForWS_NoCookieEmptyOriginAllowed(t *testing.T) {
	allowed := []string{"https://allch.at"}
	firstParty := []string{"https://allch.at"}
	req := httptest.NewRequest("GET", "/ws/overlay/abc", nil)
	// No cookie, no Origin.
	if !originAllowedForWS(allowed, firstParty, req) {
		t.Error("want true (no cookie + empty origin = non-browser), got false")
	}
}

// TestOriginAllowedForWS_NoCookieDisallowedOriginRejected (audit I1) verifies
// that a request without a cookie but with a disallowed Origin is rejected —
// the standard origin allowlist still applies regardless of auth path.
func TestOriginAllowedForWS_NoCookieDisallowedOriginRejected(t *testing.T) {
	allowed := []string{"https://allch.at"}
	firstParty := []string{"https://allch.at"}
	req := httptest.NewRequest("GET", "/ws/overlay/abc", nil)
	req.Header.Set("Origin", "https://evil.com")
	// No cookie.
	if originAllowedForWS(allowed, firstParty, req) {
		t.Error("want false (disallowed origin), got true")
	}
}

// TestOriginAllowedForWS_ExtensionWildcardCookieRejected guards audit #8: when a
// permissive WS allowlist contains an extension wildcard (moz-extension://*), a
// cookie-authenticated handshake from an extension origin must still be REJECTED
// because cookie auth requires a strict first-party Origin (FRONTEND_URL). The
// same extension origin WITHOUT a cookie (bearer path) is still accepted.
func TestOriginAllowedForWS_ExtensionWildcardCookieRejected(t *testing.T) {
	allowed := []string{"https://allch.at", "moz-extension://*"}
	firstParty := []string{"https://allch.at"}

	// Extension origin + access cookie → reject (the audit #8 attack).
	withCookie := httptest.NewRequest("GET", "/ws/overlay/abc", nil)
	withCookie.Header.Set("Origin", "moz-extension://deadbeef-1234")
	withCookie.AddCookie(&http.Cookie{Name: "access_token", Value: "victim-jwt"})
	if originAllowedForWS(allowed, firstParty, withCookie) {
		t.Error("want false (extension origin + cookie must not open the owner socket), got true")
	}

	// Same extension origin WITHOUT a cookie (bearer/token path) → still allowed.
	noCookie := httptest.NewRequest("GET", "/ws/overlay/abc", nil)
	noCookie.Header.Set("Origin", "moz-extension://deadbeef-1234")
	if !originAllowedForWS(allowed, firstParty, noCookie) {
		t.Error("want true (extension origin via bearer/token path), got false")
	}

	// First-party origin + cookie (monitor view) → allowed.
	monitor := httptest.NewRequest("GET", "/ws/overlay/abc", nil)
	monitor.Header.Set("Origin", "https://allch.at")
	monitor.AddCookie(&http.Cookie{Name: "access_token", Value: "owner-jwt"})
	if !originAllowedForWS(allowed, firstParty, monitor) {
		t.Error("want true (first-party monitor view + cookie), got false")
	}
}
