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

// UpdateStreamHistory updates the stream history when live status changes
// Uses the database function created in migration 010
func (r *Repository) UpdateStreamHistory(ctx context.Context, channelID, channelName string, isLive bool) error {
	_, err := r.db.Exec(ctx,
		`SELECT update_stream_history_on_detection($1, $2, $3, $4)`,
		"youtube", channelID, channelName, isLive,
	)
	if err != nil {
		r.logger.Error("Failed to update stream history",
			zap.String("channel_id", channelID),
			zap.Bool("is_live", isLive),
			zap.Error(err),
		)
		return fmt.Errorf("failed to update stream history: %w", err)
	}

	r.logger.Debug("Updated stream history",
		zap.String("channel_id", channelID),
		zap.Bool("is_live", isLive),
	)

	return nil
}

// GetChannelName gets the channel name for a given channel ID
func (r *Repository) GetChannelName(ctx context.Context, channelID string) (string, error) {
	query := `
		SELECT channel_name
		FROM overlay_chat_sources
		WHERE platform = 'youtube'
		  AND channel_id = $1
		LIMIT 1
	`

	var channelName string
	err := r.db.QueryRow(ctx, query, channelID).Scan(&channelName)
	if err != nil {
		r.logger.Warn("Failed to get channel name, using channel ID",
			zap.String("channel_id", channelID),
			zap.Error(err),
		)
		return channelID, nil // Fallback to channel ID
	}

	return channelName, nil
}

// SetSourceActive updates the is_active flag for YouTube sources with the given channel ID
func (r *Repository) SetSourceActive(ctx context.Context, channelID string, isActive bool) error {
	query := `
		UPDATE overlay_chat_sources
		SET is_active = $1, updated_at = NOW()
		WHERE platform = 'youtube'
		  AND channel_id = $2
	`

	result, err := r.db.Exec(ctx, query, isActive, channelID)
	if err != nil {
		r.logger.Error("Failed to update source status",
			zap.String("channel_id", channelID),
			zap.Bool("is_active", isActive),
			zap.Error(err),
		)
		return fmt.Errorf("failed to update source status: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		// Not an error - source may have been removed
		r.logger.Debug("No sources updated (may have been removed)",
			zap.String("channel_id", channelID),
		)
		return nil
	}

	r.logger.Debug("Updated source status",
		zap.String("channel_id", channelID),
		zap.Bool("is_active", isActive),
		zap.Int64("rows_affected", rowsAffected),
	)

	return nil
}

// GetCachedVideoID retrieves the cached video ID for a channel from youtube_channel_quota table
func (r *Repository) GetCachedVideoID(ctx context.Context, channelID string) (string, error) {
	query := `
		SELECT cached_video_id
		FROM youtube_channel_quota
		WHERE channel_id = $1
		  AND cached_video_id IS NOT NULL
	`

	var cachedVideoID string
	err := r.db.QueryRow(ctx, query, channelID).Scan(&cachedVideoID)
	if err != nil {
		// No cached video ID is not an error, just return empty string
		return "", err
	}

	return cachedVideoID, nil
}

// UpdateCachedVideoID updates the cached video ID for a channel in youtube_channel_quota table
func (r *Repository) UpdateCachedVideoID(ctx context.Context, channelID, videoID, videoTitle string) error {
	query := `
		UPDATE youtube_channel_quota
		SET cached_video_id = $2,
		    cached_video_title = $3,
		    consecutive_offline_checks = 0
		WHERE channel_id = $1
	`

	result, err := r.db.Exec(ctx, query, channelID, videoID, videoTitle)
	if err != nil {
		return fmt.Errorf("failed to update cached video ID: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		r.logger.Warn("No quota record found to update cached video ID",
			zap.String("channel_id", channelID),
		)
		// Try to create quota record
		insertQuery := `
			INSERT INTO youtube_channel_quota (channel_id, user_id, cached_video_id, cached_video_title)
			SELECT $1, 
			       (SELECT user_id FROM overlays WHERE id = (
			           SELECT overlay_id FROM overlay_chat_sources 
			           WHERE channel_id = $1 AND platform = 'youtube' LIMIT 1
			       ) LIMIT 1),
			       $2, $3
			ON CONFLICT (channel_id) DO UPDATE
			SET cached_video_id = EXCLUDED.cached_video_id,
			    cached_video_title = EXCLUDED.cached_video_title,
			    consecutive_offline_checks = 0
		`
		_, err := r.db.Exec(ctx, insertQuery, channelID, videoID, videoTitle)
		if err != nil {
			return fmt.Errorf("failed to insert cached video ID: %w", err)
		}
	}

	r.logger.Debug("Updated cached video ID",
		zap.String("channel_id", channelID),
		zap.String("video_id", videoID),
	)

	return nil
}

// ClearCachedVideoID clears the cached video ID for a channel
func (r *Repository) ClearCachedVideoID(ctx context.Context, channelID string) error {
	query := `
		UPDATE youtube_channel_quota
		SET cached_video_id = NULL,
		    cached_video_title = NULL
		WHERE channel_id = $1
	`

	_, err := r.db.Exec(ctx, query, channelID)
	if err != nil {
		return fmt.Errorf("failed to clear cached video ID: %w", err)
	}

	r.logger.Debug("Cleared cached video ID",
		zap.String("channel_id", channelID),
	)

	return nil
}
