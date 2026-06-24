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
