package channels

import (
	"context"
	"fmt"

	"github.com/caesar/all-chat/services/twitch-listener/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RepositoryInterface defines the interface for channel repository
type RepositoryInterface interface {
	GetActiveChannels(ctx context.Context) ([]models.ChannelSource, error)
	GetUniqueChannels(ctx context.Context) ([]string, error)
	SetSourceActive(ctx context.Context, channelName string, isActive bool) error
}

// Repository handles database queries for channel management
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a new channel repository
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// GetActiveChannels returns all active Twitch channels that should be monitored
// based on active overlays with Twitch sources
// For Twitch IRC, we need the channel_name (username) not the channel_id (numeric ID)
func (r *Repository) GetActiveChannels(ctx context.Context) ([]models.ChannelSource, error) {
	query := `
		SELECT DISTINCT
			o.id as overlay_id,
			ocs.channel_name
		FROM overlays o
		JOIN overlay_chat_sources ocs ON o.id = ocs.overlay_id
		WHERE o.is_active = true
		  AND ocs.platform = 'twitch'
		ORDER BY ocs.channel_name
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query active channels: %w", err)
	}
	defer rows.Close()

	var channels []models.ChannelSource
	for rows.Next() {
		var ch models.ChannelSource
		if err := rows.Scan(&ch.OverlayID, &ch.ChannelID); err != nil {
			return nil, fmt.Errorf("failed to scan channel row: %w", err)
		}
		channels = append(channels, ch)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating channel rows: %w", err)
	}

	return channels, nil
}

// GetUniqueChannels returns a deduplicated list of channel names (usernames)
// For Twitch IRC, we need the channel_name (username) not the channel_id (numeric ID)
// NOTE: This query intentionally does NOT check ocs.is_active because that field
// is used as a runtime status indicator (is the listener currently connected),
// not a configuration flag. We determine what to listen to based on active overlays.
func (r *Repository) GetUniqueChannels(ctx context.Context) ([]string, error) {
	query := `
		SELECT DISTINCT ocs.channel_name
		FROM overlays o
		JOIN overlay_chat_sources ocs ON o.id = ocs.overlay_id
		WHERE o.is_active = true
		  AND ocs.platform = 'twitch'
		ORDER BY ocs.channel_name
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query unique channels: %w", err)
	}
	defer rows.Close()

	var channels []string
	for rows.Next() {
		var channelID string
		if err := rows.Scan(&channelID); err != nil {
			return nil, fmt.Errorf("failed to scan channel ID: %w", err)
		}
		channels = append(channels, channelID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating channel rows: %w", err)
	}

	return channels, nil
}

// SetSourceActive updates the is_active flag for all Twitch sources with the given channel name
func (r *Repository) SetSourceActive(ctx context.Context, channelName string, isActive bool) error {
	query := `
		UPDATE overlay_chat_sources
		SET is_active = $1, updated_at = NOW()
		WHERE platform = 'twitch'
		  AND channel_name = $2
	`

	result, err := r.db.Exec(ctx, query, isActive, channelName)
	if err != nil {
		return fmt.Errorf("failed to update source status: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		// Not an error - channel may have been removed
		return nil
	}

	return nil
}
