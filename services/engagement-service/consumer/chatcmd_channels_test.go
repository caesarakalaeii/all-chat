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
	"testing"

	"github.com/alicebob/miniredis/v2"
	mpmodels "github.com/caesar/all-chat/services/message-processor/models"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type fakeChannelLister struct{ members []string }

func (f *fakeChannelLister) ChatCommandChannels(context.Context) ([]string, error) {
	return f.members, nil
}

// The sync replaces the whole set, lowercased the way message-processor reads it, and
// clears it when no streamer has a command left.
func TestChannelSync(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()
	ctx := context.Background()
	lister := &fakeChannelLister{members: []string{"twitch:CaesarLP", "kick:some-slug"}}
	s := NewChannelSync(rdb, lister, zap.NewNop())

	require.NoError(t, s.sync(ctx))
	got, err := rdb.SMembers(ctx, mpmodels.ChatCommandChannelsKey).Result()
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"twitch:caesarlp", "kick:some-slug"}, got)

	lister.members = []string{"youtube:UCabc"}
	require.NoError(t, s.sync(ctx))
	got, _ = rdb.SMembers(ctx, mpmodels.ChatCommandChannelsKey).Result()
	assert.Equal(t, []string{"youtube:ucabc"}, got)

	lister.members = nil
	require.NoError(t, s.sync(ctx))
	n, _ := rdb.Exists(ctx, mpmodels.ChatCommandChannelsKey).Result()
	assert.Zero(t, n)
}
