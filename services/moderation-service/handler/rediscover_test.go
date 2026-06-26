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

package handler

import (
	"context"
	"net/http"
	"testing"

	"github.com/caesar/all-chat/services/moderation-service/repository"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type fakeRediscover struct {
	published bool // what Publish reports (false = within cooldown)
	err       error
	calls     int
	gotChans  []string
}

func (f *fakeRediscover) Publish(_ context.Context, _, channelID string) (bool, error) {
	f.calls++
	f.gotChans = append(f.gotChans, channelID)
	return f.published, f.err
}

func newRediscoverRouter(h *Handler, userID string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		if userID != "" {
			c.Set("user_id", userID)
		}
		c.Next()
	})
	r.Group("/api/v1/moderation").POST("/overlays/:id/youtube/rediscover", h.HandleYouTubeRediscover)
	return r
}

func newRediscoverHandler(auth Authorizer, pub RediscoverPublisher) *Handler {
	h := New(auth, &fakeEmitter{}, &fakeRecorder{}, NoScopeChecker{}, DryRunDispatcher{}, zap.NewNop())
	if pub != nil {
		h.SetRediscoverPublisher(pub)
	}
	return h
}

const rediscoverPath = "/api/v1/moderation/overlays/" + overlayID + "/youtube/rediscover"

func TestRediscover_OwnerPublishesPerYouTubeSource(t *testing.T) {
	auth := &fakeAuthorizer{owns: true, sources: []repository.Source{
		{Platform: "youtube", ChannelID: "UCabc"},
		{Platform: "twitch", ChannelID: "somestreamer"}, // ignored — not youtube
	}}
	pub := &fakeRediscover{published: true}
	r := newRediscoverRouter(newRediscoverHandler(auth, pub), ownerID)

	resp := do(r, http.MethodPost, rediscoverPath, `{}`)

	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
	assert.Equal(t, 1, pub.calls, "should publish exactly once for the single youtube source")
	assert.Equal(t, []string{"UCabc"}, pub.gotChans)
}

func TestRediscover_NonOwnerForbidden(t *testing.T) {
	auth := &fakeAuthorizer{owns: false, sources: []repository.Source{{Platform: "youtube", ChannelID: "UCabc"}}}
	pub := &fakeRediscover{published: true}
	r := newRediscoverRouter(newRediscoverHandler(auth, pub), ownerID)

	resp := do(r, http.MethodPost, rediscoverPath, `{}`)

	require.Equal(t, http.StatusForbidden, resp.Code)
	assert.Zero(t, pub.calls, "a non-owner must not trigger any rediscovery")
}

func TestRediscover_NoYouTubeSourceBadRequest(t *testing.T) {
	auth := &fakeAuthorizer{owns: true, sources: []repository.Source{{Platform: "twitch", ChannelID: "somestreamer"}}}
	pub := &fakeRediscover{published: true}
	r := newRediscoverRouter(newRediscoverHandler(auth, pub), ownerID)

	resp := do(r, http.MethodPost, rediscoverPath, `{}`)

	require.Equal(t, http.StatusBadRequest, resp.Code)
	assert.Zero(t, pub.calls)
}

func TestRediscover_CooldownTooManyRequests(t *testing.T) {
	auth := &fakeAuthorizer{owns: true, sources: []repository.Source{{Platform: "youtube", ChannelID: "UCabc"}}}
	pub := &fakeRediscover{published: false} // within cooldown window
	r := newRediscoverRouter(newRediscoverHandler(auth, pub), ownerID)

	resp := do(r, http.MethodPost, rediscoverPath, `{}`)

	require.Equal(t, http.StatusTooManyRequests, resp.Code)
	assert.Equal(t, 1, pub.calls)
}

func TestRediscover_UnavailableWhenPublisherUnset(t *testing.T) {
	auth := &fakeAuthorizer{owns: true, sources: []repository.Source{{Platform: "youtube", ChannelID: "UCabc"}}}
	r := newRediscoverRouter(newRediscoverHandler(auth, nil), ownerID)

	resp := do(r, http.MethodPost, rediscoverPath, `{}`)

	require.Equal(t, http.StatusServiceUnavailable, resp.Code)
}

func TestRediscover_MissingUserUnauthorized(t *testing.T) {
	auth := &fakeAuthorizer{owns: true, sources: []repository.Source{{Platform: "youtube", ChannelID: "UCabc"}}}
	pub := &fakeRediscover{published: true}
	r := newRediscoverRouter(newRediscoverHandler(auth, pub), "") // no user_id set

	resp := do(r, http.MethodPost, rediscoverPath, `{}`)

	require.Equal(t, http.StatusUnauthorized, resp.Code)
	assert.Zero(t, pub.calls)
}
