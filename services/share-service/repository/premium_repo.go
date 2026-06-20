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
func (r *PremiumRepository) UpdateUserPremium(ctx context.Context, userID string, isPremium bool) error {
	var override *bool
	if isPremium {
		v := true
		override = &v
	}

	result, err := r.db.Exec(ctx,
		"UPDATE users SET premium_admin_override = $1 WHERE id = $2",
		override, userID)
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
		zap.Bool("is_premium", isPremium))

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
