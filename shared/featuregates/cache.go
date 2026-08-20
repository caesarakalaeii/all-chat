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

// Package featuregates provides an in-memory cache for feature gate state
// backed by the feature_gates PostgreSQL table. The cache is refreshed via
// Redis Pub/Sub invalidation (instant) and a 60s TTL ticker (fallback).
//
// ADR-0008: Feature Gate Infrastructure
package featuregates

import (
	"context"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	// PubSubChannel is the Redis Pub/Sub channel name for cache invalidation.
	// Publish any message to this channel to trigger all listeners to reload
	// from the database immediately.
	PubSubChannel = "feature-gates:invalidate"

	// refreshInterval is the TTL-based fallback refresh period.
	refreshInterval = 60 * time.Second

	// GateSharing is the feature key for overlay share requests.
	// Allows users to create and accept chat overlay shares.
	GateSharing = "sharing"

	// GateStreamSelection is the feature key for YouTube stream selection strategy.
	// Allows premium users to choose how the innertube listener picks among
	// multiple concurrent live streams (most viewers, title match, etc.).
	GateStreamSelection = "stream_selection"

	// GateModeration is the feature key for cross-platform chat moderation
	// (ADR-0017). Seeded premium-only so the write-path rolls out to a small
	// cohort first; flip to is_premium=false to graduate to all users.
	GateModeration = "moderation"

	// GateDelegatedModeration is the feature key for handing the moderation write-path to
	// someone else's account (ADR-0048). Deliberately separate from GateModeration so
	// delegation can be rolled back without disabling owner moderation, and keyed on the
	// overlay OWNER: a premium streamer's moderators moderate for free. Seeded premium-only
	// (migration 080).
	GateDelegatedModeration = "delegated_moderation"

	// GateEngagement is the feature key for starting All-Chat polls/predictions
	// (issue #523). Seeded premium-only: opening a round posts the round + participate
	// link to chat (announce_on_start), which consumes the streamer's send quota — a
	// paid capability. Flip to is_premium=false to graduate to all users. Viewer
	// participation and points earning are NOT gated by this.
	GateEngagement = "engagement"

	// GateDesktopControlSurfaces is the feature key for pairing a desktop control surface
	// (Stream Deck / StreamController) with a device token — ADR-0049 release requirement 1.
	// Mounted by auth-service on POST /me/devices/approve ONLY: gating the pairing step keeps
	// enforcement in one place and leaves the per-action gates (GateEngagement on starting a
	// round) untouched, so a paired device still clears exactly the gates a browser session
	// does. Seeded is_premium=false (migration 089) because three shipped documents state both
	// plugins are free; flip it with the feature-gate admin endpoint, no redeploy. The pasted
	// personal-access-token path (ADR-0051) predates this gate and is not covered by it.
	GateDesktopControlSurfaces = "desktop_control_surfaces"
)

// FeatureGate represents a single row from the feature_gates table.
type FeatureGate struct {
	Key         string
	IsPremium   bool
	EarlyAccess bool
	Description string
	UpdatedAt   time.Time
}

// gateFlags holds the per-key gate dimensions kept in the cache. is_premium
// (ADR-0008) and early_access (ADR-0020) are orthogonal: a feature can require
// premium, early-access (beta-tester), both, or neither.
type gateFlags struct {
	isPremium   bool
	earlyAccess bool
}

// FeatureGateCache maintains an in-memory map of feature gate states.
// It is safe for concurrent use. Zero DB hits at request time (D-10).
type FeatureGateCache struct {
	db     *pgxpool.Pool
	redis  *redis.Client
	logger *zap.Logger

	mu    sync.RWMutex
	gates map[string]gateFlags

	// refreshIntervalOverride allows tests to inject a shorter ticker period.
	refreshIntervalOverride time.Duration

	// onReload is called after each successful gates map update (test hook).
	onReload func()
}

// NewFeatureGateCache creates a new FeatureGateCache backed by db and rc.
// Call Start(ctx) to begin the background refresh goroutine.
func NewFeatureGateCache(db *pgxpool.Pool, rc *redis.Client, logger *zap.Logger) *FeatureGateCache {
	return &FeatureGateCache{
		db:     db,
		redis:  rc,
		logger: logger,
		gates:  make(map[string]gateFlags),
	}
}

// NewFeatureGateCacheWithGates creates a FeatureGateCache pre-populated with
// the given premium-gate map (key -> is_premium). Intended for unit tests that do
// not need a DB or Redis. early_access defaults to false for every key; use
// NewFeatureGateCacheWithEarlyAccess to seed early-access gates.
func NewFeatureGateCacheWithGates(gates map[string]bool) *FeatureGateCache {
	c := &FeatureGateCache{
		gates: make(map[string]gateFlags, len(gates)),
	}
	for k, v := range gates {
		c.gates[k] = gateFlags{isPremium: v}
	}
	return c
}

// NewFeatureGateCacheWithEarlyAccess creates a FeatureGateCache pre-populated with
// the given early-access map (key -> early_access). Intended for unit tests of the
// early-access gate. is_premium defaults to false for every key.
func NewFeatureGateCacheWithEarlyAccess(gates map[string]bool) *FeatureGateCache {
	c := &FeatureGateCache{
		gates: make(map[string]gateFlags, len(gates)),
	}
	for k, v := range gates {
		c.gates[k] = gateFlags{earlyAccess: v}
	}
	return c
}

// NewFeatureGateCacheForTest creates a FeatureGateCache backed only by Redis
// (no DB). onReload is called every time the cache would reload from DB —
// useful for asserting that Pub/Sub and ticker callbacks fire correctly.
func NewFeatureGateCacheForTest(rc *redis.Client, onReload func()) *FeatureGateCache {
	return &FeatureGateCache{
		redis:    rc,
		gates:    make(map[string]gateFlags),
		onReload: onReload,
	}
}

// NewFeatureGateCacheForTestWithInterval is like NewFeatureGateCacheForTest but
// allows overriding the refresh ticker interval for fast periodic-reload tests.
func NewFeatureGateCacheForTestWithInterval(rc *redis.Client, onReload func(), interval time.Duration) *FeatureGateCache {
	return &FeatureGateCache{
		redis:                   rc,
		gates:                   make(map[string]gateFlags),
		onReload:                onReload,
		refreshIntervalOverride: interval,
	}
}

// IsPremium returns true if the given feature gate requires premium access.
//
// Safe default: returns true for any unknown key (key not in DB). Unknown keys
// are treated as premium-required to avoid accidentally opening unreviewd
// features to all users. (D-10 pitfall 2)
func (c *FeatureGateCache) IsPremium(key string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	val, ok := c.gates[key]
	if !ok {
		return true // unknown key: safe default, treat as premium-required
	}
	return val.isPremium
}

// IsEarlyAccess returns true if the given feature gate is an early-access feature
// (ADR-0020), reachable only by beta-testers via middleware.RequireEarlyAccess.
//
// Safe default: returns true for any unknown key, mirroring IsPremium — an
// unseeded key fails closed (beta-only) rather than silently opening the feature.
func (c *FeatureGateCache) IsEarlyAccess(key string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	val, ok := c.gates[key]
	if !ok {
		return true // unknown key: safe default, treat as early-access-required
	}
	return val.earlyAccess
}

// Start performs an initial reload from DB, subscribes to the Pub/Sub
// invalidation channel, and launches a background goroutine for periodic
// and event-driven refreshes.
//
// Start is non-blocking — the background loop runs in a goroutine.
// ctx cancellation stops the background goroutine.
func (c *FeatureGateCache) Start(ctx context.Context) error {
	// Initial reload (may be skipped in test mode where db is nil)
	if c.db != nil {
		if err := c.reload(ctx); err != nil {
			if c.logger != nil {
				c.logger.Warn("FeatureGateCache: initial reload failed", zap.Error(err))
			}
			// Non-fatal: cache starts empty, will retry on next tick/invalidation
		}
	}

	if c.redis == nil {
		// No Redis — run without Pub/Sub (just periodic reload if db is available)
		if c.db != nil {
			go c.runWithoutPubSub(ctx)
		}
		return nil
	}

	pubsub := c.redis.Subscribe(ctx, PubSubChannel)
	go c.run(ctx, pubsub)
	return nil
}

// run is the main background loop. It listens on the Pub/Sub channel and the
// periodic ticker, reloading the gates map on each event.
// Follows the lifecycle_subscriber.go pattern exactly.
func (c *FeatureGateCache) run(ctx context.Context, pubsub *redis.PubSub) {
	defer pubsub.Close()

	if c.logger != nil {
		c.logger.Info("FeatureGateCache started",
			zap.String("channel", PubSubChannel))
	}

	interval := refreshInterval
	if c.refreshIntervalOverride > 0 {
		interval = c.refreshIntervalOverride
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			if c.logger != nil {
				c.logger.Info("FeatureGateCache stopping")
			}
			return
		case _, ok := <-ch:
			if !ok {
				if c.logger != nil {
					c.logger.Warn("FeatureGateCache: Pub/Sub channel closed")
				}
				return
			}
			c.triggerReload(ctx)
		case <-ticker.C:
			c.triggerReload(ctx)
		}
	}
}

// runWithoutPubSub is a simplified loop used when Redis is nil but a DB is
// available. Only periodic reload (no Pub/Sub).
func (c *FeatureGateCache) runWithoutPubSub(ctx context.Context) {
	interval := refreshInterval
	if c.refreshIntervalOverride > 0 {
		interval = c.refreshIntervalOverride
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.triggerReload(ctx)
		}
	}
}

// triggerReload reloads from DB (if db is non-nil) and always calls onReload.
func (c *FeatureGateCache) triggerReload(ctx context.Context) {
	if c.db != nil {
		if err := c.reload(ctx); err != nil {
			if c.logger != nil {
				c.logger.Warn("FeatureGateCache: reload failed", zap.Error(err))
			}
		}
	}
	if c.onReload != nil {
		c.onReload()
	}
}

// reload queries the DB for all feature gates and atomically swaps the
// in-memory map under a write lock.
func (c *FeatureGateCache) reload(ctx context.Context) error {
	rows, err := c.db.Query(ctx, "SELECT feature_key, is_premium, early_access FROM feature_gates")
	if err != nil {
		return err
	}
	defer rows.Close()

	newGates := make(map[string]gateFlags)
	for rows.Next() {
		var key string
		var flags gateFlags
		if err := rows.Scan(&key, &flags.isPremium, &flags.earlyAccess); err != nil {
			return err
		}
		newGates[key] = flags
	}
	if err := rows.Err(); err != nil {
		return err
	}

	c.mu.Lock()
	c.gates = newGates
	c.mu.Unlock()

	if c.logger != nil {
		c.logger.Debug("FeatureGateCache reloaded", zap.Int("count", len(newGates)))
	}
	return nil
}

// GetAll returns all feature gates from the DB. Used by admin endpoints.
func (c *FeatureGateCache) GetAll(ctx context.Context) ([]FeatureGate, error) {
	if c.db == nil {
		return nil, nil
	}

	rows, err := c.db.Query(ctx,
		"SELECT feature_key, is_premium, early_access, description, updated_at FROM feature_gates ORDER BY feature_key")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var gates []FeatureGate
	for rows.Next() {
		var g FeatureGate
		if err := rows.Scan(&g.Key, &g.IsPremium, &g.EarlyAccess, &g.Description, &g.UpdatedAt); err != nil {
			return nil, err
		}
		gates = append(gates, g)
	}
	return gates, rows.Err()
}
