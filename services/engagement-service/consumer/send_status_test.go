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
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/caesar/all-chat/services/engagement-service/announcer"
	"github.com/caesar/all-chat/services/engagement-service/models"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func newSendStatusTest(t *testing.T) (*SendStatus, *fakeOutput) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rdb.Close() })
	out := &fakeOutput{}
	return NewSendStatus(rdb, out, zap.NewNop()), out
}

// A reply the platform refused is remembered with who sent it and what the
// platform said, until the next delivered post on that platform clears it.
func TestSendStatusRemembersTheLastFailure(t *testing.T) {
	s, out := newSendStatusTest(t)
	ctx := context.Background()
	out.sendErr = &announcer.BotSendError{Results: []announcer.SendResult{{Platform: "twitch",
		Sender: "allchat_bot", ErrorKind: "message_dropped", Detail: "A verified phone number is required."}}}

	err := s.BotSend(ctx, ownerID.String(), "woof", "twitch", announcer.PurposeCommand)
	require.Error(t, err, "the wrapper never hides a failure from the caller")
	got, err := s.Failures(ctx, ownerID)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, models.SendFailure{Platform: "twitch", Sender: "allchat_bot", ErrorKind: "message_dropped",
		Detail: "A verified phone number is required.", At: got[0].At}, got[0])
	assert.False(t, got[0].At.IsZero())

	out.sendErr = nil
	require.NoError(t, s.BotSend(ctx, ownerID.String(), "woof", "twitch", announcer.PurposeCommand))
	got, err = s.Failures(ctx, ownerID)
	require.NoError(t, err)
	assert.Empty(t, got)
}

// A failure that is not a platform outcome (auth-service down, no identity) is
// remembered without leaking the internal error text to the streamer.
func TestSendStatusUnstructuredFailure(t *testing.T) {
	s, out := newSendStatusTest(t)
	ctx := context.Background()
	out.sendErr = errors.New("bot-send endpoint returned 502: upstream connect error")
	_ = s.BotSend(ctx, ownerID.String(), "woof", "kick", announcer.PurposeTimer)

	out.sendErr = &announcer.BotSendError{}
	_ = s.BotSend(ctx, ownerID.String(), "woof", "youtube", announcer.PurposeCommand)

	got, err := s.Failures(ctx, ownerID)
	require.NoError(t, err)
	kinds := map[string]models.SendFailure{}
	for _, f := range got {
		kinds[f.Platform] = f
	}
	assert.Equal(t, "send_failed", kinds["kick"].ErrorKind)
	assert.Empty(t, kinds["kick"].Detail)
	assert.Equal(t, "not_connected", kinds["youtube"].ErrorKind)
}
