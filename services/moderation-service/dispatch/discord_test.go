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

package dispatch

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/caesar/all-chat/services/moderation-service/clients"
	"github.com/caesar/all-chat/services/moderation-service/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type fakeDiscordAPI struct {
	err               error
	deleteCalls       int
	timeoutCalls      int
	banCalls          int
	unbanCalls        int
	channelID, msgID  string
	guildID, targetID string
	until             time.Time
}

func (f *fakeDiscordAPI) DeleteMessage(_ context.Context, channelID, messageID string) error {
	f.deleteCalls++
	f.channelID, f.msgID = channelID, messageID
	return f.err
}

func (f *fakeDiscordAPI) TimeoutMember(_ context.Context, guildID, userID string, until time.Time) error {
	f.timeoutCalls++
	f.guildID, f.targetID, f.until = guildID, userID, until
	return f.err
}

func (f *fakeDiscordAPI) BanMember(_ context.Context, guildID, userID string) error {
	f.banCalls++
	f.guildID, f.targetID = guildID, userID
	return f.err
}

func (f *fakeDiscordAPI) UnbanMember(_ context.Context, guildID, userID string) error {
	f.unbanCalls++
	f.guildID, f.targetID = guildID, userID
	return f.err
}

// fakeGuildResolver maps any channel to guildID (or returns err). The live permission reads are
// the delegated path's business and are exercised in discord_delegated_test.go; here they answer
// nothing, which is enough because the owner path performs neither.
type fakeGuildResolver struct {
	guildID string
	err     error
	calls   int
}

func (f *fakeGuildResolver) GuildID(_ context.Context, _ string) (string, error) {
	f.calls++
	return f.guildID, f.err
}

func (f *fakeGuildResolver) GuildBotPermissions(_ context.Context, _ string) (uint64, error) {
	return 0, errors.New("no live permission read is expected on the owner path")
}

func (f *fakeGuildResolver) MemberAuthority(_ context.Context, _, _ string) (clients.DiscordMember, error) {
	return clients.DiscordMember{}, errors.New("no live member read is expected on the owner path")
}

// ownerAnchorStore satisfies the owner-reach anchor for any guild: these tests are about the
// dispatch mechanics, and the anchor itself is exercised in discord_delegated_test.go.
type ownerAnchorStore struct{}

func (ownerAnchorStore) DiscordIdentity(_ context.Context, _ string) (string, bool, error) {
	return "", false, nil
}

func (ownerAnchorStore) DiscordGuildConnectedBy(_ context.Context, _, _ string) (bool, error) {
	return true, nil
}

func newDiscordDispatcher(api *fakeDiscordAPI, guilds *fakeGuildResolver) *Discord {
	return NewDiscord(api, guilds, ownerAnchorStore{}, zap.NewNop())
}

func TestDiscordDispatch_NonDiscordIsDryRun(t *testing.T) {
	api := &fakeDiscordAPI{}
	d := newDiscordDispatcher(api, &fakeGuildResolver{})
	res, err := d.Dispatch(context.Background(), owner("u1"), models.ActionDelete, models.DispatchRequest{Platform: "twitch", ChannelID: "c"})
	require.NoError(t, err)
	assert.Equal(t, models.DispatchDryRun, res.Outcome)
	assert.Zero(t, api.deleteCalls)
}

func TestDiscordDispatch_DeletePerformed(t *testing.T) {
	api := &fakeDiscordAPI{}
	guilds := &fakeGuildResolver{guildID: "g"}
	d := newDiscordDispatcher(api, guilds)
	res, err := d.Dispatch(context.Background(), owner("u1"), models.ActionDelete,
		models.DispatchRequest{Platform: "discord", ChannelID: "chan-1", NativeMessageID: "msg-1"})
	require.NoError(t, err)
	assert.Equal(t, models.DispatchPerformed, res.Outcome)
	assert.Equal(t, 1, api.deleteCalls)
	assert.Equal(t, "chan-1", api.channelID)
	assert.Equal(t, "msg-1", api.msgID, "the Discord native message id (snowflake) is the delete target")
	assert.Equal(t, 1, guilds.calls,
		"the write is channel-scoped but authority is per-guild, so even a delete resolves the guild to anchor against")
}

func TestDiscordDispatch_BanResolvesGuildAndPerforms(t *testing.T) {
	api := &fakeDiscordAPI{}
	guilds := &fakeGuildResolver{guildID: "guild-42"}
	d := newDiscordDispatcher(api, guilds)
	res, err := d.Dispatch(context.Background(), owner("u1"), models.ActionBan,
		models.DispatchRequest{Platform: "discord", ChannelID: "chan-1", TargetUserID: "member-7"})
	require.NoError(t, err)
	assert.Equal(t, models.DispatchPerformed, res.Outcome)
	assert.Equal(t, 1, api.banCalls)
	assert.Equal(t, "guild-42", api.guildID, "ban is guild-scoped; the guild is resolved from the channel")
	assert.Equal(t, "member-7", api.targetID)
}

func TestDiscordDispatch_TimeoutSetsFutureUntil(t *testing.T) {
	api := &fakeDiscordAPI{}
	guilds := &fakeGuildResolver{guildID: "g"}
	d := newDiscordDispatcher(api, guilds)
	res, err := d.Dispatch(context.Background(), owner("u1"), models.ActionTimeout,
		models.DispatchRequest{Platform: "discord", ChannelID: "c", TargetUserID: "u", DurationSeconds: 600})
	require.NoError(t, err)
	assert.Equal(t, models.DispatchPerformed, res.Outcome)
	assert.Equal(t, 1, api.timeoutCalls)
	assert.True(t, api.until.After(time.Now()), "timeout until must be in the future")
}

func TestDiscordDispatch_UnbanPerformed(t *testing.T) {
	api := &fakeDiscordAPI{}
	guilds := &fakeGuildResolver{guildID: "g"}
	d := newDiscordDispatcher(api, guilds)
	res, err := d.Dispatch(context.Background(), owner("u1"), models.ActionUnban,
		models.DispatchRequest{Platform: "discord", ChannelID: "c", TargetUserID: "u"})
	require.NoError(t, err)
	assert.Equal(t, models.DispatchPerformed, res.Outcome)
	assert.Equal(t, 1, api.unbanCalls)
}

func TestDiscordDispatch_ForbiddenIsError(t *testing.T) {
	api := &fakeDiscordAPI{err: clients.ErrDiscordForbidden}
	d := newDiscordDispatcher(api, &fakeGuildResolver{guildID: "g"})
	res, err := d.Dispatch(context.Background(), owner("u1"), models.ActionDelete,
		models.DispatchRequest{Platform: "discord", ChannelID: "c", NativeMessageID: "m"})
	require.Error(t, err, "a missing bot permission must fail the dispatch so no reflect-back fires")
	assert.NotEqual(t, models.DispatchPerformed, res.Outcome)
	assert.ErrorIs(t, err, clients.ErrDiscordForbidden)
}

func TestDiscordDispatch_BanForbiddenSurfacesReinvite(t *testing.T) {
	api := &fakeDiscordAPI{err: clients.ErrDiscordForbidden}
	d := newDiscordDispatcher(api, &fakeGuildResolver{guildID: "g"})
	_, err := d.Dispatch(context.Background(), owner("u1"), models.ActionBan,
		models.DispatchRequest{Platform: "discord", ChannelID: "c", TargetUserID: "u"})
	require.Error(t, err)
	assert.ErrorIs(t, err, clients.ErrDiscordForbidden)
	assert.Contains(t, err.Error(), "re-invite", "a 403 on a member op points the owner at the re-invite path")
}

func TestDiscordDispatch_GuildResolutionFailureIsError(t *testing.T) {
	api := &fakeDiscordAPI{}
	guilds := &fakeGuildResolver{err: errors.New("channel gone")}
	d := newDiscordDispatcher(api, guilds)
	_, err := d.Dispatch(context.Background(), owner("u1"), models.ActionBan,
		models.DispatchRequest{Platform: "discord", ChannelID: "c", TargetUserID: "u"})
	require.Error(t, err)
	assert.Zero(t, api.banCalls, "no ban is attempted when the guild cannot be resolved")
}

// The owner path performs no live permission read at all, on any action. That is ADR-0048's
// anchor-strength decision made checkable: the connected-guild row alone anchors an owner, and
// adding a live read here would have switched Discord moderation off for every existing Discord
// streamer until each completed a new account link.
func TestDiscordDispatch_OwnerPathMakesNoLivePermissionRead(t *testing.T) {
	for _, action := range []models.Action{
		models.ActionDelete, models.ActionTimeout, models.ActionBan, models.ActionUnban,
	} {
		t.Run(string(action), func(t *testing.T) {
			api := &fakeDiscordAPI{}
			// This resolver errors on every live read, so any attempt to make one fails the test
			// rather than passing silently.
			d := newDiscordDispatcher(api, &fakeGuildResolver{guildID: "g1"})

			res, err := d.Dispatch(context.Background(), owner("u1"), action,
				models.DispatchRequest{Platform: "discord", ChannelID: "c", NativeMessageID: "m", TargetUserID: "42", DurationSeconds: 60})

			require.NoError(t, err)
			assert.Equal(t, models.DispatchPerformed, res.Outcome)
		})
	}
}
