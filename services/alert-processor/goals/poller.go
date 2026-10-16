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

// Package goals polls the platform counts behind follower and subscriber goal
// bars (issue #955, ADR-0072). It polls a goal only while its overlay is
// connected — the gateway's overlay:connected:{id} demand key, the same
// signal source-manager builds listener demand from — and only one replica
// polls a goal per interval, through a per-overlay Redis lock.
package goals

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/caesar/all-chat/services/alert-processor/metrics"
	"github.com/caesar/all-chat/services/alert-processor/publisher"
	"github.com/caesar/all-chat/services/alert-processor/repository"
	"github.com/caesar/all-chat/shared/alerts"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

var (
	// ErrReconnect: the owner holds no usable credential for the goal's
	// channel. The editor shows a "reconnect" hint; the bar keeps its last count.
	ErrReconnect = errors.New("goals: credential missing or revoked")
	// ErrHidden: the channel hides its subscriber count.
	ErrHidden = errors.New("goals: count hidden by the channel")
	// ErrQuota: the shared YouTube quota refused the reservation.
	ErrQuota = errors.New("goals: youtube quota unavailable")
)

// Counter fetches one goal's authoritative count from its platform.
type Counter interface {
	Count(ctx context.Context, g repository.PolledGoal) (int64, error)
}

// Lister lists the goals that would be polled if connected.
type Lister interface {
	PolledGoals(ctx context.Context) ([]repository.PolledGoal, error)
}

// Poller polls connected goal overlays every interval.
type Poller struct {
	lister   Lister
	counters map[string]Counter // by platform
	rdb      redis.UniversalClient
	pub      *publisher.Publisher
	interval time.Duration
	log      *zap.Logger
}

// NewPoller creates a Poller. counters is keyed by platform.
func NewPoller(lister Lister, counters map[string]Counter, rdb redis.UniversalClient, pub *publisher.Publisher, interval time.Duration, log *zap.Logger) *Poller {
	return &Poller{lister: lister, counters: counters, rdb: rdb, pub: pub, interval: interval, log: log}
}

// Run polls every interval until ctx ends.
func (p *Poller) Run(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.PollOnce(ctx)
		}
	}
}

// PollOnce polls every connected goal whose lock this replica wins.
func (p *Poller) PollOnce(ctx context.Context) {
	goals, err := p.lister.PolledGoals(ctx)
	if err != nil {
		p.log.Warn("Listing polled goals failed", zap.Error(err))
		return
	}
	for _, g := range goals {
		counter, ok := p.counters[g.Platform]
		if !ok || !p.claim(ctx, g.OverlayID) {
			continue
		}
		p.poll(ctx, counter, g)
	}
}

// claim reports whether the overlay is connected and this replica won its
// poll lock. The lock lives one second less than the interval, so the replica
// that holds it reliably wins it again on its next tick.
func (p *Poller) claim(ctx context.Context, overlayID string) bool {
	connected, err := p.rdb.Exists(ctx, "overlay:connected:"+overlayID).Result()
	if err != nil || connected == 0 {
		return false
	}
	ttl := max(p.interval-time.Second, time.Second)
	won, err := p.rdb.SetNX(ctx, fmt.Sprintf("overlay:%s:goal-poll-lock", overlayID), "1", ttl).Result()
	return err == nil && won
}

func (p *Poller) poll(ctx context.Context, counter Counter, g repository.PolledGoal) {
	n, err := counter.Count(ctx, g)
	statusKey := alerts.GoalStatusKey(g.OverlayID)
	switch {
	case err == nil:
		metrics.RecordGoalPoll(g.Platform, "ok")
		if err := p.rdb.Set(ctx, alerts.GoalKey(g.OverlayID, g.Metric), n, 0).Err(); err != nil {
			p.log.Warn("Storing goal count failed", zap.String("overlay_id", g.OverlayID), zap.Error(err))
		}
		p.rdb.Del(ctx, statusKey)
		frame := alerts.GoalFrame{Type: "goal", OverlayID: g.OverlayID, Goal: g.Metric, Current: float64(n), Target: g.Target, Title: g.Title}
		if err := p.pub.PublishFrame(ctx, g.OverlayID, frame); err != nil {
			p.log.Warn("Goal frame publish failed", zap.String("overlay_id", g.OverlayID), zap.Error(err))
		}
	case errors.Is(err, ErrReconnect), errors.Is(err, ErrHidden):
		status := "reconnect"
		if errors.Is(err, ErrHidden) {
			status = "hidden"
		}
		metrics.RecordGoalPoll(g.Platform, status)
		// Outlives a few missed polls; expires once nobody is polling.
		p.rdb.Set(ctx, statusKey, status, 3*p.interval)
	case errors.Is(err, ErrQuota):
		metrics.RecordGoalPoll(g.Platform, "quota_skipped")
	default:
		metrics.RecordGoalPoll(g.Platform, "error")
		p.log.Warn("Goal poll failed", zap.String("overlay_id", g.OverlayID), zap.String("platform", g.Platform), zap.Error(err))
	}
}
