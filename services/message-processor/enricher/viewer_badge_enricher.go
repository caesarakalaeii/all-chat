package enricher

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/caesar/all-chat/services/message-processor/models"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	// ViewerIdentityCacheTTL is how long to cache viewer identity lookups (5 minutes)
	ViewerIdentityCacheTTL    = 5 * time.Minute
	viewerIdentityCachePrefix = "viewer:identity:"
	viewerNullSentinel        = "null"
)

// pgxErrNoRows is an alias for pgx.ErrNoRows for use in tests without importing pgx directly.
var pgxErrNoRows = pgx.ErrNoRows

// pgxRowScanner is the interface for a single-row query result.
type pgxRowScanner interface {
	Scan(dest ...interface{}) error
}

// viewerDB abstracts the DB pool for testability.
type viewerDB interface {
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgxRowScanner
}

// pgxPoolAdapter wraps *pgxpool.Pool to implement viewerDB.
// We use a concrete adapter so main.go can pass *pgxpool.Pool directly.
type pgxPoolAdapter struct {
	pool interface {
		QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
	}
}

func (a *pgxPoolAdapter) QueryRow(ctx context.Context, sql string, args ...interface{}) pgxRowScanner {
	return a.pool.QueryRow(ctx, sql, args...)
}

// viewerIdentityCache is the structure stored in Redis.
type viewerIdentityCache struct {
	ViewerID     string  `json:"viewer_id"`
	NameColor    *string `json:"name_color"`               // nil = viewer registered but no color set
	NameGradient []byte  `json:"name_gradient,omitempty"` // Phase 29: raw JSONB bytes, nil when not set
}

// ViewerBadgeEnricher injects viewer name_color and name_gradient into messages for registered viewers.
type ViewerBadgeEnricher struct {
	redis  *redis.Client
	db     viewerDB
	logger *zap.Logger
}

// NewViewerBadgeEnricher creates a new ViewerBadgeEnricher.
// db must be a *pgxpool.Pool (passed via pgxPoolAdapter internally) or any viewerDB implementation.
func NewViewerBadgeEnricher(redisClient *redis.Client, db interface {
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
}, logger *zap.Logger) *ViewerBadgeEnricher {
	return &ViewerBadgeEnricher{
		redis:  redisClient,
		db:     &pgxPoolAdapter{pool: db},
		logger: logger,
	}
}

// Enrich resolves the message's platform user to a viewer identity and injects name_color and name_gradient.
// Returns nil on all soft failures (Redis/DB errors) to avoid blocking message delivery.
func (e *ViewerBadgeEnricher) Enrich(ctx context.Context, msg *models.UnifiedChatMessage) error {
	if msg.User.ID == "" {
		return nil
	}

	cacheKey := fmt.Sprintf("%s%s:%s", viewerIdentityCachePrefix, msg.Platform, msg.User.ID)

	// 1. Check Redis cache
	cached, err := e.redis.Get(ctx, cacheKey).Result()
	if err == nil {
		if cached == viewerNullSentinel {
			return nil // known unknown — viewer not in All-Chat
		}
		var identity viewerIdentityCache
		if jsonErr := json.Unmarshal([]byte(cached), &identity); jsonErr == nil {
			if identity.NameColor != nil {
				msg.User.Color = *identity.NameColor
			}
			// Phase 29: propagate gradient from cache
			if len(identity.NameGradient) > 0 {
				msg.User.NameGradient = string(identity.NameGradient)
			}
			return nil
		}
		// Malformed cache entry — fall through to DB
	}

	// 2. Cache miss — query DB (Phase 29: include vc.name_gradient in SELECT)
	var viewerID string
	var nameColor *string
	var nameGradientBytes []byte
	row := e.db.QueryRow(ctx, `
		SELECT vpi.viewer_id::text, vc.name_color, vc.name_gradient
		FROM viewer_platform_identities vpi
		LEFT JOIN viewer_cosmetics vc ON vc.viewer_id = vpi.viewer_id
		WHERE vpi.platform = $1 AND vpi.platform_user_id = $2
	`, msg.Platform, msg.User.ID)

	if scanErr := row.Scan(&viewerID, &nameColor, &nameGradientBytes); scanErr != nil {
		if scanErr == pgx.ErrNoRows {
			// Viewer not in All-Chat — cache null sentinel
			e.redis.Set(ctx, cacheKey, viewerNullSentinel, ViewerIdentityCacheTTL)
			return nil
		}
		e.logger.Warn("ViewerBadgeEnricher: DB query failed",
			zap.String("platform", msg.Platform),
			zap.String("user_id", msg.User.ID),
			zap.Error(scanErr),
		)
		return nil // soft failure
	}

	// 3. Cache the result
	identity := viewerIdentityCache{
		ViewerID:     viewerID,
		NameColor:    nameColor,
		NameGradient: nameGradientBytes, // nil guard: json omitempty handles nil slice
	}
	if jsonBytes, jsonErr := json.Marshal(identity); jsonErr == nil {
		e.redis.Set(ctx, cacheKey, string(jsonBytes), ViewerIdentityCacheTTL)
	}

	// 4. Inject color if set
	if nameColor != nil {
		msg.User.Color = *nameColor
	}

	// 5. Phase 29: inject gradient if set (guard against nil to avoid empty string pollution)
	if len(nameGradientBytes) > 0 {
		msg.User.NameGradient = string(nameGradientBytes)
	}

	return nil
}
