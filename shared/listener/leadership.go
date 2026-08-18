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

package listener

import (
	"context"
	"sync"
	"time"

	"github.com/caesar/all-chat/shared/sourcemanager"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// LeadershipConfig holds the minimal configuration for LeadershipListener.
type LeadershipConfig struct {
	// Platform is the platform identifier for this listener (e.g. "kick", "twitch", "youtube").
	// It identifies the listener for leadership coordination.
	Platform string

	// DemandPlatform is the platform value to match against incoming demand-update sources
	// (which carry the overlay_chat_sources.platform, e.g. "twitch"). It defaults to Platform
	// when empty. Set it when the leadership identifier differs from the source platform —
	// e.g. twitch-eventsub-listener coordinates as "twitch-eventsub" but reads "twitch" sources.
	DemandPlatform string

	// DisableDemandFiltering treats all sources as having demand.
	// When true, the SDK demand subscriber loop exits immediately without subscribing
	// to source:demand Pub/Sub.
	DisableDemandFiltering bool
}

// demandPlatform returns the platform used to filter demand updates: DemandPlatform when
// set, otherwise Platform.
func (c LeadershipConfig) demandPlatform() string {
	if c.DemandPlatform != "" {
		return c.DemandPlatform
	}
	return c.Platform
}

// LeadershipListener is a standalone struct that owns leadership coordination and
// the demand subscriber loop. It is the sole SDK entry point for all listeners.
//
// When SOURCE_MANAGER_SECRET is absent, leadership is disabled and all methods
// are nil-safe (sourcemanager.LeadershipCoordinator handles nil receiver).
type LeadershipListener struct {
	config      LeadershipConfig
	redisClient *redis.Client
	logger      *zap.Logger
	coordinator *sourcemanager.LeadershipCoordinator
	smClient    *sourcemanager.Client
	cancel      context.CancelFunc
	wg          sync.WaitGroup
}

// NewLeadershipListenerFromEnv constructs a LeadershipListener reading SOURCE_MANAGER_SECRET
// and SOURCE_MANAGER_URL from the environment.
//
// When SOURCE_MANAGER_SECRET is absent, coordination is disabled (coordinator is nil).
// All *sourcemanager.LeadershipCoordinator methods are nil-safe, so callers may
// call l.LeadershipCoordinator().EnsureLeadership / Stop without nil-checks.
func NewLeadershipListenerFromEnv(platform string, redisClient *redis.Client, logger *zap.Logger) (*LeadershipListener, error) {
	secret := Env("SOURCE_MANAGER_SECRET", "")
	if secret == "" {
		if logger != nil {
			logger.Info("SOURCE_MANAGER_SECRET not set — leadership coordination disabled")
		}
		return &LeadershipListener{
			config:      LeadershipConfig{Platform: platform},
			redisClient: redisClient,
			logger:      logger,
		}, nil
	}

	smURL := Env("SOURCE_MANAGER_URL", "http://source-manager:8083")
	tokenSource := sourcemanager.NewSigningTokenSource(platform+"-listener", secret, 15*time.Minute)
	smClient, err := sourcemanager.NewClient(smURL, tokenSource)
	if err != nil {
		return nil, err
	}

	coordinator := sourcemanager.NewLeadershipCoordinator(platform, smClient, 5*time.Second, logger)

	return &LeadershipListener{
		config:      LeadershipConfig{Platform: platform},
		redisClient: redisClient,
		logger:      logger,
		coordinator: coordinator,
		smClient:    smClient,
	}, nil
}

// NewLeadershipListener constructs a LeadershipListener from an explicit config.
// Intended for tests — does not read environment variables, returns nil coordinator/smClient.
func NewLeadershipListener(config LeadershipConfig, redisClient *redis.Client, logger *zap.Logger) (*LeadershipListener, error) {
	return &LeadershipListener{
		config:      config,
		redisClient: redisClient,
		logger:      logger,
	}, nil
}

// Start calls mgr.Start and, unless demand filtering is disabled, launches the
// demand subscriber loop in a background goroutine.
func (ll *LeadershipListener) Start(ctx context.Context, mgr ChannelManager) error {
	if err := mgr.Start(ctx); err != nil {
		return err
	}

	internalCtx, cancel := context.WithCancel(ctx)
	ll.cancel = cancel

	if !ll.config.DisableDemandFiltering && ll.redisClient != nil {
		ll.wg.Add(1)
		go ll.startDemandSubscriberLoop(internalCtx, mgr)
	}

	return nil
}

// Stop cancels the internal context and waits for all background goroutines to exit.
func (ll *LeadershipListener) Stop() {
	if ll.cancel != nil {
		ll.cancel()
	}
	ll.wg.Wait()
}

// LeadershipCoordinator returns the leadership coordinator.
// May be nil when SOURCE_MANAGER_SECRET was absent — all methods on
// *sourcemanager.LeadershipCoordinator are nil-safe.
func (ll *LeadershipListener) LeadershipCoordinator() *sourcemanager.LeadershipCoordinator {
	return ll.coordinator
}

// SMClient returns the source manager client.
// May be nil when SOURCE_MANAGER_SECRET was absent — callers must nil-check before use.
func (ll *LeadershipListener) SMClient() *sourcemanager.Client {
	return ll.smClient
}

// SidecarCoordinator returns an additional coordinator that shares this
// listener's source-manager client but coordinates under its own platform, so
// its leases are invisible to the main coordinator.
//
// Use it for background work that needs single-instance election but is not a
// user-facing stream — a canary poller, for example. Keeping such leases out of
// the main coordinator matters because Rebalance caps a pod at
// ceil(totalStreams/peerCount) leases and compares that against the coordinator's
// *total* lease count. A lease the caller never counts in totalStreams therefore
// makes the pod look over-subscribed and sheds a real stream instead.
//
// Returns nil when leadership is disabled (SOURCE_MANAGER_SECRET absent), matching
// LeadershipCoordinator; all coordinator methods are nil-safe.
//
// Each call constructs a new coordinator, so callers should hold on to the result
// and Stop it themselves rather than calling this repeatedly.
func (ll *LeadershipListener) SidecarCoordinator(platform string) *sourcemanager.LeadershipCoordinator {
	if ll.smClient == nil {
		return nil
	}
	return sourcemanager.NewLeadershipCoordinator(platform, ll.smClient, 5*time.Second, ll.logger)
}

// SetDisableDemandFiltering configures whether the demand subscriber loop is skipped.
// Must be called before Start. When true, all platform sources are treated as in-demand.
func (ll *LeadershipListener) SetDisableDemandFiltering(v bool) {
	ll.config.DisableDemandFiltering = v
}

// SetDemandPlatform overrides the platform used to match incoming demand-update sources.
// Use it when the leadership identifier differs from the source platform (e.g.
// twitch-eventsub-listener coordinates as "twitch-eventsub" but reads "twitch" sources).
// Must be called before Start().
func (ll *LeadershipListener) SetDemandPlatform(p string) {
	ll.config.DemandPlatform = p
}
