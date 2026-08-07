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
	"github.com/caesar/all-chat/services/moderation-service/tokens"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type fakeYTTokens struct {
	cred       *tokens.YouTubeCredential
	resolveErr error
	refreshErr error
	refreshes  int
	onRefresh  func(*tokens.YouTubeCredential)
}

func (f *fakeYTTokens) Resolve(context.Context, string, string) (*tokens.YouTubeCredential, error) {
	if f.resolveErr != nil {
		return nil, f.resolveErr
	}
	return f.cred, nil
}

func (f *fakeYTTokens) Refresh(_ context.Context, cred *tokens.YouTubeCredential) error {
	f.refreshes++
	if f.refreshErr != nil {
		return f.refreshErr
	}
	if f.onRefresh != nil {
		f.onRefresh(cred)
	}
	return nil
}

type fakeYTAPI struct {
	results    []error
	calls      int
	tokensSeen []string
	liveChat   string
	banned     string
}

func (f *fakeYTAPI) BanUser(_ context.Context, token, liveChatID, bannedChannelID string) error {
	f.tokensSeen = append(f.tokensSeen, token)
	f.liveChat, f.banned = liveChatID, bannedChannelID
	var err error
	if f.calls < len(f.results) {
		err = f.results[f.calls]
	}
	f.calls++
	return err
}

type fakeLiveChat struct {
	id  string
	err error
}

func (f fakeLiveChat) Resolve(context.Context, string) (string, error) { return f.id, f.err }

type fakeQuota struct {
	reserveOK          bool
	reserveErr         error
	reserves, confirms int
	rollbacks          int
}

func (f *fakeQuota) Reserve(context.Context, int) (bool, error) {
	f.reserves++
	return f.reserveOK, f.reserveErr
}
func (f *fakeQuota) Confirm(context.Context, int) error  { f.confirms++; return nil }
func (f *fakeQuota) Rollback(context.Context, int) error { f.rollbacks++; return nil }

func ytCredWith(scopes ...string) *tokens.YouTubeCredential {
	return &tokens.YouTubeCredential{AccessToken: "tok", RefreshToken: "ref", GrantedScopes: scopes, ExpiresAt: time.Now().Add(time.Hour)}
}

func newYT(src youtubeTokenSource, api youtubeAPI, lc liveChatResolver, q quotaReserver) *YouTube {
	return NewYouTube(src, api, lc, q, zap.NewNop())
}

func banReq() models.DispatchRequest {
	return models.DispatchRequest{Platform: "youtube", ChannelID: "UCchan", TargetUserID: "UCbanned"}
}

func TestYouTubeDispatch_NonYouTubeIsDryRun(t *testing.T) {
	api, q := &fakeYTAPI{}, &fakeQuota{reserveOK: true}
	d := newYT(&fakeYTTokens{}, api, fakeLiveChat{id: "lc"}, q)
	res, err := d.Dispatch(context.Background(), owner("u1"), models.ActionBan, models.DispatchRequest{Platform: "twitch"})
	require.NoError(t, err)
	assert.Equal(t, models.DispatchDryRun, res.Outcome)
	assert.Zero(t, api.calls)
	assert.Zero(t, q.reserves)
}

func TestYouTubeDispatch_NoCredential(t *testing.T) {
	api, q := &fakeYTAPI{}, &fakeQuota{reserveOK: true}
	d := newYT(&fakeYTTokens{resolveErr: tokens.ErrNoCredential}, api, fakeLiveChat{id: "lc"}, q)
	res, err := d.Dispatch(context.Background(), owner("u1"), models.ActionBan, banReq())
	require.NoError(t, err)
	assert.Equal(t, models.DispatchNoCredential, res.Outcome)
	assert.Zero(t, q.reserves)
}

func TestYouTubeDispatch_MissingScopeSkipsEverything(t *testing.T) {
	api, q := &fakeYTAPI{}, &fakeQuota{reserveOK: true}
	d := newYT(&fakeYTTokens{cred: ytCredWith("https://www.googleapis.com/auth/youtube.readonly")}, api, fakeLiveChat{id: "lc"}, q)
	res, err := d.Dispatch(context.Background(), owner("u1"), models.ActionBan, banReq())
	require.NoError(t, err)
	assert.Equal(t, models.DispatchReauthRequired, res.Outcome)
	assert.Equal(t, []string{models.ScopeYouTubeModeration}, res.MissingScopes)
	assert.Zero(t, q.reserves, "no quota is reserved when the scope is absent")
	assert.Zero(t, api.calls)
}

func TestYouTubeDispatch_NotLiveIsErrorNoQuota(t *testing.T) {
	api, q := &fakeYTAPI{}, &fakeQuota{reserveOK: true}
	d := newYT(&fakeYTTokens{cred: ytCredWith(models.ScopeYouTubeModeration)}, api, fakeLiveChat{err: clients.ErrYouTubeNotLive}, q)
	_, err := d.Dispatch(context.Background(), owner("u1"), models.ActionBan, banReq())
	require.Error(t, err)
	assert.Zero(t, q.reserves, "no quota reserved when there is no live chat to ban in")
	assert.Zero(t, api.calls)
}

func TestYouTubeDispatch_QuotaExhausted(t *testing.T) {
	api, q := &fakeYTAPI{}, &fakeQuota{reserveOK: false}
	d := newYT(&fakeYTTokens{cred: ytCredWith(models.ScopeYouTubeModeration)}, api, fakeLiveChat{id: "lc"}, q)
	_, err := d.Dispatch(context.Background(), owner("u1"), models.ActionBan, banReq())
	require.Error(t, err)
	assert.Equal(t, 1, q.reserves)
	assert.Zero(t, api.calls, "no API call when the reservation is refused")
}

func TestYouTubeDispatch_BanPerformedConfirmsQuota(t *testing.T) {
	api, q := &fakeYTAPI{results: []error{nil}}, &fakeQuota{reserveOK: true}
	d := newYT(&fakeYTTokens{cred: ytCredWith(models.ScopeYouTubeModeration)}, api, fakeLiveChat{id: "lc-1"}, q)
	res, err := d.Dispatch(context.Background(), owner("u1"), models.ActionBan, banReq())
	require.NoError(t, err)
	assert.Equal(t, models.DispatchPerformed, res.Outcome)
	assert.Equal(t, "lc-1", api.liveChat)
	assert.Equal(t, "UCbanned", api.banned)
	assert.Equal(t, 1, q.confirms)
	assert.Zero(t, q.rollbacks)
}

func TestYouTubeDispatch_UnauthorizedRefreshesAndRetries(t *testing.T) {
	api := &fakeYTAPI{results: []error{clients.ErrYouTubeUnauthorized, nil}}
	q := &fakeQuota{reserveOK: true}
	src := &fakeYTTokens{cred: ytCredWith(models.ScopeYouTubeModeration), onRefresh: func(c *tokens.YouTubeCredential) { c.AccessToken = "tok2" }}
	d := newYT(src, api, fakeLiveChat{id: "lc"}, q)
	res, err := d.Dispatch(context.Background(), owner("u1"), models.ActionBan, banReq())
	require.NoError(t, err)
	assert.Equal(t, models.DispatchPerformed, res.Outcome)
	assert.Equal(t, 1, src.refreshes)
	assert.Equal(t, []string{"tok", "tok2"}, api.tokensSeen, "the retry uses the refreshed token")
	assert.Equal(t, 1, q.confirms, "quota reserved once, confirmed once after the retry succeeds")
	assert.Equal(t, 1, q.reserves)
}

func TestYouTubeDispatch_ForbiddenRollsBackQuota(t *testing.T) {
	api, q := &fakeYTAPI{results: []error{clients.ErrYouTubeForbidden}}, &fakeQuota{reserveOK: true}
	d := newYT(&fakeYTTokens{cred: ytCredWith(models.ScopeYouTubeModeration)}, api, fakeLiveChat{id: "lc"}, q)
	res, err := d.Dispatch(context.Background(), owner("u1"), models.ActionBan, banReq())
	require.NoError(t, err)
	assert.Equal(t, models.DispatchReauthRequired, res.Outcome)
	assert.Equal(t, []string{models.ScopeYouTubeModeration}, res.MissingScopes)
	assert.Equal(t, 1, q.rollbacks, "a failed ban releases the reserved quota")
	assert.Zero(t, q.confirms)
}

func TestYouTubeDispatch_OtherErrorRollsBack(t *testing.T) {
	api, q := &fakeYTAPI{results: []error{errors.New("boom")}}, &fakeQuota{reserveOK: true}
	d := newYT(&fakeYTTokens{cred: ytCredWith(models.ScopeYouTubeModeration)}, api, fakeLiveChat{id: "lc"}, q)
	_, err := d.Dispatch(context.Background(), owner("u1"), models.ActionBan, banReq())
	require.Error(t, err)
	assert.Equal(t, 1, q.rollbacks)
}

// YouTube's delegated leg is not built, and its write path is permanent-ban-only besides —
// handing a volunteer permanent-ban-only would be a moderation-safety problem, so the refusal is
// explicit rather than incidental.
func TestYouTube_DelegatedActionIsRefused(t *testing.T) {
	api, q := &fakeYTAPI{}, &fakeQuota{reserveOK: true}
	d := newYT(&fakeYTTokens{cred: ytCredWith(models.ScopeYouTubeModeration)}, api, fakeLiveChat{id: "lc"}, q)

	res, err := d.Dispatch(context.Background(), moderator("mod", "own"), models.ActionBan, banReq())

	require.NoError(t, err)
	assert.Equal(t, models.DispatchDelegationUnsupported, res.Outcome)
	assert.Zero(t, api.calls)
	assert.Zero(t, q.reserves, "no quota is spent on an action that cannot happen")
}
