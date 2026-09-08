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

package usage

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// UserStatsKey is the shared Redis key the landing page's /api/v1/stats reads
// its "streamers on board" figure from. Written by auth-service, which owns
// the users table; no TTL — the ticker refreshes it in place.
const UserStatsKey = "stats:users:total"

// UserStatsInterval is how often the public user count is refreshed. The
// query is a single GROUP BY aggregate and the number is a landing-page
// decoration, so a minutes-scale cadence is plenty.
const UserStatsInterval = 5 * time.Minute

// userCounter reads the per-provider user counts (implemented by
// repository.UserRepository).
type userCounter interface {
	CountByAuthProvider(ctx context.Context) (map[string]int64, error)
}

// UserStatsPublisher periodically writes the total user count to Redis for
// the public stats endpoint.
type UserStatsPublisher struct {
	repo     userCounter
	redis    *redis.Client
	logger   *zap.Logger
	interval time.Duration
}

// NewUserStatsPublisher creates a publisher. A non-positive interval falls
// back to UserStatsInterval.
func NewUserStatsPublisher(repo userCounter, redis *redis.Client, logger *zap.Logger, interval time.Duration) *UserStatsPublisher {
	if logger == nil {
		logger = zap.NewNop()
	}
	if interval <= 0 {
		interval = UserStatsInterval
	}
	return &UserStatsPublisher{repo: repo, redis: redis, logger: logger, interval: interval}
}

// Publish runs one query and writes the key. Errors are returned for the
// caller to log; the previous value stays in Redis, so a transient database
// blip shows as a stale count rather than a zero.
func (p *UserStatsPublisher) Publish(ctx context.Context) error {
	counts, err := p.repo.CountByAuthProvider(ctx)
	if err != nil {
		return err
	}

	var total int64
	for _, n := range counts {
		total += n
	}

	return p.redis.Set(ctx, UserStatsKey, total, 0).Err()
}

// Run publishes immediately (so the key exists before the first landing-page
// hit after a cold start, not one interval later) and then on every tick
// until ctx is cancelled.
func (p *UserStatsPublisher) Run(ctx context.Context) {
	p.publishAndLog(ctx)

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("User stats publisher stopped")
			return
		case <-ticker.C:
			p.publishAndLog(ctx)
		}
	}
}

func (p *UserStatsPublisher) publishAndLog(ctx context.Context) {
	if err := p.Publish(ctx); err != nil {
		if ctx.Err() != nil {
			// Shutdown cancelled the query mid-flight; not a fault worth alarming on.
			return
		}
		p.logger.Warn("Failed to publish user count to Redis (non-fatal, key keeps last value)", zap.Error(err))
	}
}
