package streams

import (
	"context"
	"fmt"

	"github.com/caesar/all-chat/services/youtube-listener/models"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// Repository handles database operations for stream sources
type Repository struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

// NewRepository creates a new stream repository
func NewRepository(db *pgxpool.Pool, logger *zap.Logger) *Repository {
	return &Repository{
		db:     db,
		logger: logger,
	}
}

// GetActiveSources returns all active YouTube sources that should be monitored
func (r *Repository) GetActiveSources(ctx context.Context) ([]*models.StreamSource, error) {
	query := `
		SELECT DISTINCT
			ocs.overlay_id,
			ocs.channel_id
		FROM overlay_chat_sources ocs
		JOIN overlays o ON ocs.overlay_id = o.id
		WHERE o.is_active = true
		  AND ocs.platform = 'youtube'
		ORDER BY ocs.channel_id
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		r.logger.Error("Failed to query active YouTube sources", zap.Error(err))
		return nil, fmt.Errorf("failed to query active sources: %w", err)
	}
	defer rows.Close()

	sources := make([]*models.StreamSource, 0)

	for rows.Next() {
		var source models.StreamSource
		if err := rows.Scan(&source.OverlayID, &source.ChannelID); err != nil {
			r.logger.Error("Failed to scan stream source", zap.Error(err))
			continue
		}
		sources = append(sources, &source)
	}

	if err := rows.Err(); err != nil {
		r.logger.Error("Error iterating stream sources", zap.Error(err))
		return nil, fmt.Errorf("error iterating sources: %w", err)
	}

	r.logger.Debug("Fetched active YouTube sources",
		zap.Int("count", len(sources)),
	)

	return sources, nil
}

// GetUserIDForChannel gets the user ID associated with a YouTube channel
// This is needed to retrieve OAuth tokens
func (r *Repository) GetUserIDForChannel(ctx context.Context, channelID string) (string, error) {
	query := `
		SELECT DISTINCT u.id
		FROM users u
		JOIN overlays o ON u.id = o.user_id
		JOIN overlay_chat_sources ocs ON o.id = ocs.overlay_id
		WHERE ocs.platform = 'youtube'
		  AND ocs.channel_id = $1
		  AND o.is_active = true
		LIMIT 1
	`

	var userID string
	err := r.db.QueryRow(ctx, query, channelID).Scan(&userID)
	if err != nil {
		r.logger.Error("Failed to get user ID for channel",
			zap.String("channel_id", channelID),
			zap.Error(err),
		)
		return "", fmt.Errorf("failed to get user ID: %w", err)
	}

	return userID, nil
}
