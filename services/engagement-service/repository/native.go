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
)

// NativeOutcomeInput is one mirrored Twitch choice/outcome tally (issue #523,
// task H). Twitch owns the individual votes/wagers, so only aggregates arrive;
// they land in poll_options.mirror_votes / prediction_outcomes.mirror_points.
type NativeOutcomeInput struct {
	ExternalID string
	Idx        int
	Label      string
	Color      string
	Votes      int64
	Points     int64
	Entrants   int64
}

// UpsertNativePoll mirrors a Twitch-native poll for one overlay, keyed by
// (overlay_id, source='twitch_native', external_id). It inserts the poll + its
// options on the first event and updates state/tallies on later events, all in
// one transaction.
//
// EventSub does not guarantee ordered delivery (and a transiently-failed webhook
// is redelivered later), so the state update is MONOTONIC: the guard blocks a
// backward transition (a late progress can't reopen a CLOSED poll). Tallies use
// GREATEST so a stale, smaller count can't overwrite a newer one, and timestamps
// COALESCE so a later event without a deadline can't null an earlier one. When
// the guard blocks a stale event, the upsert is a no-op and (nil, nil) is
// returned so the caller skips broadcasting.
func (r *Repository) UpsertNativePoll(ctx context.Context, overlayID uuid.UUID, externalID, question, state string, outcomes []NativeOutcomeInput, endsAt, closedAt *time.Time) (*models.Poll, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin upsert native poll: %w", err)
	}
	defer tx.Rollback(ctx)

	var pollID uuid.UUID
	err = tx.QueryRow(ctx,
		`INSERT INTO polls (overlay_id, source, external_id, question, state, allow_change, ends_at, closed_at)
		 VALUES ($1, $2, $3, $4, $5, FALSE, $6, $7)
		 ON CONFLICT (overlay_id, source, external_id) WHERE external_id IS NOT NULL
		 DO UPDATE SET question = EXCLUDED.question, state = EXCLUDED.state,
		               ends_at = COALESCE(EXCLUDED.ends_at, polls.ends_at),
		               closed_at = COALESCE(EXCLUDED.closed_at, polls.closed_at)
		 WHERE (CASE polls.state WHEN 'CLOSED' THEN 1 ELSE 0 END)
		    <= (CASE EXCLUDED.state WHEN 'CLOSED' THEN 1 ELSE 0 END)
		 RETURNING id`,
		overlayID, models.SourceTwitchNative, externalID, question, state, endsAt, closedAt).Scan(&pollID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil // stale/out-of-order event: guard blocked a state regression
	}
	if err != nil {
		return nil, fmt.Errorf("upsert native poll: %w", err)
	}

	for _, o := range outcomes {
		// Options are keyed by (poll_id, idx); Twitch preserves choice order so idx
		// is stable across events. GREATEST keeps the running tally monotonic under
		// out-of-order delivery (Twitch vote counts only grow within a round).
		if _, err := tx.Exec(ctx,
			`INSERT INTO poll_options (poll_id, idx, label, mirror_votes)
			 VALUES ($1, $2, $3, $4)
			 ON CONFLICT (poll_id, idx)
			 DO UPDATE SET label = EXCLUDED.label,
			               mirror_votes = GREATEST(poll_options.mirror_votes, EXCLUDED.mirror_votes)`,
			pollID, o.Idx, o.Label, o.Votes); err != nil {
			return nil, fmt.Errorf("upsert native poll option: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit upsert native poll: %w", err)
	}
	return r.GetPoll(ctx, pollID)
}

// UpsertNativePrediction mirrors a Twitch-native prediction for one overlay,
// keyed by (overlay_id, source='twitch_native', external_id). Tallies land in
// prediction_outcomes.mirror_points / .mirror_entrants. winningExternalID (the
// Twitch outcome id) is resolved to the mirrored outcome row on resolution.
//
// Like UpsertNativePoll, the state update is MONOTONIC (CREATED<ACTIVE<LOCKED<
// RESOLVED|CANCELED) so a late/redelivered event can't downgrade a locked round;
// tallies use GREATEST and timestamps COALESCE. The two terminal states share the
// top rank, so an extra guard makes them mutually ABSORBING: once RESOLVED or
// CANCELED, a differing terminal (or any other) state can't overwrite it — otherwise
// a redelivered CANCELED could laterally flip a RESOLVED round's outcome (L-C1). A
// same-state redelivery still passes (allowing a late tally correction). A blocked
// stale event returns (nil, nil) so the caller skips broadcasting.
//
// One EXCEPTION to the absorbing guard: a SYNTHETIC cancel written by the stale-sweep
// (sweep_canceled=TRUE) is NOT authoritative — a LOCKED Twitch round has no forced-
// resolution deadline, so the genuine terminal may still arrive. A genuine terminal
// (any real mirror event carries sweep_canceled=FALSE) is therefore allowed to override
// a synthetic cancel, and the row is re-tagged authoritative (P2-4). Genuine
// RESOLVED<->CANCELED lateral flips stay blocked.
func (r *Repository) UpsertNativePrediction(ctx context.Context, overlayID uuid.UUID, externalID, title, state, winningExternalID string, outcomes []NativeOutcomeInput, autoLockAt, lockedAt, resolvedAt *time.Time) (*models.Prediction, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin upsert native prediction: %w", err)
	}
	defer tx.Rollback(ctx)

	var pid uuid.UUID
	err = tx.QueryRow(ctx,
		`INSERT INTO predictions (overlay_id, source, external_id, title, state, auto_lock_at, locked_at, resolved_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 ON CONFLICT (overlay_id, source, external_id) WHERE external_id IS NOT NULL
		 DO UPDATE SET title = EXCLUDED.title, state = EXCLUDED.state,
		               auto_lock_at = COALESCE(EXCLUDED.auto_lock_at, predictions.auto_lock_at),
		               locked_at = COALESCE(EXCLUDED.locked_at, predictions.locked_at),
		               resolved_at = COALESCE(EXCLUDED.resolved_at, predictions.resolved_at),
		               sweep_canceled = FALSE
		 WHERE (CASE predictions.state WHEN 'CREATED' THEN 0 WHEN 'ACTIVE' THEN 1 WHEN 'LOCKED' THEN 2 ELSE 3 END)
		    <= (CASE EXCLUDED.state WHEN 'CREATED' THEN 0 WHEN 'ACTIVE' THEN 1 WHEN 'LOCKED' THEN 2 ELSE 3 END)
		   AND NOT (
		       predictions.state IN ('RESOLVED','CANCELED')
		       AND EXCLUDED.state <> predictions.state
		       -- but a genuine terminal MAY override a synthetic (sweep) cancel:
		       AND NOT (predictions.sweep_canceled AND EXCLUDED.state IN ('RESOLVED','CANCELED'))
		   )
		 RETURNING id`,
		overlayID, models.SourceTwitchNative, externalID, title, state, autoLockAt, lockedAt, resolvedAt).Scan(&pid)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil // stale/out-of-order event: guard blocked a state regression
	}
	if err != nil {
		return nil, fmt.Errorf("upsert native prediction: %w", err)
	}

	// Map Twitch outcome external ids → our outcome UUIDs so we can set the winner.
	winnerUUID := map[string]uuid.UUID{}
	for _, o := range outcomes {
		var oid uuid.UUID
		var color *string
		if o.Color != "" {
			color = &o.Color
		}
		if err := tx.QueryRow(ctx,
			`INSERT INTO prediction_outcomes (prediction_id, idx, label, color, mirror_points, mirror_entrants)
			 VALUES ($1, $2, $3, $4, $5, $6)
			 ON CONFLICT (prediction_id, idx)
			 DO UPDATE SET label = EXCLUDED.label, color = EXCLUDED.color,
			               mirror_points = GREATEST(prediction_outcomes.mirror_points, EXCLUDED.mirror_points),
			               mirror_entrants = GREATEST(prediction_outcomes.mirror_entrants, EXCLUDED.mirror_entrants)
			 RETURNING id`,
			pid, o.Idx, o.Label, color, o.Points, o.Entrants).Scan(&oid); err != nil {
			return nil, fmt.Errorf("upsert native prediction outcome: %w", err)
		}
		winnerUUID[o.ExternalID] = oid
	}

	if winningExternalID != "" {
		if oid, ok := winnerUUID[winningExternalID]; ok {
			if _, err := tx.Exec(ctx,
				`UPDATE predictions SET winning_outcome_id = $2 WHERE id = $1`, pid, oid); err != nil {
				return nil, fmt.Errorf("set native winning outcome: %w", err)
			}
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit upsert native prediction: %w", err)
	}
	return r.GetPrediction(ctx, pid)
}

// HasLiveNativePoll reports whether the overlay currently has an ACTIVE mirrored
// Twitch poll. Used by the all-chat create endpoint to yield precedence to Twitch.
func (r *Repository) HasLiveNativePoll(ctx context.Context, overlayID uuid.UUID) (bool, error) {
	var exists bool
	if err := r.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM polls
		   WHERE overlay_id = $1 AND source = $2 AND state = 'ACTIVE')`,
		overlayID, models.SourceTwitchNative).Scan(&exists); err != nil {
		return false, fmt.Errorf("check live native poll: %w", err)
	}
	return exists, nil
}

// HasLiveNativePrediction reports whether the overlay has a live (ACTIVE or
// LOCKED) mirrored Twitch prediction.
func (r *Repository) HasLiveNativePrediction(ctx context.Context, overlayID uuid.UUID) (bool, error) {
	var exists bool
	if err := r.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM predictions
		   WHERE overlay_id = $1 AND source = $2 AND state IN ('ACTIVE','LOCKED'))`,
		overlayID, models.SourceTwitchNative).Scan(&exists); err != nil {
		return false, fmt.Errorf("check live native prediction: %w", err)
	}
	return exists, nil
}

// ForceCloseStaleNativePolls force-closes mirrored Twitch polls stuck ACTIVE past
// ttl, returning the affected (id, overlay_id) so the caller can broadcast the
// terminal frame and clear active flags. Native rows have no terminal sweep of
// their own (CloseExpired is source='allchat' only), so a never-delivered
// channel.poll.end (revoked sub, rotated secret, source removed mid-round) would
// otherwise strand the row live forever and permanently 409-block All-Chat polls on
// the overlay (HasLiveNativePoll). created_at is the only always-present age column
// for native rows (ends_at/closed_at are populated only when Twitch supplies them).
func (r *Repository) ForceCloseStaleNativePolls(ctx context.Context, ttl time.Duration) ([]PollRef, error) {
	rows, err := r.db.Query(ctx,
		`UPDATE polls SET state = 'CLOSED', closed_at = NOW()
		 WHERE source = $1 AND state = 'ACTIVE' AND created_at <= NOW() - make_interval(secs => $2)
		 RETURNING id, overlay_id`,
		models.SourceTwitchNative, int(ttl.Seconds()))
	if err != nil {
		return nil, fmt.Errorf("force-close stale native polls: %w", err)
	}
	defer rows.Close()
	var out []PollRef
	for rows.Next() {
		var ref PollRef
		if err := rows.Scan(&ref.PollID, &ref.OverlayID); err != nil {
			return nil, fmt.Errorf("scan stale native poll ref: %w", err)
		}
		out = append(out, ref)
	}
	return out, rows.Err()
}

// ForceCloseStaleNativePredictions cancels mirrored Twitch predictions stuck
// ACTIVE/LOCKED past ttl — CANCELED, not RESOLVED, because there is no known winner.
// Native rows never touch viewer_points, so canceling a mirror has NO points-economy
// side effect (no payout to reconcile, no refund). See ForceCloseStaleNativePolls.
//
// The cancel is tagged sweep_canceled=TRUE so it is a SYNTHETIC terminal, not a real
// Twitch outcome: a LOCKED Twitch prediction has no forced-resolution deadline, so the
// genuine channel.prediction.end may still arrive after this sweep fires. UpsertNative-
// Prediction's absorbing guard lets a later genuine terminal override a synthetic cancel,
// so the real winner still displays instead of a permanent wrong CANCELED (P2-4).
func (r *Repository) ForceCloseStaleNativePredictions(ctx context.Context, ttl time.Duration) ([]PredictionRef, error) {
	rows, err := r.db.Query(ctx,
		`UPDATE predictions SET state = 'CANCELED', resolved_at = NOW(), sweep_canceled = TRUE
		 WHERE source = $1 AND state IN ('ACTIVE','LOCKED') AND created_at <= NOW() - make_interval(secs => $2)
		 RETURNING id, overlay_id`,
		models.SourceTwitchNative, int(ttl.Seconds()))
	if err != nil {
		return nil, fmt.Errorf("force-close stale native predictions: %w", err)
	}
	defer rows.Close()
	var out []PredictionRef
	for rows.Next() {
		var ref PredictionRef
		if err := rows.Scan(&ref.PredictionID, &ref.OverlayID); err != nil {
			return nil, fmt.Errorf("scan stale native prediction ref: %w", err)
		}
		out = append(out, ref)
	}
	return out, rows.Err()
}
