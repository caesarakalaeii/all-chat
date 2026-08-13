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
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/caesar/all-chat/services/auth-service/repository"
	"github.com/caesar/all-chat/shared/metrics"
	"go.uber.org/zap"
)

// The production types must keep satisfying the sampler's narrow interfaces —
// these assertions fail the build if either signature drifts.
var (
	_ activeUserCounter = (*repository.UsageRepository)(nil)
	_ activeUserGauges  = (*metrics.BusinessMetrics)(nil)
)

type fakeRepo struct {
	mu     sync.Mutex
	counts repository.ActiveUserCounts
	err    error
	calls  int
}

func (f *fakeRepo) ActiveUserCounts(context.Context) (repository.ActiveUserCounts, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.err != nil {
		return repository.ActiveUserCounts{}, f.err
	}
	return f.counts, nil
}

func (f *fakeRepo) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

type fakeGauges struct {
	mu     sync.Mutex
	values map[string]int
}

func newFakeGauges() *fakeGauges {
	return &fakeGauges{values: make(map[string]int)}
}

func (f *fakeGauges) SetActiveUsers(window string, count int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.values[window] = count
}

func (f *fakeGauges) snapshot() map[string]int {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make(map[string]int, len(f.values))
	for k, v := range f.values {
		out[k] = v
	}
	return out
}

func TestSampleSetsOneGaugePerWindow(t *testing.T) {
	repo := &fakeRepo{counts: repository.ActiveUserCounts{Day: 7, Week: 23, Month: 61}}
	gauges := newFakeGauges()
	sampler := NewSampler(repo, gauges, zap.NewNop(), time.Minute)

	if err := sampler.Sample(context.Background()); err != nil {
		t.Fatalf("Sample() returned error: %v", err)
	}

	want := map[string]int{
		metrics.ActiveUserWindowDay:   7,
		metrics.ActiveUserWindowWeek:  23,
		metrics.ActiveUserWindowMonth: 61,
	}
	got := gauges.snapshot()
	if len(got) != len(want) {
		t.Fatalf("expected %d windows set, got %d (%v)", len(want), len(got), got)
	}
	for window, count := range want {
		if got[window] != count {
			t.Errorf("window %q: expected %d, got %d", window, count, got[window])
		}
	}
}

// A failed query must leave the gauges alone: Prometheus then shows a flat line
// across the blip instead of a cliff to zero that would read as "nobody is using
// the product".
func TestSampleLeavesGaugesUntouchedOnError(t *testing.T) {
	repo := &fakeRepo{counts: repository.ActiveUserCounts{Day: 5, Week: 5, Month: 5}}
	gauges := newFakeGauges()
	sampler := NewSampler(repo, gauges, zap.NewNop(), time.Minute)

	if err := sampler.Sample(context.Background()); err != nil {
		t.Fatalf("first Sample() returned error: %v", err)
	}

	repo.err = errors.New("connection refused")
	if err := sampler.Sample(context.Background()); err == nil {
		t.Fatal("expected Sample() to return the query error")
	}

	if got := gauges.snapshot()[metrics.ActiveUserWindowDay]; got != 5 {
		t.Errorf("expected the previous value 5 to survive the failed sample, got %d", got)
	}
}

// Run must publish before the first scrape rather than one interval later, so the
// gauges are never scraped as a pre-initialised zero.
func TestRunSamplesImmediatelyAndStopsOnCancel(t *testing.T) {
	repo := &fakeRepo{counts: repository.ActiveUserCounts{Day: 1, Week: 2, Month: 3}}
	gauges := newFakeGauges()
	sampler := NewSampler(repo, gauges, zap.NewNop(), time.Hour) // ticker must not fire

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		sampler.Run(ctx)
		close(done)
	}()

	deadline := time.After(2 * time.Second)
	for repo.callCount() == 0 {
		select {
		case <-deadline:
			t.Fatal("Run() did not sample before the first tick")
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}

	if got := gauges.snapshot()[metrics.ActiveUserWindowMonth]; got != 3 {
		t.Errorf("expected the immediate sample to publish 3 for the 30d window, got %d", got)
	}

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run() did not return after context cancellation")
	}
}

func TestRunResamplesOnEveryTick(t *testing.T) {
	repo := &fakeRepo{counts: repository.ActiveUserCounts{Day: 1, Week: 1, Month: 1}}
	sampler := NewSampler(repo, newFakeGauges(), zap.NewNop(), 10*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go sampler.Run(ctx)

	deadline := time.After(2 * time.Second)
	for repo.callCount() < 3 {
		select {
		case <-deadline:
			t.Fatalf("expected repeated samples, got %d", repo.callCount())
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}
}

func TestNewSamplerFallsBackToDefaultInterval(t *testing.T) {
	for _, interval := range []time.Duration{0, -time.Second} {
		if got := NewSampler(&fakeRepo{}, newFakeGauges(), zap.NewNop(), interval).interval; got != DefaultInterval {
			t.Errorf("interval %v: expected fallback to %v, got %v", interval, DefaultInterval, got)
		}
	}
}
