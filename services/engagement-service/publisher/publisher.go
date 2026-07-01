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

// Package publisher broadcasts aggregate poll/prediction snapshots to overlays
// and maintains the Redis "active engagement" flags that gate the
// message-processor hot path (issue #523).
package publisher

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/caesar/all-chat/services/engagement-service/models"
	"github.com/caesar/all-chat/services/engagement-service/repository"
	mpmodels "github.com/caesar/all-chat/services/message-processor/models"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// activeTTL is a safety net so an active flag left behind by a crash (before
// ClearActive ran) eventually expires rather than routing chat commands forever.
const activeTTL = 24 * time.Hour

// Publisher fans engagement state out over Redis Pub/Sub and manages active flags.
type Publisher struct {
	rdb redis.UniversalClient
	log *zap.Logger
}

// New creates a Publisher.
func New(rdb redis.UniversalClient, log *zap.Logger) *Publisher {
	return &Publisher{rdb: rdb, log: log}
}

// PublishPoll broadcasts a poll snapshot to overlay:{id}:poll. The gateway maps
// that channel suffix to the poll_update WS type. State conveys active vs ended.
func (p *Publisher) PublishPoll(ctx context.Context, poll *models.Poll) {
	var total int64
	for _, o := range poll.Options {
		total += o.Votes
	}
	p.publish(ctx, fmt.Sprintf("overlay:%s:poll", poll.OverlayID), models.PollSnapshot{Poll: *poll, TotalVotes: total})
}

// PublishPrediction broadcasts a prediction snapshot to overlay:{id}:prediction.
func (p *Publisher) PublishPrediction(ctx context.Context, pred *models.Prediction) {
	var total int64
	for _, o := range pred.Outcomes {
		total += o.TotalPts
	}
	p.publish(ctx, fmt.Sprintf("overlay:%s:prediction", pred.OverlayID), models.PredictionSnapshot{Prediction: *pred, TotalPts: total})
}

func (p *Publisher) publish(ctx context.Context, channel string, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		p.log.Error("marshal engagement snapshot", zap.String("channel", channel), zap.Error(err))
		return
	}
	if err := p.rdb.Publish(ctx, channel, data).Err(); err != nil {
		// Pub/Sub is best-effort; a transient failure just means overlays refresh
		// on their next poll/reconnect. Log and move on.
		p.log.Warn("publish engagement snapshot failed", zap.String("channel", channel), zap.Error(err))
	}
}

// SetActive flags every source channel of an overlay as having a live engagement,
// so message-processor forwards chat commands from those channels. Refcounted by
// engagement id (a SET member per active poll/prediction); the set auto-clears
// when the last engagement ends.
func (p *Publisher) SetActive(ctx context.Context, engagementID uuid.UUID, channels []repository.ChannelRef) {
	for _, c := range channels {
		key := mpmodels.EngagementActiveKey(c.Platform, c.ChannelID)
		if err := p.rdb.SAdd(ctx, key, engagementID.String()).Err(); err != nil {
			p.log.Warn("set active flag failed", zap.String("key", key), zap.Error(err))
			continue
		}
		_ = p.rdb.Expire(ctx, key, activeTTL).Err()
	}
}

// ClearActive removes an engagement's active flag from its channels. When a
// channel has no other live engagement the key disappears and the hot path stops
// forwarding its commands.
func (p *Publisher) ClearActive(ctx context.Context, engagementID uuid.UUID, channels []repository.ChannelRef) {
	for _, c := range channels {
		key := mpmodels.EngagementActiveKey(c.Platform, c.ChannelID)
		if err := p.rdb.SRem(ctx, key, engagementID.String()).Err(); err != nil {
			p.log.Warn("clear active flag failed", zap.String("key", key), zap.Error(err))
		}
	}
}
