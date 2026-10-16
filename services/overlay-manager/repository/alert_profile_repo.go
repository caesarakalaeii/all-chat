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

package repository

import (
	"context"
	"fmt"

	"github.com/caesar/all-chat/shared/alerts"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AlertProfileRepository persists alert_profiles (migration 107, ADR-0070).
type AlertProfileRepository struct {
	pool *pgxpool.Pool
}

// NewAlertProfileRepository creates a repository from an existing pool.
func NewAlertProfileRepository(pool *pgxpool.Pool) *AlertProfileRepository {
	return &AlertProfileRepository{pool: pool}
}

// ListAlertProfiles returns the overlay's saved profiles (no defaults).
func (r *AlertProfileRepository) ListAlertProfiles(ctx context.Context, overlayID string) ([]alerts.Profile, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT event_type, text_template, sound_url, image_url, duration_ms, volume, enabled, sort_after,
		       custom_html, custom_css, custom_js
		FROM alert_profiles
		WHERE overlay_id = $1
		ORDER BY event_type`, overlayID)
	if err != nil {
		return nil, fmt.Errorf("list alert profiles: %w", err)
	}
	profiles, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (alerts.Profile, error) {
		var p alerts.Profile
		err := row.Scan(&p.EventType, &p.TextTemplate, &p.SoundURL, &p.ImageURL, &p.DurationMs, &p.Volume, &p.Enabled, &p.SortAfter,
			&p.CustomHTML, &p.CustomCSS, &p.CustomJS)
		return p, err
	})
	if err != nil {
		return nil, fmt.Errorf("scan alert profiles: %w", err)
	}
	return profiles, nil
}

// UpsertAlertProfiles writes every profile in one transaction: all or nothing.
func (r *AlertProfileRepository) UpsertAlertProfiles(ctx context.Context, overlayID string, profiles []alerts.Profile) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		for _, p := range profiles {
			if _, err := tx.Exec(ctx, `
				INSERT INTO alert_profiles
					(overlay_id, event_type, text_template, sound_url, image_url, duration_ms, volume, enabled, sort_after,
					 custom_html, custom_css, custom_js)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
				ON CONFLICT (overlay_id, event_type) DO UPDATE SET
					text_template = EXCLUDED.text_template,
					sound_url     = EXCLUDED.sound_url,
					image_url     = EXCLUDED.image_url,
					duration_ms   = EXCLUDED.duration_ms,
					volume        = EXCLUDED.volume,
					enabled       = EXCLUDED.enabled,
					sort_after    = EXCLUDED.sort_after,
					custom_html   = EXCLUDED.custom_html,
					custom_css    = EXCLUDED.custom_css,
					custom_js     = EXCLUDED.custom_js,
					updated_at    = now()`,
				overlayID, p.EventType, p.TextTemplate, p.SoundURL, p.ImageURL, p.DurationMs, p.Volume, p.Enabled, p.SortAfter,
				p.CustomHTML, p.CustomCSS, p.CustomJS,
			); err != nil {
				return fmt.Errorf("upsert alert profile %s: %w", p.EventType, err)
			}
		}
		return nil
	})
}
