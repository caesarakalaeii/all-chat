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

	"github.com/caesar/all-chat/services/engagement-service/engine"
	"github.com/caesar/all-chat/services/engagement-service/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// WagerResult reports the outcome of a wager attempt. Reason is a short machine
// code the web handler maps to a 4xx and the chat consumer logs silently.
type WagerResult struct {
	Accepted    bool
	Reason      string // "" on success; "not_found"|"not_active"|"bad_outcome"|"already_wagered"|"insufficient"|"native"
	NewBalance  int64
}

// CreatePrediction creates an ACTIVE All-Chat-native prediction with its outcomes.
// Returns ErrConflict if the overlay already has a live All-Chat prediction.
func (r *Repository) CreatePrediction(ctx context.Context, overlayID uuid.UUID, title string, outcomes []string, autoLockAt *time.Time) (*models.Prediction, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin create prediction: %w", err)
	}
	defer tx.Rollback(ctx)

	var pid uuid.UUID
	err = tx.QueryRow(ctx,
		`INSERT INTO predictions (overlay_id, source, title, state, auto_lock_at)
		 VALUES ($1, $2, $3, 'ACTIVE', $4) RETURNING id`,
		overlayID, models.SourceAllChat, title, autoLockAt).Scan(&pid)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrConflict
		}
		return nil, fmt.Errorf("insert prediction: %w", err)
	}

	p := &models.Prediction{
		ID: pid, OverlayID: overlayID, Source: models.SourceAllChat,
		Title: title, State: models.PredActive, AutoLockAt: autoLockAt,
	}
	for i, label := range outcomes {
		var oid uuid.UUID
		if err := tx.QueryRow(ctx,
			`INSERT INTO prediction_outcomes (prediction_id, idx, label) VALUES ($1, $2, $3) RETURNING id`,
			pid, i+1, label).Scan(&oid); err != nil {
			return nil, fmt.Errorf("insert outcome: %w", err)
		}
		p.Outcomes = append(p.Outcomes, models.PredictionOutcome{ID: oid, Idx: i + 1, Label: label})
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit create prediction: %w", err)
	}
	return p, nil
}

func (r *Repository) scanPrediction(ctx context.Context, q dbtx, where string, arg any) (*models.Prediction, error) {
	var p models.Prediction
	err := q.QueryRow(ctx,
		`SELECT id, overlay_id, source, external_id, title, state, winning_outcome_id, auto_lock_at, created_at, locked_at, resolved_at
		 FROM predictions WHERE `+where, arg).
		Scan(&p.ID, &p.OverlayID, &p.Source, &p.ExternalID, &p.Title, &p.State, &p.WinningOutcomeID,
			&p.AutoLockAt, &p.CreatedAt, &p.LockedAt, &p.ResolvedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan prediction: %w", err)
	}
	outs, err := r.loadOutcomes(ctx, q, p.ID)
	if err != nil {
		return nil, err
	}
	p.Outcomes = outs
	return &p, nil
}

// loadOutcomes returns outcomes with live wagered pool + entrant counts. Like
// poll options, the totals are the All-Chat wagers plus the mirror_* aggregates:
// an All-Chat prediction only populates prediction_entries, a Twitch-native one
// only the mirror columns, so the sum is exact for both sources.
func (r *Repository) loadOutcomes(ctx context.Context, q dbtx, predictionID uuid.UUID) ([]models.PredictionOutcome, error) {
	rows, err := q.Query(ctx,
		`SELECT o.id, o.idx, o.label, o.color,
		        COALESCE(e.total, 0) + o.mirror_points, COALESCE(e.entrants, 0) + o.mirror_entrants
		 FROM prediction_outcomes o
		 LEFT JOIN (SELECT outcome_id, SUM(amount) AS total, COUNT(*) AS entrants
		            FROM prediction_entries WHERE prediction_id = $1 GROUP BY outcome_id) e
		        ON e.outcome_id = o.id
		 WHERE o.prediction_id = $1 ORDER BY o.idx`, predictionID)
	if err != nil {
		return nil, fmt.Errorf("load outcomes: %w", err)
	}
	defer rows.Close()
	var outs []models.PredictionOutcome
	for rows.Next() {
		var o models.PredictionOutcome
		if err := rows.Scan(&o.ID, &o.Idx, &o.Label, &o.Color, &o.TotalPts, &o.Entrants); err != nil {
			return nil, fmt.Errorf("scan outcome: %w", err)
		}
		outs = append(outs, o)
	}
	return outs, rows.Err()
}

// GetActivePrediction returns the overlay's live (ACTIVE or LOCKED) All-Chat
// prediction. Write-path query (chat wagers + viewer private state), so
// source='allchat' only; public display uses GetActiveDisplayPrediction.
func (r *Repository) GetActivePrediction(ctx context.Context, overlayID uuid.UUID) (*models.Prediction, error) {
	return r.scanPrediction(ctx, r.db,
		`overlay_id = $1 AND state IN ('ACTIVE','LOCKED') AND source = 'allchat'`, overlayID)
}

// GetActiveDisplayPrediction returns the overlay's live prediction of EITHER
// source for public rendering. If both are somehow live, the All-Chat prediction
// wins the display: it holds real wagered viewer points that MUST stay resolvable
// from the control panel, so it can't be shadowed by a mirrored Twitch round
// (which carries no All-Chat points). The create-time 409 already blocks a NEW
// All-Chat prediction while a native one is live; this covers the reverse order
// (a Twitch prediction beginning after an All-Chat one is already running).
func (r *Repository) GetActiveDisplayPrediction(ctx context.Context, overlayID uuid.UUID) (*models.Prediction, error) {
	var id uuid.UUID
	err := r.db.QueryRow(ctx,
		`SELECT id FROM predictions WHERE overlay_id = $1 AND state IN ('ACTIVE','LOCKED')
		 ORDER BY (source = 'allchat') DESC, created_at DESC LIMIT 1`, overlayID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get active display prediction: %w", err)
	}
	return r.GetPrediction(ctx, id)
}

// GetPrediction returns a prediction by id.
func (r *Repository) GetPrediction(ctx context.Context, predictionID uuid.UUID) (*models.Prediction, error) {
	return r.scanPrediction(ctx, r.db, `id = $1`, predictionID)
}

// LockPrediction transitions ACTIVE→LOCKED (guarded). Idempotent: returns the
// current prediction whether or not this call performed the transition.
func (r *Repository) LockPrediction(ctx context.Context, predictionID uuid.UUID) (*models.Prediction, error) {
	if _, err := r.db.Exec(ctx,
		`UPDATE predictions SET state = 'LOCKED', locked_at = NOW()
		 WHERE id = $1 AND state = 'ACTIVE' AND source = 'allchat'`, predictionID); err != nil {
		return nil, fmt.Errorf("lock prediction: %w", err)
	}
	return r.GetPrediction(ctx, predictionID)
}

// Wager places a viewer's stake on an outcome atomically: it row-locks the
// prediction (so a concurrent lock/resolve serializes), enforces one wager per
// viewer, and debits the balance under a guard. All-Chat-native only — a
// twitch_native prediction uses Twitch Channel Points, so it is rejected here.
func (r *Repository) Wager(ctx context.Context, predictionID, viewerID, overlayID uuid.UUID, outcomeIdx int, amount int64, platform string, sourceMessageID *uuid.UUID) (WagerResult, error) {
	if amount <= 0 {
		return WagerResult{Reason: "bad_outcome"}, nil
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return WagerResult{}, fmt.Errorf("begin wager: %w", err)
	}
	defer tx.Rollback(ctx)

	var source, state string
	err = tx.QueryRow(ctx, `SELECT source, state FROM predictions WHERE id = $1 FOR UPDATE`, predictionID).Scan(&source, &state)
	if errors.Is(err, pgx.ErrNoRows) {
		return WagerResult{Reason: "not_found"}, nil
	}
	if err != nil {
		return WagerResult{}, fmt.Errorf("lock prediction row: %w", err)
	}
	if source != models.SourceAllChat {
		return WagerResult{Reason: "native"}, nil
	}
	if state != models.PredActive {
		return WagerResult{Reason: "not_active"}, nil
	}

	var outcomeID uuid.UUID
	err = tx.QueryRow(ctx, `SELECT id FROM prediction_outcomes WHERE prediction_id = $1 AND idx = $2`, predictionID, outcomeIdx).Scan(&outcomeID)
	if errors.Is(err, pgx.ErrNoRows) {
		return WagerResult{Reason: "bad_outcome"}, nil
	}
	if err != nil {
		return WagerResult{}, fmt.Errorf("resolve outcome: %w", err)
	}

	tag, err := tx.Exec(ctx,
		`INSERT INTO prediction_entries (prediction_id, viewer_id, outcome_id, amount, platform, source_message_id)
		 VALUES ($1,$2,$3,$4,$5,$6) ON CONFLICT (prediction_id, viewer_id) DO NOTHING`,
		predictionID, viewerID, outcomeID, amount, platform, sourceMessageID)
	if err != nil {
		return WagerResult{}, fmt.Errorf("insert entry: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return WagerResult{Reason: "already_wagered"}, nil // one wager per viewer per prediction
	}

	applied, err := r.applyLedger(ctx, tx, viewerID, overlayID, -amount, "wager", "prediction", &predictionID,
		fmt.Sprintf("wager:%s:%s", predictionID, viewerID))
	if errors.Is(err, ErrInsufficientBalance) {
		return WagerResult{Reason: "insufficient"}, nil
	}
	if err != nil {
		return WagerResult{}, err
	}
	_ = applied // always true here (fresh dedup key inside this tx)

	if err := tx.Commit(ctx); err != nil {
		return WagerResult{}, fmt.Errorf("commit wager: %w", err)
	}
	bal, _ := r.GetBalance(ctx, viewerID, overlayID)
	return WagerResult{Accepted: true, NewBalance: bal}, nil
}

// GetViewerEntry returns a viewer's wager (outcome + amount) on a prediction, or nil.
func (r *Repository) GetViewerEntry(ctx context.Context, predictionID, viewerID uuid.UUID) (*uuid.UUID, int64, error) {
	var oid uuid.UUID
	var amt int64
	err := r.db.QueryRow(ctx,
		`SELECT outcome_id, amount FROM prediction_entries WHERE prediction_id = $1 AND viewer_id = $2`,
		predictionID, viewerID).Scan(&oid, &amt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, 0, nil
	}
	if err != nil {
		return nil, 0, fmt.Errorf("get viewer entry: %w", err)
	}
	return &oid, amt, nil
}

// ResolvePrediction transitions LOCKED→RESOLVED and pays out winners in one tx.
// Guarded + idempotent: a re-run after resolution is a no-op (winners are credited
// exactly once via payout: dedup keys). All-Chat-native only.
func (r *Repository) ResolvePrediction(ctx context.Context, predictionID, winningOutcomeID uuid.UUID) (*models.Prediction, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin resolve: %w", err)
	}
	defer tx.Rollback(ctx)

	// Validate the winning outcome belongs to this prediction.
	var overlayID uuid.UUID
	var exists bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM prediction_outcomes WHERE id = $1 AND prediction_id = $2)`,
		winningOutcomeID, predictionID).Scan(&exists); err != nil {
		return nil, fmt.Errorf("validate winning outcome: %w", err)
	}
	if !exists {
		return nil, ErrNotFound
	}

	tag, err := tx.Exec(ctx,
		`UPDATE predictions SET state = 'RESOLVED', winning_outcome_id = $2, resolved_at = NOW()
		 WHERE id = $1 AND state = 'LOCKED' AND source = 'allchat'`, predictionID, winningOutcomeID)
	if err != nil {
		return nil, fmt.Errorf("resolve prediction: %w", err)
	}
	if tag.RowsAffected() == 0 {
		// Not lockable/already resolved — idempotent no-op.
		_ = tx.Rollback(ctx)
		return r.GetPrediction(ctx, predictionID)
	}

	if err := tx.QueryRow(ctx, `SELECT overlay_id FROM predictions WHERE id = $1`, predictionID).Scan(&overlayID); err != nil {
		return nil, fmt.Errorf("load overlay for payout: %w", err)
	}

	entries, err := r.loadEntries(ctx, tx, predictionID)
	if err != nil {
		return nil, err
	}
	result := engine.ComputePayouts(entries, winningOutcomeID)
	reason := "payout"
	if result.Refund {
		reason = "refund"
	}
	for viewerID, credit := range result.Credits {
		if credit == 0 {
			continue
		}
		if _, err := r.applyLedger(ctx, tx, viewerID, overlayID, credit, reason, "prediction", &predictionID,
			fmt.Sprintf("%s:%s:%s", reason, predictionID, viewerID)); err != nil {
			return nil, fmt.Errorf("credit payout: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit resolve: %w", err)
	}
	return r.GetPrediction(ctx, predictionID)
}

// CancelPrediction transitions a live prediction to CANCELED and refunds every
// stake in one tx. Guarded + idempotent. All-Chat-native only.
func (r *Repository) CancelPrediction(ctx context.Context, predictionID uuid.UUID) (*models.Prediction, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin cancel: %w", err)
	}
	defer tx.Rollback(ctx)

	var overlayID uuid.UUID
	tag, err := tx.Exec(ctx,
		`UPDATE predictions SET state = 'CANCELED', resolved_at = NOW()
		 WHERE id = $1 AND state IN ('ACTIVE','LOCKED') AND source = 'allchat'`, predictionID)
	if err != nil {
		return nil, fmt.Errorf("cancel prediction: %w", err)
	}
	if tag.RowsAffected() == 0 {
		_ = tx.Rollback(ctx)
		return r.GetPrediction(ctx, predictionID)
	}
	if err := tx.QueryRow(ctx, `SELECT overlay_id FROM predictions WHERE id = $1`, predictionID).Scan(&overlayID); err != nil {
		return nil, fmt.Errorf("load overlay for refund: %w", err)
	}
	entries, err := r.loadEntries(ctx, tx, predictionID)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if _, err := r.applyLedger(ctx, tx, e.ViewerID, overlayID, e.Amount, "refund", "prediction", &predictionID,
			fmt.Sprintf("refund:%s:%s", predictionID, e.ViewerID)); err != nil {
			return nil, fmt.Errorf("refund stake: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit cancel: %w", err)
	}
	return r.GetPrediction(ctx, predictionID)
}

// loadEntries reads all wagers for a prediction (for payout/refund).
func (r *Repository) loadEntries(ctx context.Context, q dbtx, predictionID uuid.UUID) ([]engine.WagerEntry, error) {
	rows, err := q.Query(ctx, `SELECT viewer_id, outcome_id, amount FROM prediction_entries WHERE prediction_id = $1`, predictionID)
	if err != nil {
		return nil, fmt.Errorf("load entries: %w", err)
	}
	defer rows.Close()
	var out []engine.WagerEntry
	for rows.Next() {
		var e engine.WagerEntry
		if err := rows.Scan(&e.ViewerID, &e.OutcomeID, &e.Amount); err != nil {
			return nil, fmt.Errorf("scan entry: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// LockExpired locks every ACTIVE All-Chat prediction whose auto_lock_at has
// passed, returning the affected (id, overlay_id) so the caller can broadcast the
// new state and clear the active flags. Restart-safe (no in-memory timers).
func (r *Repository) LockExpired(ctx context.Context) ([]PredictionRef, error) {
	rows, err := r.db.Query(ctx,
		`UPDATE predictions SET state = 'LOCKED', locked_at = NOW()
		 WHERE state = 'ACTIVE' AND source = 'allchat' AND auto_lock_at IS NOT NULL AND auto_lock_at <= NOW()
		 RETURNING id, overlay_id`)
	if err != nil {
		return nil, fmt.Errorf("lock expired: %w", err)
	}
	defer rows.Close()
	var out []PredictionRef
	for rows.Next() {
		var ref PredictionRef
		if err := rows.Scan(&ref.PredictionID, &ref.OverlayID); err != nil {
			return nil, fmt.Errorf("scan locked ref: %w", err)
		}
		out = append(out, ref)
	}
	return out, rows.Err()
}

// PredictionRef identifies a prediction and its overlay.
type PredictionRef struct {
	PredictionID uuid.UUID
	OverlayID    uuid.UUID
}
