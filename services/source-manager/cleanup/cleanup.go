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

package cleanup

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

const (
	// DefaultStaleThreshold - sources not updated in this time are marked inactive.
	// Must be long enough that manual activations and future listener heartbeats aren't
	// prematurely cleaned up. Set to 24h until all listeners implement proper heartbeating.
	DefaultStaleThreshold = 24 * time.Hour

	// DefaultCleanupInterval - how often to run cleanup
	DefaultCleanupInterval = 2 * time.Minute
)

// Job periodically marks stale sources as inactive
type Job struct {
	db               *pgxpool.Pool
	logger           *zap.Logger
	staleThreshold   time.Duration
	cleanupInterval  time.Duration
	stopChan         chan struct{}
}

// NewJob creates a new cleanup job
func NewJob(db *pgxpool.Pool, logger *zap.Logger) *Job {
	return &Job{
		db:              db,
		logger:          logger,
		staleThreshold:  DefaultStaleThreshold,
		cleanupInterval: DefaultCleanupInterval,
		stopChan:        make(chan struct{}),
	}
}

// Start begins the periodic cleanup process
func (j *Job) Start(ctx context.Context) error {
	j.logger.Info("Starting source cleanup job",
		zap.Duration("interval", j.cleanupInterval),
		zap.Duration("stale_threshold", j.staleThreshold),
	)

	// Run initial cleanup
	if err := j.runCleanup(ctx); err != nil {
		j.logger.Error("Initial cleanup failed", zap.Error(err))
	}

	// Start periodic cleanup
	ticker := time.NewTicker(j.cleanupInterval)
	go func() {
		for {
			select {
			case <-ticker.C:
				if err := j.runCleanup(context.Background()); err != nil {
					j.logger.Error("Periodic cleanup failed", zap.Error(err))
				}
			case <-j.stopChan:
				ticker.Stop()
				return
			}
		}
	}()

	return nil
}

// Stop gracefully stops the cleanup job
func (j *Job) Stop() {
	close(j.stopChan)
}

// runCleanup marks sources as inactive if they haven't been updated recently
func (j *Job) runCleanup(ctx context.Context) error {
	// Mark sources as inactive if they were last updated more than staleThreshold ago
	// AND they are currently marked as active
	query := `
		UPDATE overlay_chat_sources
		SET is_active = false, updated_at = NOW()
		WHERE is_active = true
		  AND updated_at < NOW() - $1::interval
	`

	result, err := j.db.Exec(ctx, query, j.staleThreshold)
	if err != nil {
		return fmt.Errorf("failed to cleanup stale sources: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected > 0 {
		j.logger.Info("Marked stale sources as inactive",
			zap.Int64("count", rowsAffected),
			zap.Duration("stale_threshold", j.staleThreshold),
		)
	}

	return nil
}
