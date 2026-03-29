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
)

// FeatureGate represents a single row from the feature_gates table.
type FeatureGate struct {
	Key         string
	IsPremium   bool
	Description string
	UpdatedAt   time.Time
}

// FeatureGateCache maintains an in-memory map of feature gate states.
// It is safe for concurrent use. Zero DB hits at request time (D-10).
type FeatureGateCache struct {
	db     *pgxpool.Pool
	redis  *redis.Client
	logger *zap.Logger

	mu    sync.RWMutex
	gates map[string]bool

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
		gates:  make(map[string]bool),
	}
}

// NewFeatureGateCacheWithGates creates a FeatureGateCache pre-populated with
// the given gates map. Intended for unit tests that do not need a DB or Redis.
func NewFeatureGateCacheWithGates(gates map[string]bool) *FeatureGateCache {
	c := &FeatureGateCache{
		gates: make(map[string]bool, len(gates)),
	}
	for k, v := range gates {
		c.gates[k] = v
	}
	return c
}

// NewFeatureGateCacheForTest creates a FeatureGateCache backed only by Redis
// (no DB). onReload is called every time the cache would reload from DB —
// useful for asserting that Pub/Sub and ticker callbacks fire correctly.
func NewFeatureGateCacheForTest(rc *redis.Client, onReload func()) *FeatureGateCache {
	return &FeatureGateCache{
		redis:    rc,
		gates:    make(map[string]bool),
		onReload: onReload,
	}
}

// NewFeatureGateCacheForTestWithInterval is like NewFeatureGateCacheForTest but
// allows overriding the refresh ticker interval for fast periodic-reload tests.
func NewFeatureGateCacheForTestWithInterval(rc *redis.Client, onReload func(), interval time.Duration) *FeatureGateCache {
	return &FeatureGateCache{
		redis:                   rc,
		gates:                   make(map[string]bool),
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
	return val
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
	rows, err := c.db.Query(ctx, "SELECT feature_key, is_premium FROM feature_gates")
	if err != nil {
		return err
	}
	defer rows.Close()

	newGates := make(map[string]bool)
	for rows.Next() {
		var key string
		var isPremium bool
		if err := rows.Scan(&key, &isPremium); err != nil {
			return err
		}
		newGates[key] = isPremium
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
		"SELECT feature_key, is_premium, description, updated_at FROM feature_gates ORDER BY feature_key")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var gates []FeatureGate
	for rows.Next() {
		var g FeatureGate
		if err := rows.Scan(&g.Key, &g.IsPremium, &g.Description, &g.UpdatedAt); err != nil {
			return nil, err
		}
		gates = append(gates, g)
	}
	return gates, rows.Err()
}
