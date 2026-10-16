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

// Package listpush debounces leaderboard changes into list_update frames
// (issue #955, ADR-0072): at most one frame per (overlay, category) per
// window across every replica, and never a lost final change.
package listpush

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/caesar/all-chat/services/alert-processor/publisher"
	"github.com/caesar/all-chat/shared/alerts"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type board struct{ overlayID, category string }

// Debouncer collects dirty boards and flushes them once per window. The
// window is claimed in Redis (SET NX PX), so two replicas that both scored a
// board share one window; the replica that loses keeps its board dirty and
// sends on a later tick, which is what keeps the trailing change.
type Debouncer struct {
	rdb    redis.UniversalClient
	pub    *publisher.Publisher
	window time.Duration
	log    *zap.Logger

	mu    sync.Mutex
	dirty map[board]struct{}
}

// New creates a Debouncer with the given window (the issue's 2s).
func New(rdb redis.UniversalClient, pub *publisher.Publisher, window time.Duration, log *zap.Logger) *Debouncer {
	return &Debouncer{rdb: rdb, pub: pub, window: window, log: log, dirty: make(map[board]struct{})}
}

// MarkDirty records that a board's scores changed.
func (d *Debouncer) MarkDirty(overlayID, category string) {
	d.mu.Lock()
	d.dirty[board{overlayID, category}] = struct{}{}
	d.mu.Unlock()
}

// Run flushes every window until ctx ends. A tick of window/2 keeps the
// worst-case delay of a deferred change under 1.5 windows.
func (d *Debouncer) Run(ctx context.Context) {
	ticker := time.NewTicker(d.window / 2)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.Flush(ctx)
		}
	}
}

// Flush sends a list_update for each dirty board whose window is free.
func (d *Debouncer) Flush(ctx context.Context) {
	d.mu.Lock()
	boards := make([]board, 0, len(d.dirty))
	for b := range d.dirty {
		boards = append(boards, b)
	}
	d.mu.Unlock()

	for _, b := range boards {
		if err := d.send(ctx, b); err != nil {
			d.MarkDirty(b.overlayID, b.category)
			d.log.Warn("list_update failed; retrying next tick",
				zap.String("overlay_id", b.overlayID), zap.String("category", b.category), zap.Error(err))
		}
	}
}

// send claims the board's window and, if won, publishes the board. A board
// whose window another replica holds stays dirty.
func (d *Debouncer) send(ctx context.Context, b board) error {
	windowKey := fmt.Sprintf("overlay:%s:list-push:%s", b.overlayID, b.category)
	won, err := d.rdb.SetNX(ctx, windowKey, "1", d.window).Result()
	if err != nil || !won {
		return err
	}
	// Cleared before the read, so a change scored while this frame is being
	// built marks the board dirty again instead of being lost.
	d.mu.Lock()
	delete(d.dirty, b)
	d.mu.Unlock()

	entries, err := alerts.TopEntries(ctx, d.rdb, b.overlayID, b.category, alerts.MaxListLimit)
	if err != nil {
		return err
	}
	frame := alerts.ListUpdateFrame{Type: "list_update", OverlayID: b.overlayID, Category: b.category, Entries: entries}
	return d.pub.PublishFrame(ctx, b.overlayID, frame)
}
