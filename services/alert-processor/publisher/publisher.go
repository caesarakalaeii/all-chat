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

// Package publisher broadcasts alert envelopes to per-overlay Redis Pub/Sub
// channels for real-time delivery.
package publisher

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/caesar/all-chat/services/alert-processor/models"
	"github.com/redis/go-redis/v9"
)

// Publisher fans alert envelopes out over Redis Pub/Sub. It carries no logger:
// every failure propagates to the caller, which logs it and leaves the stream
// entry unacked for redelivery.
type Publisher struct {
	rdb redis.UniversalClient
}

// New creates a Publisher.
func New(rdb redis.UniversalClient) *Publisher {
	return &Publisher{rdb: rdb}
}

// Channel is the Pub/Sub channel one overlay's alerts are delivered on. The
// gateway maps this suffix to the alerts stream of that overlay's WS session.
func Channel(overlayID string) string {
	return fmt.Sprintf("overlay:%s:alerts", overlayID)
}

// Publish sends one alert envelope to the overlay's channel.
//
// Errors PROPAGATE, unlike the engagement-service publisher, which treats
// Pub/Sub as best-effort: that service re-sends full state on the next poll, so
// a dropped snapshot is self-healing. A dropped alert is never re-sent — the
// only recovery is the at-least-once stream redelivery, which requires the
// caller to know the publish failed so it can leave the entry unacked.
func (p *Publisher) Publish(ctx context.Context, alert *models.Alert) error {
	channel := Channel(alert.OverlayID)
	data, err := json.Marshal(alert)
	if err != nil {
		return fmt.Errorf("failed to marshal alert for %s: %w", channel, err)
	}
	if err := p.rdb.Publish(ctx, channel, data).Err(); err != nil {
		return fmt.Errorf("failed to publish alert to %s: %w", channel, err)
	}
	return nil
}