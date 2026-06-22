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

package monitor

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/caesar/all-chat/shared/metrics"
	"github.com/caesar/all-chat/shared/quota"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// getTestMetrics returns a process-wide ListenerMetrics. promauto registers on the
// default registry, so it can only be built once per test binary.
var (
	testMetricsOnce sync.Once
	testMetrics     *metrics.ListenerMetrics
)

func getTestMetrics() *metrics.ListenerMetrics {
	testMetricsOnce.Do(func() {
		testMetrics = metrics.NewListenerMetrics("youtube", "youtube-quota-monitor")
	})
	return testMetrics
}

type fakeReader struct {
	snap Snapshot
	err  error
}

func (f *fakeReader) Read(_ context.Context) (Snapshot, error) { return f.snap, f.err }

type transition struct{ old, next quota.QuotaState }

type fakeNotifier struct {
	transitions []transition
	thresholds  []float64
	recoveries  int
}

func (f *fakeNotifier) NotifyStateTransition(_ context.Context, oldState, newState quota.QuotaState, _ float64, _, _ int) error {
	f.transitions = append(f.transitions, transition{oldState, newState})
	return nil
}

func (f *fakeNotifier) NotifyThresholdCrossed(_ context.Context, _ quota.QuotaState, threshold, _ float64, _, _ int) error {
	f.thresholds = append(f.thresholds, threshold)
	return nil
}

func (f *fakeNotifier) NotifyQuotaRecovered(_ context.Context, _ float64, _, _ int) error {
	f.recoveries++
	return nil
}

// snapPct builds a snapshot at a given usage percentage over a 1000-unit limit.
func snapPct(pct float64) Snapshot {
	limit := 1000
	total := int(pct / 100 * float64(limit))
	return Snapshot{Used: total, Reserved: 0, Available: limit - total, Limit: limit, Percentage: pct}
}

func newTestMonitor(r QuotaReader, n Notifier) *Monitor {
	return New(r, n, getTestMetrics(), quota.DefaultThresholds(), time.Minute, zap.NewNop())
}

func TestRunOnce_StateTransitionFiresOnce(t *testing.T) {
	r := &fakeReader{}
	n := &fakeNotifier{}
	mon := newTestMonitor(r, n)

	for _, p := range []float64{60, 90, 90} {
		r.snap = snapPct(p)
		mon.RunOnce(context.Background())
	}

	require.Len(t, n.transitions, 1, "exactly one HEALTHY→CRITICAL transition")
	assert.Equal(t, quota.QuotaStateHealthy, n.transitions[0].old)
	assert.Equal(t, quota.QuotaStateCritical, n.transitions[0].next)
}

func TestRunOnce_ThresholdDedup(t *testing.T) {
	r := &fakeReader{}
	n := &fakeNotifier{}
	mon := newTestMonitor(r, n)

	for _, p := range []float64{50, 76, 77, 81} {
		r.snap = snapPct(p)
		mon.RunOnce(context.Background())
	}

	// 50% is below the 75% floor; 76→band 75, 77→still band 75 (deduped), 81→band 80.
	assert.Equal(t, []float64{75, 80}, n.thresholds)
}

func TestRunOnce_RecoveryResetsDedup(t *testing.T) {
	r := &fakeReader{}
	n := &fakeNotifier{}
	mon := newTestMonitor(r, n)

	for _, p := range []float64{96, 10, 76} {
		r.snap = snapPct(p)
		mon.RunOnce(context.Background())
	}

	assert.Equal(t, 1, n.recoveries, "one recovery when usage drops back to HEALTHY")
	// 96→band 95 fires; recovery resets dedup; 76→band 75 fires again afterwards.
	assert.Equal(t, []float64{95, 75}, n.thresholds)
}

func TestRunOnce_SetsMetricsEachTick(t *testing.T) {
	r := &fakeReader{snap: snapPct(80)}
	n := &fakeNotifier{}
	mon := newTestMonitor(r, n)

	mon.RunOnce(context.Background())

	got := testutil.ToFloat64(getTestMetrics().QuotaUsagePercent.WithLabelValues("youtube", serviceLabel, "daily"))
	assert.InDelta(t, 80.0, got, 0.001)

	snap, _, have := mon.Snapshot()
	assert.True(t, have)
	assert.Equal(t, 80.0, snap.Percentage)
}

func TestRunOnce_ReaderErrorIsInert(t *testing.T) {
	r := &fakeReader{err: errors.New("db down")}
	n := &fakeNotifier{}
	mon := newTestMonitor(r, n)

	mon.RunOnce(context.Background())

	assert.Empty(t, n.transitions)
	assert.Empty(t, n.thresholds)
	assert.Zero(t, n.recoveries)
	_, _, have := mon.Snapshot()
	assert.False(t, have, "a failed read must not mark data as available")
}
