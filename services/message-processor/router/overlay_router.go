package router

import (
	"context"
	"fmt"

	"github.com/caesar/all-chat/services/message-processor/models"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// Repository handles database queries for overlay routing
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a new overlay router repository
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// FindOverlaysForMessage finds all overlays that should receive this message
func (r *Repository) FindOverlaysForMessage(ctx context.Context, platform, channelID string) ([]models.OverlayTarget, error) {
	query := `
		SELECT DISTINCT o.id, o.user_id
		FROM overlays o
		JOIN overlay_chat_sources ocs ON o.id = ocs.overlay_id
		WHERE o.is_active = true
		  AND ocs.platform = $1
		  AND ocs.channel_id = $2
	`

	rows, err := r.db.Query(ctx, query, platform, channelID)
	if err != nil {
		return nil, fmt.Errorf("failed to query overlays: %w", err)
	}
	defer rows.Close()

	var overlays []models.OverlayTarget
	for rows.Next() {
		var target models.OverlayTarget
		if err := rows.Scan(&target.OverlayID, &target.UserID); err != nil {
			return nil, fmt.Errorf("failed to scan overlay row: %w", err)
		}
		overlays = append(overlays, target)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating overlay rows: %w", err)
	}

	return overlays, nil
}

// Router routes messages to the appropriate overlays
type Router struct {
	repo   *Repository
	logger *zap.Logger
}

// NewRouter creates a new overlay router
func NewRouter(repo *Repository, logger *zap.Logger) *Router {
	return &Router{
		repo:   repo,
		logger: logger,
	}
}

// Route finds all overlays that should receive this message
func (r *Router) Route(ctx context.Context, platform, channelID string) ([]models.OverlayTarget, error) {
	overlays, err := r.repo.FindOverlaysForMessage(ctx, platform, channelID)
	if err != nil {
		return nil, err
	}

	r.logger.Debug("Routed message to overlays",
		zap.String("platform", platform),
		zap.String("channel", channelID),
		zap.Int("overlay_count", len(overlays)),
	)

	return overlays, nil
}
