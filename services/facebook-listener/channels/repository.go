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
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// SourceRecord is one Facebook chat source row (overlay_chat_sources) as the
// listener sees it: the source IS the streamer's Page, resolved through their
// stored Page token.
type SourceRecord struct {
	SourceID    string
	OverlayID   string
	PageID      string // channel_id — the Facebook Page id
	PageName    string // channel_name — the Facebook Page name
	UserID      string // overlay owner — for the token store lookup
	StreamID    string // pinned live video id (optional override)
	StreamSince string // optional config: only comments newer than this
	IsActive    bool
	Config      map[string]interface{}
}

// PgQuerier is the subset of *pgxpool.Pool the repository uses, so tests can
// inject a fake.
type PgQuerier interface {
	Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error)
}

// Repository reads Facebook chat sources from PostgreSQL.
// Mirrors kick-listener's channels/repository.go query shape.
type Repository struct {
	db PgQuerier
}

// NewRepository builds the repository over a pgx pool ( PgQuerier keeps it
// testable with a fake).
func NewRepository(db PgQuerier) *Repository {
	return &Repository{db: db}
}

// GetActiveSources returns every Facebook source on an active overlay owned by
// a non-banned user, with the owner id callers need to resolve the Page token.
func (r *Repository) GetActiveSources(ctx context.Context) ([]*SourceRecord, error) {
	const query = `
		SELECT
			ocs.id,
			ocs.overlay_id,
			ocs.channel_id,
			COALESCE(ocs.channel_name, ocs.channel_id) AS channel_name,
			o.user_id,
			COALESCE(ocs.config->>'stream_id', '') AS stream_id,
			COALESCE(ocs.config->>'stream_since', '') AS stream_since,
			ocs.is_active,
			COALESCE(ocs.config, '{}'::jsonb) AS config
		FROM overlay_chat_sources ocs
		JOIN overlays o ON ocs.overlay_id = o.id
		JOIN users u ON o.user_id = u.id
		WHERE ocs.platform = 'facebook'
		  AND o.is_active = true
		  AND u.is_banned = false
		ORDER BY ocs.created_at
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query facebook sources: %w", err)
	}
	defer rows.Close()

	sources := make([]*SourceRecord, 0)
	for rows.Next() {
		var s SourceRecord
		var rawConfig []byte
		if err := rows.Scan(&s.SourceID, &s.OverlayID, &s.PageID, &s.PageName, &s.UserID, &s.StreamID, &s.StreamSince, &s.IsActive, &rawConfig); err != nil {
			return nil, fmt.Errorf("scan facebook source: %w", err)
		}
		_ = json.Unmarshal(rawConfig, &s.Config)
		sources = append(sources, &s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate facebook sources: %w", err)
	}
	return sources, nil
}

// ActivateSource heartbeats the source so the 24h cleanup doesn't drop it
// (ADR-0032: every actively-polled source is kept fresh by its listener).
func (r *Repository) ActivateSource(ctx context.Context, pageID string) error {
	const query = `UPDATE overlay_chat_sources SET is_active = true, updated_at = NOW() WHERE platform = 'facebook' AND channel_id = $1`
	_, err := r.db.Exec(ctx, query, pageID)
	return err
}

// DeactivateSource marks the source inactive when the listener stops polling.
func (r *Repository) DeactivateSource(ctx context.Context, pageID string) error {
	const query = `UPDATE overlay_chat_sources SET is_active = false, updated_at = NOW() WHERE platform = 'facebook' AND channel_id = $1 AND updated_at < NOW() - INTERVAL '5 minutes'`
	_, err := r.db.Exec(ctx, query, pageID)
	return err
}

// UnresolvedTokenError reports that the user's Page token is missing/invalid,
// which is a credential problem (streamer must reconnect) rather than a
// discovery problem (Page not live right now) — listeners log and skip.
type UnresolvedTokenError struct{ PageID string }

func (e *UnresolvedTokenError) Error() string {
	return fmt.Sprintf("facebook: no usable page token for page %s", e.PageID)
}

// TokenStore resolves the encrypted Page token for a poll.
type TokenStore interface {
	GetPageToken(ctx context.Context, userID, pageID string) (string, error)
}

// PollerState is persisted per source: the live video being watched and the
// newest comment timestamp. Redis-cached, mirroring youtube-listener's
// stream-state store.
type PollerState struct {
	LiveVideoID string    `json:"live_video_id"`
	LastSince   string    `json:"last_since"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// StateStore persists poller state across restarts/replicas.
type StateStore interface {
	Get(ctx context.Context, sourceID string) (*PollerState, error)
	Set(ctx context.Context, sourceID string, state *PollerState) error
	Clear(ctx context.Context, sourceID string) error
}
