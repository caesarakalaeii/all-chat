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

package jobs

import (
	"context"
	"time"

	"github.com/caesar/all-chat/services/share-service/repository"
	"go.uber.org/zap"
)

// ExpiryJob runs a background ticker to expire old pending share requests
type ExpiryJob struct {
	repo   *repository.ShareRepository
	logger *zap.Logger
	ticker *time.Ticker
	done   chan bool
}

// NewExpiryJob creates a new expiry job with a 5-minute interval
func NewExpiryJob(repo *repository.ShareRepository, logger *zap.Logger) *ExpiryJob {
	return &ExpiryJob{
		repo:   repo,
		logger: logger,
		ticker: time.NewTicker(5 * time.Minute),
		done:   make(chan bool),
	}
}

// Start begins the expiry job in a background goroutine
// Runs immediately on start, then every 5 minutes
func (j *ExpiryJob) Start(ctx context.Context) {
	j.logger.Info("Starting share request expiry job",
		zap.String("interval", "5 minutes"))

	go func() {
		// Run immediately on start (don't wait 5 minutes)
		j.expireOldRequests(ctx)

		for {
			select {
			case <-j.done:
				j.logger.Info("Expiry job stopped")
				return
			case <-j.ticker.C:
				j.expireOldRequests(ctx)
			}
		}
	}()
}

// Stop gracefully stops the expiry job
func (j *ExpiryJob) Stop() {
	j.logger.Info("Stopping expiry job...")
	j.ticker.Stop()
	j.done <- true
}

// expireOldRequests calls the repository to expire pending requests and timed accepted shares
func (j *ExpiryJob) expireOldRequests(ctx context.Context) {
	// Add timeout to prevent indefinite blocking
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Expire pending requests past 7-day acceptance window
	count, err := j.repo.ExpirePendingRequests(ctx)
	if err != nil {
		j.logger.Error("Failed to expire pending requests", zap.Error(err))
	} else if count > 0 {
		j.logger.Info("Expired pending share requests", zap.Int("count", count))
	} else {
		j.logger.Debug("No pending requests to expire")
	}

	// Expire accepted shares past their custom time-based expiry
	timedCount, err := j.repo.ExpireTimedAcceptedShares(ctx)
	if err != nil {
		j.logger.Error("Failed to expire timed accepted shares", zap.Error(err))
	} else if timedCount > 0 {
		j.logger.Info("Expired timed accepted shares", zap.Int("count", timedCount))
	}
}
