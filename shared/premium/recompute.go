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

// Package premium derives users.is_premium from its two independent inputs:
// the admin override and the user's Patreon subscription state (ADR-0018).
//
// users.is_premium is a MATERIALIZED column so that all existing readers
// (shared/middleware/premium.go, moderation-service) stay unchanged. Both the
// admin endpoint (share-service) and payment-service call RecomputePremium after
// changing an input.
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
