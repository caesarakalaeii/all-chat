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

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// ActiveInstance is one demanded Owncast source row. The "channel" is the
// instance URL (ADR-0058), stored in channel_id.
type ActiveInstance struct {
	SourceID    string // UUID from overlay_chat_sources.id
	OverlayID   string
	InstanceURL string // normalized Owncast base URL
	IsActive    bool
}

// queryExecutor matches the kick/youtube repository pattern; pgxpool.Pool
// implements it and tests can substitute a fake.
type queryExecutor interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// Repository handles database operations for Owncast instances.
type Repository struct {
	db     queryExecutor
	logger *zap.Logger
}

// NewRepository creates a new instance repository.
func NewRepository(db *pgxpool.Pool, logger *zap.Logger) *Repository {
	return &Repository{db: db, logger: logger}
}

// GetActiveInstances retrieves all Owncast sources of active overlays. Like
// the Kick repository, it filters by overlay.is_active only: source.is_active
// is a status flag owned by the listener, not an eligibility filter.
func (r *Repository) GetActiveInstances(ctx context.Context) ([]*ActiveInstance, error) {
	query := `
		SELECT
			ocs.id as source_id,
			ocs.overlay_id,
			COALESCE(ocs.channel_handle, ocs.channel_id) as instance_url,
			ocs.is_active
		FROM overlay_chat_sources ocs
		JOIN overlays o ON ocs.overlay_id = o.id
		WHERE ocs.platform = 'owncast'
		  AND o.is_active = true
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query active owncast instances: %w", err)
	}
	defer rows.Close()

	var out []*ActiveInstance
	for rows.Next() {
		var a ActiveInstance
		if err := rows.Scan(&a.SourceID, &a.OverlayID, &a.InstanceURL, &a.IsActive); err != nil {
			r.logger.Error("Failed to scan owncast instance row", zap.Error(err))
			continue
		}
		out = append(out, &a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating owncast instance rows: %w", err)
	}

	r.logger.Info("Retrieved active Owncast instances", zap.Int("count", len(out)))
	return out, nil
}

// SetSourceActive flips is_active for every Owncast source matching the
// instance URL — same note-on-change pattern as kick-listener so the
// chat_source_changes NOTIFY trigger does not storm.
func (r *Repository) SetSourceActive(ctx context.Context, instanceURL string, isActive bool) error {
	query := `
		UPDATE overlay_chat_sources
		SET is_active = $1, updated_at = NOW()
		WHERE platform = 'owncast'
		  AND (channel_handle = $2 OR channel_id = $2)
		  AND is_active != $1
	`
	result, err := r.db.Exec(ctx, query, isActive, instanceURL)
	if err != nil {
		r.logger.Error("Failed to update owncast source status",
			zap.String("instance", instanceURL),
			zap.Bool("is_active", isActive),
			zap.Error(err),
		)
		return fmt.Errorf("failed to update owncast source status: %w", err)
	}
	if result.RowsAffected() == 0 {
		r.logger.Debug("No owncast sources updated (may have been removed)",
			zap.String("instance", instanceURL))
	}
	return nil
}
