package repository

import (
	"context"
	"encoding/json"

	"github.com/caesar/all-chat/internal/chat-listener/core/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresChannelRepository implements the ChannelRepository interface
type PostgresChannelRepository struct {
	db *pgxpool.Pool
}

// NewPostgresChannelRepository creates a new PostgreSQL channel repository
func NewPostgresChannelRepository(db *pgxpool.Pool) *PostgresChannelRepository {
	return &PostgresChannelRepository{db: db}
}

// GetActiveChannels retrieves all channels that should be monitored
func (r *PostgresChannelRepository) GetActiveChannels(ctx context.Context) ([]domain.ActiveChannel, error) {
	query := `
		SELECT
			o.id,
			oc.twitch_channel,
			oc.enable_7tv,
			oc.enable_bttv,
			oc.enable_ffz,
			COALESCE(oc.filter_settings->>'blocked_users', '[]')::text,
			COALESCE(oc.filter_settings->>'blocked_words', '[]')::text
		FROM overlays o
		INNER JOIN overlay_configs oc ON o.id = oc.overlay_id
		WHERE o.is_active = true
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	channels := make([]domain.ActiveChannel, 0)
	for rows.Next() {
		var channel domain.ActiveChannel
		var blockedUsersJSON, blockedWordsJSON string

		err := rows.Scan(
			&channel.OverlayID,
			&channel.Channel,
			&channel.Enable7TV,
			&channel.EnableBTTV,
			&channel.EnableFFZ,
			&blockedUsersJSON,
			&blockedWordsJSON,
		)
		if err != nil {
			return nil, err
		}

		// Parse JSON arrays
		if err := json.Unmarshal([]byte(blockedUsersJSON), &channel.BlockedUsers); err != nil {
			channel.BlockedUsers = []string{}
		}
		if err := json.Unmarshal([]byte(blockedWordsJSON), &channel.BlockedWords); err != nil {
			channel.BlockedWords = []string{}
		}

		channels = append(channels, channel)
	}

	return channels, rows.Err()
}
