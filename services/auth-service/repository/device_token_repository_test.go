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

package repository

// The device-link state machine lives in SQL, deliberately: "claim this if it is still
// unclaimed" has to be one atomic statement or a replay wins the race. That makes the
// statements themselves the thing worth asserting on, and it makes them assertable
// WITHOUT a database — which matters, because the repository tests that do need one
// skip when Docker is absent, and the properties below are exactly the ones nobody
// wants silently unchecked.
//
// These are shape assertions, not behaviour: they catch the class of edit that quietly
// removes a guard clause. The behaviour is covered by the handler tests in
// handlers/device_link_test.go, which drive the whole flow over a fake store.

import (
	"strings"
	"testing"
)

func TestConsumeAuthCodeSQL_IsOneAtomicStatementWithEveryGuard(t *testing.T) {
	// Read the statement out of the function rather than duplicating it: the point is
	// to pin what the production query does.
	//
	// The guards, and why each one has to be in the WHERE clause rather than a
	// preceding SELECT:
	//
	//   consumed_at IS NULL      makes the claim single-use. Read separately, two
	//                            concurrent exchanges would both see NULL and both win.
	//   approved_at IS NOT NULL  is what "the streamer said yes" means. Without it an
	//                            unapproved request could be exchanged for a token.
	//   the digest match         is the authentication. It is a digest comparison in
	//                            SQL because the plaintext is never stored.
	//   the expiry               bounds the window, per flow.
	sql := consumeAuthCodeSQL
	for _, guard := range []string{
		"consumed_at IS NULL",
		"approved_at IS NOT NULL",
		"auth_code_hash = $2",
		"user_code_hash = $2",
		"auth_code_expires_at > NOW()",
		"expires_at > NOW()",
	} {
		if !strings.Contains(sql, guard) {
			t.Errorf("the claim statement no longer contains the guard %q. Every one of "+
				"these belongs in the WHERE clause: a guard evaluated in a preceding SELECT "+
				"lets two concurrent exchanges both observe an unconsumed row and both mint "+
				"a token, which is precisely the replay this feature must not have.", guard)
		}
	}
	// One statement, not two. An UPDATE that RETURNs is what makes the guard and the
	// effect indivisible.
	if !strings.Contains(sql, "UPDATE device_link_requests") || !strings.Contains(sql, "RETURNING") {
		t.Fatal("the claim is no longer a single UPDATE ... RETURNING")
	}
	// The auth code is cleared by the claim, so a loopback code cannot be presented
	// twice even against a row that was somehow approved twice.
	if !strings.Contains(sql, "auth_code_hash = NULL") {
		t.Error("the claim no longer clears auth_code_hash; the loopback code stops being one-time")
	}
	// Both flows are named, which is what keeps the two delivery paths on ONE state
	// machine: the loopback path claims with the authorization code's digest, the code
	// path with the pairing code's, and neither can claim a row belonging to the other.
	if !strings.Contains(sql, "flow = $3") || !strings.Contains(sql, "flow = $4") {
		t.Error("the claim no longer distinguishes the two flows; a pairing code could " +
			"claim a loopback row, or vice versa")
	}
}

func TestBruteForceBoundIsFiveAttemptsAndCheckedInSQL(t *testing.T) {
	// ADR-0049 flags the pairing-code brute-force bound as a security-review item, so
	// the number and the mechanism are both pinned. The gateway rate limit is defence
	// in depth; THIS is the bound.
	if MaxLinkAttempts != 5 {
		t.Errorf("MaxLinkAttempts = %d, want 5 (ADR-0049's stated bound)", MaxLinkAttempts)
	}
	sql := registerFailedAttemptSQL
	if !strings.Contains(sql, "attempts = attempts + 1") {
		t.Error("the attempt counter is no longer incremented in SQL. Read-then-write lets " +
			"two concurrent guesses both observe the same count, which is not a bound.")
	}
	if !strings.Contains(sql, "RETURNING attempts") {
		t.Error("the incremented count is no longer returned, so the caller cannot tell " +
			"when a request has exhausted its attempts")
	}
}

func TestLinkRequestTTLsAreShort(t *testing.T) {
	// A pending link is a thing an attacker would like to be long-lived. Ten minutes is
	// enough for a streamer to find the dashboard on a second machine; the one-time
	// code gets a tighter window still, because the plugin collects it in milliseconds.
	if LinkRequestTTL.Minutes() != 10 {
		t.Errorf("LinkRequestTTL = %v, want 10m", LinkRequestTTL)
	}
	if AuthCodeTTL.Minutes() > 5 {
		t.Errorf("AuthCodeTTL = %v, want <= 5m (ADR-0049)", AuthCodeTTL)
	}
	if AuthCodeTTL >= LinkRequestTTL {
		t.Error("the one-time code must not outlive the request it belongs to")
	}
}

func TestNoProjectionSelectsADigest(t *testing.T) {
	// A digest that never leaves this file cannot be serialised into a response by
	// accident. Same reasoning as api_token_repository.go's apiTokenColumns comment.
	for name, columns := range map[string]string{
		"deviceTokenColumns": deviceTokenColumns,
		"linkRequestColumns": linkRequestColumns,
	} {
		for _, secret := range []string{"token_hash", "user_code_hash", "auth_code_hash"} {
			if strings.Contains(columns, secret) {
				t.Errorf("%s selects %s. No read projection may carry a digest: the struct it "+
					"scans into is what the endpoints serialise.", name, secret)
			}
		}
	}
}
