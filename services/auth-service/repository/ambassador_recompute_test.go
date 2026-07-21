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

import (
	"context"
	"testing"

	"github.com/caesar/all-chat/shared/premium"
)

// TestAmbassadorFoldsIntoPremium proves the ADR-0041 entitlement composition end to
// end against the real migration set: an ambassador is premium, an admin force-deny
// still wins, revoking the role drops premium, and the early-access SQL treats an
// ambassador (even a non-beta one) as eligible.
func TestAmbassadorFoldsIntoPremium(t *testing.T) {
	pool, cleanup := setupMigrationTestDB(t)
	defer cleanup()
	runMigrations(t, pool, loadUpMigrations(t))
	ctx := context.Background()

	var id string
	err := pool.QueryRow(ctx, `
		INSERT INTO users (twitch_id, auth_provider, username, display_name,
		                   access_token, refresh_token, token_expires_at, granted_scopes)
		VALUES ('900001', 'twitch', 'amb_canary', 'Ambassador Canary',
		        'access-token', 'refresh-token', NOW() + INTERVAL '4 hours', ARRAY[]::text[])
		RETURNING id`).Scan(&id)
	if err != nil {
		t.Fatalf("failed to insert ambassador canary: %v", err)
	}

	rec := premium.NewRecomputer(pool, nil)

	assertPremiumRecompute := func(when string, want bool) {
		t.Helper()
		got, err := rec.Recompute(ctx, id)
		if err != nil {
			t.Fatalf("%s: Recompute error: %v", when, err)
		}
		if got != want {
			t.Fatalf("%s: Recompute = %v, want %v", when, got, want)
		}
		var col bool
		if err := pool.QueryRow(ctx, `SELECT is_premium FROM users WHERE id = $1`, id).Scan(&col); err != nil {
			t.Fatalf("%s: read is_premium: %v", when, err)
		}
		if col != want {
			t.Errorf("%s: materialized is_premium = %v, want %v", when, col, want)
		}
	}
	mustExec := func(sql string) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, id); err != nil {
			t.Fatalf("exec %q: %v", sql, err)
		}
	}

	// Baseline: no override, no subscription, not beta, not ambassador => not premium.
	assertPremiumRecompute("baseline", false)

	// Grant ambassador => premium (folded into is_premium like beta-tester).
	mustExec(`UPDATE users SET is_ambassador = TRUE WHERE id = $1`)
	assertPremiumRecompute("after grant", true)

	// Admin force-deny (override FALSE) still wins over the ambassador grant.
	mustExec(`UPDATE users SET premium_admin_override = FALSE WHERE id = $1`)
	assertPremiumRecompute("force-deny", false)

	// Clear the override and revoke the role => back to not premium.
	mustExec(`UPDATE users SET premium_admin_override = NULL, is_ambassador = FALSE WHERE id = $1`)
	assertPremiumRecompute("after revoke", false)

	// Early-access eligibility (shared/middleware.RequireEarlyAccess SQL): an
	// ambassador who is NOT a beta tester is still eligible.
	mustExec(`UPDATE users SET is_ambassador = TRUE, is_beta_tester = FALSE WHERE id = $1`)
	var eligible bool
	if err := pool.QueryRow(ctx,
		`SELECT (is_beta_tester OR is_ambassador) FROM users WHERE id = $1`, id).Scan(&eligible); err != nil {
		t.Fatalf("early-access query failed: %v", err)
	}
	if !eligible {
		t.Errorf("ambassador (non-beta) should be early-access eligible, got false")
	}
}
