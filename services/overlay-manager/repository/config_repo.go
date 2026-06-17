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
	"encoding/json"
	"errors"
	"fmt"

	"github.com/caesar/all-chat/services/overlay-manager/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// OverlayConfigRepository handles persistence for overlay_configs
type OverlayConfigRepository struct {
	pool *pgxpool.Pool
}

// NewOverlayConfigRepository creates a repository backed by PostgreSQL
func NewOverlayConfigRepository(connString string) (*OverlayConfigRepository, error) {
	pool, err := pgxpool.New(context.Background(), connString)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &OverlayConfigRepository{pool: pool}, nil
}

// GetByOverlayID returns the config record for the provided overlay
func (r *OverlayConfigRepository) GetByOverlayID(ctx context.Context, overlayID string) (*models.OverlayConfig, error) {
	query := `
		SELECT id, overlay_id, display_settings, filter_settings,
		       enable_7tv, enable_bttv, enable_ffz, custom_css, visual_settings,
		       seventv_emote_set_id, theme_id, created_at, updated_at
		FROM overlay_configs
		WHERE overlay_id = $1
	`

	row := r.pool.QueryRow(ctx, query, overlayID)
	return scanOverlayConfig(row)
}

// Update persists the provided config
func (r *OverlayConfigRepository) Update(ctx context.Context, config *models.OverlayConfig) error {
	config.EnsureMaps()

	displaySettings, err := json.Marshal(config.DisplaySettings)
	if err != nil {
		return fmt.Errorf("failed to marshal display settings: %w", err)
	}
	filterSettings, err := json.Marshal(config.FilterSettings)
	if err != nil {
		return fmt.Errorf("failed to marshal filter settings: %w", err)
	}
	visualSettings, err := json.Marshal(config.VisualSettings)
	if err != nil {
		return fmt.Errorf("failed to marshal visual settings: %w", err)
	}

	// NULL when empty so the unset state is distinct from an empty-string override.
	var seventvSetID any
	if config.SevenTVEmoteSetID != "" {
		seventvSetID = config.SevenTVEmoteSetID
	}

	// NULL when empty: "no bundled theme" is distinct from any theme id.
	var themeID any
	if config.ThemeID != "" {
		themeID = config.ThemeID
	}

	query := `
		UPDATE overlay_configs
		SET display_settings = $1,
		    filter_settings = $2,
		    enable_7tv = $3,
		    enable_bttv = $4,
		    enable_ffz = $5,
		    custom_css = $6,
		    visual_settings = $7,
		    seventv_emote_set_id = $8,
		    theme_id = $9,
		    updated_at = NOW()
		WHERE overlay_id = $10
		RETURNING id, overlay_id, display_settings, filter_settings,
		          enable_7tv, enable_bttv, enable_ffz, custom_css, visual_settings,
		          seventv_emote_set_id, theme_id, created_at, updated_at
	`

	row := r.pool.QueryRow(ctx, query,
		displaySettings,
		filterSettings,
		config.Enable7TV,
		config.EnableBTTV,
		config.EnableFFZ,
		config.CustomCSS,
		visualSettings,
		seventvSetID,
		themeID,
		config.OverlayID,
	)

	updated, err := scanOverlayConfig(row)
	if err != nil {
		return err
	}

	*config = *updated
	return nil
}

func scanOverlayConfig(row pgx.Row) (*models.OverlayConfig, error) {
	config := &models.OverlayConfig{}
	var displaySettingsJSON, filterSettingsJSON, visualSettingsJSON []byte
	var seventvSetID *string
	var themeID *string

	err := row.Scan(
		&config.ID,
		&config.OverlayID,
		&displaySettingsJSON,
		&filterSettingsJSON,
		&config.Enable7TV,
		&config.EnableBTTV,
		&config.EnableFFZ,
		&config.CustomCSS,
		&visualSettingsJSON,
		&seventvSetID,
		&themeID,
		&config.CreatedAt,
		&config.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("overlay config not found")
		}
		return nil, fmt.Errorf("failed to scan overlay config: %w", err)
	}

	if seventvSetID != nil {
		config.SevenTVEmoteSetID = *seventvSetID
	}
	if themeID != nil {
		config.ThemeID = *themeID
	}

	if err := json.Unmarshal(displaySettingsJSON, &config.DisplaySettings); err != nil {
		config.DisplaySettings = map[string]any{}
	}
	if err := json.Unmarshal(filterSettingsJSON, &config.FilterSettings); err != nil {
		config.FilterSettings = map[string]any{}
	}
	if err := json.Unmarshal(visualSettingsJSON, &config.VisualSettings); err != nil {
		config.VisualSettings = map[string]any{}
	}

	config.EnsureMaps()
	return config, nil
}
