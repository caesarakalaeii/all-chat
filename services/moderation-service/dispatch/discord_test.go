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
	"testing"

	"github.com/caesar/all-chat/services/moderation-service/clients"
	"github.com/caesar/all-chat/services/moderation-service/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type fakeDiscordAPI struct {
	err                  error
	calls                int
	channelID, messageID string
}

func (f *fakeDiscordAPI) DeleteMessage(_ context.Context, channelID, messageID string) error {
	f.calls++
	f.channelID, f.messageID = channelID, messageID
	return f.err
}

func TestDiscordDispatch_NonDiscordIsDryRun(t *testing.T) {
	api := &fakeDiscordAPI{}
	d := NewDiscord(api, zap.NewNop())
	res, err := d.Dispatch(context.Background(), "u1", models.ActionDelete, models.DispatchRequest{Platform: "twitch", ChannelID: "c"})
	require.NoError(t, err)
	assert.Equal(t, models.DispatchDryRun, res.Outcome)
	assert.Zero(t, api.calls)
}

func TestDiscordDispatch_DeletePerformed(t *testing.T) {
	api := &fakeDiscordAPI{}
	d := NewDiscord(api, zap.NewNop())
	res, err := d.Dispatch(context.Background(), "u1", models.ActionDelete,
		models.DispatchRequest{Platform: "discord", ChannelID: "chan-1", NativeMessageID: "msg-1"})
	require.NoError(t, err)
	assert.Equal(t, models.DispatchPerformed, res.Outcome)
	assert.Equal(t, 1, api.calls)
	assert.Equal(t, "chan-1", api.channelID)
	assert.Equal(t, "msg-1", api.messageID, "the Discord native message id (snowflake) is the delete target")
}

func TestDiscordDispatch_ForbiddenIsError(t *testing.T) {
	api := &fakeDiscordAPI{err: clients.ErrDiscordForbidden}
	d := NewDiscord(api, zap.NewNop())
	res, err := d.Dispatch(context.Background(), "u1", models.ActionDelete,
		models.DispatchRequest{Platform: "discord", ChannelID: "c", NativeMessageID: "m"})
	require.Error(t, err, "a missing bot permission must fail the dispatch so no reflect-back fires")
	assert.NotEqual(t, models.DispatchPerformed, res.Outcome)
	assert.ErrorIs(t, err, clients.ErrDiscordForbidden)
}

func TestDiscordDispatch_OtherErrorIsError(t *testing.T) {
	api := &fakeDiscordAPI{err: clients.ErrDiscordUnauthorized}
	d := NewDiscord(api, zap.NewNop())
	_, err := d.Dispatch(context.Background(), "u1", models.ActionDelete,
		models.DispatchRequest{Platform: "discord", ChannelID: "c", NativeMessageID: "m"})
	require.Error(t, err)
	assert.ErrorIs(t, err, clients.ErrDiscordUnauthorized)
}

func TestDiscordDispatch_NonDeleteActionIsError(t *testing.T) {
	api := &fakeDiscordAPI{}
	d := NewDiscord(api, zap.NewNop())
	// Discord supports only delete; the handler gates this, but the dispatcher is defensive.
	_, err := d.Dispatch(context.Background(), "u1", models.ActionBan,
		models.DispatchRequest{Platform: "discord", ChannelID: "c", TargetUserID: "u"})
	require.Error(t, err)
	assert.Zero(t, api.calls)
}
