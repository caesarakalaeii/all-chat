package handlers

import (
	"context"
	"net/http"

	"github.com/caesar/all-chat/services/share-service/featuregates"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// FeatureGateResponse is the JSON shape returned by ListGates.
type FeatureGateResponse struct {
	FeatureKey  string `json:"feature_key"`
	IsPremium   bool   `json:"is_premium"`
	Description string `json:"description"`
}

// featureGateDB is a narrow interface over pgxpool.Pool for the feature gate handler.
// This allows mock injection in tests without pgxmock.
type featureGateDB interface {
	QueryFeatureGates(ctx context.Context) ([]FeatureGateResponse, error)
	UpdateFeatureGate(ctx context.Context, key string, isPremium bool) (int64, error)
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
		"SELECT feature_key, is_premium, description FROM feature_gates ORDER BY feature_key")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	gates := make([]FeatureGateResponse, 0)
	for rows.Next() {
		var g FeatureGateResponse
		if err := rows.Scan(&g.FeatureKey, &g.IsPremium, &g.Description); err != nil {
			return nil, err
		}
		gates = append(gates, g)
	}
	return gates, rows.Err()
}

func (p *pgxFeatureGateDB) UpdateFeatureGate(ctx context.Context, key string, isPremium bool) (int64, error) {
	tag, err := p.pool.Exec(ctx,
		"UPDATE feature_gates SET is_premium = $1 WHERE feature_key = $2",
		isPremium, key)
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
// Updates is_premium in DB and publishes a Redis invalidation event.
//
// Uses *bool for is_premium to avoid Gin's binding:"required" rejecting false
// (Gin treats the zero value of bool as "not provided" for required validation).
func (h *AdminFeatureGatesHandler) UpdateGate(c *gin.Context) {
	key := c.Param("key")

	type updateFeatureGateRequest struct {
		IsPremium *bool `json:"is_premium"`
	}

	var req updateFeatureGateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request body",
		})
		return
	}

	// Validate: nil means the field was absent in the request body
	if req.IsPremium == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "is_premium field required",
		})
		return
	}

	rowsAffected, err := h.db.UpdateFeatureGate(c.Request.Context(), key, *req.IsPremium)
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

	h.logger.Info("Feature gate updated",
		zap.String("key", key),
		zap.Bool("is_premium", *req.IsPremium))

	c.JSON(http.StatusOK, gin.H{
		"feature_key": key,
		"is_premium":  *req.IsPremium,
	})
}
