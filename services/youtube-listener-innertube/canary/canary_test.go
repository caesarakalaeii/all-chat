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

package canary

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/caesar/all-chat/services/youtube-listener-innertube/metrics"
	"github.com/caesar/all-chat/services/youtube-listener-innertube/poller"
)

type fakeSource struct {
	mu   sync.Mutex
	err  error
	got  []Target
	cont string
}

func (f *fakeSource) GetInitialContinuation(_ context.Context, videoID, channelID string) (string, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.got = append(f.got, Target{ChannelID: channelID, VideoID: videoID})
	if f.err != nil {
		return "", "", f.err
	}
	return f.cont, "visitor", nil
}

func (f *fakeSource) calls() []Target {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]Target(nil), f.got...)
}

type fakeDiscoverer struct {
	videoID string
	err     error
	calls   int
	mu      sync.Mutex
}

func (f *fakeDiscoverer) DiscoverLiveStream(_ context.Context, _, _, _ string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	return f.videoID, f.err
}

type fakeLeader struct {
	isLeader bool
	err      error
	released []string
	mu       sync.Mutex
}

func (f *fakeLeader) EnsureLeadership(_ context.Context, _ string, _ func()) (bool, error) {
	return f.isLeader, f.err
}

func (f *fakeLeader) Release(streamID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.released = append(f.released, streamID)
}

func (f *fakeLeader) releasedIDs() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.released...)
}

// fakeRunner reports a fixed poll shape, then returns err. observed records the
// (actions, messages) tuples it fed the observer, which is how the tests assert
// that a zero-message poll is still counted as a poll.
type fakeRunner struct {
	actions  int
	messages int
	err      error

	mu      sync.Mutex
	runs    []Target
	started chan struct{}
}

func (f *fakeRunner) Run(ctx context.Context, t Target, _, _ string, observe poller.PollObserver) error {
	f.mu.Lock()
	f.runs = append(f.runs, t)
	f.mu.Unlock()
	if f.started != nil {
		select {
		case f.started <- struct{}{}:
		default:
		}
	}
	if observe != nil {
		observe(f.actions, f.messages)
	}
	if f.err != nil {
		return f.err
	}
	<-ctx.Done()
	return ctx.Err()
}

func (f *fakeRunner) ranTargets() []Target {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]Target(nil), f.runs...)
}

// testMetrics returns a process-wide InnerTubeMetrics. NewInnerTubeMetrics
// registers with the default promauto registry, so calling it twice panics with
// a duplicate-collector error — which is what `go test -count=2` would do.
var (
	testMetricsOnce sync.Once
	testMetricsInst *metrics.InnerTubeMetrics
)

func testMetrics() *metrics.InnerTubeMetrics {
	testMetricsOnce.Do(func() { testMetricsInst = metrics.NewInnerTubeMetrics() })
	return testMetricsInst
}

// uniqueID mints a label value no other run has used, so counter assertions
// stay absolute instead of having to reason about carry-over.
var uniqueSeq atomic.Int64

func uniqueID(prefix string) string {
	return fmt.Sprintf("UC%s%d", prefix, uniqueSeq.Add(1))
}

func newTestCanary(cfg Config, src ContinuationSource, runner pollerRunner) *Canary {
	return &Canary{
		cfg:    cfg,
		source: src,
		runner: runner,
		logger: zap.NewNop(),
	}
}

func TestStartIsNoOpWhenDisabled(t *testing.T) {
	src := &fakeSource{cont: "tok"}
	runner := &fakeRunner{}
	c := newTestCanary(Config{Enabled: false, Targets: []Target{{"UCchan", "vid"}}}, src, runner)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c.Start(ctx)
	c.Wait()

	if got := len(src.calls()); got != 0 {
		t.Errorf("disabled canary made %d continuation calls, want 0", got)
	}
}

// The blind spot is capture, so the canary must count a poll even when that
// poll captured nothing — that zero is the entire signal the alert reads. A
// canary that only counted productive polls would go silent in exactly the
// state it exists to report, and YouTubeInnerTubeCanaryDown would fire instead
// of YouTubeInnerTubeCapturingNothing.
func TestObserverCountsPollsIncludingEmptyOnes(t *testing.T) {
	m := testMetrics()
	c := &Canary{logger: zap.NewNop(), metrics: m}
	// Unique label values: counters live in a process-wide registry and would
	// otherwise carry over between repeated runs (`go test -count=2`).
	channelID := uniqueID("chan")
	obs := c.observer(Target{channelID, "vid"})

	obs(0, 0) // an empty poll: still a poll
	obs(5, 3) // three captured messages
	obs(2, 0) // actions that parsed to nothing

	polls := testutil.ToFloat64(m.CanaryPolls.WithLabelValues(metrics.ServiceLabel, channelID, "vid"))
	if polls != 3 {
		t.Errorf("canary polls = %v, want 3", polls)
	}
	messages := testutil.ToFloat64(m.CanaryMessages.WithLabelValues(metrics.ServiceLabel, channelID, "vid"))
	if messages != 3 {
		t.Errorf("canary messages = %v, want 3", messages)
	}
}

// The alert reads `canary_polls > 0 and canary_messages ~= 0`, and a CounterVec
// child does not exist until WithLabelValues touches it. So a canary that has
// never captured a message must still EXPORT the messages series at zero —
// otherwise the `and` has an empty right-hand side and
// YouTubeInnerTubeCapturingNothing cannot fire in precisely the case it was
// rewritten to catch: blind from process start, a bad continuation shipped or a
// pod restarted while capture was already broken.
//
// This asserts on the gathered registry rather than testutil.ToFloat64, because
// ToFloat64 calls WithLabelValues and would itself create the child, reporting 0
// for a series Prometheus would never have scraped.
func TestObserverExportsMessagesSeriesBeforeAnyMessageIsCaptured(t *testing.T) {
	m := testMetrics()
	channelID := uniqueID("blind")
	c := &Canary{logger: zap.NewNop(), metrics: m}

	// Only empty polls: the listener is blind and has captured nothing, ever.
	obs := c.observer(Target{channelID, "vid"})
	obs(0, 0)
	obs(0, 0)

	if !seriesExists(t, "youtube_innertube_canary_messages_total", channelID) {
		t.Error("youtube_innertube_canary_messages_total is absent after empty polls; " +
			"the critical alert's `and` would evaluate to an empty vector and never fire")
	}
	if !seriesExists(t, "youtube_innertube_canary_polls_total", channelID) {
		t.Error("youtube_innertube_canary_polls_total is absent; the canary looks dead while it is polling")
	}
}

// seriesExists reports whether the default gatherer would expose a sample of
// metricName carrying channelID — i.e. whether Prometheus would actually scrape
// it, as opposed to whether we can conjure it by asking for it.
func seriesExists(t *testing.T, metricName, channelID string) bool {
	t.Helper()
	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	for _, fam := range families {
		if fam.GetName() != metricName {
			continue
		}
		for _, metric := range fam.GetMetric() {
			for _, label := range metric.GetLabel() {
				if label.GetName() == "channel_id" && label.GetValue() == channelID {
					return true
				}
			}
		}
	}
	return false
}

// realRunner is where the design's central invariant actually lives: the canary
// counts messages and then DROPS them. That property comes from omitting
// collaborators rather than from a mode flag, so it is invisible to every test
// that fakes pollerRunner — an edit adding Publisher: or Metrics: to that struct
// literal would push a busy 24/7 channel's chat onto chat:raw, or move the
// production counters the aggregate backstop alert selects on, with nothing
// failing. This pins it.
func TestRealRunnerBuildsAPollerThatCannotPublishOrPolluteMetrics(t *testing.T) {
	r := &realRunner{
		client:   nil, // never polled: we only inspect how the poller was built
		source:   &fakeSource{cont: "tok"},
		interval: 2 * time.Second,
		logger:   zap.NewNop(),
	}

	p := r.newPoller(Target{"UCchan", "vid"}, "tok", "visitor", func(int, int) {})

	if p.HasPublisher() {
		t.Error("canary poller has a Publisher: canary traffic would reach the lifecycle stream")
	}
	if p.HasRepository() {
		t.Error("canary poller has a Repository: the canary would write stream state for a stream nobody demanded")
	}
	if p.HasMetrics() {
		t.Error("canary poller has Metrics: canary polls would move the production counters " +
			"(youtube_listener_zero_action_polls_total and friends), so the aggregate backstop " +
			"alert would be measuring the canary instead of the users")
	}
}

// Production skips its sleep after any poll that returned messages, to keep
// viewer latency near zero. The canary is pinned by design to channels whose
// chat is continuously busy, so inheriting that would mean it never sleeps:
// several requests per second per target, out of the same 429 budget as real
// capture, for a detector that only needs to know whether capture works. It
// would also stop the configured interval being a rate floor, which the
// aggregate backstop alert's threshold reasoning relies on.
func TestRealRunnerPollerAlwaysSleepsSoTheIntervalIsARateFloor(t *testing.T) {
	r := &realRunner{
		source:   &fakeSource{cont: "tok"},
		interval: 2 * time.Second,
		logger:   zap.NewNop(),
	}

	p := r.newPoller(Target{"UCchan", "vid"}, "tok", "", func(int, int) {})

	if !p.AlwaysSleeps() {
		t.Error("canary poller re-polls immediately after a non-empty poll; on a busy canary channel " +
			"that is several req/s per target, not one per interval")
	}
}

// A Canary built without metrics must not panic: a detector that takes the
// process down is worse than no detector.
func TestObserverToleratesNilMetrics(t *testing.T) {
	c := &Canary{logger: zap.NewNop()}
	c.observer(Target{"UCchan", "vid"})(0, 0)
}

func TestPollOnceSkipsWhenNotLeader(t *testing.T) {
	src := &fakeSource{cont: "tok"}
	runner := &fakeRunner{}
	c := newTestCanary(Config{Enabled: true}, src, runner)
	c.leader = &fakeLeader{isLeader: false}

	if err := c.pollOnce(context.Background(), Target{"UCchan", "vid"}); err != nil {
		t.Fatalf("pollOnce: %v", err)
	}
	if got := len(src.calls()); got != 0 {
		t.Errorf("non-leader fetched %d continuations, want 0 — every replica polling would multiply our YouTube rate", got)
	}
	if got := len(runner.ranTargets()); got != 0 {
		t.Errorf("non-leader ran %d pollers, want 0", got)
	}
}

func TestPollOnceReleasesLeadershipWhenThePollEnds(t *testing.T) {
	src := &fakeSource{cont: "tok"}
	runner := &fakeRunner{err: errors.New("boom")}
	leader := &fakeLeader{isLeader: true}
	c := newTestCanary(Config{Enabled: true}, src, runner)
	c.leader = leader

	_ = c.pollOnce(context.Background(), Target{"UCchan", "vid"})

	released := leader.releasedIDs()
	if len(released) != 1 || released[0] != "canary:vid" {
		t.Errorf("released = %v, want [canary:vid]", released)
	}
}

func TestPollOnceUsesTheProductionContinuationPath(t *testing.T) {
	src := &fakeSource{cont: "tok"}
	runner := &fakeRunner{err: errors.New("stop")}
	c := newTestCanary(Config{Enabled: true}, src, runner)

	_ = c.pollOnce(context.Background(), Target{"UCchan", "vid"})

	calls := src.calls()
	if len(calls) != 1 || calls[0].VideoID != "vid" || calls[0].ChannelID != "UCchan" {
		t.Fatalf("GetInitialContinuation calls = %v, want one for UCchan/vid", calls)
	}
	ran := runner.ranTargets()
	if len(ran) != 1 || ran[0].VideoID != "vid" {
		t.Fatalf("poller runs = %v, want one for vid", ran)
	}
}

func TestPollOnceDoesNotPollWhenContinuationFails(t *testing.T) {
	src := &fakeSource{err: errors.New("429 rate limited")}
	runner := &fakeRunner{}
	c := newTestCanary(Config{Enabled: true}, src, runner)

	if err := c.pollOnce(context.Background(), Target{"UCchan", "vid"}); err == nil {
		t.Fatal("pollOnce returned nil, want the continuation error")
	}
	if got := len(runner.ranTargets()); got != 0 {
		t.Errorf("ran %d pollers without a continuation, want 0", got)
	}
}

// A transient failure must never un-pin the canary: re-pinning on a 429 is how
// a canary drifts onto a chat-less simulcast and starts lying.
func TestRediscoverKeepsPinOnFailure(t *testing.T) {
	c := newTestCanary(Config{Enabled: true}, &fakeSource{}, &fakeRunner{})
	c.discoverer = &fakeDiscoverer{err: errors.New("429")}

	if got := c.rediscover(context.Background(), Target{"UCchan", "vid"}); got != "" {
		t.Errorf("rediscover = %q, want \"\" (keep the pin)", got)
	}
}

func TestRediscoverRepinsWhenAStreamEnded(t *testing.T) {
	c := newTestCanary(Config{Enabled: true}, &fakeSource{}, &fakeRunner{})
	c.discoverer = &fakeDiscoverer{videoID: "newvid"}

	if got := c.rediscover(context.Background(), Target{"UCchan", "oldvid"}); got != "newvid" {
		t.Errorf("rediscover = %q, want newvid", got)
	}
}

func TestRediscoverIgnoresAnUnchangedVideo(t *testing.T) {
	c := newTestCanary(Config{Enabled: true}, &fakeSource{}, &fakeRunner{})
	c.discoverer = &fakeDiscoverer{videoID: "vid"}

	if got := c.rediscover(context.Background(), Target{"UCchan", "vid"}); got != "" {
		t.Errorf("rediscover = %q, want \"\" for an unchanged pin", got)
	}
}

func TestIsStreamEnded(t *testing.T) {
	cases := map[string]bool{
		"stream may have ended (no liveChatRenderer)": true,
		"isReplay is true":                            true,
		"no live chat for this video":                 true,
		"429 Too Many Requests":                       false,
		"dial tcp: i/o timeout":                       false,
	}
	for msg, want := range cases {
		if got := isStreamEnded(errors.New(msg)); got != want {
			t.Errorf("isStreamEnded(%q) = %v, want %v", msg, got, want)
		}
	}
	if isStreamEnded(nil) {
		t.Error("isStreamEnded(nil) = true, want false")
	}
	// The sentinel the real runner returns must be recognised, otherwise a
	// canary whose stream ended never re-pins and stays dark forever.
	if !isStreamEnded(errStreamEnded) {
		t.Error("errStreamEnded not recognised as a stream end")
	}
}

// The supervisor must keep a target alive across restarts, and must re-pin
// (without waiting out the backoff) when the pinned video has ended.
func TestSuperviseRepinsAfterStreamEnd(t *testing.T) {
	src := &fakeSource{cont: "tok"}
	runner := &fakeRunner{err: errStreamEnded, started: make(chan struct{}, 4)}
	c := newTestCanary(Config{
		Enabled:            true,
		Targets:            []Target{{"UCchan", "oldvid"}},
		RediscoverInterval: time.Hour, // must not be reached on the re-pin path
	}, src, runner)
	c.discoverer = &fakeDiscoverer{videoID: "newvid"}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		c.supervise(ctx, Target{"UCchan", "oldvid"})
	}()

	// Two runs: the ended pin, then the re-pinned video. If the supervisor had
	// fallen through to the hour-long backoff instead, this would time out.
	deadline := time.After(2 * time.Second)
	for i := 0; i < 2; i++ {
		select {
		case <-runner.started:
		case <-deadline:
			t.Fatalf("only %d poller runs before the deadline, want 2", i)
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("supervise did not exit on context cancellation")
	}

	ran := runner.ranTargets()
	if len(ran) < 2 || ran[0].VideoID != "oldvid" || ran[1].VideoID != "newvid" {
		t.Fatalf("runs = %v, want oldvid then newvid", ran)
	}
}

// A pin that is simply wrong — never live, or long since archived — must not
// leave the canary dark forever. It fails the continuation fetch rather than
// reporting a stream end, so the supervisor needs a second route to re-pinning.
func TestSuperviseRepinsAfterRepeatedFailures(t *testing.T) {
	src := &fakeSource{err: errors.New("unexpected status code from next API: 404")}
	runner := &fakeRunner{started: make(chan struct{}, 4)}
	disc := &fakeDiscoverer{videoID: "newvid"}
	c := newTestCanary(Config{
		Enabled:            true,
		Targets:            []Target{{"UCchan", "badvid"}},
		RediscoverInterval: time.Millisecond,
	}, src, runner)
	c.discoverer = disc

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		c.supervise(ctx, Target{"UCchan", "badvid"})
	}()

	deadline := time.After(2 * time.Second)
	for {
		disc.mu.Lock()
		calls := disc.calls
		disc.mu.Unlock()
		if calls > 0 {
			break
		}
		select {
		case <-deadline:
			cancel()
			t.Fatal("a permanently failing pin never triggered rediscovery")
		case <-time.After(5 * time.Millisecond):
		}
	}
	cancel()
	<-done

	// The re-pinned video must be what gets attempted next.
	calls := src.calls()
	var sawNew bool
	for _, call := range calls {
		if call.VideoID == "newvid" {
			sawNew = true
		}
	}
	if !sawNew {
		t.Errorf("continuation attempts = %v, want one for the re-pinned newvid", calls)
	}
}

func TestSuperviseStopsOnContextCancel(t *testing.T) {
	src := &fakeSource{err: errors.New("429")}
	c := newTestCanary(Config{
		Enabled:            true,
		Targets:            []Target{{"UCchan", "vid"}},
		RediscoverInterval: 10 * time.Millisecond,
	}, src, &fakeRunner{})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		c.supervise(ctx, Target{"UCchan", "vid"})
	}()
	time.Sleep(30 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("supervise ignored context cancellation")
	}
}

func TestSleepCtxReturnsFalseOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if sleepCtx(ctx, time.Hour) {
		t.Error("sleepCtx returned true on a cancelled context")
	}
}

// The correlated-channel warning must survive the wiring check: it reports a
// property of the configuration, and an operator whose canary is both
// misconfigured and unwired should hear about both, not just the second. This
// ordering regressed once already.
func TestStartWarnsOnCorrelatedChannels(t *testing.T) {
	for _, tc := range []struct {
		name    string
		targets []Target
		wantLog bool
	}{
		{"same channel warns", []Target{{"UCaaa", "v1"}, {"UCaaa", "v2"}}, true},
		{"distinct channels stay silent", []Target{{"UCaaa", "v1"}, {"UCbbb", "v2"}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			core, logs := observer.New(zap.WarnLevel)
			// Deliberately unwired Options: the warning is about config, so it
			// must not be gated behind having a client and a source.
			c := New(Config{Enabled: true, Targets: tc.targets}, zap.New(core), Options{})
			c.Start(t.Context())
			if got := logs.FilterMessageSnippet("share a channel").Len() > 0; got != tc.wantLog {
				t.Fatalf("correlated-channel warning logged = %v, want %v", got, tc.wantLog)
			}
		})
	}
}

// callbackLeader hands the lost-leadership callback back to the test so it can
// simulate the coordinator dropping the lease mid-session.
type callbackLeader struct {
	mu   sync.Mutex
	lost func()
	got  chan struct{}
}

func (l *callbackLeader) EnsureLeadership(_ context.Context, _ string, lostCallback func()) (bool, error) {
	l.mu.Lock()
	l.lost = lostCallback
	l.mu.Unlock()
	if l.got != nil {
		select {
		case l.got <- struct{}{}:
		default:
		}
	}
	return true, nil
}

func (l *callbackLeader) Release(string) {}

func (l *callbackLeader) fireLost() {
	l.mu.Lock()
	cb := l.lost
	l.mu.Unlock()
	if cb != nil {
		cb()
	}
}

// TestCanary_LostLeadershipStopsPolling asserts that losing the lease ends the
// poll session rather than merely logging.
//
// The lease can be dropped underneath a live session — a failed heartbeat
// renewal, or the coordinator shedding it during a rebalance. If the session
// kept running, this replica would go on polling a target another replica has
// since claimed: two replicas on the same continuation, double the YouTube
// request rate, and both canary counters incremented twice per poll, which is
// exactly the duplication the leadership gate exists to prevent.
func TestCanary_LostLeadershipStopsPolling(t *testing.T) {
	leader := &callbackLeader{got: make(chan struct{}, 1)}
	runner := &fakeRunner{started: make(chan struct{}, 1)}

	c := New(Config{
		Enabled:            true,
		Targets:            []Target{{ChannelID: "UCtest", VideoID: "vid123"}},
		PollInterval:       time.Hour,
		RediscoverInterval: time.Hour,
	}, zap.NewNop(), Options{
		Source:  &fakeSource{cont: "cont"},
		Leader:  leader,
		Metrics: testMetrics(),
	})
	c.runner = runner

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- c.pollOnce(ctx, Target{ChannelID: "UCtest", VideoID: "vid123"}) }()

	// Wait until the session is actually polling before dropping the lease,
	// otherwise the test could pass by racing ahead of the poll entirely.
	select {
	case <-runner.started:
	case <-time.After(2 * time.Second):
		t.Fatal("poll session never started")
	}

	leader.fireLost()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("losing the lease must cancel the poll session, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("poll session kept running after leadership was lost")
	}

	// The parent context is still live: only the lease loss stopped the session.
	if ctx.Err() != nil {
		t.Fatalf("parent context should still be live, got %v", ctx.Err())
	}
}
