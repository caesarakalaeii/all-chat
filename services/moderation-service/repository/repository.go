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

// Package repository holds the read-only authorization queries the moderation
// service runs against the shared database. Per ADR-0004 the few queries it needs
// are duplicated here rather than sharing an interface package; they mirror
// services/api-gateway/subscription/repository.go.
package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// sharedOverlayPlatform is excluded from moderation everywhere: a shared overlay
// displays another streamer's chat, and the recipient must never moderate the
// original streamer's channel (owner-only authorization, ADR-0017).
const sharedOverlayPlatform = "shared_overlay"

// Repository runs the moderation service's authorization queries.
type Repository struct {
	db *pgxpool.Pool
}

// New creates a Repository over the given pool.
func New(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// VerifyOverlayOwnership reports whether userID owns overlayID. Mirrors
// api-gateway/subscription/repository.go so the moderation service makes its own
// authoritative ownership check rather than trusting the gateway proxy.
func (r *Repository) VerifyOverlayOwnership(ctx context.Context, overlayID, userID string) (bool, error) {
	const query = `SELECT EXISTS(SELECT 1 FROM overlays WHERE id = $1 AND user_id = $2)`
	var exists bool
	if err := r.db.QueryRow(ctx, query, overlayID, userID).Scan(&exists); err != nil {
		return false, fmt.Errorf("verify overlay ownership: %w", err)
	}
	return exists, nil
}

// Source is a moderatable chat source on an overlay.
type Source struct {
	Platform    string
	ChannelID   string
	ChannelName string
}

// ListModeratableSources returns the overlay's chat sources eligible for
// moderation — every configured source except shared_overlay. Whether each is
// actually moderatable (platform support + granted scopes) is decided by the
// capability handler; this only reports which real channels the overlay carries.
func (r *Repository) ListModeratableSources(ctx context.Context, overlayID string) ([]Source, error) {
	const query = `
		SELECT platform, channel_id, channel_name
		FROM overlay_chat_sources
		WHERE overlay_id = $1 AND platform <> $2
		ORDER BY platform, channel_name`
	rows, err := r.db.Query(ctx, query, overlayID, sharedOverlayPlatform)
	if err != nil {
		return nil, fmt.Errorf("list moderatable sources: %w", err)
	}
	defer rows.Close()

	sources := make([]Source, 0)
	for rows.Next() {
		var s Source
		if err := rows.Scan(&s.Platform, &s.ChannelID, &s.ChannelName); err != nil {
			return nil, fmt.Errorf("scan source: %w", err)
		}
		sources = append(sources, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sources: %w", err)
	}
	return sources, nil
}

// IsUserPremium reports whether the user has premium access. Used by the moderation
// feature gate (ADR-0008): when the gate is premium-only, only premium users are in
// the rollout cohort. Mirrors shared/middleware/premium.go's query.
func (r *Repository) IsUserPremium(ctx context.Context, userID string) (bool, error) {
	const query = `SELECT is_premium FROM users WHERE id = $1`
	var isPremium bool
	if err := r.db.QueryRow(ctx, query, userID).Scan(&isPremium); err != nil {
		return false, fmt.Errorf("check user premium: %w", err)
	}
	return isPremium, nil
}

// IsModeratableSource reports whether (platform, channelID) is a real, non-shared
// source on the overlay. This gate prevents moderating a channel that is not on the
// overlay and blocks shared_overlay sources outright.
func (r *Repository) IsModeratableSource(ctx context.Context, overlayID, platform, channelID string) (bool, error) {
	const query = `
		SELECT EXISTS(
			SELECT 1 FROM overlay_chat_sources
			WHERE overlay_id = $1 AND platform = $2 AND channel_id = $3 AND platform <> $4
		)`
	var exists bool
	if err := r.db.QueryRow(ctx, query, overlayID, platform, channelID, sharedOverlayPlatform).Scan(&exists); err != nil {
		return false, fmt.Errorf("check moderatable source: %w", err)
	}
	return exists, nil
}
