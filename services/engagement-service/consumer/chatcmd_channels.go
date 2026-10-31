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

package consumer

import (
	"context"
	"strings"
	"time"

	mpmodels "github.com/caesar/all-chat/services/message-processor/models"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// channelSyncInterval also picks up source changes (a streamer adding a channel to an
// overlay), which no chat command write announces.
const channelSyncInterval = 30 * time.Second

type chatCmdChannelLister interface {
	ChatCommandChannels(ctx context.Context) ([]string, error)
}

// ChannelSync keeps chatcmd:channels, the set of channels message-processor forwards
// to chatcmd:jobs (ADR-0079), equal to what the DB says: rebuilt on an interval and
// right after any chat command write.
type ChannelSync struct {
	rdb     *redis.Client
	lister  chatCmdChannelLister
	log     *zap.Logger
	trigger chan struct{}
}

// NewChannelSync creates the sync. Call Run in the background.
func NewChannelSync(rdb *redis.Client, lister chatCmdChannelLister, log *zap.Logger) *ChannelSync {
	return &ChannelSync{rdb: rdb, lister: lister, log: log, trigger: make(chan struct{}, 1)}
}

// Trigger asks for a rebuild soon. Never blocks; requests coalesce.
func (s *ChannelSync) Trigger() {
	select {
	case s.trigger <- struct{}{}:
	default:
	}
}

// Run rebuilds the set now, then on every interval and Trigger until ctx ends.
func (s *ChannelSync) Run(ctx context.Context) {
	ticker := time.NewTicker(channelSyncInterval)
	defer ticker.Stop()
	for {
		if err := s.sync(ctx); err != nil && ctx.Err() == nil {
			s.log.Warn("sync chat command channels", zap.Error(err))
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		case <-s.trigger:
		}
	}
}

// sync swaps in the new set atomically (SADD to a temp key, RENAME over), so the hot
// path never reads a half-built set.
func (s *ChannelSync) sync(ctx context.Context) error {
	members, err := s.lister.ChatCommandChannels(ctx)
	if err != nil {
		return err
	}
	if len(members) == 0 {
		return s.rdb.Del(ctx, mpmodels.ChatCommandChannelsKey).Err()
	}
	vals := make([]interface{}, len(members))
	for i, m := range members {
		vals[i] = strings.ToLower(m)
	}
	tmp := mpmodels.ChatCommandChannelsKey + ":tmp:" + uuid.NewString()
	pipe := s.rdb.TxPipeline()
	pipe.SAdd(ctx, tmp, vals...)
	pipe.Rename(ctx, tmp, mpmodels.ChatCommandChannelsKey)
	_, err = pipe.Exec(ctx)
	return err
}
