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
		       created_at, updated_at
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

	query := `
		UPDATE overlay_configs
		SET display_settings = $1,
		    filter_settings = $2,
		    enable_7tv = $3,
		    enable_bttv = $4,
		    enable_ffz = $5,
		    custom_css = $6,
		    visual_settings = $7,
		    updated_at = NOW()
		WHERE overlay_id = $8
		RETURNING id, overlay_id, display_settings, filter_settings,
		          enable_7tv, enable_bttv, enable_ffz, custom_css, visual_settings,
		          created_at, updated_at
	`

	row := r.pool.QueryRow(ctx, query,
		displaySettings,
		filterSettings,
		config.Enable7TV,
		config.EnableBTTV,
		config.EnableFFZ,
		config.CustomCSS,
		visualSettings,
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
		&config.CreatedAt,
		&config.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("overlay config not found")
		}
		return nil, fmt.Errorf("failed to scan overlay config: %w", err)
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
