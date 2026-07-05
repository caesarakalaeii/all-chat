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

package integrationtest

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/caesar/all-chat/services/payment-service/reconcile"
	"github.com/caesar/all-chat/shared/premium"
)

// TestRecomputeHonorsOverrideExpiry validates the ADR-0027 expiry semantics against
// the real recompute SQL: a future-dated override is honored, an expired one is
// ignored (premium falls through to the subscription), and a permanent (NULL-expiry)
// override behaves exactly as before.
func TestRecomputeHonorsOverrideExpiry(t *testing.T) {
	if testing.Short() {
		t.Skip("requires Docker")
	}
	pool, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()
	rc := premium.NewRecomputer(pool, zap.NewNop())

	// (a) Force-grant with a FUTURE expiry, no subscription -> premium (still active).
	future := insertUser(t, pool, "exp-future")
	setUserOverride(t, pool, future, true, "NOW() + INTERVAL '1 hour'")
	got, err := rc.Recompute(ctx, future)
	require.NoError(t, err)
	assert.True(t, got, "an unexpired time-limited grant is premium")
	assert.True(t, dbPremium(t, pool, future))

	// (b) Force-grant with a PAST expiry, no subscription -> not premium (lapsed).
	past := insertUser(t, pool, "exp-past")
	setUserOverride(t, pool, past, true, "NOW() - INTERVAL '1 hour'")
	got, err = rc.Recompute(ctx, past)
	require.NoError(t, err)
	assert.False(t, got, "an expired grant falls through to the (absent) subscription")
	assert.False(t, dbPremium(t, pool, past))

	// (c) Force-grant with a PAST expiry but an ACTIVE subscription -> premium via sub.
	pastSub := insertUser(t, pool, "exp-past-sub")
	setUserOverride(t, pool, pastSub, true, "NOW() - INTERVAL '1 hour'")
	_, err = pool.Exec(ctx,
		"INSERT INTO premium_subscriptions (user_id, provider, provider_user_id, status, cents) VALUES ($1,'patreon',$2,'active',500)",
		pastSub, "pu-"+pastSub)
	require.NoError(t, err)
	got, err = rc.Recompute(ctx, pastSub)
	require.NoError(t, err)
	assert.True(t, got, "an expired override defers to an active subscription")

	// (d) Permanent force-grant (NULL expiry), no subscription -> premium (unchanged).
	perm := insertUser(t, pool, "exp-perm")
	setUserOverride(t, pool, perm, true, "NULL")
	got, err = rc.Recompute(ctx, perm)
	require.NoError(t, err)
	assert.True(t, got, "a permanent grant (no expiry) is premium")
}

// TestExpireUserOverrideIfDue validates the atomic guarded clear: a lapsed override is
// cleared and is_premium recomputed, while a still-valid (future) override is left
// untouched — the guard that prevents clobbering a fresh re-grant.
func TestExpireUserOverrideIfDue(t *testing.T) {
	if testing.Short() {
		t.Skip("requires Docker")
	}
	pool, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()
	rc := premium.NewRecomputer(pool, zap.NewNop())

	// A lapsed grant whose materialized is_premium is still stale-TRUE (no other write
	// since it expired) — exactly what the sweep must converge.
	lapsed := insertUser(t, pool, "due-lapsed")
	setStaleExpiredUser(t, pool, lapsed)
	assert.True(t, dbPremium(t, pool, lapsed), "precondition: stale premium before sweep")

	didExpire, err := rc.ExpireUserOverrideIfDue(ctx, lapsed)
	require.NoError(t, err)
	assert.True(t, didExpire, "a due override should be expired")
	assert.False(t, dbPremium(t, pool, lapsed), "is_premium recomputed to false after expiry")
	ov, exp := dbUserOverride(t, pool, lapsed)
	assert.Nil(t, ov, "override cleared")
	assert.Nil(t, exp, "expiry cleared")

	// A future-dated (valid) grant must NOT be touched.
	valid := insertUser(t, pool, "due-valid")
	setUserOverride(t, pool, valid, true, "NOW() + INTERVAL '1 hour'")
	_, err = rc.Recompute(ctx, valid)
	require.NoError(t, err)

	didExpire, err = rc.ExpireUserOverrideIfDue(ctx, valid)
	require.NoError(t, err)
	assert.False(t, didExpire, "a still-valid grant is not expired")
	assert.True(t, dbPremium(t, pool, valid), "valid grant stays premium")
	ov, _ = dbUserOverride(t, pool, valid)
	require.NotNil(t, ov)
	assert.True(t, *ov, "valid override preserved")
}

// TestOverrideExpirySweeper is the end-to-end backstop check: the sweeper flips every
// lapsed user and viewer grant to not-premium and clears its columns, while leaving a
// still-valid grant alone.
func TestOverrideExpirySweeper(t *testing.T) {
	if testing.Short() {
		t.Skip("requires Docker")
	}
	pool, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()

	// Two lapsed subjects (stale-premium) and one still-valid streamer grant.
	lapsedUser := insertUser(t, pool, "sweep-user")
	setStaleExpiredUser(t, pool, lapsedUser)

	lapsedViewer := insertViewer(t, pool)
	setStaleExpiredViewer(t, pool, lapsedViewer)

	validUser := insertUser(t, pool, "sweep-valid")
	setUserOverride(t, pool, validUser, true, "NOW() + INTERVAL '2 hours'")
	_, err := premium.NewRecomputer(pool, zap.NewNop()).Recompute(ctx, validUser)
	require.NoError(t, err)

	sweeper := reconcile.NewOverrideExpirySweeper(pool, premium.NewRecomputer(pool, zap.NewNop()), time.Minute, 500, zap.NewNop())
	require.NoError(t, sweeper.SweepOnce(ctx))

	// Lapsed subjects converged to not-premium with cleared columns.
	assert.False(t, dbPremium(t, pool, lapsedUser), "lapsed streamer grant swept to not-premium")
	ov, exp := dbUserOverride(t, pool, lapsedUser)
	assert.Nil(t, ov)
	assert.Nil(t, exp)
	assert.False(t, dbViewerPremium(t, pool, lapsedViewer), "lapsed viewer grant swept to not-premium")

	// Still-valid grant untouched.
	assert.True(t, dbPremium(t, pool, validUser), "valid grant survives the sweep")
	ov, _ = dbUserOverride(t, pool, validUser)
	require.NotNil(t, ov)
	assert.True(t, *ov)

	// Idempotent: a second pass expires nothing new and changes nothing.
	require.NoError(t, sweeper.SweepOnce(ctx))
	assert.True(t, dbPremium(t, pool, validUser))
}

// --- helpers ---

// setUserOverride sets users.premium_admin_override and its expiry (a raw SQL
// expression like "NOW() + INTERVAL '1 hour'" or "NULL") without recomputing.
func setUserOverride(t *testing.T, pool *pgxpool.Pool, userID string, override bool, expiryExpr string) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		"UPDATE users SET premium_admin_override = $2, premium_admin_override_expires_at = "+expiryExpr+" WHERE id = $1",
		userID, override)
	require.NoError(t, err)
}

// setStaleExpiredUser puts a user in the post-expiry stale state the sweep must fix:
// is_premium still TRUE, override TRUE, expiry in the past.
func setStaleExpiredUser(t *testing.T, pool *pgxpool.Pool, userID string) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`UPDATE users SET is_premium = TRUE, premium_admin_override = TRUE,
		 premium_admin_override_expires_at = NOW() - INTERVAL '1 minute' WHERE id = $1`, userID)
	require.NoError(t, err)
}

// setStaleExpiredViewer is the viewer counterpart of setStaleExpiredUser.
func setStaleExpiredViewer(t *testing.T, pool *pgxpool.Pool, viewerID string) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`UPDATE viewers SET is_premium = TRUE, premium_admin_override = TRUE,
		 premium_admin_override_expires_at = NOW() - INTERVAL '1 minute' WHERE id = $1`, viewerID)
	require.NoError(t, err)
}

func dbUserOverride(t *testing.T, pool *pgxpool.Pool, userID string) (*bool, *time.Time) {
	t.Helper()
	var ov *bool
	var exp *time.Time
	require.NoError(t, pool.QueryRow(context.Background(),
		"SELECT premium_admin_override, premium_admin_override_expires_at FROM users WHERE id = $1", userID).Scan(&ov, &exp))
	return ov, exp
}
