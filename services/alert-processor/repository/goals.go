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
)

// PolledGoal is one goal overlay whose current value is fetched from a
// platform API (ADR-0072): followers on Twitch, subscribers on YouTube.
type PolledGoal struct {
	OverlayID   string
	OwnerUserID string // whose linked credential the poll uses (ADR-0016)
	Metric      string
	Platform    string
	ChannelID   string // the overlay's source on Platform
	Target      int64
	Title       string
}

// GoalConfig returns the overlay's goal_configs row, or nil when none is saved.
func (r *Repository) GoalConfig(ctx context.Context, overlayID string) (*alerts.GoalConfig, error) {
	rows, err := r.db.Query(ctx, `
		SELECT metric, platform, target, starting_value, title
		FROM goal_configs
		WHERE overlay_id = $1`, overlayID)
	if err != nil {
		return nil, fmt.Errorf("failed to query goal config: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, rows.Err()
	}
	var g alerts.GoalConfig
	if err := rows.Scan(&g.Metric, &g.Platform, &g.Target, &g.StartingValue, &g.Title); err != nil {
		return nil, fmt.Errorf("failed to scan goal config: %w", err)
	}
	return &g, nil
}

// PolledGoals lists every active goal overlay with an API-polled metric and an
// active source on the goal's platform (its oldest one when several).
// Whether the overlay is connected is decided by the poller, from Redis.
func (r *Repository) PolledGoals(ctx context.Context) ([]PolledGoal, error) {
	rows, err := r.db.Query(ctx, `
		SELECT o.id::text, o.user_id::text, gc.metric, gc.platform, src.channel_id, gc.target, gc.title
		FROM goal_configs gc
		JOIN overlays o ON o.id = gc.overlay_id
		JOIN LATERAL (
			SELECT ocs.channel_id
			FROM overlay_chat_sources ocs
			WHERE ocs.overlay_id = o.id
			  AND ocs.platform = gc.platform
			  AND ocs.is_active = true
			ORDER BY ocs.created_at
			LIMIT 1
		) src ON true
		WHERE o.is_active = true
		  AND o.overlay_type = 'goal'
		  AND gc.metric IN ('followers','subscribers')`)
	if err != nil {
		return nil, fmt.Errorf("failed to query polled goals: %w", err)
	}
	defer rows.Close()

	var goals []PolledGoal
	for rows.Next() {
		var g PolledGoal
		if err := rows.Scan(&g.OverlayID, &g.OwnerUserID, &g.Metric, &g.Platform, &g.ChannelID, &g.Target, &g.Title); err != nil {
			return nil, fmt.Errorf("failed to scan polled goal: %w", err)
		}
		goals = append(goals, g)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating polled goals: %w", err)
	}
	return goals, nil
}
