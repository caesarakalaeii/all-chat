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
	"context"
	"crypto/sha256"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/caesar/all-chat/shared/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
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

// TestTTLIntervalsAreNotStringConcatenated is the cheap half of the guard against the
// bug that took device linking down completely: every POST /device/link/start answered
// 500 with
//
//	failed to encode args[7]: unable to encode 600 into text format for text (OID 25):
//	cannot find encode plan
//
// `NOW() + ($8 || ' seconds')::INTERVAL` looks harmless, but `||` has only a text
// overload, so PostgreSQL infers $8 as text and pgx v5 will not encode an int64 into a
// text parameter. The working form multiplies instead: `NOW() + $8 * INTERVAL '1 second'`
// takes the numeric straight through.
//
// This runs without a database, which is the point — the DB-backed test below skips when
// Docker is absent, and this class of break must not depend on Docker being present to
// be noticed.
func TestTTLIntervalsAreNotStringConcatenated(t *testing.T) {
	raw, err := os.ReadFile("device_token_repository.go")
	if err != nil {
		t.Fatalf("failed to read the repository source: %v", err)
	}
	// Comments are stripped first, because the statements above are commented with the
	// broken form as a warning — a naive scan would flag the documentation as the bug.
	var code strings.Builder
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "//") {
			continue
		}
		code.WriteString(line)
		code.WriteString("\n")
	}
	source := code.String()

	if strings.Contains(source, "|| ' seconds')") {
		t.Error("a TTL is being concatenated into an INTERVAL again. `($n || ' seconds')::INTERVAL` " +
			"types the parameter as text, and pgx v5 cannot encode an int64 into text — every call " +
			"fails with a 500 (\"cannot find encode plan\"). Use `$n * INTERVAL '1 second'`.")
	}
	// Three statements take a TTL parameter — CreateLinkRequest, ApproveLinkRequest and
	// CreateDeviceToken — and all three must use the multiplied form. Counted rather than
	// matched literally so reindenting a query does not fail the test for no reason.
	if got := strings.Count(source, "* INTERVAL '1 second'"); got != 3 {
		t.Errorf("found %d multiplied TTL intervals, want 3 (CreateLinkRequest, "+
			"ApproveLinkRequest, CreateDeviceToken). If a statement was added or removed, "+
			"update this count — but every TTL parameter must still be multiplied, not "+
			"concatenated.", got)
	}
}

// TestDeviceLinkTTLsExecuteAgainstRealPostgres is the half that would actually have
// caught the outage. Everything else in this file, and every test in
// handlers/device_link_test.go, asserts against strings or a fake store — so all of them
// stayed green while all three statements were unexecutable. A parameter-type mismatch
// only exists once PostgreSQL is asked to plan the query, which means the only test that
// can see it is one that runs it.
//
// It drives the real repository against the real migration set: open a link request,
// approve it, mint the token. Each step asserts the expiry actually landed the configured
// TTL in the future, because "the statement ran" and "the interval was computed from the
// argument" are different claims and only the second one is useful.
func TestDeviceLinkTTLsExecuteAgainstRealPostgres(t *testing.T) {
	pool, cleanup := setupMigrationTestDB(t)
	defer cleanup()
	runMigrations(t, pool, loadUpMigrations(t))

	ctx := context.Background()

	// The two FK targets a device token needs: an owner and an overlay of theirs.
	var userID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO users (twitch_id, auth_provider, username, display_name,
		                   access_token, refresh_token, token_expires_at)
		VALUES ('990001', 'twitch', 'interval_canary', 'Interval Canary',
		        'access-token', 'refresh-token', NOW() + INTERVAL '4 hours')
		RETURNING id::text`).Scan(&userID); err != nil {
		t.Fatalf("failed to insert the owning user: %v", err)
	}
	var overlayID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO overlays (user_id, name) VALUES ($1, 'canary overlay')
		RETURNING id::text`, userID).Scan(&overlayID); err != nil {
		t.Fatalf("failed to insert the overlay: %v", err)
	}

	repo := NewDeviceTokenRepository(pool)

	// 1. CreateLinkRequest — the statement behind POST /device/link/start, the one the
	//    bug report was filed against.
	req, err := repo.CreateLinkRequest(ctx, FlowLoopback, nil,
		"a-pkce-challenge", "S256", "http://127.0.0.1:41234/callback",
		"Stream Deck", []string{"chat:write", "engagement:write"}, LinkRequestTTL)
	if err != nil {
		t.Fatalf("CreateLinkRequest failed against a real PostgreSQL: %v\n\n"+
			"This is the exact failure users saw as a stuck \"Starting…\" and a 500.", err)
	}
	assertTTLLanded(t, pool, "CreateLinkRequest", LinkRequestTTL,
		`SELECT EXTRACT(EPOCH FROM
		          ((expires_at AT TIME ZONE current_setting('TimeZone')) - NOW()))
		   FROM device_link_requests WHERE id = $1`, req.ID)

	// 2. ApproveLinkRequest — the streamer pressing Approve.
	authCode := sha256.Sum256([]byte("an-auth-code"))
	approved, err := repo.ApproveLinkRequest(ctx, req.ID, userID, overlayID,
		[]string{"chat:write"}, "Stream Deck", authCode[:], AuthCodeTTL)
	if err != nil {
		t.Fatalf("ApproveLinkRequest failed against a real PostgreSQL: %v", err)
	}
	assertTTLLanded(t, pool, "ApproveLinkRequest", AuthCodeTTL,
		`SELECT EXTRACT(EPOCH FROM
		          ((auth_code_expires_at AT TIME ZONE current_setting('TimeZone')) - NOW()))
		   FROM device_link_requests WHERE id = $1`, approved.ID)

	// 3. CreateDeviceToken — minting the credential the plugin ends up holding, with the
	//    lifetime the production caller passes.
	tokenHash := sha256.Sum256([]byte("a-device-token"))
	device, err := repo.CreateDeviceToken(ctx, userID, overlayID, "Stream Deck",
		tokenHash[:], []string{"chat:write"}, middleware.DeviceTokenLifetime, req.ID)
	if err != nil {
		t.Fatalf("CreateDeviceToken failed against a real PostgreSQL: %v", err)
	}
	assertTTLLanded(t, pool, "CreateDeviceToken", middleware.DeviceTokenLifetime,
		`SELECT EXTRACT(EPOCH FROM
		          ((expires_at AT TIME ZONE current_setting('TimeZone')) - NOW()))
		   FROM device_tokens WHERE id = $1`, device.ID)
}

// assertTTLLanded checks an expiry landed one TTL in the future rather than merely being
// set: a statement that silently computed a zero interval would still have "worked".
//
// The remaining time is measured BY POSTGRESQL, not by comparing a scanned timestamp
// against Go's clock. These columns are `timestamp without time zone` while NOW() is a
// timestamptz, so on a server outside UTC the stored value is local wall-clock and pgx
// hands it back as though it were UTC. Comparing that against Go's clock reports a
// whole-hour error while the interval itself is perfectly correct, and a TTL spanning a
// DST change (the 90-day one does) is off by the DST offset on top. Converting the column
// back to an absolute instant with `AT TIME ZONE current_setting('TimeZone')` makes the
// assertion exact on any server timezone rather than only on a UTC one.
func assertTTLLanded(t *testing.T, pool *pgxpool.Pool, what string, ttl time.Duration, query, id string) {
	t.Helper()
	var seconds float64
	if err := pool.QueryRow(context.Background(), query, id).Scan(&seconds); err != nil {
		t.Fatalf("%s: failed to read back the expiry: %v", what, err)
	}
	got := time.Duration(seconds * float64(time.Second))
	// Generous slack: the assertion is that the interval was derived from the argument at
	// all, not that it is accurate to the millisecond.
	const slack = 2 * time.Minute
	if got < ttl-slack || got > ttl+slack {
		t.Errorf("%s: expiry is %v away, want ~%v. The TTL argument is not reaching the "+
			"interval.", what, got.Round(time.Second), ttl)
	}
}
