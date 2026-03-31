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
			COALESCE(ocs.config->>'stream_select', '') as stream_select,
			COALESCE(ocs.config->>'stream_match', '') as stream_match,
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
			&source.StreamSelect,
			&source.StreamMatch,
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
			COALESCE(ocs.config->>'stream_select', '') as stream_select,
			COALESCE(ocs.config->>'stream_match', '') as stream_match,
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
			&source.StreamSelect,
			&source.StreamMatch,
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

// GetSourcesForOverlays returns all active sources belonging to the given overlay IDs.
// Used by the demand subscriber to resolve which sources have active demand.
func (r *Repository) GetSourcesForOverlays(ctx context.Context, overlayIDs []string) ([]*models.ActiveSource, error) {
	if len(overlayIDs) == 0 {
		return make([]*models.ActiveSource, 0), nil
	}

	query := `
		SELECT
			ocs.id,
			ocs.overlay_id,
			ocs.platform,
			ocs.channel_id,
			COALESCE(ocs.config->>'stream_id', '') as stream_id,
			COALESCE(ocs.config->>'stream_select', '') as stream_select,
			COALESCE(ocs.config->>'stream_match', '') as stream_match,
			o.is_active,
			ocs.created_at,
			ocs.updated_at
		FROM overlay_chat_sources ocs
		JOIN overlays o ON ocs.overlay_id = o.id
		JOIN users u ON o.user_id = u.id
		WHERE o.is_active = true
		  AND u.is_banned = false
		  AND ocs.overlay_id = ANY($1::uuid[])
		ORDER BY ocs.platform, ocs.created_at
	`

	rows, err := r.db.Query(ctx, query, overlayIDs)
	if err != nil {
		r.logger.Error("Failed to query sources for overlays",
			zap.Int("overlay_count", len(overlayIDs)),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to query sources for overlays: %w", err)
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
			&source.StreamSelect,
			&source.StreamMatch,
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
		r.logger.Error("Error iterating sources for overlays", zap.Error(err))
		return nil, fmt.Errorf("error iterating sources for overlays: %w", err)
	}

	r.logger.Debug("Fetched sources for overlays",
		zap.Int("overlay_count", len(overlayIDs)),
		zap.Int("source_count", len(sources)),
	)

	return sources, nil
}

// ActivateSource marks a source as active and refreshes updated_at.
// Called by listeners when they start polling a channel, to prevent the cleanup
// job from marking the source inactive due to staleness.
func (r *Repository) ActivateSource(ctx context.Context, platform, channelID string) (int64, error) {
	query := `
		UPDATE overlay_chat_sources
		SET is_active = true, updated_at = NOW()
		WHERE platform = $1 AND channel_id = $2
	`
	result, err := r.db.Exec(ctx, query, platform, channelID)
	if err != nil {
		return 0, fmt.Errorf("failed to activate source: %w", err)
	}
	return result.RowsAffected(), nil
}

// DeactivateSource marks a source as inactive immediately.
// Called by listeners when they stop polling a channel (stream ended, overlay
// disconnected, demand lost, or service shutdown) so the admin panel reflects
// the actual polling state rather than waiting 24 h for cleanup.
func (r *Repository) DeactivateSource(ctx context.Context, platform, channelID string) (int64, error) {
	query := `
		UPDATE overlay_chat_sources
		SET is_active = false, updated_at = NOW()
		WHERE platform = $1 AND channel_id = $2 AND is_active = true
	`
	result, err := r.db.Exec(ctx, query, platform, channelID)
	if err != nil {
		return 0, fmt.Errorf("failed to deactivate source: %w", err)
	}
	return result.RowsAffected(), nil
}
