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

package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/caesar/all-chat/shared/featuregates"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// FeatureGateResponse is the JSON shape returned by ListGates.
type FeatureGateResponse struct {
	FeatureKey  string `json:"feature_key"`
	IsPremium   bool   `json:"is_premium"`
	EarlyAccess bool   `json:"early_access"`
	Description string `json:"description"`
}

// featureGateDB is a narrow interface over pgxpool.Pool for the feature gate handler.
// This allows mock injection in tests without pgxmock.
type featureGateDB interface {
	QueryFeatureGates(ctx context.Context) ([]FeatureGateResponse, error)
	// UpdateFeatureGate updates whichever of is_premium / early_access is non-nil
	// (ADR-0020 added early_access). Returns rows affected.
	UpdateFeatureGate(ctx context.Context, key string, isPremium, earlyAccess *bool) (int64, error)
}

// featureGateRedis is a narrow interface over redis.Client for the feature gate handler.
type featureGateRedis interface {
	Publish(ctx context.Context, channel string, payload interface{}) error
}

// pgxFeatureGateDB wraps *pgxpool.Pool to satisfy featureGateDB.
type pgxFeatureGateDB struct {
	pool *pgxpool.Pool
}

func (p *pgxFeatureGateDB) QueryFeatureGates(ctx context.Context) ([]FeatureGateResponse, error) {
	rows, err := p.pool.Query(ctx,
		"SELECT feature_key, is_premium, early_access, description FROM feature_gates ORDER BY feature_key")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	gates := make([]FeatureGateResponse, 0)
	for rows.Next() {
		var g FeatureGateResponse
		if err := rows.Scan(&g.FeatureKey, &g.IsPremium, &g.EarlyAccess, &g.Description); err != nil {
			return nil, err
		}
		gates = append(gates, g)
	}
	return gates, rows.Err()
}

func (p *pgxFeatureGateDB) UpdateFeatureGate(ctx context.Context, key string, isPremium, earlyAccess *bool) (int64, error) {
	// Build a dynamic UPDATE over only the provided columns. Column names are
	// hardcoded; only values are parameterized.
	sets := make([]string, 0, 2)
	args := make([]any, 0, 3)
	if isPremium != nil {
		sets = append(sets, fmt.Sprintf("is_premium = $%d", len(args)+1))
		args = append(args, *isPremium)
	}
	if earlyAccess != nil {
		sets = append(sets, fmt.Sprintf("early_access = $%d", len(args)+1))
		args = append(args, *earlyAccess)
	}
	if len(sets) == 0 {
		return 0, nil // nothing to update; handler guards against this
	}
	args = append(args, key)
	query := "UPDATE feature_gates SET " + strings.Join(sets, ", ") +
		fmt.Sprintf(" WHERE feature_key = $%d", len(args))
	tag, err := p.pool.Exec(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// redisFeatureGateClient wraps *redis.Client to satisfy featureGateRedis.
type redisFeatureGateClient struct {
	client *redis.Client
}

func (r *redisFeatureGateClient) Publish(ctx context.Context, channel string, payload interface{}) error {
	return r.client.Publish(ctx, channel, payload).Err()
}

// AdminFeatureGatesHandler handles admin CRUD operations for feature gates.
type AdminFeatureGatesHandler struct {
	db     featureGateDB
	redis  featureGateRedis
	logger *zap.Logger
}

// NewAdminFeatureGatesHandler creates a new handler wired to real DB and Redis clients.
func NewAdminFeatureGatesHandler(db *pgxpool.Pool, rc *redis.Client, logger *zap.Logger) *AdminFeatureGatesHandler {
	return &AdminFeatureGatesHandler{
		db:     &pgxFeatureGateDB{pool: db},
		redis:  &redisFeatureGateClient{client: rc},
		logger: logger,
	}
}

// ListGates handles GET /api/v1/admin/feature-gates.
// Returns all feature gate rows as a JSON array (never null).
func (h *AdminFeatureGatesHandler) ListGates(c *gin.Context) {
	gates, err := h.db.QueryFeatureGates(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to load feature gates", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to load feature gates",
		})
		return
	}

	c.JSON(http.StatusOK, gates)
}

// UpdateGate handles PATCH /api/v1/admin/feature-gates/:key.
// Updates is_premium and/or early_access (ADR-0020) in DB and publishes a Redis
// invalidation event.
//
// Uses *bool for each field to avoid Gin's binding:"required" rejecting false
// (Gin treats the zero value of bool as "not provided" for required validation).
// At least one field must be present.
func (h *AdminFeatureGatesHandler) UpdateGate(c *gin.Context) {
	key := c.Param("key")

	type updateFeatureGateRequest struct {
		IsPremium   *bool `json:"is_premium"`
		EarlyAccess *bool `json:"early_access"`
	}

	var req updateFeatureGateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request body",
		})
		return
	}

	// Validate: nil means the field was absent. Require at least one.
	if req.IsPremium == nil && req.EarlyAccess == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "is_premium or early_access field required",
		})
		return
	}

	rowsAffected, err := h.db.UpdateFeatureGate(c.Request.Context(), key, req.IsPremium, req.EarlyAccess)
	if err != nil {
		h.logger.Error("Failed to update feature gate",
			zap.String("key", key),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to update feature gate",
		})
		return
	}

	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "feature gate not found",
		})
		return
	}

	// Publish invalidation to Redis (best-effort — DB is already updated).
	// All services subscribed to feature-gates:invalidate will reload immediately.
	// If publish fails, the 60s TTL refresh (D-09) ensures eventual consistency.
	if err := h.redis.Publish(c.Request.Context(), featuregates.PubSubChannel, key); err != nil {
		h.logger.Warn("Failed to publish feature gate invalidation",
			zap.String("key", key),
			zap.Error(err))
		// Do NOT return error — DB already updated, TTL will catch up.
	}

	resp := gin.H{"feature_key": key}
	logFields := []zap.Field{zap.String("key", key)}
	if req.IsPremium != nil {
		resp["is_premium"] = *req.IsPremium
		logFields = append(logFields, zap.Bool("is_premium", *req.IsPremium))
	}
	if req.EarlyAccess != nil {
		resp["early_access"] = *req.EarlyAccess
		logFields = append(logFields, zap.Bool("early_access", *req.EarlyAccess))
	}
	h.logger.Info("Feature gate updated", logFields...)

	c.JSON(http.StatusOK, resp)
}
