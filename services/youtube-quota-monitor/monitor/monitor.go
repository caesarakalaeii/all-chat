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

// Package monitor reads the shared youtube_quota_usage table on an interval, exports
// the Prometheus quota metrics the alert rules evaluate, and publishes state-transition
// and threshold-crossing QuotaEvents to the "quota:alerts" channel the discord-bot
// renders. It is the single-owner replacement for the quota tracking the no-longer-
// deployed youtube-listener used to do (ADR-0023). Run it as a single replica: the
// dedup state (lastState / lastNotifiedThreshold) is in-memory, so a second replica
// would double-publish alerts.
package monitor

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/caesar/all-chat/shared/metrics"
	"github.com/caesar/all-chat/shared/quota"
	"go.uber.org/zap"
)

// serviceLabel is the `service` metric label and matches what the discord-bot and the
// Prometheus rules expect alongside platform="youtube".
const serviceLabel = "youtube-quota-monitor"

// Notifier publishes quota alerts. Satisfied by *shared/quota.Notifier.
type Notifier interface {
	NotifyStateTransition(ctx context.Context, oldState, newState quota.QuotaState, percentage float64, used, limit int) error
	NotifyThresholdCrossed(ctx context.Context, state quota.QuotaState, threshold, percentage float64, used, limit int) error
	NotifyQuotaRecovered(ctx context.Context, percentage float64, used, limit int) error
}

// Monitor evaluates the shared quota table and drives metrics + alerts.
type Monitor struct {
	reader     QuotaReader
	notifier   Notifier
	metrics    *metrics.ListenerMetrics
	thresholds quota.Thresholds
	interval   time.Duration
	logger     *zap.Logger

	mu                    sync.RWMutex
	lastState             quota.QuotaState
	lastNotifiedThreshold float64
	last                  Snapshot
	haveData              bool
}

// New builds a Monitor seeded in the HEALTHY state.
func New(reader QuotaReader, notifier Notifier, m *metrics.ListenerMetrics, th quota.Thresholds, interval time.Duration, logger *zap.Logger) *Monitor {
	return &Monitor{
		reader:     reader,
		notifier:   notifier,
		metrics:    m,
		thresholds: th,
		interval:   interval,
		logger:     logger,
		lastState:  quota.QuotaStateHealthy,
	}
}

// Run evaluates immediately, then on every interval tick until ctx is cancelled.
func (mon *Monitor) Run(ctx context.Context) {
	mon.RunOnce(ctx)
	ticker := time.NewTicker(mon.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			mon.logger.Info("quota monitor stopped")
			return
		case <-ticker.C:
			mon.RunOnce(ctx)
		}
	}
}

// RunOnce performs a single evaluation: refresh metrics, then emit threshold-crossing
// and state-transition alerts as needed. Safe to call directly (used by Run and tests).
func (mon *Monitor) RunOnce(ctx context.Context) {
	snap, err := mon.reader.Read(ctx)
	if err != nil {
		// Keep the last good metric value and state rather than emitting a false
		// recovery/alert off a transient read failure.
		mon.logger.Warn("quota read failed; keeping last known state", zap.Error(err))
		return
	}

	// Always refresh the gauges so Prometheus evaluates fresh data every scrape.
	// used+reserved is the committed total, matching the SQL percentage.
	used := snap.Used + snap.Reserved
	mon.metrics.SetQuotaUsagePercent("youtube", serviceLabel, "daily", snap.Percentage)
	mon.metrics.SetQuotaRemaining("youtube", serviceLabel, "daily", strconv.Itoa(snap.Limit), float64(snap.Available))

	mon.mu.Lock()
	mon.last = snap
	mon.haveData = true
	oldState := mon.lastState
	lastThreshold := mon.lastNotifiedThreshold
	mon.mu.Unlock()

	newState := quota.CalculateState(snap.Percentage, mon.thresholds)

	// Threshold crossings on 5% boundaries >= 75% (deduped so each band alerts once
	// per day; reset when we recover to HEALTHY below).
	currentThreshold := float64(int(snap.Percentage/5.0)) * 5.0
	if currentThreshold >= 75.0 && currentThreshold > lastThreshold {
		mon.mu.Lock()
		mon.lastNotifiedThreshold = currentThreshold
		mon.mu.Unlock()
		mon.logger.Info("crossed 5% quota threshold",
			zap.Float64("threshold", currentThreshold),
			zap.Float64("percentage", snap.Percentage),
			zap.Int("used", used),
		)
		if err := mon.notifier.NotifyThresholdCrossed(ctx, newState, currentThreshold, snap.Percentage, used, snap.Limit); err != nil {
			mon.logger.Warn("threshold notification failed", zap.Error(err))
		}
	}

	if newState != oldState {
		mon.mu.Lock()
		mon.lastState = newState
		mon.mu.Unlock()
		mon.logger.Warn("quota state transition",
			zap.String("old_state", string(oldState)),
			zap.String("new_state", string(newState)),
			zap.Float64("percentage", snap.Percentage),
			zap.Int("used", used),
			zap.Int("limit", snap.Limit),
		)
		if newState == quota.QuotaStateHealthy {
			// Recovered (typically the midnight PT reset): clear the threshold dedup so
			// re-crossing a band the next day alerts again.
			mon.mu.Lock()
			mon.lastNotifiedThreshold = 0
			mon.mu.Unlock()
			if err := mon.notifier.NotifyQuotaRecovered(ctx, snap.Percentage, used, snap.Limit); err != nil {
				mon.logger.Warn("recovery notification failed", zap.Error(err))
			}
		} else if err := mon.notifier.NotifyStateTransition(ctx, oldState, newState, snap.Percentage, used, snap.Limit); err != nil {
			mon.logger.Warn("state transition notification failed", zap.Error(err))
		}
	}
}

// Snapshot returns the last read snapshot, the current state, and whether any read has
// succeeded yet. Used by the /quota/status handler.
func (mon *Monitor) Snapshot() (Snapshot, quota.QuotaState, bool) {
	mon.mu.RLock()
	defer mon.mu.RUnlock()
	return mon.last, mon.lastState, mon.haveData
}
