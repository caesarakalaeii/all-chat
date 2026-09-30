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

package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/caesar/all-chat/services/twitch-eventsub-listener/channels"
)

// wrapChatScopeError is the seam the whole re-auth hint rests on: the manager
// publishes the Auth Required offline status only when errors.Is sees
// channels.ErrChatScopesMissing. If this wrap regressed to the pre-incident
// `return err` shape, every scope-failed channel would publish a bare offline
// status and the streamer would never be told to re-auth — the incident's own
// symptom. These tests pin both ends of the contract.

func TestWrapChatScopeError_CarriesSentinel(t *testing.T) {
	err := wrapChatScopeError("1532957417")
	if !errors.Is(err, channels.ErrChatScopesMissing) {
		t.Fatalf("wrapped error does not match channels.ErrChatScopesMissing: %v", err)
	}
	if !strings.Contains(err.Error(), "1532957417") {
		t.Fatalf("wrapped error does not name the broadcaster: %v", err)
	}
}

// isScopeError is the only predicate standing between a Twitch 403 and the
// sentinel. It string-matches the two shapes Twitch actually returns for a
// missing chat grant; a Twitch 5xx/429 must NOT match, or a platform outage
// would tell every streamer to re-auth.
func TestIsScopeError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"twitch 403 with missing authorization phrase", errors.New("subscription failed with status 403: missing proper authorization"), true},
		{"bare 403", errors.New("subscription failed with status 403"), true},
		{"twitch 5xx", errors.New("subscription failed with status 503"), false},
		{"rate limit", errors.New("subscription failed with status 429"), false},
		{"network error", errors.New("connection reset by peer"), false},
	}
	for _, tc := range cases {
		if got := isScopeError(tc.err); got != tc.want {
			t.Errorf("%s: isScopeError = %v, want %v", tc.name, got, tc.want)
		}
	}
}
