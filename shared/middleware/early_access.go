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

package middleware

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// betaTesterQuerier is an injectable function type for querying a user's
// beta-tester status. Mirrors premiumQuerier so the early-access gate can be unit
// tested without a real pgxpool.Pool.
type betaTesterQuerier func(ctx context.Context, userID string) (isBetaTester bool, err error)

// newBetaTesterDBQuerier returns a betaTesterQuerier backed by a pgxpool.Pool.
func newBetaTesterDBQuerier(db *pgxpool.Pool) betaTesterQuerier {
	return func(ctx context.Context, userID string) (bool, error) {
		var isBetaTester bool
		err := db.QueryRow(ctx,
			"SELECT is_beta_tester FROM users WHERE id = $1", userID).Scan(&isBetaTester)
		return isBetaTester, err
	}
}

// RequireEarlyAccess returns middleware that gates early-access features (ADR-0020)
// to beta-testers. It mirrors RequirePremium structurally — gate-driven, DB-backed,
// and fresh on grant (no stale-JWT window) — but the dimension is early_access and
// the entitlement is users.is_beta_tester rather than is_premium.
//
// Decision flow:
//  1. Check the user is authenticated (user_id in context) — 401 if missing.
//  2. If gates.IsEarlyAccess(featureKey) is false: the feature is not early-access
//     (or has graduated), so allow through. Any premium requirement on the same
//     route is enforced separately by RequirePremium.
//  3. If gates.IsEarlyAccess(featureKey) is true: query the DB and require
//     users.is_beta_tester = true, else 403.
//
// Note: beta-testers are also premium (shared/premium.Recompute folds is_beta_tester
// into is_premium), so they transparently pass any RequirePremium gate as well.
func RequireEarlyAccess(db *pgxpool.Pool, gates GateChecker, featureKey string, logger *zap.Logger) gin.HandlerFunc {
	querier := newBetaTesterDBQuerier(db)
	return requireEarlyAccessCore(gates, featureKey, querier, logger)
}

// RequireEarlyAccessWithQuerier is a testable variant of RequireEarlyAccess that
// accepts an injectable betaTesterQuerier instead of a *pgxpool.Pool. Tests only.
func RequireEarlyAccessWithQuerier(gates GateChecker, featureKey string, querier betaTesterQuerier, logger *zap.Logger) gin.HandlerFunc {
	return requireEarlyAccessCore(gates, featureKey, querier, logger)
}

// requireEarlyAccessCore is the shared implementation behind both constructors.
func requireEarlyAccessCore(gates GateChecker, featureKey string, querier betaTesterQuerier, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Step 1: Authentication check — user must be authenticated regardless of gate state.
		userID := c.GetString("user_id")
		if userID == "" {
			if logger != nil {
				logger.Warn("Early-access check failed: no user_id in context")
			}
			c.JSON(401, gin.H{
				"error": "authentication required",
			})
			c.Abort()
			return
		}

		// Step 2: Gate check — if the feature is not early-access, allow through.
		if !gates.IsEarlyAccess(featureKey) {
			c.Next()
			return
		}

		// Step 3: Gate says early-access — require beta-tester status.
		isBetaTester, err := querier(c.Request.Context(), userID)
		if err != nil {
			if logger != nil {
				logger.Error("Failed to verify beta-tester status",
					zap.String("user_id", userID),
					zap.Error(err))
			}
			c.JSON(500, gin.H{
				"error": "failed to verify beta-tester status",
			})
			c.Abort()
			return
		}

		if !isBetaTester {
			if logger != nil {
				logger.Info("Early-access feature access denied",
					zap.String("user_id", userID),
					zap.String("feature", featureKey))
			}
			c.JSON(403, gin.H{
				"error":   "Early access feature",
				"message": "This is an early-access feature available to beta testers only.",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
