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

package reconcile

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/caesar/all-chat/shared/premium"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// OverrideExpirySweeper is the backstop that expires time-limited admin premium
// overrides (ADR-0027). Recompute already treats a lapsed override as absent on any
// recompute, so is_premium is correct the moment anything else touches the subject;
// this sweep exists for the subject that has NO other write after its grant lapses —
// it clears the due override columns (so the row stops matching) and recomputes the
// materialized is_premium down to what the subscription alone warrants.
//
// It lives in payment-service because that service is the single-replica entitlement
// authority (like the Patreon reconcile Manager). Each per-subject write is atomic and
// guarded (see premium.ExpireUserOverrideIfDue), so it is in fact safe under
// concurrency — but running one replica keeps the periodic scan from being redundant.
type OverrideExpirySweeper struct {
	db         *pgxpool.Pool
	recomputer *premium.Recomputer
	interval   time.Duration
	batchSize  int
	logger     *zap.Logger

	mu        sync.Mutex
	isRunning bool
}

// NewOverrideExpirySweeper builds a sweeper. batchSize caps how many due subjects of
// each kind are processed per pass; a due subject not reached this pass is picked up
// on the next one.
func NewOverrideExpirySweeper(db *pgxpool.Pool, recomputer *premium.Recomputer, interval time.Duration, batchSize int, logger *zap.Logger) *OverrideExpirySweeper {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	if batchSize <= 0 {
		batchSize = 500
	}
	return &OverrideExpirySweeper{
		db:         db,
		recomputer: recomputer,
		interval:   interval,
		batchSize:  batchSize,
		logger:     logger,
	}
}

// Start runs the sweep until ctx is cancelled, processing immediately on startup and
// then every interval.
func (s *OverrideExpirySweeper) Start(ctx context.Context) error {
	s.logger.Info("Starting premium override-expiry sweeper",
		zap.Duration("interval", s.interval),
		zap.Int("batch_size", s.batchSize))

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	if err := s.SweepOnce(ctx); err != nil {
		s.logger.Error("Initial override-expiry sweep failed", zap.Error(err))
	}

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("Premium override-expiry sweeper stopping")
			return ctx.Err()
		case <-ticker.C:
			if err := s.SweepOnce(ctx); err != nil {
				s.logger.Error("Override-expiry sweep failed", zap.Error(err))
			}
		}
	}
}

// SweepOnce expires every due user and viewer override (up to batchSize each) and
// recomputes their is_premium. Safe to call directly (tests do). Overlapping passes
// are skipped rather than stacked.
func (s *OverrideExpirySweeper) SweepOnce(ctx context.Context) error {
	s.mu.Lock()
	if s.isRunning {
		s.mu.Unlock()
		s.logger.Warn("Override-expiry sweep already in progress, skipping")
		return nil
	}
	s.isRunning = true
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.isRunning = false
		s.mu.Unlock()
	}()

	users, uErr := s.sweepUsers(ctx)
	viewers, vErr := s.sweepViewers(ctx)

	s.logger.Info("Override-expiry sweep complete",
		zap.Int("users_expired", users),
		zap.Int("viewers_expired", viewers))

	if uErr != nil {
		return uErr
	}
	return vErr
}

// sweepUsers clears due user overrides and returns how many were expired.
func (s *OverrideExpirySweeper) sweepUsers(ctx context.Context) (int, error) {
	ids, err := s.dueIDs(ctx, "users")
	if err != nil {
		return 0, fmt.Errorf("failed to list due user overrides: %w", err)
	}
	expired := 0
	for _, id := range ids {
		didExpire, err := s.recomputer.ExpireUserOverrideIfDue(ctx, id)
		if err != nil {
			s.logger.Error("Failed to expire user override", zap.String("user_id", id), zap.Error(err))
			continue
		}
		if didExpire {
			expired++
		}
	}
	return expired, nil
}

// sweepViewers clears due viewer overrides and returns how many were expired.
func (s *OverrideExpirySweeper) sweepViewers(ctx context.Context) (int, error) {
	ids, err := s.dueIDs(ctx, "viewers")
	if err != nil {
		return 0, fmt.Errorf("failed to list due viewer overrides: %w", err)
	}
	expired := 0
	for _, id := range ids {
		didExpire, err := s.recomputer.ExpireViewerOverrideIfDue(ctx, id)
		if err != nil {
			s.logger.Error("Failed to expire viewer override", zap.String("viewer_id", id), zap.Error(err))
			continue
		}
		if didExpire {
			expired++
		}
	}
	return expired, nil
}

// dueIDs returns the ids of rows in the given table (users or viewers) whose
// time-limited admin override has lapsed. The partial index on
// premium_admin_override_expires_at (migration 067) serves this predicate. The table
// name is a fixed internal constant, never user input.
func (s *OverrideExpirySweeper) dueIDs(ctx context.Context, table string) ([]string, error) {
	query := fmt.Sprintf(`
		SELECT id FROM %s
		WHERE premium_admin_override IS NOT NULL
		  AND premium_admin_override_expires_at IS NOT NULL
		  AND premium_admin_override_expires_at <= NOW()
		ORDER BY premium_admin_override_expires_at
		LIMIT $1`, table)

	rows, err := s.db.Query(ctx, query, s.batchSize)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
