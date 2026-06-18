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

// Package testgen drives a fake chat/event stream onto a single dedicated
// overlay's Pub/Sub channel so external tools can be tested against a realistic
// WebSocket feed without any real streaming platform. It deliberately targets
// one fixed overlay ID (see migration 058) to bound the blast radius of the
// public trigger endpoint.
package testgen

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/caesar/all-chat/services/message-processor/enricher"
	"github.com/caesar/all-chat/services/message-processor/models"
	"github.com/caesar/all-chat/services/message-processor/publisher"
	"go.uber.org/zap"
)

const (
	defaultDurationSeconds = 60
	defaultRatePerSecond   = 5.0
	defaultVoteRatio       = 0.4
	defaultEventEveryN     = 12

	maxDurationSeconds = 3600
	maxRatePerSecond   = 50.0
)

// Config controls a single run of the generator.
type Config struct {
	DurationSeconds int     `json:"duration_seconds"`
	RatePerSecond   float64 `json:"rate_per_second"`
	VoteRatio       float64 `json:"vote_ratio"`    // fraction of chat messages that are pure poll votes ("1".."4")
	EventEveryN     int     `json:"event_every_n"` // emit a platform event roughly every N published items (0 = never)
}

func (c *Config) applyDefaults() {
	if c.DurationSeconds <= 0 {
		c.DurationSeconds = defaultDurationSeconds
	}
	if c.DurationSeconds > maxDurationSeconds {
		c.DurationSeconds = maxDurationSeconds
	}
	if c.RatePerSecond <= 0 {
		c.RatePerSecond = defaultRatePerSecond
	}
	if c.RatePerSecond > maxRatePerSecond {
		c.RatePerSecond = maxRatePerSecond
	}
	if c.VoteRatio < 0 {
		c.VoteRatio = 0
	}
	if c.VoteRatio > 1 {
		c.VoteRatio = 1
	}
	if c.VoteRatio == 0 && c.EventEveryN == 0 {
		// Caller sent an essentially empty body — fall back to a useful mix.
		c.VoteRatio = defaultVoteRatio
		c.EventEveryN = defaultEventEveryN
	}
}

// Run modes. "demand" runs are driven by WebSocket connection presence (see
// DemandWatcher) and run continuously until the client disconnects; "manual"
// runs are started via the HTTP endpoint and are duration-bounded.
const (
	modeManual = "manual"
	modeDemand = "demand"
)

// Status is a snapshot of the generator state, safe to serialize as JSON.
type Status struct {
	Running      bool   `json:"running"`
	Mode         string `json:"mode,omitempty"` // "demand" or "manual"
	Continuous   bool   `json:"continuous"`
	OverlayID    string `json:"overlay_id"`
	StartedUnix  int64  `json:"started_unix,omitempty"`
	EndsUnix     int64  `json:"ends_unix,omitempty"`
	MessagesSent int64  `json:"messages_sent"`
	EventsSent   int64  `json:"events_sent"`
	Config       Config `json:"config"`
}

// Generator publishes a fake stream to a fixed overlay. It is a singleton:
// at most one run is active at a time.
type Generator struct {
	overlayID string
	pub       *publisher.PubSubPublisher
	emote     *enricher.Enricher
	cheermote *enricher.CheermoteEnricher
	logger    *zap.Logger

	mu         sync.Mutex
	running    bool
	mode       string
	continuous bool
	cancel     context.CancelFunc
	cfg        Config
	started    time.Time
	ends       time.Time
	msgCount   int64
	evtCount   int64
}

// NewGenerator wires a generator to the shared publisher/enrichers.
func NewGenerator(overlayID string, pub *publisher.PubSubPublisher, emote *enricher.Enricher, cheermote *enricher.CheermoteEnricher, logger *zap.Logger) *Generator {
	return &Generator{
		overlayID: overlayID,
		pub:       pub,
		emote:     emote,
		cheermote: cheermote,
		logger:    logger,
	}
}

// OverlayID returns the fixed overlay this generator targets.
func (g *Generator) OverlayID() string { return g.overlayID }

// Start kicks off a duration-bounded manual run (HTTP trigger). It returns an
// error if a run is already active.
func (g *Generator) Start(cfg Config) (Status, error) {
	cfg.applyDefaults()
	return g.start(cfg, false, modeManual)
}

// StartDemand kicks off a continuous run that streams until StopDemand is
// called. Used by the DemandWatcher while a client is connected.
func (g *Generator) StartDemand(cfg Config) (Status, error) {
	cfg.applyDefaults()
	return g.start(cfg, true, modeDemand)
}

func (g *Generator) start(cfg Config, continuous bool, mode string) (Status, error) {
	g.mu.Lock()
	if g.running {
		st := g.statusLocked()
		g.mu.Unlock()
		return st, fmt.Errorf("test stream already running")
	}

	ctx, cancel := context.WithCancel(context.Background())
	g.running = true
	g.mode = mode
	g.continuous = continuous
	g.cancel = cancel
	g.cfg = cfg
	g.started = time.Now()
	if continuous {
		g.ends = time.Time{}
	} else {
		g.ends = g.started.Add(time.Duration(cfg.DurationSeconds) * time.Second)
	}
	g.msgCount = 0
	g.evtCount = 0
	st := g.statusLocked()
	g.mu.Unlock()

	g.logger.Info("Test stream started",
		zap.String("overlay_id", g.overlayID),
		zap.String("mode", mode),
		zap.Bool("continuous", continuous),
		zap.Int("duration_seconds", cfg.DurationSeconds),
		zap.Float64("rate_per_second", cfg.RatePerSecond),
		zap.Float64("vote_ratio", cfg.VoteRatio),
	)

	go g.run(ctx, cfg, continuous)
	return st, nil
}

// Stop cancels any active run. Safe to call when nothing is running.
func (g *Generator) Stop() Status {
	g.mu.Lock()
	if g.running && g.cancel != nil {
		g.cancel()
	}
	st := g.statusLocked()
	g.mu.Unlock()
	return st
}

// StopDemand cancels an active run only if it was started by the DemandWatcher,
// so it never tears down a manual run.
func (g *Generator) StopDemand() Status {
	g.mu.Lock()
	if g.running && g.mode == modeDemand && g.cancel != nil {
		g.cancel()
	}
	st := g.statusLocked()
	g.mu.Unlock()
	return st
}

// State returns whether a run is active and which mode it is in.
func (g *Generator) State() (running bool, mode string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.running, g.mode
}

// Status returns a snapshot of the current state.
func (g *Generator) Status() Status {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.statusLocked()
}

func (g *Generator) statusLocked() Status {
	st := Status{
		Running:      g.running,
		Mode:         g.mode,
		Continuous:   g.continuous,
		OverlayID:    g.overlayID,
		MessagesSent: g.msgCount,
		EventsSent:   g.evtCount,
		Config:       g.cfg,
	}
	if !g.started.IsZero() {
		st.StartedUnix = g.started.Unix()
	}
	if !g.ends.IsZero() {
		st.EndsUnix = g.ends.Unix()
	}
	return st
}

func (g *Generator) run(ctx context.Context, cfg Config, continuous bool) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	interval := time.Duration(float64(time.Second) / cfg.RatePerSecond)
	if interval <= 0 {
		interval = time.Millisecond
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// A nil channel blocks forever in select, so continuous runs never time out.
	var timeoutC <-chan time.Time
	if !continuous {
		timeout := time.NewTimer(time.Duration(cfg.DurationSeconds) * time.Second)
		defer timeout.Stop()
		timeoutC = timeout.C
	}

	var n int64
	for {
		select {
		case <-ctx.Done():
			g.finish("cancelled")
			return
		case <-timeoutC:
			g.finish("duration_elapsed")
			return
		case <-ticker.C:
			n++
			isEvent := cfg.EventEveryN > 0 && n%int64(cfg.EventEveryN) == 0
			var msg *models.UnifiedChatMessage
			if isEvent {
				msg = g.buildEvent(rng)
			} else {
				msg = g.buildChat(rng, cfg.VoteRatio)
			}
			g.publish(ctx, msg, isEvent)
		}
	}
}

func (g *Generator) publish(ctx context.Context, msg *models.UnifiedChatMessage, isEvent bool) {
	// Best-effort enrichment so the stream looks like real traffic.
	if g.emote != nil {
		g.enrichEmotes(ctx, msg)
	}
	if g.cheermote != nil && msg.Platform == "twitch" {
		if err := g.cheermote.Enrich(ctx, msg); err != nil {
			g.logger.Debug("test stream cheermote enrich failed", zap.Error(err))
		}
	}
	if err := g.pub.Publish(ctx, g.overlayID, msg); err != nil {
		g.logger.Warn("test stream publish failed", zap.Error(err))
		return
	}
	g.mu.Lock()
	g.msgCount++
	if isEvent {
		g.evtCount++
	}
	g.mu.Unlock()
}

// enrichEmotes resolves third-party emotes for a generated message. The
// synthetic test channel doesn't exist on any emote provider, so we redirect the
// lookup to a real fallback channel (emoteFallbackChannel) and restore the
// message's own channel identity afterwards — external tools still see the test
// channel, but the stream carries realistic, resolvable emotes. The twitch
// channel hint covers non-Twitch platforms, which resolve 7TV channel emotes via
// a sibling Twitch channel.
func (g *Generator) enrichEmotes(ctx context.Context, msg *models.UnifiedChatMessage) {
	origChannelID := msg.ChannelID
	msg.ChannelID = emoteFallbackChannel

	if msg.Metadata == nil {
		msg.Metadata = map[string]interface{}{}
	}
	origHint, hadHint := msg.Metadata["twitch_channel_hint"]
	msg.Metadata["twitch_channel_hint"] = emoteFallbackChannel

	defer func() {
		msg.ChannelID = origChannelID
		if hadHint {
			msg.Metadata["twitch_channel_hint"] = origHint
		} else {
			delete(msg.Metadata, "twitch_channel_hint")
		}
	}()

	if err := g.emote.Enrich(ctx, msg); err != nil {
		g.logger.Debug("test stream emote enrich failed", zap.Error(err))
	}
}

func (g *Generator) finish(reason string) {
	g.mu.Lock()
	g.running = false
	g.cancel = nil
	sent := g.msgCount
	events := g.evtCount
	g.mu.Unlock()
	g.logger.Info("Test stream finished",
		zap.String("overlay_id", g.overlayID),
		zap.String("reason", reason),
		zap.Int64("messages_sent", sent),
		zap.Int64("events_sent", events),
	)
}
