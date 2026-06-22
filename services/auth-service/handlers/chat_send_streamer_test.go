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
	"errors"
	"fmt"
	"net/http"
	"testing"
)

func TestClassifyPlatformStatus(t *testing.T) {
	cases := []struct {
		status int
		want   sendErrKind
	}{
		{http.StatusUnauthorized, sendErrReauth},
		{http.StatusForbidden, sendErrMissingScope},
		{http.StatusInternalServerError, sendErrUpstream},
		{http.StatusBadGateway, sendErrUpstream},
	}
	for _, tc := range cases {
		if got := classifyPlatformStatus("twitch", tc.status, "body"); got.kind != tc.want {
			t.Errorf("status %d: kind = %q, want %q", tc.status, got.kind, tc.want)
		}
	}
}

func TestStreamerSendHTTPResponse(t *testing.T) {
	// Typed missing-scope ⇒ 403 + machine code + platform (drives the opt-in prompt).
	if status, body := streamerSendHTTPResponse("kick", &streamerSendError{kind: sendErrMissingScope, msg: "x"}); status != http.StatusForbidden || body["error"] != string(sendErrMissingScope) || body["platform"] != "kick" {
		t.Errorf("missing_scope mapping wrong: status=%d body=%v", status, body)
	}
	// Typed re-auth ⇒ 401.
	if status, _ := streamerSendHTTPResponse("twitch", &streamerSendError{kind: sendErrReauth, msg: "x"}); status != http.StatusUnauthorized {
		t.Errorf("reauth status = %d, want 401", status)
	}
	// Typed quota ⇒ 422 + quota code.
	if status, body := streamerSendHTTPResponse("youtube", &streamerSendError{kind: sendErrQuota, msg: "x"}); status != http.StatusUnprocessableEntity || body["error"] != string(sendErrQuota) {
		t.Errorf("quota mapping wrong: status=%d body=%v", status, body)
	}
	// Untyped "not live" message ⇒ classifier maps to 422.
	if status, _ := streamerSendHTTPResponse("twitch", fmt.Errorf("the streamer is not currently live")); status != http.StatusUnprocessableEntity {
		t.Errorf("offline fallback status = %d, want 422", status)
	}
	// Untyped generic failure ⇒ 502.
	if status, _ := streamerSendHTTPResponse("twitch", errors.New("kaboom")); status != http.StatusBadGateway {
		t.Errorf("generic fallback status = %d, want 502", status)
	}
}
