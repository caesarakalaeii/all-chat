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
	Accepted   bool
	Reason     string // "" on success; "not_found"|"not_active"|"bad_outcome"|"already_wagered"|"insufficient"|"native"|"duplicate"
	NewBalance int64
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

func (r *Repository) scanPrediction(ctx context.Context, q dbtx, where string, args ...any) (*models.Prediction, error) {
	var p models.Prediction
	err := q.QueryRow(ctx,
		`SELECT id, overlay_id, source, external_id, title, state, winning_outcome_id, auto_lock_at, created_at, locked_at, resolved_at
		 FROM predictions WHERE `+where, args...).
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
// source for public rendering, plus one resolved/canceled within the last
// displayGraceSeconds so the OBS widget can show the "who won" reveal before it
// clears. If both are somehow live, the All-Chat prediction wins the display: it
// holds real wagered viewer points that MUST stay resolvable from the control
// panel, so it can't be shadowed by a mirrored Twitch round (which carries no
// All-Chat points). The create-time 409 already blocks a NEW All-Chat prediction
// while a native one is live; this covers the reverse order (a Twitch prediction
// beginning after an All-Chat one is already running). A still-live round always
// outranks a terminal grace-window one (see the leading ORDER BY key).
//
// Accepted trade-off: because live outranks terminal, a native round going ACTIVE
// at the same moment an All-Chat round RESOLVED/CANCELED can win the display during
// the latter's grace window and pre-empt its "who won" reveal frame. This is
// display-only (points are already credited by resolve/cancel) and is accepted
// rather than special-cased — the create-side 409 keeps both-live uncommon.
func (r *Repository) GetActiveDisplayPrediction(ctx context.Context, overlayID uuid.UUID) (*models.Prediction, error) {
	var id uuid.UUID
	err := r.db.QueryRow(ctx,
		`SELECT id FROM predictions
		 WHERE overlay_id = $1
		   AND (state IN ('ACTIVE','LOCKED')
		        OR (state IN ('RESOLVED','CANCELED') AND resolved_at IS NOT NULL AND resolved_at > NOW() - make_interval(secs => $2)))
		 ORDER BY (state IN ('ACTIVE','LOCKED')) DESC, (source = 'allchat') DESC, created_at DESC LIMIT 1`,
		overlayID, displayGraceSeconds).Scan(&id)
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

// GetPredictionForOverlay returns a prediction by id scoped to an overlay. A id
// that belongs to a different overlay (or does not exist) yields ErrNotFound, which
// the handlers map to 404 — so an owner of overlay A can never read or mutate a
// prediction owned by overlay B via the guarded lifecycle endpoints.
func (r *Repository) GetPredictionForOverlay(ctx context.Context, predictionID, overlayID uuid.UUID) (*models.Prediction, error) {
	return r.scanPrediction(ctx, r.db, `id = $1 AND overlay_id = $2`, predictionID, overlayID)
}

// LockPrediction transitions ACTIVE→LOCKED (guarded). Idempotent: returns the
// current prediction whether or not this call performed the transition. Scoped to
// overlayID so a caller can only lock a prediction on an overlay they own — a
// mismatched (cross-tenant) id updates 0 rows and returns ErrNotFound.
func (r *Repository) LockPrediction(ctx context.Context, predictionID, overlayID uuid.UUID) (*models.Prediction, error) {
	if _, err := r.db.Exec(ctx,
		`UPDATE predictions SET state = 'LOCKED', locked_at = NOW()
		 WHERE id = $1 AND overlay_id = $2 AND state = 'ACTIVE' AND source = 'allchat'`, predictionID, overlayID); err != nil {
		return nil, fmt.Errorf("lock prediction: %w", err)
	}
	return r.GetPredictionForOverlay(ctx, predictionID, overlayID)
}

// Wager places a viewer's stake on an outcome atomically: it row-locks the
// prediction (so a concurrent lock/resolve serializes), enforces one wager per
// viewer, and debits the balance under a guard. All-Chat-native only — a
// twitch_native prediction uses Twitch Channel Points, so it is rejected here.
//
// replayToken, when non-empty (the chat path passes the Redis stream entry id), makes
// the debit dedup ROUND-INDEPENDENT: see the dedupKey comment below. The web path passes
// "" (a direct HTTP call has no redelivery).
func (r *Repository) Wager(ctx context.Context, predictionID, viewerID, overlayID uuid.UUID, outcomeIdx int, amount int64, platform string, sourceMessageID *uuid.UUID, replayToken string) (WagerResult, error) {
	if amount <= 0 {
		return WagerResult{Reason: "bad_outcome"}, nil
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return WagerResult{}, fmt.Errorf("begin wager: %w", err)
	}
	defer tx.Rollback(ctx)

	var source, state string
	var rowOverlayID uuid.UUID
	err = tx.QueryRow(ctx, `SELECT source, state, overlay_id FROM predictions WHERE id = $1 FOR UPDATE`, predictionID).Scan(&source, &state, &rowOverlayID)
	if errors.Is(err, pgx.ErrNoRows) {
		return WagerResult{Reason: "not_found"}, nil
	}
	if err != nil {
		return WagerResult{}, fmt.Errorf("lock prediction row: %w", err)
	}
	// The balance is a per-(viewer, overlay) economy (ADR-0029). The caller passes
	// overlayID from the :id path, but resolution later credits the prediction's
	// OWN overlay — so if the path overlay does not own this prediction, the stake
	// would be debited from one economy and the payout minted into another
	// (cross-economy inflation). Bind the debit to the prediction's real overlay;
	// a mismatch is reported as not_found (no cross-tenant existence oracle).
	if rowOverlayID != overlayID {
		return WagerResult{Reason: "not_found"}, nil
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

	// Round-independent replay dedup (P2-1). A redelivered chat wager (transient error +
	// PEL drain, or an ambiguous tx.Commit) re-resolves the overlay's CURRENTLY active
	// round — which may be a NEW round if the original one resolved in between. Keying the
	// ledger dedup on (overlay, message) rather than (prediction, viewer) means the same
	// chat message can never debit an overlay's economy twice, even across rounds, while
	// still fanning ONE message out to N overlays (ADR-0028; the key includes overlayID).
	// The web path has no redelivery, so it keeps the (prediction, viewer) key.
	dedupKey := fmt.Sprintf("wager:%s:%s", predictionID, viewerID)
	if replayToken != "" {
		dedupKey = fmt.Sprintf("wager:overlay:%s:%s", overlayID, replayToken)
	}
	applied, err := r.applyLedger(ctx, tx, viewerID, overlayID, -amount, "wager", "prediction", &predictionID, dedupKey)
	if errors.Is(err, ErrInsufficientBalance) {
		return WagerResult{Reason: "insufficient"}, nil
	}
	if err != nil {
		return WagerResult{}, err
	}
	if !applied {
		// This message already placed a wager on this overlay (a redelivery, possibly now
		// resolving to a DIFFERENT round than the original). Roll back the entry insert
		// above (defer tx.Rollback) and report a no-op — never debit again, never leave a
		// phantom entry on the new round.
		return WagerResult{Reason: "duplicate"}, nil
	}

	// Read the post-debit balance INSIDE the tx: applyLedger already debited
	// viewer_points above, so this row holds the exact post-wager value and is
	// immune to a concurrent uncommitted credit (unlike a racy post-commit read).
	var newBal int64
	if err := tx.QueryRow(ctx, `SELECT balance FROM viewer_points WHERE viewer_id = $1 AND overlay_id = $2`, viewerID, overlayID).Scan(&newBal); err != nil {
		return WagerResult{}, fmt.Errorf("read post-debit balance: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return WagerResult{}, fmt.Errorf("commit wager: %w", err)
	}
	return WagerResult{Accepted: true, NewBalance: newBal}, nil
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
func (r *Repository) ResolvePrediction(ctx context.Context, predictionID, winningOutcomeID, overlayID uuid.UUID) (*models.Prediction, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin resolve: %w", err)
	}
	defer tx.Rollback(ctx)

	// Validate the winning outcome belongs to this prediction.
	var exists bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM prediction_outcomes WHERE id = $1 AND prediction_id = $2)`,
		winningOutcomeID, predictionID).Scan(&exists); err != nil {
		return nil, fmt.Errorf("validate winning outcome: %w", err)
	}
	if !exists {
		return nil, ErrNotFound
	}

	// overlayID scopes the guarded UPDATE to the caller's overlay: a cross-tenant
	// prediction id affects 0 rows and falls through to the scoped ErrNotFound tail.
	tag, err := tx.Exec(ctx,
		`UPDATE predictions SET state = 'RESOLVED', winning_outcome_id = $2, resolved_at = NOW()
		 WHERE id = $1 AND state = 'LOCKED' AND source = 'allchat' AND overlay_id = $3`, predictionID, winningOutcomeID, overlayID)
	if err != nil {
		return nil, fmt.Errorf("resolve prediction: %w", err)
	}
	if tag.RowsAffected() == 0 {
		// Not lockable/already resolved, or not this overlay's prediction — idempotent
		// no-op that returns the current row only when it belongs to this overlay.
		_ = tx.Rollback(ctx)
		return r.GetPredictionForOverlay(ctx, predictionID, overlayID)
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
	return r.GetPredictionForOverlay(ctx, predictionID, overlayID)
}

// CancelPrediction transitions a live prediction to CANCELED and refunds every
// stake in one tx. Guarded + idempotent. All-Chat-native only.
func (r *Repository) CancelPrediction(ctx context.Context, predictionID, overlayID uuid.UUID) (*models.Prediction, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin cancel: %w", err)
	}
	defer tx.Rollback(ctx)

	// overlayID scopes the guarded UPDATE to the caller's overlay (cross-tenant → 0 rows).
	tag, err := tx.Exec(ctx,
		`UPDATE predictions SET state = 'CANCELED', resolved_at = NOW()
		 WHERE id = $1 AND state IN ('ACTIVE','LOCKED') AND source = 'allchat' AND overlay_id = $2`, predictionID, overlayID)
	if err != nil {
		return nil, fmt.Errorf("cancel prediction: %w", err)
	}
	if tag.RowsAffected() == 0 {
		_ = tx.Rollback(ctx)
		return r.GetPredictionForOverlay(ctx, predictionID, overlayID)
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
	return r.GetPredictionForOverlay(ctx, predictionID, overlayID)
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
