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
	"encoding/json"
	"time"

	"go.uber.org/zap"
)

// demandUpdate is the wire format of messages published to the source:demand Redis Pub/Sub channel.
type demandUpdate struct {
	Type      string               `json:"type"`
	Sources   []demandUpdateSource `json:"sources"`
	Timestamp string               `json:"timestamp"`
}

type demandUpdateSource struct {
	SourceID  string `json:"source_id"`
	ChannelID string `json:"channel_id"`
	Platform  string `json:"platform"`
	OverlayID string `json:"overlay_id"`
}

// startDemandSubscriberLoop subscribes to the source:demand Redis Pub/Sub channel and
// calls mgr.UpdateDemandedSourceIDs on each message after filtering by platform.
// It retries with exponential backoff on failure.
func (ll *LeadershipListener) startDemandSubscriberLoop(ctx context.Context, mgr ChannelManager) {
	defer ll.wg.Done()

	backoff := time.Second
	const maxBackoff = 30 * time.Second

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if err := ll.runDemandSubscriber(ctx, mgr); err != nil {
			if ll.logger != nil {
				ll.logger.Error("Demand subscriber failed, retrying",
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
func (ll *LeadershipListener) runDemandSubscriber(ctx context.Context, mgr ChannelManager) error {
	pubsub := ll.redisClient.Subscribe(ctx, "source:demand")
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
				if ll.logger != nil {
					ll.logger.Warn("Failed to parse demand update",
						zap.String("payload", msg.Payload),
						zap.Error(err),
					)
				}
				continue
			}
			ll.reconcileDemand(mgr, update)
		}
	}
}

// reconcileDemand filters the demand update by platform and calls
// mgr.UpdateDemandedSourceIDs with the result.
func (ll *LeadershipListener) reconcileDemand(mgr ChannelManager, update demandUpdate) {
	demanded := make(map[string]DemandedSource, len(update.Sources))
	for _, src := range update.Sources {
		if ll.config.Platform != "" && src.Platform != ll.config.Platform {
			continue
		}
		demanded[src.SourceID] = DemandedSource{
			SourceID:  src.SourceID,
			ChannelID: src.ChannelID,
			Platform:  src.Platform,
			OverlayID: src.OverlayID,
		}
	}
	if ll.logger != nil {
		ll.logger.Debug("Demand update reconciled",
			zap.Int("total_in_update", len(update.Sources)),
			zap.Int("demanded_after_filter", len(demanded)),
			zap.String("platform_filter", ll.config.Platform),
		)
	}
	mgr.UpdateDemandedSourceIDs(demanded)
}
