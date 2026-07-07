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
	"time"

	"github.com/caesar/all-chat/services/engagement-service/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// isUniqueViolation reports whether err is a Postgres unique_violation (23505),
// used to map the partial-unique "one active engagement per overlay" indexes to
// ErrConflict.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// CreatePoll creates an ACTIVE All-Chat-native poll with its options (1-based idx).
// Returns ErrConflict if the overlay already has an active All-Chat poll.
func (r *Repository) CreatePoll(ctx context.Context, overlayID uuid.UUID, question string, options []string, allowChange bool, endsAt *time.Time) (*models.Poll, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin create poll: %w", err)
	}
	defer tx.Rollback(ctx)

	var pollID uuid.UUID
	err = tx.QueryRow(ctx,
		`INSERT INTO polls (overlay_id, source, question, state, allow_change, ends_at)
		 VALUES ($1, $2, $3, 'ACTIVE', $4, $5) RETURNING id`,
		overlayID, models.SourceAllChat, question, allowChange, endsAt).Scan(&pollID)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrConflict
		}
		return nil, fmt.Errorf("insert poll: %w", err)
	}

	poll := &models.Poll{
		ID: pollID, OverlayID: overlayID, Source: models.SourceAllChat,
		Question: question, State: models.PollActive, AllowChange: allowChange, EndsAt: endsAt,
	}
	for i, label := range options {
		var optID uuid.UUID
		if err := tx.QueryRow(ctx,
			`INSERT INTO poll_options (poll_id, idx, label) VALUES ($1, $2, $3) RETURNING id`,
			pollID, i+1, label).Scan(&optID); err != nil {
			return nil, fmt.Errorf("insert poll option: %w", err)
		}
		poll.Options = append(poll.Options, models.PollOption{ID: optID, Idx: i + 1, Label: label})
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit create poll: %w", err)
	}
	return poll, nil
}

// scanPoll loads a poll header (without options) using q.
func (r *Repository) scanPoll(ctx context.Context, q dbtx, where string, args ...any) (*models.Poll, error) {
	var p models.Poll
	err := q.QueryRow(ctx,
		`SELECT id, overlay_id, source, external_id, question, state, allow_change, created_at, ends_at, closed_at
		 FROM polls WHERE `+where, args...).
		Scan(&p.ID, &p.OverlayID, &p.Source, &p.ExternalID, &p.Question, &p.State, &p.AllowChange,
			&p.CreatedAt, &p.EndsAt, &p.ClosedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan poll: %w", err)
	}
	opts, err := r.loadPollOptions(ctx, q, p.ID)
	if err != nil {
		return nil, err
	}
	p.Options = opts
	return &p, nil
}

// loadPollOptions returns options with live vote tallies, ordered by idx. The
// tally is the All-Chat vote count plus mirror_votes: an All-Chat poll only has
// the former (mirror_votes = 0), a Twitch-native poll only the latter (no
// poll_votes rows), so the sum is exact for both sources.
func (r *Repository) loadPollOptions(ctx context.Context, q dbtx, pollID uuid.UUID) ([]models.PollOption, error) {
	rows, err := q.Query(ctx,
		`SELECT o.id, o.idx, o.label, COALESCE(v.cnt, 0) + o.mirror_votes
		 FROM poll_options o
		 LEFT JOIN (SELECT option_id, COUNT(*) AS cnt FROM poll_votes WHERE poll_id = $1 GROUP BY option_id) v
		        ON v.option_id = o.id
		 WHERE o.poll_id = $1 ORDER BY o.idx`, pollID)
	if err != nil {
		return nil, fmt.Errorf("load poll options: %w", err)
	}
	defer rows.Close()
	var opts []models.PollOption
	for rows.Next() {
		var o models.PollOption
		if err := rows.Scan(&o.ID, &o.Idx, &o.Label, &o.Votes); err != nil {
			return nil, fmt.Errorf("scan poll option: %w", err)
		}
		opts = append(opts, o)
	}
	return opts, rows.Err()
}

// GetActivePoll returns the overlay's active All-Chat poll, or ErrNotFound. This
// is the write-path query: the chat-command consumer and the viewer's private
// state resolve the poll to record/reflect an All-Chat vote, which never applies
// to a mirrored Twitch poll — so it stays source='allchat' only. Public display
// uses GetActiveDisplayPoll instead.
func (r *Repository) GetActivePoll(ctx context.Context, overlayID uuid.UUID) (*models.Poll, error) {
	return r.scanPoll(ctx, r.db, `overlay_id = $1 AND state = 'ACTIVE' AND source = 'allchat'`, overlayID)
}

// displayGraceSeconds is how long a just-ended round keeps being served by the
// public display queries (GetActiveDisplayPoll / GetActiveDisplayPrediction) after
// it closes/resolves, so the OBS widget and web page can render the final result —
// the on-stream "who won" reveal — before the widget clears. A still-live round
// always outranks a terminal one within the window (see the ORDER BY).
const displayGraceSeconds = 20

// GetActiveDisplayPoll returns the overlay's active poll of EITHER source for
// public rendering (OBS widgets, web page, monitor), plus a poll that closed within
// the last displayGraceSeconds so the final tally is shown before it clears. If an
// All-Chat and a mirrored Twitch poll are somehow live at once — the create-time
// 409 blocks a NEW All-Chat poll while a native one is live, but a native poll can
// begin on Twitch after an All-Chat poll is already running — the All-Chat poll
// wins the display so the owner can still close it from the control panel; once it
// ends, the native poll shows. A poll has no points at stake, but keeping the rule
// identical to predictions avoids a surprising per-type difference.
func (r *Repository) GetActiveDisplayPoll(ctx context.Context, overlayID uuid.UUID) (*models.Poll, error) {
	var id uuid.UUID
	err := r.db.QueryRow(ctx,
		`SELECT id FROM polls
		 WHERE overlay_id = $1
		   AND (state = 'ACTIVE'
		        OR (state = 'CLOSED' AND closed_at IS NOT NULL AND closed_at > NOW() - make_interval(secs => $2)))
		 ORDER BY (state = 'ACTIVE') DESC, (source = 'allchat') DESC, created_at DESC LIMIT 1`,
		overlayID, displayGraceSeconds).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get active display poll: %w", err)
	}
	return r.GetPoll(ctx, id)
}

// GetPoll returns a poll by id.
func (r *Repository) GetPoll(ctx context.Context, pollID uuid.UUID) (*models.Poll, error) {
	return r.scanPoll(ctx, r.db, `id = $1`, pollID)
}

// GetPollForOverlay returns a poll by id scoped to an overlay. A id owned by a
// different overlay (or missing) yields ErrNotFound → 404, so an owner of overlay A
// cannot read or close a poll on overlay B.
func (r *Repository) GetPollForOverlay(ctx context.Context, pollID, overlayID uuid.UUID) (*models.Poll, error) {
	return r.scanPoll(ctx, r.db, `id = $1 AND overlay_id = $2`, pollID, overlayID)
}

// ClosePoll transitions an ACTIVE poll to CLOSED (guarded), scoped to overlayID so
// a caller can only close a poll on an overlay they own. Returns the poll (idempotent
// close). source='allchat' only: a mirrored twitch_native poll is read-only (Twitch
// owns its lifecycle), and closing it here would not merely flip the mirror once — via
// UpsertNativePoll's monotonic CLOSED>ACTIVE guard, every subsequent Twitch tally
// event would then be dropped, permanently freezing the mirror. Matches the source
// guard on the prediction lifecycle siblings.
func (r *Repository) ClosePoll(ctx context.Context, pollID, overlayID uuid.UUID) (*models.Poll, error) {
	tag, err := r.db.Exec(ctx,
		`UPDATE polls SET state = 'CLOSED', closed_at = NOW() WHERE id = $1 AND overlay_id = $2 AND state = 'ACTIVE' AND source = 'allchat'`, pollID, overlayID)
	if err != nil {
		return nil, fmt.Errorf("close poll: %w", err)
	}
	if tag.RowsAffected() == 0 {
		// Already closed, not found, or not this overlay's poll — surface current state
		// only when it belongs to this overlay (else ErrNotFound → 404).
		return r.GetPollForOverlay(ctx, pollID, overlayID)
	}
	return r.GetPollForOverlay(ctx, pollID, overlayID)
}

// RecordVote records (or changes) a viewer's vote by option index. overlayID is
// the overlay from the request path (web) or the chat-command's resolved overlay:
// the vote is bound to it, so a poll owned by a different overlay is rejected. Returns
// accepted=false without error when the option index is invalid, the poll is not
// active, or the overlay/source does not match (the chat path must not error-spam).
// Idempotent per (poll, viewer).
//
// seq is a monotonic per-vote ordering token (chat: the Redis stream entry's epoch-ms;
// web: request-time epoch-ms). A vote CHANGE is applied only when seq >= the stored seq,
// so a 5m-drained redelivery of an OLDER vote can't revert a viewer's newer choice (P3-3).
func (r *Repository) RecordVote(ctx context.Context, pollID, viewerID, overlayID uuid.UUID, optionIdx int, platform string, sourceMessageID *uuid.UUID, seq int64) (bool, error) {
	var state, source string
	var allowChange bool
	var rowOverlayID uuid.UUID
	err := r.db.QueryRow(ctx, `SELECT state, allow_change, source, overlay_id FROM polls WHERE id = $1`, pollID).Scan(&state, &allowChange, &source, &rowOverlayID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("load poll for vote: %w", err)
	}
	// Bind the vote to the request/command overlay: a poll owned by a DIFFERENT
	// overlay must not accept a vote routed via another overlay's path (cross-tenant
	// tally integrity). Mirrors Wager's overlay binding. Silent (false, no error) like
	// the source/state checks so the chat path never spams and there is no cross-tenant
	// existence oracle.
	if rowOverlayID != overlayID {
		return false, nil
	}
	// All-Chat votes must never land on a mirrored Twitch poll — its votes live on
	// Twitch and its tally comes from mirror_votes, so an inserted poll_votes row
	// would corrupt the displayed count. Mirrors the source guard in Wager. Silent
	// (return false, no error) like the state/option checks: the chat path never spams.
	if source != models.SourceAllChat {
		return false, nil
	}
	if state != models.PollActive {
		return false, nil
	}

	var optionID uuid.UUID
	err = r.db.QueryRow(ctx, `SELECT id FROM poll_options WHERE poll_id = $1 AND idx = $2`, pollID, optionIdx).Scan(&optionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil // out-of-range option: silently ignore
	}
	if err != nil {
		return false, fmt.Errorf("resolve option: %w", err)
	}

	// Guard the change on seq so a stale redelivery can't revert a newer vote (P3-3): a
	// change applies only when its ordering token is >= the stored one. A same-seq
	// redelivery still re-applies (idempotent). allow_change=false keeps first-vote-wins.
	conflict := `DO UPDATE SET option_id = EXCLUDED.option_id, platform = EXCLUDED.platform,
		source_message_id = EXCLUDED.source_message_id, seq = EXCLUDED.seq
		WHERE EXCLUDED.seq >= poll_votes.seq`
	if !allowChange {
		conflict = `DO NOTHING`
	}
	_, err = r.db.Exec(ctx,
		`INSERT INTO poll_votes (poll_id, viewer_id, option_id, platform, source_message_id, seq)
		 VALUES ($1,$2,$3,$4,$5,$6) ON CONFLICT (poll_id, viewer_id) `+conflict,
		pollID, viewerID, optionID, platform, sourceMessageID, seq)
	if err != nil {
		return false, fmt.Errorf("record vote: %w", err)
	}
	return true, nil
}

// GetViewerVote returns the option a viewer voted for on a poll, or nil.
func (r *Repository) GetViewerVote(ctx context.Context, pollID, viewerID uuid.UUID) (*uuid.UUID, error) {
	var optID uuid.UUID
	err := r.db.QueryRow(ctx, `SELECT option_id FROM poll_votes WHERE poll_id = $1 AND viewer_id = $2`, pollID, viewerID).Scan(&optID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get viewer vote: %w", err)
	}
	return &optID, nil
}

// PollRef identifies a poll and its overlay.
type PollRef struct {
	PollID    uuid.UUID
	OverlayID uuid.UUID
}

// CloseExpired closes every ACTIVE All-Chat poll whose ends_at has passed,
// returning the affected (id, overlay_id) so the caller can broadcast the final
// state and clear active flags. Restart-safe (no in-memory timers).
func (r *Repository) CloseExpired(ctx context.Context) ([]PollRef, error) {
	rows, err := r.db.Query(ctx,
		`UPDATE polls SET state = 'CLOSED', closed_at = NOW()
		 WHERE state = 'ACTIVE' AND source = 'allchat' AND ends_at IS NOT NULL AND ends_at <= NOW()
		 RETURNING id, overlay_id`)
	if err != nil {
		return nil, fmt.Errorf("close expired polls: %w", err)
	}
	defer rows.Close()
	var out []PollRef
	for rows.Next() {
		var ref PollRef
		if err := rows.Scan(&ref.PollID, &ref.OverlayID); err != nil {
			return nil, fmt.Errorf("scan closed poll ref: %w", err)
		}
		out = append(out, ref)
	}
	return out, rows.Err()
}

// TotalVotes returns the total number of votes cast on a poll.
func (r *Repository) TotalVotes(ctx context.Context, pollID uuid.UUID) (int64, error) {
	var n int64
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM poll_votes WHERE poll_id = $1`, pollID).Scan(&n); err != nil {
		return 0, fmt.Errorf("total votes: %w", err)
	}
	return n, nil
}
