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

package subscription

import (
	"context"
	"errors"
	"fmt"

	"github.com/caesar/all-chat/shared/alerts"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository handles database queries for subscription management
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a new subscription repository
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// VerifyOverlayOwnership verifies that a user owns an overlay
func (r *Repository) VerifyOverlayOwnership(ctx context.Context, overlayID, userID string) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM overlays
			WHERE id = $1 AND user_id = $2
		)
	`

	var exists bool
	err := r.db.QueryRow(ctx, query, overlayID, userID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to verify overlay ownership: %w", err)
	}

	return exists, nil
}

// OverlayStatus reports whether an overlay is active and its overlay_type
// (ADR-0068). A missing overlay reads as inactive.
func (r *Repository) OverlayStatus(ctx context.Context, overlayID string) (isActive bool, overlayType string, err error) {
	err = r.db.QueryRow(ctx, `SELECT is_active, overlay_type FROM overlays WHERE id = $1`, overlayID).Scan(&isActive, &overlayType)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, "", nil
	}
	if err != nil {
		return false, "", fmt.Errorf("failed to check overlay status: %w", err)
	}
	return isActive, overlayType, nil
}

// AlertProfile returns the overlay's saved profile for eventType
// (alert_profiles, migrations 107 + 109); found is false when none is saved.
func (r *Repository) AlertProfile(ctx context.Context, overlayID, eventType string) (p alerts.Profile, found bool, err error) {
	err = r.db.QueryRow(ctx, `
		SELECT event_type, text_template, sound_url, image_url, duration_ms, volume, enabled, sort_after,
		       custom_html, custom_css, custom_js
		FROM alert_profiles WHERE overlay_id = $1 AND event_type = $2`, overlayID, eventType,
	).Scan(&p.EventType, &p.TextTemplate, &p.SoundURL, &p.ImageURL, &p.DurationMs, &p.Volume, &p.Enabled, &p.SortAfter,
		&p.CustomHTML, &p.CustomCSS, &p.CustomJS)
	if errors.Is(err, pgx.ErrNoRows) {
		return alerts.Profile{}, false, nil
	}
	if err != nil {
		return alerts.Profile{}, false, fmt.Errorf("failed to load alert profile: %w", err)
	}
	return p, true, nil
}

// GetPublicOverlayByUsername gets the public overlay ID for a streamer by username
// Returns the overlay ID if the streamer has a public overlay, empty string if not found
func (r *Repository) GetPublicOverlayByUsername(ctx context.Context, username string) (string, error) {
	query := `
		SELECT o.id
		FROM overlays o
		JOIN users u ON o.user_id = u.id
		WHERE u.username = $1
		  AND o.is_active = true
		  AND o.is_public_for_viewers = true
		  AND u.is_banned = false
		ORDER BY o.created_at ASC
		LIMIT 1
	`

	var overlayID string
	err := r.db.QueryRow(ctx, query, username).Scan(&overlayID)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return "", nil
		}
		return "", fmt.Errorf("failed to get public overlay: %w", err)
	}

	return overlayID, nil
}

// ActivateSourcesForOverlay activates all sources for an overlay
// This is called when a WebSocket connection is established
// Skips shared_overlay sources whose share_request is revoked or expired.
func (r *Repository) ActivateSourcesForOverlay(ctx context.Context, overlayID string) (int, error) {
	query := `
		UPDATE overlay_chat_sources ocs
		SET is_active = true,
		    updated_at = NOW()
		WHERE ocs.overlay_id = $1
		  AND ocs.is_active = false
		  AND NOT (
		    ocs.platform = 'shared_overlay'
		    AND EXISTS (
		      SELECT 1 FROM share_requests sr
		      WHERE sr.id::text = ocs.channel_id
		        AND sr.status IN ('revoked', 'expired')
		    )
		  )
	`

	result, err := r.db.Exec(ctx, query, overlayID)
	if err != nil {
		return 0, fmt.Errorf("failed to activate sources: %w", err)
	}

	return int(result.RowsAffected()), nil
}

// OverlaySource represents a single chat source for an overlay
type OverlaySource struct {
	Platform    string
	ChannelID   string
	ChannelName string
}

// GetOverlaySources returns all platform+channel_id pairs configured for an overlay
func (r *Repository) GetOverlaySources(ctx context.Context, overlayID string) ([]OverlaySource, error) {
	query := `
		SELECT platform, channel_id, channel_name
		FROM overlay_chat_sources
		WHERE overlay_id = $1
	`

	rows, err := r.db.Query(ctx, query, overlayID)
	if err != nil {
		return nil, fmt.Errorf("failed to get overlay sources: %w", err)
	}
	defer rows.Close()

	sources := make([]OverlaySource, 0)
	for rows.Next() {
		var src OverlaySource
		if err := rows.Scan(&src.Platform, &src.ChannelID, &src.ChannelName); err != nil {
			return nil, fmt.Errorf("failed to scan overlay source: %w", err)
		}
		sources = append(sources, src)
	}

	return sources, nil
}

// DeactivateSourcesForOverlay deactivates all sources for an overlay
// This is called when the last WebSocket connection for an overlay disconnects
func (r *Repository) DeactivateSourcesForOverlay(ctx context.Context, overlayID string) (int, error) {
	query := `
		UPDATE overlay_chat_sources
		SET is_active = false,
		    updated_at = NOW()
		WHERE overlay_id = $1
		  AND is_active = true
	`

	result, err := r.db.Exec(ctx, query, overlayID)
	if err != nil {
		return 0, fmt.Errorf("failed to deactivate sources: %w", err)
	}

	return int(result.RowsAffected()), nil
}
