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

// GateChecker is a narrow interface for checking feature gate premium status.
// Implemented by featuregates.FeatureGateCache in production and by mocks in tests.
type GateChecker interface {
	IsPremium(key string) bool
}

// premiumQuerier is an injectable function type for querying a user's premium status.
// Used internally to allow test injection without requiring a real pgxpool.Pool.
type premiumQuerier func(ctx context.Context, userID string) (isPremium bool, err error)

// newDBQuerier returns a premiumQuerier backed by a pgxpool.Pool.
func newDBQuerier(db *pgxpool.Pool) premiumQuerier {
	return func(ctx context.Context, userID string) (bool, error) {
		var isPremium bool
		err := db.QueryRow(ctx,
			"SELECT is_premium FROM users WHERE id = $1", userID).Scan(&isPremium)
		return isPremium, err
	}
}

// RequirePremium returns middleware that checks whether a feature gate requires
// premium access, then enforces user premium status if needed.
//
// Decision flow (D-11, D-15, D-16):
//  1. Check user is authenticated (user_id in context) — returns 401 if missing.
//  2. If gates.IsPremium(featureKey) returns false: feature is free, allow all authenticated users.
//  3. If gates.IsPremium(featureKey) returns true: query DB and require user.is_premium=true.
//
// Note: Authentication check happens before gate check to ensure only authenticated
// users can access even free-gated features (standard AuthN/AuthZ ordering).
func RequirePremium(db *pgxpool.Pool, gates GateChecker, featureKey string, logger *zap.Logger) gin.HandlerFunc {
	querier := newDBQuerier(db)
	return requirePremiumCore(gates, featureKey, querier, logger)
}

// RequirePremiumWithQuerier is a testable variant of RequirePremium that accepts
// an injectable premiumQuerier instead of a *pgxpool.Pool. Used in unit tests only.
func RequirePremiumWithQuerier(gates GateChecker, featureKey string, querier premiumQuerier, logger *zap.Logger) gin.HandlerFunc {
	return requirePremiumCore(gates, featureKey, querier, logger)
}

// requirePremiumCore is the shared implementation used by both RequirePremium and
// RequirePremiumWithQuerier.
func requirePremiumCore(gates GateChecker, featureKey string, querier premiumQuerier, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Step 1: Authentication check — user must be authenticated regardless of gate state.
		userID := c.GetString("user_id")
		if userID == "" {
			if logger != nil {
				logger.Warn("Premium check failed: no user_id in context")
			}
			c.JSON(401, gin.H{
				"error": "authentication required",
			})
			c.Abort()
			return
		}

		// Step 2: Gate check — if feature is free, skip premium user check.
		if !gates.IsPremium(featureKey) {
			// Feature is free for all authenticated users (D-15).
			c.Next()
			return
		}

		// Step 3: Gate says premium required — check user premium status (D-16).
		isPremium, err := querier(c.Request.Context(), userID)
		if err != nil {
			if logger != nil {
				logger.Error("Failed to verify premium status",
					zap.String("user_id", userID),
					zap.Error(err))
			}
			c.JSON(500, gin.H{
				"error": "failed to verify premium status",
			})
			c.Abort()
			return
		}

		if !isPremium {
			if logger != nil {
				logger.Info("Premium feature access denied",
					zap.String("user_id", userID),
					zap.String("feature", featureKey))
			}
			c.JSON(403, gin.H{
				"error":       "Premium feature required",
				"message":     "This is a premium feature. Upgrade your account to access this functionality.",
				"upgrade_url": "/upgrade",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
