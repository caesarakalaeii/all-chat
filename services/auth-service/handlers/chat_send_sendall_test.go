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

package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/caesar/all-chat/shared/sendall"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// decideSendAllPill must turn the per-platform send outcomes into the right reconcile
// action so the combined pill lists ONLY the platforms that actually received the message.
func TestDecideSendAllPill(t *testing.T) {
	all := []string{"twitch", "youtube", "kick"}

	t.Run("all succeed: nothing to reconcile", func(t *testing.T) {
		d := decideSendAllPill(all, []streamerSendResult{
			{Platform: "twitch", Success: true},
			{Platform: "youtube", Success: true},
			{Platform: "kick", Success: true},
		})
		assert.Equal(t, sendAllNoChange, d.action)
		assert.Equal(t, all, d.platforms)
	})

	t.Run("one fails, two remain: rewrite to the successes in order", func(t *testing.T) {
		d := decideSendAllPill(all, []streamerSendResult{
			{Platform: "twitch", Success: true},
			{Platform: "youtube", Success: false, ErrorKind: string(sendErrOffline)},
			{Platform: "kick", Success: true},
		})
		assert.Equal(t, sendAllRewrite, d.action)
		assert.Equal(t, []string{"twitch", "kick"}, d.platforms)
	})

	t.Run("only one succeeds: delete group, no combined pill", func(t *testing.T) {
		d := decideSendAllPill(all, []streamerSendResult{
			{Platform: "twitch", Success: true},
			{Platform: "youtube", Success: false},
			{Platform: "kick", Success: false},
		})
		assert.Equal(t, sendAllDelete, d.action)
		assert.Equal(t, []string{"twitch"}, d.platforms)
	})

	t.Run("all fail: delete group", func(t *testing.T) {
		d := decideSendAllPill([]string{"twitch", "youtube"}, []streamerSendResult{
			{Platform: "twitch", Success: false},
			{Platform: "youtube", Success: false},
		})
		assert.Equal(t, sendAllDelete, d.action)
		assert.Empty(t, d.platforms)
	})
}

// sendResultErrorKind must classify untyped per-platform failures the same way the
// single-send path does, so the monitor shows a meaningful kind instead of send_failed.
func TestSendResultErrorKind(t *testing.T) {
	// Typed errors carry their kind directly.
	assert.Equal(t, sendErrQuota, sendResultErrorKind(&streamerSendError{kind: sendErrQuota, msg: "x"}))
	assert.Equal(t, sendErrMissingScope, sendResultErrorKind(&streamerSendError{kind: sendErrMissingScope, msg: "x"}))

	// Untyped "not live" discovery failure ⇒ stream_offline (the common legitimate case).
	assert.Equal(t, sendErrOffline,
		sendResultErrorKind(errors.New("streamer is not currently live on YouTube (no live videos found for channel UC1)")))

	// Untyped auth failure ⇒ reauth_required.
	assert.Equal(t, sendErrReauth, sendResultErrorKind(errors.New("failed to refresh token: bad")))

	// Untyped genuine upstream failure (the observed liveChatId 403) ⇒ send_failed.
	assert.Equal(t, sendErrUpstream,
		sendResultErrorKind(errors.New("failed to get live chat ID: youtube search API error: status=403 body=accountDelegationForbidden")))
}

// The reconcile must drop a failed platform's echo key and rewrite the survivors so the
// pill reflects exactly the platforms the message reached. This is the regression guard
// for the reported bug: YouTube failed yet the pill still showed Twitch + YouTube.
func TestWriteSendAllKeys_DropsFailedPlatformOnReconcile(t *testing.T) {
	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rc.Close()

	h := &ChatSendHandler{log: zap.NewNop(), redisClient: rc}
	ctx := context.Background()
	ids := map[string]string{"twitch": "T1", "youtube": "Y1", "kick": "K1"}
	msg := "Hello  there" // double space exercises NormalizeText in the key derivation
	groupID := "grp-1"

	// Pre-register the full intended set, exactly as the handler does before sending.
	h.writeSendAllKeys(ctx, ids, msg, []string{"twitch", "youtube", "kick"}, groupID)

	// YouTube fails; Twitch + Kick succeed.
	d := decideSendAllPill([]string{"twitch", "youtube", "kick"}, []streamerSendResult{
		{Platform: "twitch", Success: true},
		{Platform: "youtube", Success: false},
		{Platform: "kick", Success: true},
	})
	require.Equal(t, sendAllRewrite, d.action)
	h.writeSendAllKeys(ctx, ids, msg, d.platforms, groupID)

	// YouTube's echo key must be gone — it never echoes, so it must not appear in the pill.
	_, err := rc.Get(ctx, sendall.Key("youtube", "Y1", msg)).Result()
	assert.ErrorIs(t, err, redis.Nil, "youtube key should be deleted after reconcile")

	// Surviving keys carry the reduced platform set under the same group id.
	for _, p := range []struct{ plat, id string }{{"twitch", "T1"}, {"kick", "K1"}} {
		raw, err := rc.Get(ctx, sendall.Key(p.plat, p.id, msg)).Result()
		require.NoError(t, err, "expected surviving key for %s", p.plat)
		var reg sendall.Registration
		require.NoError(t, json.Unmarshal([]byte(raw), &reg))
		assert.Equal(t, groupID, reg.GroupID)
		assert.Equal(t, []string{"twitch", "kick"}, reg.Platforms, "pill set for %s", p.plat)
	}
}

// When fewer than two platforms succeed there is no combined pill, so the whole group is
// dropped and the lone echo flows through as an ordinary single-platform message.
func TestDeleteSendAllKeys_RemovesEntireGroup(t *testing.T) {
	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rc.Close()

	h := &ChatSendHandler{log: zap.NewNop(), redisClient: rc}
	ctx := context.Background()
	ids := map[string]string{"twitch": "T1", "youtube": "Y1"}
	msg := "single survivor"

	h.writeSendAllKeys(ctx, ids, msg, []string{"twitch", "youtube"}, "grp-2")
	h.deleteSendAllKeys(ctx, ids, msg)

	for _, p := range []struct{ plat, id string }{{"twitch", "T1"}, {"youtube", "Y1"}} {
		_, err := rc.Get(ctx, sendall.Key(p.plat, p.id, msg)).Result()
		assert.ErrorIs(t, err, redis.Nil, "key for %s should be deleted", p.plat)
	}
}

// Both helpers must no-op (not panic) when Redis is unconfigured.
func TestSendAllKeys_NilRedisNoop(t *testing.T) {
	h := &ChatSendHandler{log: zap.NewNop(), redisClient: nil}
	h.writeSendAllKeys(context.Background(), map[string]string{"twitch": "T1"}, "hi", []string{"twitch"}, "g")
	h.deleteSendAllKeys(context.Background(), map[string]string{"twitch": "T1"}, "hi")
}
