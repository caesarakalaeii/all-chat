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

// Package premium derives the materialized is_premium columns from their
// independent inputs, so existing readers stay unchanged and the multiple writers
// never clobber each other.
//
//   - users.is_premium (ADR-0018, ADR-0020): admin override + (active Patreon
//     subscription OR beta-tester). Recompute. Readers: shared/middleware/premium.go,
//     moderation-service.
//   - viewers.is_premium (ADR-0019): viewer admin override + active viewer-product
//     subscription + inheritance from a linked premium streamer. RecomputeViewer.
//     Readers: message-processor ViewerBadgeEnricher, viewer JWT.
//
// Both columns are MATERIALIZED and recomputed by the writer after any input
// change (share-service / payment-service / auth-service).
//
// The admin override may carry an OPTIONAL expiry (ADR-0027): a time-limited comp.
// Once past its expiry the override is treated as absent (premium falls through to
// the subscription half). The expiry is compared against the database clock (NOW())
// inside the read SQL, so Effective stays a pure, time-free boolean and there is a
// single clock. The materialized column converges on any recompute; the
// payment-service expiry sweep (see ExpireUserOverrideIfDue / ExpireViewerOverrideIfDue)
// is the backstop that clears a lapsed grant when nothing else triggers a recompute.
package premium

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// Effective is the entitlement rule and the single source of truth for it.
//
//   - adminOverride == TRUE  -> premium (force-grant: comp/staff/partner)
//   - adminOverride == FALSE -> not premium (force-deny, reserved)
//   - adminOverride == NULL  -> follow the subscription (hasActiveSub)
//
// The caller supplies the *effective* override: the recompute read SQL first maps an
// expired time-limited override (ADR-0027) to NULL before calling Effective, so an
// expired grant is indistinguishable here from "no admin opinion". Effective itself
// stays time-free (its truth table is exhaustively unit-tested); only the subscription
// half's own grace is Patreon's (ADR-0018).
func Effective(adminOverride *bool, hasActiveSub bool) bool {
	if adminOverride != nil {
		return *adminOverride
	}
	return hasActiveSub
}

// Recomputer recomputes and persists users.is_premium for one user.
type Recomputer struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

// NewRecomputer builds a Recomputer backed by a pgx pool.
func NewRecomputer(db *pgxpool.Pool, logger *zap.Logger) *Recomputer {
	return &Recomputer{db: db, logger: logger}
}

// Recompute derives users.is_premium for userID from premium_admin_override (with
// its optional expiry), the user's premium_subscriptions, and the beta-tester flag,
// writes it, and returns the new value.
//
// The "premium half" (the non-override input to Effective) is the OR of an active
// subscription and is_beta_tester (ADR-0020): a beta-tester is premium, the same
// clobber-free way an admin force-grant is, while ALSO unlocking early-access gates
// that plain premium does not (see shared/middleware.RequireEarlyAccess). An admin
// force-deny (override FALSE) still wins over both.
//
// It is convergent and idempotent: the value is a pure function (Effective) of
// the user's current rows and the database clock, so calling it any number of times
// in any order — after a webhook, a reconcile pass, an admin write (premium override
// OR beta-tester), or an override lapsing — yields the correct result. The read and
// write run in one transaction with SELECT ... FOR UPDATE so concurrent recomputes
// for the same user are serialized (no lost-update race).
func (r *Recomputer) Recompute(ctx context.Context, userID string) (bool, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to begin premium recompute tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op once committed

	effective, err := r.recomputeUserTx(ctx, tx, userID)
	if err != nil {
		return false, err
	}

	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("failed to commit premium recompute: %w", err)
	}
	return effective, nil
}

// recomputeUserTx reads the user's premium inputs (mapping an expired override to
// NULL via the DB clock), applies Effective, and writes users.is_premium — all
// inside tx. The caller owns the transaction lifecycle (begin/commit). The row is
// locked FOR UPDATE so concurrent recomputes serialize.
func (r *Recomputer) recomputeUserTx(ctx context.Context, tx pgx.Tx, userID string) (bool, error) {
	var adminOverride *bool
	var hasActiveSub bool
	var isBetaTester bool
	err := tx.QueryRow(ctx, `
		SELECT CASE
		           WHEN u.premium_admin_override_expires_at IS NOT NULL
		            AND u.premium_admin_override_expires_at <= NOW()
		           THEN NULL
		           ELSE u.premium_admin_override
		       END,
		       EXISTS (
		           SELECT 1 FROM premium_subscriptions s
		           WHERE s.user_id = u.id AND s.status = 'active'
		       ),
		       u.is_beta_tester
		FROM users u
		WHERE u.id = $1
		FOR UPDATE`, userID).Scan(&adminOverride, &hasActiveSub, &isBetaTester)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, fmt.Errorf("user not found: %s", userID)
	}
	if err != nil {
		return false, fmt.Errorf("failed to read premium inputs: %w", err)
	}

	effective := Effective(adminOverride, hasActiveSub || isBetaTester)

	if _, err := tx.Exec(ctx,
		"UPDATE users SET is_premium = $1 WHERE id = $2", effective, userID); err != nil {
		return false, fmt.Errorf("failed to write is_premium: %w", err)
	}

	if r.logger != nil {
		r.logger.Debug("Recomputed premium",
			zap.String("user_id", userID),
			zap.Boolp("effective_admin_override", adminOverride),
			zap.Bool("has_active_sub", hasActiveSub),
			zap.Bool("is_beta_tester", isBetaTester),
			zap.Bool("is_premium", effective))
	}
	return effective, nil
}

// ExpireUserOverrideIfDue clears a time-limited admin override that has passed its
// expiry and recomputes users.is_premium, atomically (ADR-0027). It is the write the
// payment-service sweep performs per due user.
//
// Returns (true, nil) when an expired override was cleared and premium recomputed;
// (false, nil) when the user has no override, no expiry, or an expiry still in the
// future — the guarded UPDATE's WHERE clause matches only a genuinely-lapsed grant,
// so a concurrent admin re-grant (fresh future expiry, or permanent) is never
// clobbered. The clear and the recompute share one transaction, so a crash can never
// strand a cleared-but-not-recomputed row (which would leave is_premium stale).
func (r *Recomputer) ExpireUserOverrideIfDue(ctx context.Context, userID string) (bool, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to begin override-expiry tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op once committed

	ct, err := tx.Exec(ctx, `
		UPDATE users
		SET premium_admin_override = NULL,
		    premium_admin_override_expires_at = NULL
		WHERE id = $1
		  AND premium_admin_override IS NOT NULL
		  AND premium_admin_override_expires_at IS NOT NULL
		  AND premium_admin_override_expires_at <= NOW()`, userID)
	if err != nil {
		return false, fmt.Errorf("failed to clear expired user override: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return false, nil // not due, or concurrently re-granted / already swept
	}

	if _, err := r.recomputeUserTx(ctx, tx, userID); err != nil {
		return false, err
	}

	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("failed to commit user override expiry: %w", err)
	}
	if r.logger != nil {
		r.logger.Info("Expired time-limited admin premium override", zap.String("user_id", userID))
	}
	return true, nil
}

// RecomputeViewer derives viewers.is_premium for viewerID and writes it (ADR-0019).
// It mirrors Recompute (users) with the same Effective rule, but the "premium half"
// is the OR of three viewer inputs:
//
//   - an active viewer-product premium_subscriptions row for this viewer, and
//   - inheritance: a streamer users account linked to this viewer (via
//     viewer_sessions.user_id) that is itself premium — preserving the badge the
//     ViewerBadgeEnricher and viewer JWT already grant to a streamer's own viewer
//     identity.
//
// viewers.premium_admin_override (tri-state, with its optional ADR-0027 expiry)
// overrides both, exactly like the user side. The result is a pure function of
// current rows and the database clock, so a viewer webhook, a reconcile pass, an
// admin write, an override lapsing, or LinkViewerToUser converge regardless of order
// or concurrency. The read + write run in one transaction with SELECT ... FOR UPDATE
// on the viewers row, serializing concurrent recomputes for the same viewer.
//
// Making RecomputeViewer the single writer of viewers.is_premium also fixes the
// prior staleness where inherited premium was set at viewer-login but never revoked
// when the linked streamer lapsed: it now converges on the next recompute.
func (r *Recomputer) RecomputeViewer(ctx context.Context, viewerID string) (bool, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to begin viewer premium recompute tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op once committed

	effective, err := r.recomputeViewerTx(ctx, tx, viewerID)
	if err != nil {
		return false, err
	}

	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("failed to commit viewer premium recompute: %w", err)
	}
	return effective, nil
}

// recomputeViewerTx reads the viewer's premium inputs (mapping an expired override to
// NULL via the DB clock), applies Effective, and writes viewers.is_premium — all
// inside tx. The caller owns the transaction lifecycle.
func (r *Recomputer) recomputeViewerTx(ctx context.Context, tx pgx.Tx, viewerID string) (bool, error) {
	var adminOverride *bool
	var hasPremiumInput bool
	err := tx.QueryRow(ctx, `
		SELECT CASE
		           WHEN v.premium_admin_override_expires_at IS NOT NULL
		            AND v.premium_admin_override_expires_at <= NOW()
		           THEN NULL
		           ELSE v.premium_admin_override
		       END,
		       EXISTS (
		           SELECT 1 FROM premium_subscriptions s
		           WHERE s.viewer_id = v.id AND s.product = 'viewer' AND s.status = 'active'
		       )
		       OR EXISTS (
		           SELECT 1 FROM viewer_sessions vs
		           JOIN users u ON u.id = vs.user_id
		           WHERE vs.viewer_id = v.id AND u.is_premium IS TRUE
		       )
		FROM viewers v
		WHERE v.id = $1
		FOR UPDATE`, viewerID).Scan(&adminOverride, &hasPremiumInput)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, fmt.Errorf("viewer not found: %s", viewerID)
	}
	if err != nil {
		return false, fmt.Errorf("failed to read viewer premium inputs: %w", err)
	}

	effective := Effective(adminOverride, hasPremiumInput)

	if _, err := tx.Exec(ctx,
		"UPDATE viewers SET is_premium = $1 WHERE id = $2", effective, viewerID); err != nil {
		return false, fmt.Errorf("failed to write viewers.is_premium: %w", err)
	}

	if r.logger != nil {
		r.logger.Debug("Recomputed viewer premium",
			zap.String("viewer_id", viewerID),
			zap.Boolp("effective_admin_override", adminOverride),
			zap.Bool("has_premium_input", hasPremiumInput),
			zap.Bool("is_premium", effective))
	}
	return effective, nil
}

// ExpireViewerOverrideIfDue is the viewer counterpart of ExpireUserOverrideIfDue
// (ADR-0027): it clears a lapsed time-limited viewer override and recomputes
// viewers.is_premium atomically, with the same guarded UPDATE (never clobbers a
// concurrent re-grant) and the same crash-safety.
func (r *Recomputer) ExpireViewerOverrideIfDue(ctx context.Context, viewerID string) (bool, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to begin viewer override-expiry tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op once committed

	ct, err := tx.Exec(ctx, `
		UPDATE viewers
		SET premium_admin_override = NULL,
		    premium_admin_override_expires_at = NULL
		WHERE id = $1
		  AND premium_admin_override IS NOT NULL
		  AND premium_admin_override_expires_at IS NOT NULL
		  AND premium_admin_override_expires_at <= NOW()`, viewerID)
	if err != nil {
		return false, fmt.Errorf("failed to clear expired viewer override: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return false, nil // not due, or concurrently re-granted / already swept
	}

	if _, err := r.recomputeViewerTx(ctx, tx, viewerID); err != nil {
		return false, err
	}

	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("failed to commit viewer override expiry: %w", err)
	}
	if r.logger != nil {
		r.logger.Info("Expired time-limited admin viewer premium override", zap.String("viewer_id", viewerID))
	}
	return true, nil
}
