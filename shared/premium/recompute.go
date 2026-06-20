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
//   - users.is_premium (ADR-0018): admin override + active Patreon subscription.
//     Recompute. Readers: shared/middleware/premium.go, moderation-service.
//   - viewers.is_premium (ADR-0019): viewer admin override + active viewer-product
//     subscription + inheritance from a linked premium streamer. RecomputeViewer.
//     Readers: message-processor ViewerBadgeEnricher, viewer JWT.
//
// Both columns are MATERIALIZED and recomputed by the writer after any input
// change (share-service / payment-service / auth-service).
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
// The SQL in Recompute is a faithful transcription of this function; keep them
// in sync (the integration tests assert they agree).
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

// Recompute derives users.is_premium for userID from premium_admin_override and
// the user's premium_subscriptions, writes it, and returns the new value.
//
// It is convergent and idempotent: the value is a pure function (Effective) of
// the user's current rows, so calling it any number of times in any order — after
// a webhook, a reconcile pass, or an admin write — yields the same result. The
// read and write run in one transaction with SELECT ... FOR UPDATE so concurrent
// recomputes for the same user are serialized (no lost-update race).
func (r *Recomputer) Recompute(ctx context.Context, userID string) (bool, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to begin premium recompute tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op once committed

	var adminOverride *bool
	var hasActiveSub bool
	err = tx.QueryRow(ctx, `
		SELECT u.premium_admin_override,
		       EXISTS (
		           SELECT 1 FROM premium_subscriptions s
		           WHERE s.user_id = u.id AND s.status = 'active'
		       )
		FROM users u
		WHERE u.id = $1
		FOR UPDATE`, userID).Scan(&adminOverride, &hasActiveSub)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, fmt.Errorf("user not found: %s", userID)
	}
	if err != nil {
		return false, fmt.Errorf("failed to read premium inputs: %w", err)
	}

	effective := Effective(adminOverride, hasActiveSub)

	if _, err := tx.Exec(ctx,
		"UPDATE users SET is_premium = $1 WHERE id = $2", effective, userID); err != nil {
		return false, fmt.Errorf("failed to write is_premium: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("failed to commit premium recompute: %w", err)
	}

	if r.logger != nil {
		r.logger.Debug("Recomputed premium",
			zap.String("user_id", userID),
			zap.Boolp("admin_override", adminOverride),
			zap.Bool("has_active_sub", hasActiveSub),
			zap.Bool("is_premium", effective))
	}
	return effective, nil
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
// viewers.premium_admin_override (tri-state) overrides both, exactly like the user
// side. The result is a pure function of current rows, so a viewer webhook, a
// reconcile pass, an admin write, or LinkViewerToUser converge regardless of order
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

	var adminOverride *bool
	var hasPremiumInput bool
	err = tx.QueryRow(ctx, `
		SELECT v.premium_admin_override,
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

	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("failed to commit viewer premium recompute: %w", err)
	}

	if r.logger != nil {
		r.logger.Debug("Recomputed viewer premium",
			zap.String("viewer_id", viewerID),
			zap.Boolp("admin_override", adminOverride),
			zap.Bool("has_premium_input", hasPremiumInput),
			zap.Bool("is_premium", effective))
	}
	return effective, nil
}
