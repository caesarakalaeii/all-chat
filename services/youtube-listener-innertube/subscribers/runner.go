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

package subscribers

import (
	"context"
	"sync"

	"go.uber.org/zap"
)

// Runner owns the per-channel subscriber pollers: exactly one poller per
// channel, started when the stream manager starts a chat poller, stopped
// when it stops one. The stream manager's lifecycle calls StartChannel /
// StopChannel on its own boundaries; the Runner never touches chat state.
//
// A nil *Runner is a valid disabled runner: every method is a no-op. That is
// how the listener ships when the OAuth/quota/DB credentials for subscriber
// polling are not configured.
type Runner struct {
	tokens   TokenSource
	api      SubscriberAPI
	quota    QuotaReserver
	announce Announcer
	logger   *zap.Logger
	metrics  *Metrics

	mu      sync.Mutex
	running map[string]*runningChannel // channelID → state
}

// runningChannel is one live per-channel poll loop.
type runningChannel struct {
	videoID string // generation token for StopChannel
	cancel  context.CancelFunc
	done    chan struct{}
}

// NewRunner builds a Runner. All dependencies must be non-nil; build it only
// when the service is fully configured for subscriber polling (see the
// env-gating in cmd/main.go).
func NewRunner(
	tokens TokenSource,
	api SubscriberAPI,
	quota QuotaReserver,
	announce Announcer,
	logger *zap.Logger,
	metrics *Metrics,
) *Runner {
	if metrics == nil {
		metrics = NopMetrics()
	}
	return &Runner{
		tokens:   tokens,
		api:      api,
		quota:    quota,
		announce: announce,
		logger:   logger,
		metrics:  metrics,
		running:  make(map[string]*runningChannel),
	}
}

// StartChannel starts the subscriber poll loop for a channel's live stream.
// Idempotent: a second call for a channel already running is a no-op (the
// stream manager can start a poller for the same channel after a rediscovery;
// the chat poller is keyed by video ID, the subscriber poller by channel).
func (r *Runner) StartChannel(channelID, videoID string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.running[channelID]; exists {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	rc := &runningChannel{videoID: videoID, cancel: cancel, done: make(chan struct{})}
	r.running[channelID] = rc

	p := NewPoller(channelID, r.tokens, r.api, r.quota, r.announce, r.logger, r.metrics, PollerOptions{VideoID: videoID})

	go func() {
		defer close(rc.done)
		p.Run(ctx)
	}()

	r.logger.Info("Subscriber polling started",
		zap.String("channel_id", channelID),
		zap.String("video_id", videoID))
}

// StopChannel stops the poll loop started for videoID and waits for it to
// exit. The video ID is the generation token: when a stream is rediscovered,
// the old loop must not kill the new one. Callers that end a specific stream
// pass that stream's video id; cleanup paths that end the channel wholesale
// pass "".
func (r *Runner) StopChannel(channelID, videoID string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	rc, exists := r.running[channelID]
	if exists && (videoID == "" || rc.videoID == videoID) {
		delete(r.running, channelID)
	} else {
		exists = false
	}
	r.mu.Unlock()

	if !exists {
		return
	}
	rc.cancel()
	<-rc.done
	r.logger.Info("Subscriber polling stopped",
		zap.String("channel_id", channelID),
		zap.String("video_id", videoID))
}

// StopAll stops every running channel (shutdown path).
func (r *Runner) StopAll() {
	if r == nil {
		return
	}
	r.mu.Lock()
	channels := make([]string, 0, len(r.running))
	for ch := range r.running {
		channels = append(channels, ch)
	}
	r.mu.Unlock()

	for _, ch := range channels {
		r.StopChannel(ch, "")
	}
}
