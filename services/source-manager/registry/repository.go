package registry

import (
	"context"
	"fmt"

	"github.com/caesar/all-chat/services/source-manager/models"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// Repository handles database operations for active sources
type Repository struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

// NewRepository creates a new repository
func NewRepository(db *pgxpool.Pool, logger *zap.Logger) *Repository {
	return &Repository{
		db:     db,
		logger: logger,
	}
}

// GetActiveSources returns all active sources for a platform
func (r *Repository) GetActiveSources(ctx context.Context, platform string) ([]*models.ActiveSource, error) {
	query := `
		SELECT
			ocs.id,
			ocs.overlay_id,
			ocs.platform,
			ocs.channel_id,
			COALESCE(ocs.config->>'stream_id', '') as stream_id,
			o.is_active,
			ocs.created_at,
			ocs.updated_at
		FROM overlay_chat_sources ocs
		JOIN overlays o ON ocs.overlay_id = o.id
		JOIN users u ON o.user_id = u.id
		WHERE o.is_active = true
		  AND u.is_banned = false
		  AND ocs.platform = $1
		ORDER BY ocs.created_at
	`

	rows, err := r.db.Query(ctx, query, platform)
	if err != nil {
		r.logger.Error("Failed to query active sources",
			zap.String("platform", platform),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to query active sources: %w", err)
	}
	defer rows.Close()

	sources := make([]*models.ActiveSource, 0)

	for rows.Next() {
		var source models.ActiveSource
		if err := rows.Scan(
			&source.ID,
			&source.OverlayID,
			&source.Platform,
			&source.ChannelID,
			&source.StreamID,
			&source.IsActive,
			&source.CreatedAt,
			&source.UpdatedAt,
		); err != nil {
			r.logger.Error("Failed to scan source", zap.Error(err))
			continue
		}
		sources = append(sources, &source)
	}

	if err := rows.Err(); err != nil {
		r.logger.Error("Error iterating sources", zap.Error(err))
		return nil, fmt.Errorf("error iterating sources: %w", err)
	}

	r.logger.Debug("Fetched active sources",
		zap.String("platform", platform),
		zap.Int("count", len(sources)),
	)

	return sources, nil
}

// GetAllActiveSources returns all active sources across all platforms
func (r *Repository) GetAllActiveSources(ctx context.Context) ([]*models.ActiveSource, error) {
	query := `
		SELECT
			ocs.id,
			ocs.overlay_id,
			ocs.platform,
			ocs.channel_id,
			COALESCE(ocs.config->>'stream_id', '') as stream_id,
			o.is_active,
			ocs.created_at,
			ocs.updated_at
		FROM overlay_chat_sources ocs
		JOIN overlays o ON ocs.overlay_id = o.id
		JOIN users u ON o.user_id = u.id
		WHERE o.is_active = true
		  AND u.is_banned = false
		ORDER BY ocs.platform, ocs.created_at
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		r.logger.Error("Failed to query all active sources", zap.Error(err))
		return nil, fmt.Errorf("failed to query all active sources: %w", err)
	}
	defer rows.Close()

	sources := make([]*models.ActiveSource, 0)

	for rows.Next() {
		var source models.ActiveSource
		if err := rows.Scan(
			&source.ID,
			&source.OverlayID,
			&source.Platform,
			&source.ChannelID,
			&source.StreamID,
			&source.IsActive,
			&source.CreatedAt,
			&source.UpdatedAt,
		); err != nil {
			r.logger.Error("Failed to scan source", zap.Error(err))
			continue
		}
		sources = append(sources, &source)
	}

	if err := rows.Err(); err != nil {
		r.logger.Error("Error iterating sources", zap.Error(err))
		return nil, fmt.Errorf("error iterating sources: %w", err)
	}

	r.logger.Debug("Fetched all active sources",
		zap.Int("count", len(sources)),
	)

	return sources, nil
}
