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

// Package usage samples product-usage aggregates from the database into
// Prometheus gauges.
//
// Sign-ups are event-shaped and counted in-process where they happen
// (allchat_user_registrations_total). Usage is not: "how many streamers used an
// overlay in the last 24h" is a property of the whole fleet over a window that
// outlives any single pod, so it is polled from the database on a ticker instead
// of derived from in-process events. Same reasoning as the total-users gauge
// seeded at auth-service startup.
package usage

import (
	"context"
	"time"

	"github.com/caesar/all-chat/services/auth-service/repository"
	"github.com/caesar/all-chat/shared/metrics"
	"go.uber.org/zap"
)

// DefaultInterval is how often the active-user windows are re-sampled. The query
// is a single indexed aggregate over a 30-day slice of the overlays table, so a
// minute-scale cadence is cheap; it also comfortably out-resolves the rolling
// windows themselves.
const DefaultInterval = 2 * time.Minute

// activeUserCounter reads rolling-window active-user counts (implemented by
// repository.UsageRepository).
type activeUserCounter interface {
	ActiveUserCounts(ctx context.Context) (repository.ActiveUserCounts, error)
}

// activeUserGauges receives the sampled counts (implemented by
// metrics.BusinessMetrics).
type activeUserGauges interface {
	SetActiveUsers(window string, count int)
}

// Sampler periodically publishes allchat_active_users{window=...}.
type Sampler struct {
	repo     activeUserCounter
	metrics  activeUserGauges
	logger   *zap.Logger
	interval time.Duration
}

// NewSampler creates a usage sampler. A non-positive interval falls back to
// DefaultInterval.
func NewSampler(repo activeUserCounter, m activeUserGauges, logger *zap.Logger, interval time.Duration) *Sampler {
	if interval <= 0 {
		interval = DefaultInterval
	}
	return &Sampler{repo: repo, metrics: m, logger: logger, interval: interval}
}

// Sample runs one query and updates the gauges. Errors are returned for the
// caller to log; the gauges keep their previous values so a transient database
// blip shows as a flat line rather than a drop to zero.
func (s *Sampler) Sample(ctx context.Context) error {
	counts, err := s.repo.ActiveUserCounts(ctx)
	if err != nil {
		return err
	}

	s.metrics.SetActiveUsers(metrics.ActiveUserWindowDay, counts.Day)
	s.metrics.SetActiveUsers(metrics.ActiveUserWindowWeek, counts.Week)
	s.metrics.SetActiveUsers(metrics.ActiveUserWindowMonth, counts.Month)
	return nil
}

// Run samples immediately (so the gauges are populated before the first scrape,
// not one interval later) and then on every tick until ctx is cancelled.
func (s *Sampler) Run(ctx context.Context) {
	s.sampleAndLog(ctx)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("Usage sampler stopped")
			return
		case <-ticker.C:
			s.sampleAndLog(ctx)
		}
	}
}

func (s *Sampler) sampleAndLog(ctx context.Context) {
	if err := s.Sample(ctx); err != nil {
		if ctx.Err() != nil {
			// Shutdown cancelled the query mid-flight; not a fault worth alarming on.
			return
		}
		s.logger.Warn("Failed to sample active-user metrics (non-fatal, gauges keep last value)", zap.Error(err))
	}
}
