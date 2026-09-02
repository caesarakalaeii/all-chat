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

package status

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const PlatformStatusChannel = "platform:status"

// snapshotTTL is how long a last-known status survives without a refresh. Long enough
// to outlive an overnight discovery park (the state a streamer needs to see the next
// morning), short enough that a status nobody has re-published expires instead of
// misreporting a channel forever.
const snapshotTTL = 48 * time.Hour

// snapshotKey is where the last-known status for one channel is stored, for readers
// that were not subscribed when it was published. Consumer: platformStatusSnapshotKey
// in services/overlay-manager/handlers/sources.go, which reads it to show a parked
// YouTube channel on the dashboard. The format is duplicated there because the two
// services share no Go module, so changing it here needs the same change there.
func snapshotKey(platform, channelID string) string {
	return fmt.Sprintf("platform:status:%s:%s", platform, channelID)
}

// Message represents a platform connection status update
type Message struct {
	Platform     string     `json:"platform"`
	ChannelID    string     `json:"channel_id"`
	ChannelName  string     `json:"channel_name,omitempty"`
	Status       string     `json:"status"` // "connected", "reconnecting", "offline", "error", "paused"
	NextRetryAt  *time.Time `json:"next_retry_at,omitempty"`
	ErrorMessage string     `json:"error_message,omitempty"`
}

// Publisher publishes platform status updates to Redis Pub/Sub and stores a
// last-known snapshot of each one.
type Publisher struct {
	// redis.Cmdable rather than *redis.Client so a test can fail the snapshot write on
	// its own; *redis.Client satisfies it.
	redisClient redis.Cmdable
	logger      *zap.Logger
}

func NewPublisher(redisClient redis.Cmdable, logger *zap.Logger) *Publisher {
	return &Publisher{redisClient: redisClient, logger: logger}
}

func (p *Publisher) Publish(ctx context.Context, msg Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		p.logger.Error("Failed to marshal status message", zap.Error(err))
		return
	}
	if err := p.redisClient.Publish(ctx, PlatformStatusChannel, string(data)).Err(); err != nil {
		p.logger.Warn("Failed to publish platform status", zap.String("status", msg.Status), zap.Error(err))
	}
	// Pub/Sub only reaches whoever is subscribed at that instant, and the statuses that
	// matter most (a discovery park) fire when nobody is watching. The snapshot is what a
	// later reader sees. It is deliberately best-effort and after the publish: Redis runs
	// HA with min-replicas-to-write 1, so this write can pause under node loss and must
	// never delay or fail the live path.
	key := snapshotKey(msg.Platform, msg.ChannelID)
	if err := p.redisClient.Set(ctx, key, data, snapshotTTL).Err(); err != nil {
		p.logger.Warn("Failed to store platform status snapshot",
			zap.String("key", key), zap.String("status", msg.Status), zap.Error(err))
	}
}
