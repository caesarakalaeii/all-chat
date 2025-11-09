package repository

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/caesar/all-chat/internal/overlay-manager/core/domain"
	"github.com/caesar/all-chat/internal/overlay-manager/core/ports"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type postgresOverlayRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresOverlayRepository(pool *pgxpool.Pool) ports.OverlayRepository {
	return &postgresOverlayRepository{pool: pool}
}

func (r *postgresOverlayRepository) GetByID(ctx context.Context, id string) (*domain.Overlay, error) {
	query := `
		SELECT id, user_id, name, description, is_active, created_at, updated_at
		FROM overlays
		WHERE id = $1
	`

	var overlay domain.Overlay
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&overlay.ID,
		&overlay.UserID,
		&overlay.Name,
		&overlay.Description,
		&overlay.IsActive,
		&overlay.CreatedAt,
		&overlay.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("overlay not found")
		}
		return nil, err
	}

	return &overlay, nil
}

func (r *postgresOverlayRepository) GetByUserID(ctx context.Context, userID string) ([]*domain.Overlay, error) {
	query := `
		SELECT id, user_id, name, description, is_active, created_at, updated_at
		FROM overlays
		WHERE user_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var overlays []*domain.Overlay
	for rows.Next() {
		var overlay domain.Overlay
		err := rows.Scan(
			&overlay.ID,
			&overlay.UserID,
			&overlay.Name,
			&overlay.Description,
			&overlay.IsActive,
			&overlay.CreatedAt,
			&overlay.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		overlays = append(overlays, &overlay)
	}

	return overlays, nil
}

func (r *postgresOverlayRepository) Create(ctx context.Context, overlay *domain.Overlay) error {
	query := `
		INSERT INTO overlays (id, user_id, name, description, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err := r.pool.Exec(ctx, query,
		overlay.ID,
		overlay.UserID,
		overlay.Name,
		overlay.Description,
		overlay.IsActive,
		overlay.CreatedAt,
		overlay.UpdatedAt,
	)

	return err
}

func (r *postgresOverlayRepository) Update(ctx context.Context, overlay *domain.Overlay) error {
	query := `
		UPDATE overlays
		SET name = $2, description = $3, is_active = $4, updated_at = $5
		WHERE id = $1
	`

	result, err := r.pool.Exec(ctx, query,
		overlay.ID,
		overlay.Name,
		overlay.Description,
		overlay.IsActive,
		overlay.UpdatedAt,
	)

	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return errors.New("overlay not found")
	}

	return nil
}

func (r *postgresOverlayRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM overlays WHERE id = $1`

	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return errors.New("overlay not found")
	}

	return nil
}

func (r *postgresOverlayRepository) GetConfig(ctx context.Context, overlayID string) (*domain.OverlayConfig, error) {
	query := `
		SELECT id, overlay_id, twitch_channel, enable_7tv, enable_bttv, enable_ffz,
		       display_settings, filter_settings, created_at, updated_at
		FROM overlay_configs
		WHERE overlay_id = $1
	`

	var config domain.OverlayConfig
	var displaySettingsJSON, filterSettingsJSON []byte

	err := r.pool.QueryRow(ctx, query, overlayID).Scan(
		&config.ID,
		&config.OverlayID,
		&config.TwitchChannel,
		&config.Enable7TV,
		&config.EnableBTTV,
		&config.EnableFFZ,
		&displaySettingsJSON,
		&filterSettingsJSON,
		&config.CreatedAt,
		&config.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("config not found")
		}
		return nil, err
	}

	if err := json.Unmarshal(displaySettingsJSON, &config.DisplaySettings); err != nil {
		return nil, err
	}

	if err := json.Unmarshal(filterSettingsJSON, &config.FilterSettings); err != nil {
		return nil, err
	}

	return &config, nil
}

func (r *postgresOverlayRepository) CreateConfig(ctx context.Context, config *domain.OverlayConfig) error {
	displaySettingsJSON, err := json.Marshal(config.DisplaySettings)
	if err != nil {
		return err
	}

	filterSettingsJSON, err := json.Marshal(config.FilterSettings)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO overlay_configs (id, overlay_id, twitch_channel, enable_7tv, enable_bttv, enable_ffz,
		                             display_settings, filter_settings, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	_, err = r.pool.Exec(ctx, query,
		config.ID,
		config.OverlayID,
		config.TwitchChannel,
		config.Enable7TV,
		config.EnableBTTV,
		config.EnableFFZ,
		displaySettingsJSON,
		filterSettingsJSON,
		config.CreatedAt,
		config.UpdatedAt,
	)

	return err
}

func (r *postgresOverlayRepository) UpdateConfig(ctx context.Context, config *domain.OverlayConfig) error {
	displaySettingsJSON, err := json.Marshal(config.DisplaySettings)
	if err != nil {
		return err
	}

	filterSettingsJSON, err := json.Marshal(config.FilterSettings)
	if err != nil {
		return err
	}

	query := `
		UPDATE overlay_configs
		SET twitch_channel = $2, enable_7tv = $3, enable_bttv = $4, enable_ffz = $5,
		    display_settings = $6, filter_settings = $7, updated_at = $8
		WHERE overlay_id = $1
	`

	result, err := r.pool.Exec(ctx, query,
		config.OverlayID,
		config.TwitchChannel,
		config.Enable7TV,
		config.EnableBTTV,
		config.EnableFFZ,
		displaySettingsJSON,
		filterSettingsJSON,
		config.UpdatedAt,
	)

	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return errors.New("config not found")
	}

	return nil
}
