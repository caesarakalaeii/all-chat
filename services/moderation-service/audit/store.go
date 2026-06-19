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

// Package audit records every moderation command to the moderation_actions table
// (migration 060) for abuse forensics and accountability. A row is written for every
// command regardless of outcome (allowed, denied, dry-run, or platform failure).
package audit

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Outcome values recorded for a moderation command.
const (
	OutcomeSuccess        = "success"         // platform accepted the action
	OutcomeDryRun         = "dry_run"         // reflect-back emitted, no platform call (no client wired)
	OutcomeDenied         = "denied"          // authorization failed (not owner, not a source, etc.)
	OutcomePlatformError  = "platform_error"  // platform API rejected/failed the action
	OutcomeReauthRequired = "reauth_required" // owner's token lacks the moderation scope; needs opt-in re-consent
	OutcomeNoCredential   = "no_credential"   // owner holds no moderator credential for the channel
)

// Entry is one audited moderation command.
type Entry struct {
	OverlayID       string
	ActorUserID     string // effective identity whose token acts (the overlay owner)
	ImpersonatedBy  string // real admin when acting under impersonation; "" otherwise
	Platform        string
	ChannelID       string
	Action          string
	TargetUserID    string // timeout/ban/unban
	TargetMessageID string // delete (native platform message id)
	Reason          string
	Outcome         string
	PlatformStatus  string // platform API status/detail, if any
}

// Store writes audit rows.
type Store struct {
	db *pgxpool.Pool
}

// New creates an audit Store over the given pool.
func New(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

// Record inserts one audit row. Optional string fields are stored as NULL when
// empty (notably impersonated_by, which is a UUID column and must be NULL — not the
// empty string — for non-impersonated actions).
func (s *Store) Record(ctx context.Context, e Entry) error {
	const query = `
		INSERT INTO moderation_actions
			(overlay_id, actor_user_id, impersonated_by, platform, channel_id, action,
			 target_user_id, target_message_id, reason, outcome, platform_status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`
	_, err := s.db.Exec(ctx, query,
		e.OverlayID,
		e.ActorUserID,
		nullIfEmpty(e.ImpersonatedBy),
		e.Platform,
		e.ChannelID,
		e.Action,
		nullIfEmpty(e.TargetUserID),
		nullIfEmpty(e.TargetMessageID),
		nullIfEmpty(e.Reason),
		e.Outcome,
		nullIfEmpty(e.PlatformStatus),
	)
	if err != nil {
		return fmt.Errorf("record moderation action: %w", err)
	}
	return nil
}

// nullIfEmpty returns nil for an empty string so the column is stored as SQL NULL.
func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
