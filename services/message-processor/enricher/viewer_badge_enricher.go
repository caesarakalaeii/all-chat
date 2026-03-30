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
	ViewerID       string  `json:"viewer_id"`
	NameColor      *string `json:"name_color"`               // nil = viewer registered but no color set
	NameGradient   []byte  `json:"name_gradient,omitempty"`  // Phase 29: raw JSONB bytes, nil when not set
	AvatarFrameURL string  `json:"avatar_frame_url,omitempty"` // Phase 30: empty string when not set
	AvatarFlairURL string  `json:"avatar_flair_url,omitempty"` // Phase 30: empty string when not set
	IsAdmin        bool    `json:"is_admin,omitempty"`         // Phase 31: All-Chat admin badge
	IsPremium      bool    `json:"is_premium,omitempty"`       // Phase 31: All-Chat premium badge
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
			// Phase 30: propagate frame/flair from cache
			if identity.AvatarFrameURL != "" {
				msg.User.AvatarFrameURL = identity.AvatarFrameURL
			}
			if identity.AvatarFlairURL != "" {
				msg.User.AvatarFlairURL = identity.AvatarFlairURL
			}
			// Phase 31: inject All-Chat badges from cache
			// Prepend premium first so allchat ends up at index 0 in final slice
			if identity.IsPremium {
				msg.User.Badges = append([]models.Badge{{Name: "allchat-premium", Version: "1", IconURL: ""}}, msg.User.Badges...)
			}
			if identity.IsAdmin {
				msg.User.Badges = append([]models.Badge{{Name: "allchat", Version: "1", IconURL: ""}}, msg.User.Badges...)
			}
			return nil
		}
		// Malformed cache entry — fall through to DB
	}

	// 2. Cache miss — query DB (Phase 31: include is_admin/is_premium via LATERAL viewer_sessions JOIN)
	var viewerID string
	var nameColor *string
	var nameGradientBytes []byte
	var avatarFrameURL string
	var avatarFlairURL string
	var isAdmin bool
	var isPremium bool
	row := e.db.QueryRow(ctx, `
		SELECT vpi.viewer_id::text, vc.name_color, vc.name_gradient,
		       COALESCE(cf.image_url, '') AS avatar_frame_url,
		       COALESCE(cfl.image_url, '') AS avatar_flair_url,
		       COALESCE(u.is_admin, false) AS is_admin,
		       COALESCE(v.is_premium, false) AS is_premium
		FROM viewer_platform_identities vpi
		LEFT JOIN viewer_cosmetics vc ON vc.viewer_id = vpi.viewer_id
		LEFT JOIN cosmetic_frames cf ON cf.id = vc.avatar_frame_id
		LEFT JOIN cosmetic_flairs cfl ON cfl.id = vc.avatar_flair_id
		LEFT JOIN LATERAL (SELECT user_id FROM viewer_sessions WHERE viewer_id = vpi.viewer_id ORDER BY user_id IS NOT NULL DESC LIMIT 1) vs ON true
		LEFT JOIN users u ON u.id = vs.user_id
		-- Phase 32: viewers.is_premium is the viewer cosmetic flag (migration 036); users.is_premium is streamer-only
		LEFT JOIN viewers v ON v.id = vpi.viewer_id
		WHERE vpi.platform = $1 AND vpi.platform_user_id = $2
	`, msg.Platform, msg.User.ID)

	if scanErr := row.Scan(&viewerID, &nameColor, &nameGradientBytes, &avatarFrameURL, &avatarFlairURL, &isAdmin, &isPremium); scanErr != nil {
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
		ViewerID:       viewerID,
		NameColor:      nameColor,
		NameGradient:   nameGradientBytes, // nil guard: json omitempty handles nil slice
		AvatarFrameURL: avatarFrameURL,   // Phase 30: COALESCE guarantees non-nil string
		AvatarFlairURL: avatarFlairURL,
		IsAdmin:        isAdmin,   // Phase 31
		IsPremium:      isPremium, // Phase 31
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

	// 6. Phase 30: inject avatar frame/flair URL if set
	if avatarFrameURL != "" {
		msg.User.AvatarFrameURL = avatarFrameURL
	}
	if avatarFlairURL != "" {
		msg.User.AvatarFlairURL = avatarFlairURL
	}

	// 7. Phase 31: inject All-Chat badges for resolved viewers
	// Prepend premium first so allchat ends up at index 0 in final slice
	if isPremium {
		msg.User.Badges = append([]models.Badge{{Name: "allchat-premium", Version: "1", IconURL: ""}}, msg.User.Badges...)
	}
	if isAdmin {
		msg.User.Badges = append([]models.Badge{{Name: "allchat", Version: "1", IconURL: ""}}, msg.User.Badges...)
	}

	return nil
}
