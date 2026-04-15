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

package demand

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/caesar/all-chat/services/source-manager/models"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// overlayConnectionEvent mirrors the event published by api-gateway WebSocket manager.
type overlayConnectionEvent struct {
	Type      string    `json:"type"`       // "connected" or "disconnected"
	OverlayID string    `json:"overlay_id"`
	Timestamp time.Time `json:"timestamp"`
}

// DemandSource describes a single source that has active overlay demand.
type DemandSource struct {
	SourceID  string `json:"source_id"`
	ChannelID string `json:"channel_id"`
	Platform  string `json:"platform"`
	OverlayID string `json:"overlay_id"`
}

// DemandUpdate is the full-replacement snapshot published to source:demand.
type DemandUpdate struct {
	Type      string         `json:"type"`      // always "demand_update"
	Sources   []DemandSource `json:"sources"`
	Timestamp string         `json:"timestamp"` // ISO 8601
}

// sourceRepository is the subset of registry.Repository used by the subscriber.
// Defined here so tests can inject a mock without importing the full registry package.
type sourceRepository interface {
	GetSourcesForOverlays(ctx context.Context, overlayIDs []string) ([]*models.ActiveSource, error)
}

// OverlayDemandSubscriber subscribes to overlay:connections Pub/Sub, resolves
// sources from the database, maintains an in-memory demand set, and publishes
// full-replacement DemandUpdate snapshots to the source:demand channel.
type OverlayDemandSubscriber struct {
	redisClient *redis.Client
	repo        sourceRepository
	logger      *zap.Logger

	mu           sync.RWMutex
	demand       map[string][]DemandSource // overlay_id -> []DemandSource
	lastSnapshot string                    // fingerprint of last published snapshot; prevents redundant publishes
}

// NewOverlayDemandSubscriber creates a new OverlayDemandSubscriber.
func NewOverlayDemandSubscriber(redisClient *redis.Client, repo sourceRepository, logger *zap.Logger) *OverlayDemandSubscriber {
	return &OverlayDemandSubscriber{
		redisClient: redisClient,
		repo:        repo,
		logger:      logger,
		demand:      make(map[string][]DemandSource),
	}
}

// Start hydrates demand from existing overlay:connected:* keys, publishes the
// initial DemandUpdate, then subscribes to overlay:connections for live events.
// Must be called AFTER hydration so the initial snapshot is not empty on restart.
func (s *OverlayDemandSubscriber) Start(ctx context.Context) error {
	// Step 1: hydrate from existing keys before subscribing to live events.
	if err := s.hydrate(ctx); err != nil {
		s.logger.Warn("Startup hydration failed; continuing with empty demand", zap.Error(err))
	}

	// Step 2: publish initial snapshot.
	s.publishDemandUpdate(ctx)

	// Step 3: subscribe to live events with retry loop.
	return s.subscribeLoop(ctx)
}

// hydrate scans overlay:connected:* keys and populates the demand map.
func (s *OverlayDemandSubscriber) hydrate(ctx context.Context) error {
	keys, err := s.redisClient.Keys(ctx, "overlay:connected:*").Result()
	if err != nil {
		return err
	}

	overlayIDs := make([]string, 0, len(keys))
	for _, key := range keys {
		// key format: overlay:connected:{overlay_id}
		overlayID := strings.TrimPrefix(key, "overlay:connected:")
		if overlayID != "" {
			overlayIDs = append(overlayIDs, overlayID)
		}
	}

	if len(overlayIDs) == 0 {
		return nil
	}

	sources, err := s.repo.GetSourcesForOverlays(ctx, overlayIDs)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, src := range sources {
		ds := DemandSource{
			SourceID:  src.ID,
			ChannelID: src.ChannelID,
			Platform:  src.Platform,
			OverlayID: src.OverlayID,
		}
		s.demand[src.OverlayID] = append(s.demand[src.OverlayID], ds)
	}

	s.logger.Info("Startup hydration complete",
		zap.Int("overlay_count", len(overlayIDs)),
		zap.Int("source_count", len(sources)),
	)
	return nil
}

// subscribeLoop runs the Redis Pub/Sub subscriber with exponential backoff retry.
func (s *OverlayDemandSubscriber) subscribeLoop(ctx context.Context) error {
	backoff := time.Second
	const maxBackoff = 30 * time.Second

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		pubsub := s.redisClient.Subscribe(ctx, "overlay:connections")
		if err := s.runSubscriber(ctx, pubsub); err != nil {
			s.logger.Error("overlay:connections subscriber failed, retrying",
				zap.Duration("backoff", backoff),
				zap.Error(err),
			)
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(backoff):
			}
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}

		// runSubscriber returned nil — context cancelled.
		return nil
	}
}

// runSubscriber processes messages from a Pub/Sub subscription until ctx is done or an error occurs.
func (s *OverlayDemandSubscriber) runSubscriber(ctx context.Context, pubsub *redis.PubSub) error {
	defer pubsub.Close()

	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return nil
		case msg, ok := <-ch:
			if !ok {
				return nil
			}
			if err := s.handleConnectionEvent(ctx, msg.Payload); err != nil {
				s.logger.Error("Failed to handle connection event",
					zap.String("payload", msg.Payload),
					zap.Error(err),
				)
			}
		}
	}
}

// handleConnectionEvent parses an OverlayConnectionEvent and updates the demand map.
func (s *OverlayDemandSubscriber) handleConnectionEvent(ctx context.Context, payload string) error {
	var event overlayConnectionEvent
	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		return err
	}

	switch event.Type {
	case "connected":
		sources, err := s.repo.GetSourcesForOverlays(ctx, []string{event.OverlayID})
		if err != nil {
			return err
		}

		demandSources := make([]DemandSource, 0, len(sources))
		for _, src := range sources {
			demandSources = append(demandSources, DemandSource{
				SourceID:  src.ID,
				ChannelID: src.ChannelID,
				Platform:  src.Platform,
				OverlayID: src.OverlayID,
			})
		}

		s.mu.Lock()
		s.demand[event.OverlayID] = demandSources
		s.mu.Unlock()

		s.logger.Info("Overlay connected, demand updated",
			zap.String("overlay_id", event.OverlayID),
			zap.Int("source_count", len(demandSources)),
		)

	case "disconnected":
		s.mu.Lock()
		delete(s.demand, event.OverlayID)
		s.mu.Unlock()

		s.logger.Info("Overlay disconnected, demand updated",
			zap.String("overlay_id", event.OverlayID),
		)

	default:
		s.logger.Warn("Unknown connection event type",
			zap.String("type", event.Type),
			zap.String("overlay_id", event.OverlayID),
		)
	}

	s.publishDemandUpdate(ctx)
	return nil
}

// publishDemandUpdate flattens the demand map and publishes a DemandUpdate to source:demand.
// It skips the publish if the set of demanded source IDs has not changed since the last
// publish, preventing redundant snapshots when WebSocket clients reconnect rapidly but
// the underlying source configuration is unchanged.
func (s *OverlayDemandSubscriber) publishDemandUpdate(ctx context.Context) {
	s.mu.Lock()
	flatSources := make([]DemandSource, 0)
	for _, sources := range s.demand {
		flatSources = append(flatSources, sources...)
	}

	// Build a stable fingerprint from sorted source IDs so the comparison is
	// order-independent (map iteration order is random).
	fingerprint := demandFingerprint(flatSources)
	if fingerprint == s.lastSnapshot {
		s.mu.Unlock()
		s.logger.Debug("Demand unchanged, skipping publish",
			zap.Int("source_count", len(flatSources)),
		)
		return
	}
	s.lastSnapshot = fingerprint
	s.mu.Unlock()

	update := DemandUpdate{
		Type:      "demand_update",
		Sources:   flatSources,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	payload, err := json.Marshal(update)
	if err != nil {
		s.logger.Error("Failed to marshal DemandUpdate", zap.Error(err))
		return
	}

	if err := s.redisClient.Publish(ctx, "source:demand", payload).Err(); err != nil {
		s.logger.Error("Failed to publish DemandUpdate to source:demand", zap.Error(err))
		return
	}

	s.logger.Info("Published DemandUpdate",
		zap.Int("source_count", len(flatSources)),
	)
}

// demandFingerprint builds a stable string key from a set of DemandSource entries.
// The key is the sorted concatenation of "overlayID:sourceID" pairs so that order
// of iteration does not affect the result.
func demandFingerprint(sources []DemandSource) string {
	if len(sources) == 0 {
		return ""
	}
	keys := make([]string, len(sources))
	for i, s := range sources {
		keys[i] = s.OverlayID + ":" + s.SourceID
	}
	sort.Strings(keys)
	result := strings.Join(keys, ",")
	return result
}

// GetDemandedSources returns a snapshot of all currently demanded sources.
func (s *OverlayDemandSubscriber) GetDemandedSources() []DemandSource {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]DemandSource, 0)
	for _, sources := range s.demand {
		result = append(result, sources...)
	}
	return result
}

// GetDemandedSourcesByPlatform returns demanded sources filtered by platform.
func (s *OverlayDemandSubscriber) GetDemandedSourcesByPlatform(platform string) []DemandSource {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]DemandSource, 0)
	for _, sources := range s.demand {
		for _, src := range sources {
			if src.Platform == platform {
				result = append(result, src)
			}
		}
	}
	return result
}

// HandleConnectionEventForTest exposes handleConnectionEvent for unit testing.
func (s *OverlayDemandSubscriber) HandleConnectionEventForTest(ctx context.Context, payload string) error {
	return s.handleConnectionEvent(ctx, payload)
}

// HydrateForTest exposes hydrate for unit testing.
func (s *OverlayDemandSubscriber) HydrateForTest(ctx context.Context) error {
	return s.hydrate(ctx)
}
