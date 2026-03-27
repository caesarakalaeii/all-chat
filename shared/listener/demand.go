package listener

import (
	"context"
	"encoding/json"
	"time"

	"go.uber.org/zap"
)

// demandUpdate is the wire format of messages published to the source:demand Redis Pub/Sub channel.
type demandUpdate struct {
	Type      string              `json:"type"`
	Sources   []demandUpdateSource `json:"sources"`
	Timestamp string              `json:"timestamp"`
}

type demandUpdateSource struct {
	SourceID  string `json:"source_id"`
	ChannelID string `json:"channel_id"`
	Platform  string `json:"platform"`
	OverlayID string `json:"overlay_id"`
}

// startDemandSubscriberLoop subscribes to the source:demand Redis Pub/Sub channel and
// calls mgr.UpdateDemandedSourceIDs on each message after filtering by assigned sources
// and platform. It retries with exponential backoff on failure.
//
// If b.config.DisableDemandFiltering is true the loop exits immediately without
// subscribing (used by twitch IRC listener which always connects to all assigned sources).
//
// If b.redisClient is nil the loop exits immediately (nil-safe for tests and
// leadership-only services that don't call base.Start).
func (b *ListenerBase) startDemandSubscriberLoop(ctx context.Context, mgr ChannelManager) {
	defer b.wg.Done()

	if b.config.DisableDemandFiltering {
		if b.logger != nil {
			b.logger.Debug("Demand filtering disabled, skipping demand subscriber loop")
		}
		return
	}

	if b.redisClient == nil {
		if b.logger != nil {
			b.logger.Debug("Redis client is nil, demand subscriber loop disabled")
		}
		return
	}

	backoff := time.Second
	const maxBackoff = 30 * time.Second

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if err := b.runDemandSubscriber(ctx, mgr); err != nil {
			if b.logger != nil {
				b.logger.Error("Demand subscriber failed, retrying",
					zap.Duration("backoff", backoff),
					zap.Error(err),
				)
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}

		// runDemandSubscriber returned nil — wait for context cancellation.
		<-ctx.Done()
		return
	}
}

// runDemandSubscriber opens a Pub/Sub subscription and processes messages until the
// context is cancelled or an error occurs. Returns nil when ctx is cancelled.
func (b *ListenerBase) runDemandSubscriber(ctx context.Context, mgr ChannelManager) error {
	pubsub := b.redisClient.Subscribe(ctx, "source:demand")
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
			var update demandUpdate
			if err := json.Unmarshal([]byte(msg.Payload), &update); err != nil {
				if b.logger != nil {
					b.logger.Warn("Failed to parse demand update",
						zap.String("payload", msg.Payload),
						zap.Error(err),
					)
				}
				continue
			}
			b.reconcileDemand(mgr, update)
		}
	}
}

// reconcileDemand computes the intersection of assigned sources and demanded sources
// (filtered by platform), then calls mgr.UpdateDemandedSourceIDs with the result.
//
// It is a no-op if initial assignments have not been loaded yet (hasInitialAssignments == false).
func (b *ListenerBase) reconcileDemand(mgr ChannelManager, update demandUpdate) {
	if !b.hasInitialAssignments.Load() {
		if b.logger != nil {
			b.logger.Debug("Skipping demand update, no initial assignments yet")
		}
		return
	}

	b.assignedMu.RLock()
	assigned := b.assignedSourceIDs
	b.assignedMu.RUnlock()

	demanded := make(map[string]DemandedSource, len(update.Sources))
	for _, src := range update.Sources {
		// Filter by platform when b.config.Platform is set.
		if b.config.Platform != "" && src.Platform != b.config.Platform {
			continue
		}
		// Only include sources that are assigned to this listener pod.
		// assigned == nil means coordinator filtering is disabled (all sources pass).
		if assigned != nil && !assigned[src.SourceID] {
			continue
		}
		demanded[src.SourceID] = DemandedSource{
			SourceID:  src.SourceID,
			ChannelID: src.ChannelID,
			Platform:  src.Platform,
			OverlayID: src.OverlayID,
		}
	}

	if b.logger != nil {
		b.logger.Debug("Demand update reconciled",
			zap.Int("total_in_update", len(update.Sources)),
			zap.Int("demanded_after_filter", len(demanded)),
			zap.String("platform_filter", b.config.Platform),
		)
	}

	mgr.UpdateDemandedSourceIDs(demanded)
}
