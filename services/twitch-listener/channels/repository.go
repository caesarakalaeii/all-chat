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
	"time"

	"github.com/caesar/all-chat/services/twitch-listener/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DefaultIdleOverlayThreshold is the default value for the freshness window
// applied to overlays.last_connected_at. Overlays whose last WebSocket attach
// is older than this fall out of the listener's desired-channel set and get
// IRC-PARTed on the next sync. Configurable via IDLE_OVERLAY_THRESHOLD env var.
const DefaultIdleOverlayThreshold = 7 * 24 * time.Hour

// EventSub chat-ownership partition (ADR-0015): the IRC↔EventSub split is NO LONGER a static SQL
// scope predicate. The previous `NOT EXISTS (... 'user:read:chat' = ANY(granted_scopes) ...)`
// excluded a channel from IRC whenever its owner *could* be read via EventSub — even when EventSub
// was not actually delivering (revoked sub, partial scopes, verification failure, demand/leader
// gap), leaving the channel read by neither listener and silently losing chat. The exclusion now
// lives in channels.Manager.excludeEventSubOwnedChannels, which drops only channels that hold a
// LIVE chat-ownership claim (refreshed by the EventSub handler on delivered chat). Any channel
// EventSub is not currently serving falls through to IRC here. These queries therefore return ALL
// active Twitch channels; the claim filter is applied in the manager.

// RepositoryInterface defines the interface for channel repository
type RepositoryInterface interface {
	GetActiveChannels(ctx context.Context) ([]models.ChannelSource, error)
	GetUniqueChannels(ctx context.Context) ([]string, error)
	GetSourceIDsForChannels(ctx context.Context, channels []string) map[string]string
	GetOverlayIDsForChannel(ctx context.Context, channelName string) ([]string, error)
	SetSourceActive(ctx context.Context, channelName string, isActive bool) error
}

// Repository handles database queries for channel management
type Repository struct {
	db            *pgxpool.Pool
	idleThreshold time.Duration
}

// NewRepository creates a new channel repository.
// idleThreshold gates which overlays are considered "in use" — overlays with
// last_connected_at older than now()-idleThreshold are excluded so the listener
// PARTs their twitch channels. A non-positive value disables the filter (every
// is_active overlay is included).
func NewRepository(db *pgxpool.Pool, idleThreshold time.Duration) *Repository {
	return &Repository{db: db, idleThreshold: idleThreshold}
}

// freshnessClause renders the SQL predicate that gates overlays on
// last_connected_at. Returns an empty string when idleThreshold is non-positive
// so callers can append it unconditionally.
func (r *Repository) freshnessClause() string {
	if r.idleThreshold <= 0 {
		return ""
	}
	// Use a parameter-free interval literal because pgx's $N substitution does
	// not interpolate intervals nicely; the threshold is a server-trusted value
	// derived once from configuration, so direct embedding is safe.
	return fmt.Sprintf(" AND o.last_connected_at > NOW() - INTERVAL '%d seconds'",
		int64(r.idleThreshold.Seconds()))
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
		  AND ocs.platform = 'twitch'` + r.freshnessClause() + `
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
//
// NOTE: ocs.is_active is intentionally NOT in this filter — it's a runtime
// status flag set by the listener itself ("is currently connected"), not a
// config flag. Idle gating is done via overlays.last_connected_at instead
// (bumped by api-gateway on every WebSocket attach + heartbeat).
func (r *Repository) GetUniqueChannels(ctx context.Context) ([]string, error) {
	query := `
		SELECT DISTINCT ocs.channel_name
		FROM overlays o
		JOIN overlay_chat_sources ocs ON o.id = ocs.overlay_id
		WHERE o.is_active = true
		  AND ocs.platform = 'twitch'` + r.freshnessClause() + `
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

// GetSourceIDsForChannels returns a map of channel_name -> source_id (UUID)
// Used to filter channels by coordinator assignments
func (r *Repository) GetSourceIDsForChannels(ctx context.Context, channels []string) map[string]string {
	if len(channels) == 0 {
		return make(map[string]string)
	}

	query := `
		SELECT DISTINCT channel_name, id
		FROM overlay_chat_sources
		WHERE platform = 'twitch'
		  AND channel_name = ANY($1)
	`

	rows, err := r.db.Query(ctx, query, channels)
	if err != nil {
		return make(map[string]string)
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var channelName, sourceID string
		if err := rows.Scan(&channelName, &sourceID); err != nil {
			continue
		}
		result[channelName] = sourceID
	}

	return result
}

// GetOverlayIDsForChannel returns all overlay IDs that have this channel as a source
// Used for cross-platform event publishing
func (r *Repository) GetOverlayIDsForChannel(ctx context.Context, channelName string) ([]string, error) {
	query := `
		SELECT DISTINCT overlay_id
		FROM overlay_chat_sources
		WHERE platform = 'twitch'
		  AND channel_name = $1
		  AND is_active = true
	`

	rows, err := r.db.Query(ctx, query, channelName)
	if err != nil {
		return nil, fmt.Errorf("failed to query overlay IDs: %w", err)
	}
	defer rows.Close()

	var overlayIDs []string
	for rows.Next() {
		var overlayID string
		if err := rows.Scan(&overlayID); err != nil {
			return nil, fmt.Errorf("failed to scan overlay ID: %w", err)
		}
		overlayIDs = append(overlayIDs, overlayID)
	}

	return overlayIDs, rows.Err()
}

// SetSourceActive updates the is_active flag for all Twitch sources with the given channel name
// OPTIMIZATION: Only updates if the status actually changed to prevent notification spam
func (r *Repository) SetSourceActive(ctx context.Context, channelName string, isActive bool) error {
	query := `
		UPDATE overlay_chat_sources
		SET is_active = $1, updated_at = NOW()
		WHERE platform = 'twitch'
		  AND channel_name = $2
		  AND is_active != $1
	`

	result, err := r.db.Exec(ctx, query, isActive, channelName)
	if err != nil {
		return fmt.Errorf("failed to update source status: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		// Not an error - channel status unchanged or channel removed
		return nil
	}

	return nil
}

// SetSourceActiveByOverlay updates the is_active flag for a specific overlay's Twitch source
// OPTIMIZATION: Only updates if the status actually changed to prevent notification spam
func (r *Repository) SetSourceActiveByOverlay(ctx context.Context, overlayID, channelName string, isActive bool) error {
	query := `
		UPDATE overlay_chat_sources
		SET is_active = $1, updated_at = NOW()
		WHERE platform = 'twitch'
		  AND overlay_id = $2
		  AND channel_name = $3
		  AND is_active != $1
	`

	result, err := r.db.Exec(ctx, query, isActive, overlayID, channelName)
	if err != nil {
		return fmt.Errorf("failed to update overlay-specific source status: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		// Not an error - source may have been removed
		return nil
	}

	return nil
}
