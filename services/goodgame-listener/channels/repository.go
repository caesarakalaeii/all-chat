// This file is part of All-Chat.
// Copyright (C) 2026 caesarakalaeii
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package channels

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// ActiveChannel represents an active GoodGame channel to monitor.
// ChannelSlug is the channel identifier as the streamer entered it (their
// GoodGame stream key, e.g. "Miker"); the numeric chat id is resolved by the
// listener, not stored in the DB.
type ActiveChannel struct {
	SourceID    string
	OverlayID   string
	ChannelSlug string
	IsActive    bool
}

// Repository handles database operations for channels.
type Repository struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

// NewRepository creates a new channel repository.
func NewRepository(db *pgxpool.Pool, logger *zap.Logger) *Repository {
	return &Repository{db: db, logger: logger}
}

// GetActiveChannels retrieves all GoodGame channels eligible for subscription.
// Filters by overlay.is_active only — source.is_active is a status flag set by
// the listener when it subscribes, not an eligibility filter (demand system
// handles which channels to connect to), matching the kick-listener query.
func (r *Repository) GetActiveChannels(ctx context.Context) ([]*ActiveChannel, error) {
	query := `
		SELECT
			ocs.id as source_id,
			ocs.overlay_id,
			COALESCE(ocs.channel_handle, ocs.channel_name) as channel_slug,
			ocs.is_active
		FROM overlay_chat_sources ocs
		JOIN overlays o ON ocs.overlay_id = o.id
		WHERE ocs.platform = 'goodgame'
		  AND o.is_active = true
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query active channels: %w", err)
	}
	defer rows.Close()

	var channels []*ActiveChannel
	for rows.Next() {
		var ch ActiveChannel
		if err := rows.Scan(&ch.SourceID, &ch.OverlayID, &ch.ChannelSlug, &ch.IsActive); err != nil {
			r.logger.Error("Failed to scan channel row", zap.Error(err))
			continue
		}
		channels = append(channels, &ch)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating channel rows: %w", err)
	}

	r.logger.Info("Retrieved active GoodGame channels", zap.Int("count", len(channels)))
	return channels, nil
}

// SetSourceActive updates the is_active flag for GoodGame sources with the
// given channel slug. Only updates when the status actually changed to
// prevent notification spam.
func (r *Repository) SetSourceActive(ctx context.Context, channelSlug string, isActive bool) error {
	query := `
		UPDATE overlay_chat_sources
		SET is_active = $2
		WHERE (channel_handle = $1 OR channel_name = $1)
		  AND platform = 'goodgame'
		  AND is_active <> $2
	`

	_, err := r.db.Exec(ctx, query, channelSlug, isActive)
	if err != nil {
		return fmt.Errorf("failed to update source status: %w", err)
	}
	return nil
}
