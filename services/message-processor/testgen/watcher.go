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

package testgen

import (
	"context"
	"encoding/json"
	"os"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	// connectionsChannel and presenceKeyPrefix mirror what the api-gateway's
	// WebSocket manager writes: a presence key per connected overlay and a
	// Pub/Sub event on connect/disconnect. This is the same demand signal the
	// youtube-listener and source-manager consume.
	connectionsChannel = "overlay:connections"
	presenceKeyPrefix  = "overlay:connected:"

	// reconcileInterval bounds how long after a connect/disconnect (or a missed
	// event / pod restart) the generator state converges to presence.
	reconcileInterval = 10 * time.Second
)

// connectionEvent matches api-gateway/websocket.OverlayConnectionEvent.
type connectionEvent struct {
	Type      string `json:"type"`
	OverlayID string `json:"overlay_id"`
}

// DemandWatcher starts the generator while a WebSocket client is connected to
// the test overlay and stops it when the client disconnects. It reacts to
// overlay:connections events and reconciles against the presence key on a timer
// so it self-heals across missed events and restarts.
type DemandWatcher struct {
	overlayID string
	redis     *redis.Client
	gen       *Generator
	cfg       Config
	logger    *zap.Logger
}

// NewDemandWatcher wires a watcher to the shared Redis client and generator.
func NewDemandWatcher(overlayID string, rdb *redis.Client, gen *Generator, cfg Config, logger *zap.Logger) *DemandWatcher {
	return &DemandWatcher{
		overlayID: overlayID,
		redis:     rdb,
		gen:       gen,
		cfg:       cfg,
		logger:    logger,
	}
}

// Run blocks until ctx is cancelled, keeping the generator in sync with
// connection presence.
func (w *DemandWatcher) Run(ctx context.Context) {
	w.logger.Info("Test stream demand watcher started",
		zap.String("overlay_id", w.overlayID),
		zap.Duration("reconcile_interval", reconcileInterval),
	)

	sub := w.redis.Subscribe(ctx, connectionsChannel)
	defer sub.Close()
	events := sub.Channel()

	ticker := time.NewTicker(reconcileInterval)
	defer ticker.Stop()

	// Converge immediately in case a client is already connected at startup.
	w.reconcile(ctx)

	for {
		select {
		case <-ctx.Done():
			w.gen.StopDemand()
			return
		case <-ticker.C:
			w.reconcile(ctx)
		case msg, ok := <-events:
			if !ok {
				// Subscription closed; the ticker still drives reconciliation.
				continue
			}
			var ev connectionEvent
			if err := json.Unmarshal([]byte(msg.Payload), &ev); err != nil {
				continue
			}
			if ev.OverlayID == w.overlayID {
				w.reconcile(ctx)
			}
		}
	}
}

// reconcile ensures the generator runs iff a client is connected to the overlay.
func (w *DemandWatcher) reconcile(ctx context.Context) {
	present, err := w.present(ctx)
	if err != nil {
		w.logger.Warn("Test stream presence check failed", zap.Error(err))
		return
	}

	running, mode := w.gen.State()
	switch {
	case present && !running:
		if _, err := w.gen.StartDemand(w.cfg); err != nil {
			w.logger.Debug("Test stream demand start skipped", zap.Error(err))
		}
	case !present && running && mode == modeDemand:
		w.gen.StopDemand()
		w.logger.Info("Test stream stopped (no client connected)", zap.String("overlay_id", w.overlayID))
	}
}

func (w *DemandWatcher) present(ctx context.Context) (bool, error) {
	n, err := w.redis.Exists(ctx, presenceKeyPrefix+w.overlayID).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// ConfigFromEnv reads the demand-run tuning knobs. Zero values are filled with
// generator defaults at start time.
func ConfigFromEnv() Config {
	c := Config{}
	if v := os.Getenv("TEST_STREAM_RATE"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			c.RatePerSecond = f
		}
	}
	if v := os.Getenv("TEST_STREAM_VOTE_RATIO"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			c.VoteRatio = f
		}
	}
	if v := os.Getenv("TEST_STREAM_EVENT_EVERY_N"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.EventEveryN = n
		}
	}
	return c
}
