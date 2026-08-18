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
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/caesar/all-chat/services/youtube-listener-innertube/innertube"
	"github.com/caesar/all-chat/services/youtube-listener-innertube/metrics"
	"github.com/caesar/all-chat/services/youtube-listener-innertube/poller"
)

// ContinuationSource fetches the initial continuation token and visitorData for
// a video. Satisfied by *innertube.Discovery — the canary deliberately goes
// through the same GetInitialContinuation the production path uses, because
// that is where the continuation bug of #575 lived. A canary with its own
// shortcut would have been green throughout that outage.
type ContinuationSource interface {
	GetInitialContinuation(ctx context.Context, videoID, channelID string) (string, string, error)
}

// StreamDiscoverer re-pins a canary target's video ID when the pinned one stops
// working. Satisfied by *innertube.Discovery. Optional: without it a target
// whose stream ended simply stays down and YouTubeInnerTubeCanaryDown says so.
type StreamDiscoverer interface {
	DiscoverLiveStream(ctx context.Context, channelID, strategy, matchTerm string) (string, error)
}

// Leader gates the canary to one instance. Satisfied by
// *sourcemanager.LeadershipCoordinator. Without it every replica would poll
// every canary, multiplying our YouTube request rate by the replica count for
// no extra signal.
type Leader interface {
	EnsureLeadership(ctx context.Context, streamID string, lostCallback func()) (bool, error)
	Release(streamID string)
}

// pollerRunner starts a poller and reports when it has exited. Split out so the
// tests can drive the supervision loop without an InnerTube client.
type pollerRunner interface {
	// Run polls until ctx is cancelled or the stream ends. observe is called
	// once per successful poll with (actions, messages).
	Run(ctx context.Context, t Target, continuation, visitorData string, observe poller.PollObserver) error
}

// Canary polls known-busy channels and counts what it captures without
// publishing any of it.
type Canary struct {
	cfg        Config
	source     ContinuationSource
	discoverer StreamDiscoverer
	leader     Leader
	runner     pollerRunner
	metrics    *metrics.InnerTubeMetrics
	logger     *zap.Logger

	wg sync.WaitGroup
}

// Options carries the collaborators for New. All of them may be nil except the
// logger; a Canary with a nil ContinuationSource can be constructed but will
// not poll, which keeps main.go's wiring free of conditionals.
type Options struct {
	Source     ContinuationSource
	Discoverer StreamDiscoverer
	Leader     Leader
	Metrics    *metrics.InnerTubeMetrics
	Client     poller.ClientInterface
}

// New builds a Canary. Start is a no-op unless cfg.Enabled.
func New(cfg Config, logger *zap.Logger, opts Options) *Canary {
	c := &Canary{
		cfg:        cfg,
		source:     opts.Source,
		discoverer: opts.Discoverer,
		leader:     opts.Leader,
		metrics:    opts.Metrics,
		logger:     logger,
	}
	if opts.Client != nil {
		c.runner = &realRunner{
			client:   opts.Client,
			source:   opts.Source,
			interval: cfg.PollInterval,
			logger:   logger,
		}
	}
	return c
}

// Start launches one supervision goroutine per target. Non-blocking.
func (c *Canary) Start(ctx context.Context) {
	if !c.cfg.Enabled {
		c.logger.Info("YouTube InnerTube canary disabled")
		return
	}
	if c.source == nil || c.runner == nil {
		c.logger.Warn("YouTube InnerTube canary enabled but not wired (no continuation source or client); not starting")
		return
	}

	c.logger.Info("Starting YouTube InnerTube canary",
		zap.Int("targets", len(c.cfg.Targets)),
		zap.Duration("poll_interval", c.cfg.PollInterval),
		zap.Duration("rediscover_interval", c.cfg.RediscoverInterval),
	)

	for _, t := range c.cfg.Targets {
		c.wg.Add(1)
		go func(t Target) {
			defer c.wg.Done()
			c.supervise(ctx, t)
		}(t)
	}
}

// Wait blocks until every supervision goroutine has exited. Only meaningful
// after the context passed to Start is cancelled.
func (c *Canary) Wait() { c.wg.Wait() }

// supervise keeps one target polling: acquire leadership, fetch a continuation,
// poll until the stream ends, then back off and try again. Every failure path
// leads back to the same wait so a permanently-dead target costs one YouTube
// request per RediscoverInterval and nothing else.
func (c *Canary) supervise(ctx context.Context, t Target) {
	// pinnedVideoID starts as the configured pin and only moves when the pin
	// stops working — see rediscover.
	pinnedVideoID := t.VideoID

	for {
		if ctx.Err() != nil {
			return
		}

		target := Target{ChannelID: t.ChannelID, VideoID: pinnedVideoID}
		ended := c.pollOnce(ctx, target)

		if ended != nil && isStreamEnded(ended) {
			if newID := c.rediscover(ctx, t); newID != "" {
				pinnedVideoID = newID
				// Re-pinned: retry immediately rather than sitting out the
				// backoff, so a canary channel restarting its stream costs us
				// minutes of blindness, not tens of minutes.
				continue
			}
		}

		if !sleepCtx(ctx, c.cfg.RediscoverInterval) {
			return
		}
	}
}

// pollOnce runs a single leadership-gated polling session for a target and
// returns the error that ended it (nil if it ended cleanly or was never
// started).
func (c *Canary) pollOnce(ctx context.Context, t Target) error {
	streamID := "canary:" + t.VideoID

	if c.leader != nil {
		isLeader, err := c.leader.EnsureLeadership(ctx, streamID, func() {
			c.logger.Info("Canary lost leadership",
				zap.String("channel_id", t.ChannelID),
				zap.String("video_id", t.VideoID),
			)
		})
		if err != nil {
			c.logger.Warn("Canary leadership claim failed",
				zap.String("video_id", t.VideoID),
				zap.Error(err))
			return err
		}
		if !isLeader {
			// Another replica holds this canary. Nothing to do and nothing
			// wrong: the metrics come from whichever instance is polling.
			c.logger.Debug("Canary leadership held by another instance",
				zap.String("video_id", t.VideoID))
			return nil
		}
		defer c.leader.Release(streamID)
	}

	continuation, visitorData, err := c.source.GetInitialContinuation(ctx, t.VideoID, t.ChannelID)
	if err != nil {
		c.logger.Warn("Canary failed to get initial continuation",
			zap.String("channel_id", t.ChannelID),
			zap.String("video_id", t.VideoID),
			zap.Error(err))
		return err
	}

	c.logger.Info("Canary polling",
		zap.String("channel_id", t.ChannelID),
		zap.String("video_id", t.VideoID))

	return c.runner.Run(ctx, t, continuation, visitorData, c.observer(t))
}

// observer counts polls and messages, and drops the messages.
//
// Dropping is the whole point of the design: publishing canary traffic to
// chat:raw would push a busy 24/7 channel's chat through message-processor,
// emote enrichment, the AllChatPlatformMessagesEmpty ratio and the DAU/WAU/MAU
// aggregates, permanently skewing every one of them. The blind spot this
// detector covers is capture, and capture is fully observed here — Redis
// publish, the processor and the gateway each have their own alerts.
func (c *Canary) observer(t Target) poller.PollObserver {
	return func(actionCount, messageCount int) {
		if c.metrics == nil {
			return
		}
		c.metrics.CanaryPolls.WithLabelValues(metrics.ServiceLabel, t.ChannelID, t.VideoID).Inc()
		if messageCount > 0 {
			c.metrics.CanaryMessages.WithLabelValues(metrics.ServiceLabel, t.ChannelID, t.VideoID).
				Add(float64(messageCount))
		}
	}
}

// rediscover re-pins a target whose stream has ended. It returns "" when it
// cannot, which leaves the original pin in place — a transient YouTube failure
// must not un-pin a canary permanently.
func (c *Canary) rediscover(ctx context.Context, t Target) string {
	if c.discoverer == nil {
		return ""
	}
	// most_viewers, not first_found: 24/7 canary channels run several
	// concurrent streams and the browse order routinely puts a near-empty
	// simulcast first (#473). The pin normally protects us from that; this is
	// the one moment the pin has to be re-derived, so it uses the strategy that
	// picks the main stream.
	videoID, err := c.discoverer.DiscoverLiveStream(ctx, t.ChannelID, innertube.StrategyMostViewers, "")
	if err != nil {
		c.logger.Warn("Canary rediscovery failed, keeping pinned video",
			zap.String("channel_id", t.ChannelID),
			zap.String("pinned_video_id", t.VideoID),
			zap.Error(err))
		return ""
	}
	if videoID == "" || videoID == t.VideoID {
		return ""
	}
	c.logger.Info("Canary re-pinned to a new video",
		zap.String("channel_id", t.ChannelID),
		zap.String("old_video_id", t.VideoID),
		zap.String("new_video_id", videoID))
	return videoID
}

// isStreamEnded reports whether an error from the continuation path means the
// pinned video is no longer live (as opposed to a transient failure, where
// re-pinning would be the wrong move).
func isStreamEnded(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "stream may have ended") ||
		strings.Contains(s, "isReplay") ||
		strings.Contains(s, "no live chat")
}

// sleepCtx waits for d, returning false if the context was cancelled first.
func sleepCtx(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		d = DefaultRediscoverInterval
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-ctx.Done():
		return false
	}
}

// realRunner drives an actual poller.Poller. The canary sets no Repository and
// no Publisher: it must never write stream lifecycle state or publish a single
// message. It does set the Refresher, because stale-continuation recovery is
// part of the production code path being exercised.
type realRunner struct {
	client   poller.ClientInterface
	source   ContinuationSource
	interval time.Duration
	logger   *zap.Logger
}

func (r *realRunner) Run(ctx context.Context, t Target, continuation, visitorData string, observe poller.PollObserver) error {
	refresher, _ := r.source.(poller.ContinuationRefresher)

	p := poller.NewPoller(
		r.client,
		continuation,
		t.ChannelID,
		r.logger.With(zap.String("component", "canary")),
		&poller.PollerOptions{
			Interval:     r.interval,
			VideoID:      t.VideoID,
			VisitorData:  visitorData,
			Refresher:    refresher,
			PollObserver: observe,
			// No Metrics: the canary must not move
			// youtube_listener_zero_action_polls_total or the other
			// production counters, or the aggregate backstop alert would be
			// measuring the canary instead of the users.
			// No Repository / Publisher: nothing about a canary belongs in
			// stream state or on chat:raw.
		},
	)

	if err := p.Start(ctx); err != nil {
		return err
	}

	select {
	case <-p.IsDone():
		// Stream ended, or the poller hit a fatal error. Either way the
		// supervisor decides what to do next.
		return errStreamEnded
	case <-ctx.Done():
		p.Stop()
		return ctx.Err()
	}
}

// errStreamEnded marks a poller that exited on its own. It is matched by
// isStreamEnded so the supervisor tries to re-pin the video.
var errStreamEnded = streamEndedError{}

type streamEndedError struct{}

func (streamEndedError) Error() string { return "canary poller exited: stream may have ended" }
