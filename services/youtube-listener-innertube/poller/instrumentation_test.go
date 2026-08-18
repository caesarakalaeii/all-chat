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

package poller

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"go.uber.org/zap"

	"github.com/caesar/all-chat/services/youtube-listener-innertube/innertube"
	"github.com/caesar/all-chat/services/youtube-listener-innertube/metrics"
)

// testMetrics returns a process-wide InnerTubeMetrics. NewInnerTubeMetrics
// registers with the default promauto registry, so a second call panics with a
// duplicate-collector error — which is exactly what `go test -count=2` does.
var (
	testMetricsOnce sync.Once
	testMetricsInst *metrics.InnerTubeMetrics
)

func testMetrics() *metrics.InnerTubeMetrics {
	testMetricsOnce.Do(func() { testMetricsInst = metrics.NewInnerTubeMetrics() })
	return testMetricsInst
}

// uniqueChannelID mints a channel_id no other run has used. The counters live
// in a process-wide registry, so a fixed label would carry over between
// repeated runs (`go test -count=2`) and make the assertions below meaningless.
var uniqueSeq atomic.Int64

func uniqueChannelID() string {
	return fmt.Sprintf("test-channel-%d", uniqueSeq.Add(1))
}

// liveResponse builds a get_live_chat response that carries a continuation (so
// the poller does not treat it as offline) and the given number of chat
// actions. Zero actions is the ambiguous case at the heart of #575: YouTube
// answers HTTP 200 with an empty actions list both for an idle chat and for a
// continuation token that has gone stale.
func liveResponse(token string, actions int) *innertube.LiveChatResponse {
	resp := &innertube.LiveChatResponse{
		ContinuationContents: innertube.ContinuationContents{
			LiveChatContinuation: innertube.LiveChatContinuation{
				Continuations: []innertube.Continuation{
					{TimedContinuationData: &innertube.TimedContinuationData{Continuation: token}},
				},
			},
		},
	}
	for i := 0; i < actions; i++ {
		resp.ContinuationContents.LiveChatContinuation.Actions = append(
			resp.ContinuationContents.LiveChatContinuation.Actions,
			innertube.ChatAction{},
		)
	}
	return resp
}

// countingRefresher stands in for innertube.Discovery on the stale-continuation
// recovery path.
type countingRefresher struct {
	mu    sync.Mutex
	calls int
}

func (r *countingRefresher) GetInitialContinuation(context.Context, string, string) (string, string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	return "fresh-token", "fresh-visitor", nil
}

// A poll that returns no actions is the only externally visible symptom of a
// broken continuation, and before this counter existed it produced a log line
// and nothing else. Turning it into a metric is what makes the difference
// between a five-second diagnosis and an investigation.
func TestPollerCountsZeroActionPollsAndRefreshes(t *testing.T) {
	m := testMetrics()
	refresher := &countingRefresher{}
	channelID := uniqueChannelID()

	client := &MockClient{
		responses: []*innertube.LiveChatResponse{
			liveResponse("t1", 0),
			liveResponse("t2", 0),
			liveResponse("t3", 0),
		},
		continuations: []string{"t1", "t2", "t3"},
	}

	p := NewPoller(client, "initial", channelID, zap.NewNop(), &PollerOptions{
		Interval: time.Millisecond,
		// Refresh after two consecutive empty polls instead of the production
		// 150, so the recovery path is reachable inside a unit test.
		ZeroActionThreshold: 2,
		VideoID:             "vid",
		Refresher:           refresher,
		Metrics:             m,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := p.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	waitFor(t, func() bool {
		return testutil.ToFloat64(m.ContinuationRefreshes.WithLabelValues(metrics.ServiceLabel, channelID)) >= 1
	})
	p.Stop()

	zero := testutil.ToFloat64(m.ZeroActionPolls.WithLabelValues(metrics.ServiceLabel, channelID))
	if zero < 2 {
		t.Errorf("zero-action polls = %v, want >= 2", zero)
	}
	refreshes := testutil.ToFloat64(m.ContinuationRefreshes.WithLabelValues(metrics.ServiceLabel, channelID))
	if refreshes < 1 {
		t.Errorf("continuation refreshes = %v, want >= 1", refreshes)
	}

	refresher.mu.Lock()
	defer refresher.mu.Unlock()
	if refresher.calls < 1 {
		t.Errorf("refresher calls = %d, want >= 1 — the counter must track real refreshes, not intent", refresher.calls)
	}
}

// The canary reads its poll count from this hook, so it has to fire on polls
// that captured nothing. A hook that only fired when messages arrived would go
// silent in exactly the state the canary exists to report.
func TestPollObserverFiresOnEmptyPolls(t *testing.T) {
	var mu sync.Mutex
	var observed [][2]int

	client := &MockClient{
		responses: []*innertube.LiveChatResponse{
			liveResponse("t1", 0),
			liveResponse("t2", 0),
		},
		continuations: []string{"t1", "t2"},
	}

	p := NewPoller(client, "initial", uniqueChannelID(), zap.NewNop(), &PollerOptions{
		Interval: time.Millisecond,
		PollObserver: func(actionCount, messageCount int) {
			mu.Lock()
			observed = append(observed, [2]int{actionCount, messageCount})
			mu.Unlock()
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := p.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	waitFor(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(observed) >= 2
	})
	p.Stop()

	mu.Lock()
	defer mu.Unlock()
	if len(observed) < 2 {
		t.Fatalf("observer fired %d times, want >= 2", len(observed))
	}
	for i, o := range observed[:2] {
		if o[0] != 0 || o[1] != 0 {
			t.Errorf("observation %d = (actions %d, messages %d), want (0, 0)", i, o[0], o[1])
		}
	}
}

// waitFor polls cond until it holds or the test times out. Used instead of a
// fixed sleep so these tests do not add to the timing sensitivity this package
// already has.
func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition not met before deadline")
}
