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
	"fmt"
	"time"

	"github.com/caesar/all-chat/shared/premium"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type PremiumRepository struct {
	db         *pgxpool.Pool
	recomputer *premium.Recomputer
	logger     *zap.Logger
}

func NewPremiumRepository(db *pgxpool.Pool, recomputer *premium.Recomputer, logger *zap.Logger) *PremiumRepository {
	return &PremiumRepository{db: db, recomputer: recomputer, logger: logger}
}

// UpdateUserPremium records the admin's premium decision as a tri-state override
// (ADR-0018) and re-derives users.is_premium via shared/premium. Granting maps to a
// force-grant (override TRUE) that survives Patreon subscription lapses; revoking
// clears the override (NULL) so premium then follows any active subscription. A hard
// premium-ban (override FALSE) is reserved for a future explicit admin action.
//
// ttl makes the grant time-limited (ADR-0027): non-nil grants premium only until
// NOW()+ttl, after which Recompute (and the payment-service sweep) treat the override
// as absent. ttl is only meaningful when granting; a nil ttl (or any revoke) clears
// the expiry, leaving a permanent grant / clean slate. The expiry is computed from
// the database clock (NOW()) so the grant lasts exactly ttl regardless of app clock.
func (r *PremiumRepository) UpdateUserPremium(ctx context.Context, userID string, isPremium bool, ttl *time.Duration) error {
	var override *bool
	if isPremium {
		v := true
		override = &v
	}

	// Seconds for the expiry, computed server-side as NOW() + make_interval(secs => $2).
	// nil => no expiry (permanent grant, or a revoke that clears any prior expiry).
	var ttlSeconds *float64
	if isPremium && ttl != nil {
		s := ttl.Seconds()
		ttlSeconds = &s
	}

	result, err := r.db.Exec(ctx, `
		UPDATE users
		SET premium_admin_override = $1,
		    premium_admin_override_expires_at = CASE
		        WHEN $2::double precision IS NULL THEN NULL
		        ELSE NOW() + make_interval(secs => $2::double precision)
		    END
		WHERE id = $3`,
		override, ttlSeconds, userID)
	if err != nil {
		r.logger.Error("Failed to update premium override",
			zap.String("user_id", userID),
			zap.Bool("is_premium", isPremium),
			zap.Error(err))
		return fmt.Errorf("failed to update premium status: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("user not found: %s", userID)
	}

	if _, err := r.recomputer.Recompute(ctx, userID); err != nil {
		r.logger.Error("Failed to recompute premium after admin override",
			zap.String("user_id", userID),
			zap.Error(err))
		return fmt.Errorf("failed to recompute premium status: %w", err)
	}

	r.logger.Info("Premium override updated",
		zap.String("user_id", userID),
		zap.Bool("is_premium", isPremium),
		zap.Bool("time_limited", ttlSeconds != nil))

	return nil
}

// SetUserBetaTester records the admin's beta-tester decision (ADR-0020) and
// re-derives users.is_premium via shared/premium. A beta tester is premium (the
// recompute folds is_beta_tester into is_premium) AND unlocks early-access gates
// that plain premium does not. Unlike the premium override this is a plain boolean
// — there is no "force-deny beta" state; revoking simply clears the flag, after
// which premium follows the subscription/override again on the recompute.
//
// This is the ongoing "Grant Beta Tester" mechanism; the ~5 pre-monetization
// premium users are grandfathered by an admin calling it per user, never by a
// blanket data migration (ADR-0020 / the 009-incident class).
func (r *PremiumRepository) SetUserBetaTester(ctx context.Context, userID string, isBetaTester bool) error {
	result, err := r.db.Exec(ctx,
		"UPDATE users SET is_beta_tester = $1 WHERE id = $2",
		isBetaTester, userID)
	if err != nil {
		r.logger.Error("Failed to update beta-tester status",
			zap.String("user_id", userID),
			zap.Bool("is_beta_tester", isBetaTester),
			zap.Error(err))
		return fmt.Errorf("failed to update beta-tester status: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("user not found: %s", userID)
	}

	if _, err := r.recomputer.Recompute(ctx, userID); err != nil {
		r.logger.Error("Failed to recompute premium after beta-tester change",
			zap.String("user_id", userID),
			zap.Error(err))
		return fmt.Errorf("failed to recompute premium status: %w", err)
	}

	r.logger.Info("Beta-tester status updated",
		zap.String("user_id", userID),
		zap.Bool("is_beta_tester", isBetaTester))

	return nil
}

func (r *PremiumRepository) IsPremium(ctx context.Context, userID string) (bool, error) {
	var isPremium bool
	err := r.db.QueryRow(ctx,
		"SELECT is_premium FROM users WHERE id = $1", userID).Scan(&isPremium)

	if err != nil {
		return false, fmt.Errorf("failed to check premium status: %w", err)
	}

	return isPremium, nil
}
