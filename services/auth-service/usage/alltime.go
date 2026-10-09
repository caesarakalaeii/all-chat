// This file is part of All-Chat.
// Copyright (C) 2026 caesarakalaeii
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published
// by the Free Software Foundation, either version 3 of the License, or
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
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// AllTimeMessagesKey is the Redis counter the landing page's /api/v1/stats
// serves as all_time; message-processor increments it once per delivered
// message. The PostgreSQL landing_stats row is the durable source of truth.
const AllTimeMessagesKey = "chat:stats:total"

// DailyBucketPattern matches the message-processor's per-platform daily
// counters (chat:stats:daily:{platform}:{YYYY-MM-DD}), which expire 8 days
// after creation.
const DailyBucketPattern = "chat:stats:daily:*"

// allTimeStore is the persistence port behind AllTimeStatsKeeper.
type allTimeStore interface {
	// GetLandingStat returns the stored value for key, 0 when absent.
	GetLendingStat(ctx context.Context, key string) (int64, error)
	// SetLandingStat overwrites the stored value for key.
	SetLandingStat(ctx context.Context, key string, value int64) error
}

// AllTimeStatsKeeper periodically folds the durable PostgreSQL total into the
// Redis counter. Redis is the only writer's runtime path; the keeper repairs
// the counter after a flush (SET total = stored) and stores the Redis value
// (SET stored = total) so the row and the key converge on the larger truth.
type AllTimeStatsKeeper struct {
	redis    *redis.Client
	store    allTimeStore
	logger   *zap.Logger
	interval time.Duration
}

// AllTimeDefaultInterval is how often the keeper runs: frequent enough that a
// Redis flush costs at most a day of counting, cheap enough (two GETs and an
// UPSERT) to be noise.
const AllTimeDefaultInterval = 6 * time.Hour

// NewAllTimeStatsKeeper creates a keeper. A non-positive interval falls back
// to AllTimeDefaultInterval.
func NewAllTimeStatsKeeper(redis *redis.Client, store allTimeStore, logger *zap.Logger, interval time.Duration) *AllTimeStatsKeeper {
	if logger == nil {
		logger = zap.NewNop()
	}
	if interval <= 0 {
		interval = AllTimeDefaultInterval
	}
	return &AllTimeStatsKeeper{redis: redis, store: store, logger: logger, interval: interval}
}

// Run reconciles immediately and then on every tick until ctx is cancelled.
func (k *AllTimeStatsKeeper) Run(ctx context.Context) {
	k.reconcileAndLog(ctx)
	ticker := time.NewTicker(k.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			k.reconcileAndLog(ctx)
		}
	}
}

func (k *AllTimeStatsKeeper) reconcileAndLog(ctx context.Context) {
	if err := k.Reconcile(ctx); err != nil {
		k.logger.Error("all-time stats: reconcile failed", zap.Error(err))
	}
}

// Reconcile makes Redis and PostgreSQL agree on the all-time count:
//
//   - redis >= stored: persist the larger Redis value (a deploy restarted the
//     counter at 0 and someone seeded, or the stored row is behind).
//   - redis < stored: the counter was flushed/reset; restore it from the row.
//
// Redis keeps INCR-ing concurrently; a lost race means the next tick absorbs
// whatever it missed, and the value is a marketing number, not an invoice.
func (k *AllTimeStatsKeeper) Reconcile(ctx context.Context) error {
	redisTotal, err := k.redis.Get(ctx, AllTimeMessagesKey).Int64()
	if err != nil && err != redis.Nil {
		return fmt.Errorf("read redis total: %w", err)
	}

	stored, err := k.store.GetLendingStat(ctx, "all_time_messages")
	if err != nil {
		return fmt.Errorf("read stored total: %w", err)
	}

	if redisTotal > stored {
		return k.store.SetLandingStat(ctx, "all_time_messages", redisTotal)
	}
	if stored > redisTotal {
		return k.redis.Set(ctx, AllTimeMessagesKey, stored, 0).Err()
	}
	return nil
}
