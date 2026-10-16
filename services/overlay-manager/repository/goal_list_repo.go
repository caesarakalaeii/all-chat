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
	"errors"
	"fmt"

	"github.com/caesar/all-chat/shared/alerts"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// GoalListRepository persists goal_configs and list_configs (migrations 110,
// 111; ADR-0072). One row per overlay.
type GoalListRepository struct {
	pool *pgxpool.Pool
}

// NewGoalListRepository creates a GoalListRepository.
func NewGoalListRepository(pool *pgxpool.Pool) *GoalListRepository {
	return &GoalListRepository{pool: pool}
}

// GetGoalConfig returns the overlay's goal, or nil when none is saved.
func (r *GoalListRepository) GetGoalConfig(ctx context.Context, overlayID string) (*alerts.GoalConfig, error) {
	var g alerts.GoalConfig
	err := r.pool.QueryRow(ctx, `
		SELECT metric, platform, target, starting_value, title
		FROM goal_configs WHERE overlay_id = $1`, overlayID).
		Scan(&g.Metric, &g.Platform, &g.Target, &g.StartingValue, &g.Title)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get goal config: %w", err)
	}
	return &g, nil
}

// UpsertGoalConfig saves the overlay's goal.
func (r *GoalListRepository) UpsertGoalConfig(ctx context.Context, overlayID string, g alerts.GoalConfig) error {
	if _, err := r.pool.Exec(ctx, `
		INSERT INTO goal_configs (overlay_id, metric, platform, target, starting_value, title)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (overlay_id) DO UPDATE SET
			metric = EXCLUDED.metric, platform = EXCLUDED.platform, target = EXCLUDED.target,
			starting_value = EXCLUDED.starting_value, title = EXCLUDED.title, updated_at = now()`,
		overlayID, g.Metric, g.Platform, g.Target, g.StartingValue, g.Title); err != nil {
		return fmt.Errorf("upsert goal config: %w", err)
	}
	return nil
}

// GetListConfig returns the overlay's list config, or nil when none is saved.
func (r *GoalListRepository) GetListConfig(ctx context.Context, overlayID string) (*alerts.ListConfig, error) {
	var l alerts.ListConfig
	err := r.pool.QueryRow(ctx, `
		SELECT category, "limit", title FROM list_configs WHERE overlay_id = $1`, overlayID).
		Scan(&l.Category, &l.Limit, &l.Title)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get list config: %w", err)
	}
	return &l, nil
}

// UpsertListConfig saves the overlay's list config.
func (r *GoalListRepository) UpsertListConfig(ctx context.Context, overlayID string, l alerts.ListConfig) error {
	if _, err := r.pool.Exec(ctx, `
		INSERT INTO list_configs (overlay_id, category, "limit", title)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (overlay_id) DO UPDATE SET
			category = EXCLUDED.category, "limit" = EXCLUDED."limit", title = EXCLUDED.title, updated_at = now()`,
		overlayID, l.Category, l.Limit, l.Title); err != nil {
		return fmt.Errorf("upsert list config: %w", err)
	}
	return nil
}
