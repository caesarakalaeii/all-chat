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

package consumer

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	// pelReclaimMinIdle is how long an entry must sit unacked in a consumer's PEL
	// before it is reclaimed. It MUST exceed the worst-case time a message can be
	// legitimately in flight in the live Run loop, because the periodic drain runs on
	// the SAME consumer name and XAutoClaim can't tell "orphaned by a dead pod" from
	// "still being handled here". 5m (matching message-processor's drainPEL) is safely
	// above any real single-message processing time, so a slow-but-live handler is not
	// reclaimed out from under itself — while a genuinely crashed pod's entries (or a
	// transient-error entry this pod left pending) are still recovered, just not within
	// the first minute. Votes/wagers dedupe idempotently, so a rare overlap would only
	// waste work, but avoiding it entirely is cleaner.
	pelReclaimMinIdle = 5 * time.Minute
	// pelDrainInterval is how often each consumer sweeps for reclaimable entries, so a
	// pod that dies mid-batch while this pod is running is also recovered (not just
	// entries orphaned before this pod started). Sweeping more often than MinIdle is
	// fine — only entries idle past MinIdle are actually reclaimed.
	pelDrainInterval = 60 * time.Second
)

// drainPEL reclaims idle pending entries on (stream, group) into consumer and
// reprocesses them through process, mirroring message-processor/consumer/dlq.go.
// An entry whose process returns a (transient) error is left pending for a later
// drain to retry; a nil return — success, a business rejection, or a permanently
// unprocessable entry (bad JSON, trimmed phantom) — is acked so it can't wedge the
// group. Safe to call repeatedly (startup + on a ticker). The group must already
// exist (XGroupCreateMkStream runs first in each consumer's Run).
func drainPEL(
	ctx context.Context,
	rdb *redis.Client,
	stream, group, consumerName string,
	log *zap.Logger,
	process func(context.Context, redis.XMessage) error,
) {
	cursor := "0-0"
	for {
		if ctx.Err() != nil {
			return
		}
		messages, nextCursor, err := rdb.XAutoClaim(ctx, &redis.XAutoClaimArgs{
			Stream:   stream,
			Group:    group,
			Consumer: consumerName,
			MinIdle:  pelReclaimMinIdle,
			Start:    cursor,
			Count:    100,
		}).Result()
		if err != nil {
			if ctx.Err() == nil {
				log.Warn("xautoclaim engagement PEL", zap.String("stream", stream), zap.Error(err))
			}
			return
		}
		for _, msg := range messages {
			if err := process(ctx, msg); err != nil {
				log.Warn("reclaimed engagement entry deferred for retry",
					zap.String("stream", stream), zap.String("id", msg.ID), zap.Error(err))
				continue
			}
			if err := rdb.XAck(ctx, stream, group, msg.ID).Err(); err != nil {
				log.Warn("xack reclaimed engagement entry", zap.String("id", msg.ID), zap.Error(err))
			}
		}
		// XAutoClaim returns "0-0" once the PEL has been fully scanned.
		if nextCursor == "0-0" || len(messages) == 0 {
			return
		}
		cursor = nextCursor
	}
}

// periodicDrain runs drainPEL on a ticker until ctx is cancelled.
func periodicDrain(
	ctx context.Context,
	rdb *redis.Client,
	stream, group, consumerName string,
	log *zap.Logger,
	process func(context.Context, redis.XMessage) error,
) {
	t := time.NewTicker(pelDrainInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			drainPEL(ctx, rdb, stream, group, consumerName, log, process)
		}
	}
}
