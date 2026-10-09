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

// SourceRecord is one Instagram chat source row (overlay_chat_sources) as the
// listener sees it: the source IS the streamer's Instagram professional
// account, resolved through the token stored for that IG user.
type SourceRecord struct {
	SourceID   string
	OverlayID  string
	IGUserID   string // channel_id — the IG professional account id
	IGUsername string // channel_name — the IG @handle
	UserID     string // overlay owner — for the token store lookup
	StreamID   string // pinned live media id (optional override)
	IsActive   bool
	Config     map[string]interface{}
}

// PgQuerier is the subset of *pgxpool.Pool the repository uses, so tests can
// inject a fake.
type PgQuerier interface {
	Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error)
}

// Repository reads Instagram chat sources from PostgreSQL.
// Mirrors facebook-listener's channels/repository.go query shape.
type Repository struct {
	db PgQuerier
}

// NewRepository builds the repository over a pgx pool (PgQuerier keeps it
// testable with a fake).
func NewRepository(db PgQuerier) *Repository {
	return &Repository{db: db}
}

// GetActiveSources returns every Instagram source on an active overlay owned
// by a non-banned user, with the owner id callers need to resolve the token.
func (r *Repository) GetActiveSources(ctx context.Context) ([]*SourceRecord, error) {
	const query = `
		SELECT
			ocs.id,
			ocs.overlay_id,
			ocs.channel_id,
			COALESCE(ocs.channel_name, ocs.channel_id) AS channel_name,
			o.user_id,
			COALESCE(ocs.config->>'stream_id', '') AS stream_id,
			ocs.is_active,
			COALESCE(ocs.config, '{}'::jsonb) AS config
		FROM overlay_chat_sources ocs
		JOIN overlays o ON ocs.overlay_id = o.id
		JOIN users u ON o.user_id = u.id
		WHERE ocs.platform = 'instagram'
		  AND o.is_active = true
		  AND u.is_banned = false
		ORDER BY ocs.created_at
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query instagram sources: %w", err)
	}
	defer rows.Close()

	sources := make([]*SourceRecord, 0)
	for rows.Next() {
		var s SourceRecord
		var rawConfig []byte
		if err := rows.Scan(&s.SourceID, &s.OverlayID, &s.IGUserID, &s.IGUsername, &s.UserID, &s.StreamID, &s.IsActive, &rawConfig); err != nil {
			return nil, fmt.Errorf("scan instagram source: %w", err)
		}
		_ = json.Unmarshal(rawConfig, &s.Config)
		sources = append(sources, &s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate instagram sources: %w", err)
	}
	return sources, nil
}

// ActivateSource heartbeats the source so the 24h cleanup doesn't drop it
// (ADR-0032: every actively-polled source is kept fresh by its listener).
func (r *Repository) ActivateSource(ctx context.Context, igUserID string) error {
	const query = `UPDATE overlay_chat_sources SET is_active = true, updated_at = NOW() WHERE platform = 'instagram' AND channel_id = $1`
	_, err := r.db.Exec(ctx, query, igUserID)
	return err
}

// DeactivateSource marks the source inactive when the listener stops polling.
func (r *Repository) DeactivateSource(ctx context.Context, igUserID string) error {
	const query = `UPDATE overlay_chat_sources SET is_active = false, updated_at = NOW() WHERE platform = 'instagram' AND channel_id = $1 AND updated_at < NOW() - INTERVAL '5 minutes'`
	_, err := r.db.Exec(ctx, query, igUserID)
	return err
}

// UnresolvedTokenError reports that the user's IG token is missing, expired
// or undecryptable — a credential problem (the streamer must reconnect) rather
// than a discovery problem (not live right now). Instagram, unlike Facebook,
// has an expiry path: the row carries expires_at and an expired 60-day token
// must be replaced by re-consent.
type UnresolvedTokenError struct{ IGUserID string }

func (e *UnresolvedTokenError) Error() string {
	return fmt.Sprintf("instagram: no usable token for ig user %s", e.IGUserID)
}

// TokenStore resolves the decrypted token for a poll.
type TokenStore interface {
	GetToken(ctx context.Context, userID, igUserID string) (string, error)
}

// PollerState is persisted per source: the live media being watched and the
// last-seen comment id. Redis-cached, mirroring facebook-listener's
// stream-state store — but the cursor is an id, not a timestamp, because the
// live_comments edge paginates by comment_id and IG comments cannot be
// filtered by timestamp.
type PollerState struct {
	LiveMediaID   string    `json:"live_media_id"`
	LastCommentID string    `json:"last_comment_id"`
	UpdatedAt     time.Time `json:"updated_at"`
}
