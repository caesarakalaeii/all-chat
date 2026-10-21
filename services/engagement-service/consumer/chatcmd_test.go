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
	"errors"
	"sync"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/caesar/all-chat/services/engagement-service/models"
	"github.com/caesar/all-chat/services/engagement-service/repository"
	mpmodels "github.com/caesar/all-chat/services/message-processor/models"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type fakeCmdStore struct {
	owners  []repository.ChannelCommands
	premium bool
	uses    map[uuid.UUID]int64
}

func (f *fakeCmdStore) ChatCommandsForChannel(_ context.Context, _, _ string) ([]repository.ChannelCommands, error) {
	return f.owners, nil
}

func (f *fakeCmdStore) IncrementChatCommandUse(_ context.Context, id uuid.UUID) (int64, error) {
	f.uses[id]++
	return f.uses[id], nil
}

func (f *fakeCmdStore) OwnerStatus(_ context.Context, _ uuid.UUID) (repository.ChatCommandOwnerStatus, error) {
	return repository.ChatCommandOwnerStatus{IsPremium: f.premium}, nil
}

type sent struct{ user, text, platform string }

type fakeOutput struct {
	mu      sync.Mutex
	sent    []sent
	clips   int
	clipErr error
}

func (f *fakeOutput) Send(_ context.Context, userID, message, platform string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, sent{userID, message, platform})
	return nil
}

func (f *fakeOutput) Clip(_ context.Context, _ string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.clips++
	if f.clipErr != nil {
		return "", f.clipErr
	}
	return "https://clips.twitch.tv/Abc", nil
}

var ownerID = uuid.MustParse("11111111-1111-1111-1111-111111111111")

func newCmdTest(t *testing.T, owned bool, cmds ...models.ChatCommand) (*ChatCommandConsumer, *fakeCmdStore, *fakeOutput) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rdb.Close() })
	for i := range cmds {
		cmds[i].ID = uuid.New()
		cmds[i].UserID = ownerID
		if cmds[i].MinRole == "" {
			cmds[i].MinRole = "everyone"
		}
	}
	store := &fakeCmdStore{owners: []repository.ChannelCommands{{UserID: ownerID, Owned: owned, Commands: cmds}}, uses: map[uuid.UUID]int64{}}
	out := &fakeOutput{}
	return NewChatCommandConsumer(rdb, store, out, "test", zap.NewNop()), store, out
}

func job(id, platform, text string, roles ...string) mpmodels.CommandJob {
	return mpmodels.CommandJob{MessageID: id, Platform: platform, ChannelID: "caesarlp", UserID: "u-" + id,
		Username: "bob", DisplayName: "Bob", Text: text, Roles: roles}
}

func TestChatCommand_RepliesWithTemplate(t *testing.T) {
	c, store, out := newCmdTest(t, true, models.ChatCommand{Kind: models.TriggerCommand, Trigger: "!hello", Response: "Hello {user}, #{count}!"})
	ctx := context.Background()

	c.handle(ctx, job("1", "twitch", "!hello"))
	require.Len(t, out.sent, 1)
	assert.Equal(t, sent{ownerID.String(), "Hello Bob, #1!", "twitch"}, out.sent[0])
	assert.Len(t, store.uses, 1)

	c.handle(ctx, job("2", "twitch", "hello there"))
	assert.Len(t, out.sent, 1, "a non-matching message does nothing")
}

func TestChatCommand_MinRole(t *testing.T) {
	c, _, out := newCmdTest(t, true, models.ChatCommand{Kind: models.TriggerCommand, Trigger: "!so", Response: "go follow {touser}", MinRole: "moderator", CooldownSeconds: 0})
	ctx := context.Background()

	c.handle(ctx, job("1", "twitch", "!so @friend", "subscriber", "vip"))
	assert.Empty(t, out.sent, "a VIP sub is below moderator")

	c.handle(ctx, job("2", "twitch", "!so @friend", "moderator"))
	c.handle(ctx, job("3", "twitch", "!so @friend", "broadcaster"))
	require.Len(t, out.sent, 2)
	assert.Equal(t, "go follow friend", out.sent[0].text)
}

func TestChatCommand_Cooldown(t *testing.T) {
	c, _, out := newCmdTest(t, true, models.ChatCommand{Kind: models.TriggerCommand, Trigger: "!hi", Response: "hi", CooldownSeconds: 30})
	ctx := context.Background()
	c.handle(ctx, job("1", "twitch", "!hi"))
	c.handle(ctx, job("2", "twitch", "!hi"))
	assert.Len(t, out.sent, 1)
}

// Kick publishes one raw message per overlay carrying the channel, all with the same
// message id; the command must still fire once.
func TestChatCommand_SameMessageFiresOnce(t *testing.T) {
	c, _, out := newCmdTest(t, true, models.ChatCommand{Kind: models.TriggerCommand, Trigger: "!hi", Response: "hi", CooldownSeconds: 0})
	ctx := context.Background()
	c.handle(ctx, job("dup", "kick", "!hi"))
	c.handle(ctx, job("dup", "kick", "!hi"))
	assert.Len(t, out.sent, 1)
}

// The reply is sent as the streamer, so it comes back through chat. A keyword the
// reply itself contains must not fire again on that echo.
func TestChatCommand_IgnoresOwnReplyEcho(t *testing.T) {
	c, _, out := newCmdTest(t, true, models.ChatCommand{Kind: models.TriggerKeyword, Trigger: "hello", Response: "hello {user}", CooldownSeconds: 0})
	ctx := context.Background()
	c.handle(ctx, job("1", "twitch", "hello all"))
	require.Len(t, out.sent, 1)
	c.handle(ctx, job("2", "twitch", out.sent[0].text, "broadcaster"))
	assert.Len(t, out.sent, 1)
}

func TestChatCommand_YouTubeReplyNeedsPremium(t *testing.T) {
	c, store, out := newCmdTest(t, true, models.ChatCommand{Kind: models.TriggerCommand, Trigger: "!hi", Response: "hi", CooldownSeconds: 0})
	ctx := context.Background()
	c.handle(ctx, job("1", "youtube", "!hi"))
	assert.Empty(t, out.sent, "YouTube sends spend API quota, which is premium")

	store.premium = true
	c.handle(ctx, job("2", "youtube", "!hi"))
	require.Len(t, out.sent, 1)
	assert.Equal(t, "youtube", out.sent[0].platform)
}

func TestChatCommand_UnsendablePlatformAndForeignChannel(t *testing.T) {
	c, _, out := newCmdTest(t, true, models.ChatCommand{Kind: models.TriggerCommand, Trigger: "!hi", Response: "hi", CooldownSeconds: 0})
	c.handle(context.Background(), job("1", "tiktok", "!hi"))
	assert.Empty(t, out.sent, "there is no send path to TikTok")

	c, _, out = newCmdTest(t, false, models.ChatCommand{Kind: models.TriggerKeyword, Trigger: "clip that", Response: "clipped: {clip}", CreateClip: true})
	c.handle(context.Background(), job("2", "twitch", "CLIP THAT"))
	assert.Equal(t, 1, out.clips, "a clip trigger fires from any chat in the streamer's overlays")
	assert.Empty(t, out.sent, "but a reply only goes to the streamer's own channel")
}

func TestChatCommand_Clip(t *testing.T) {
	c, _, out := newCmdTest(t, true, models.ChatCommand{Kind: models.TriggerCommand, Trigger: "!clip", Response: "Clip: {clip}", CreateClip: true, CooldownSeconds: 0})
	ctx := context.Background()
	c.handle(ctx, job("1", "twitch", "!clip"))
	require.Len(t, out.sent, 1)
	assert.Equal(t, "Clip: https://clips.twitch.tv/Abc", out.sent[0].text)

	out.clipErr = errors.New("stream_offline")
	c.handle(ctx, job("2", "twitch", "!clip"))
	require.Len(t, out.sent, 2)
	assert.Equal(t, "Clip: "+clipFailedText, out.sent[1].text)
}
