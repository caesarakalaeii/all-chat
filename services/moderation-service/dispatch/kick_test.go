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
	"time"

	"github.com/caesar/all-chat/services/moderation-service/clients"
	"github.com/caesar/all-chat/services/moderation-service/models"
	"github.com/caesar/all-chat/services/moderation-service/tokens"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type fakeKickTokens struct {
	cred       *tokens.KickCredential
	resolveErr error
	refreshErr error
	refreshes  int
	onRefresh  func(*tokens.KickCredential)
}

func (f *fakeKickTokens) Resolve(context.Context, string, string) (*tokens.KickCredential, error) {
	if f.resolveErr != nil {
		return nil, f.resolveErr
	}
	return f.cred, nil
}

func (f *fakeKickTokens) Refresh(_ context.Context, cred *tokens.KickCredential) error {
	f.refreshes++
	if f.refreshErr != nil {
		return f.refreshErr
	}
	if f.onRefresh != nil {
		f.onRefresh(cred)
	}
	return nil
}

type fakeKickAPI struct {
	results     []error
	calls       int
	tokensSeen  []string
	method      string
	broadcaster string
	target      string
	duration    int
}

func (f *fakeKickAPI) next(token, broadcaster string) error {
	f.tokensSeen = append(f.tokensSeen, token)
	f.broadcaster = broadcaster
	var err error
	if f.calls < len(f.results) {
		err = f.results[f.calls]
	}
	f.calls++
	return err
}

func (f *fakeKickAPI) TimeoutUser(_ context.Context, token, b, target string, dur int, _ string) error {
	f.method, f.target, f.duration = "timeout", target, dur
	return f.next(token, b)
}
func (f *fakeKickAPI) BanUser(_ context.Context, token, b, target, _ string) error {
	f.method, f.target = "ban", target
	return f.next(token, b)
}
func (f *fakeKickAPI) UnbanUser(_ context.Context, token, b, target string) error {
	f.method, f.target = "unban", target
	return f.next(token, b)
}

func kickCredWith(scopes ...string) *tokens.KickCredential {
	return &tokens.KickCredential{
		AccessToken:   "tok",
		RefreshToken:  "ref",
		BroadcasterID: "555",
		GrantedScopes: scopes,
		ExpiresAt:     time.Now().Add(time.Hour),
	}
}

func TestKickDispatch_NonKickIsDryRun(t *testing.T) {
	api := &fakeKickAPI{}
	d := NewKick(&fakeKickTokens{}, api, zap.NewNop())
	res, err := d.Dispatch(context.Background(), "u1", models.ActionBan, models.DispatchRequest{Platform: "twitch", ChannelID: "c"})
	require.NoError(t, err)
	assert.Equal(t, models.DispatchDryRun, res.Outcome)
	assert.Zero(t, api.calls)
}

func TestKickDispatch_NoCredential(t *testing.T) {
	api := &fakeKickAPI{}
	d := NewKick(&fakeKickTokens{resolveErr: tokens.ErrNoCredential}, api, zap.NewNop())
	res, err := d.Dispatch(context.Background(), "u1", models.ActionBan, models.DispatchRequest{Platform: "kick", ChannelID: "c"})
	require.NoError(t, err)
	assert.Equal(t, models.DispatchNoCredential, res.Outcome)
	assert.Zero(t, api.calls)
}

func TestKickDispatch_MissingScopePreCheckSkipsAPICall(t *testing.T) {
	api := &fakeKickAPI{}
	d := NewKick(&fakeKickTokens{cred: kickCredWith("user:read")}, api, zap.NewNop())
	res, err := d.Dispatch(context.Background(), "u1", models.ActionBan, models.DispatchRequest{Platform: "kick", ChannelID: "c", TargetUserID: "42"})
	require.NoError(t, err)
	assert.Equal(t, models.DispatchReauthRequired, res.Outcome)
	assert.Equal(t, []string{models.ScopeKickModeration}, res.MissingScopes)
	assert.Zero(t, api.calls, "must not call the Kick API when the scope is absent")
}

func TestKickDispatch_BanPerformed(t *testing.T) {
	api := &fakeKickAPI{results: []error{nil}}
	d := NewKick(&fakeKickTokens{cred: kickCredWith(models.ScopeKickModeration)}, api, zap.NewNop())
	res, err := d.Dispatch(context.Background(), "u1", models.ActionBan, models.DispatchRequest{Platform: "kick", ChannelID: "c", TargetUserID: "42"})
	require.NoError(t, err)
	assert.Equal(t, models.DispatchPerformed, res.Outcome)
	assert.Equal(t, "ban", api.method)
	assert.Equal(t, "42", api.target)
	assert.Equal(t, "555", api.broadcaster, "broadcaster id comes from the resolved credential (kick_id)")
}

func TestKickDispatch_TimeoutThreadsDuration(t *testing.T) {
	api := &fakeKickAPI{results: []error{nil}}
	d := NewKick(&fakeKickTokens{cred: kickCredWith(models.ScopeKickModeration)}, api, zap.NewNop())
	res, err := d.Dispatch(context.Background(), "u1", models.ActionTimeout,
		models.DispatchRequest{Platform: "kick", ChannelID: "c", TargetUserID: "42", DurationSeconds: 600})
	require.NoError(t, err)
	assert.Equal(t, models.DispatchPerformed, res.Outcome)
	assert.Equal(t, "timeout", api.method)
	assert.Equal(t, 600, api.duration, "duration (seconds) is threaded to the client, which converts to minutes")
}

func TestKickDispatch_UnauthorizedRefreshesAndRetries(t *testing.T) {
	api := &fakeKickAPI{results: []error{clients.ErrKickUnauthorized, nil}}
	src := &fakeKickTokens{
		cred:      kickCredWith(models.ScopeKickModeration),
		onRefresh: func(c *tokens.KickCredential) { c.AccessToken = "tok2" },
	}
	d := NewKick(src, api, zap.NewNop())
	res, err := d.Dispatch(context.Background(), "u1", models.ActionBan, models.DispatchRequest{Platform: "kick", ChannelID: "c", TargetUserID: "42"})
	require.NoError(t, err)
	assert.Equal(t, models.DispatchPerformed, res.Outcome)
	assert.Equal(t, 1, src.refreshes, "a 401 triggers exactly one reactive refresh")
	assert.Equal(t, []string{"tok", "tok2"}, api.tokensSeen, "the retry uses the refreshed token")
}

func TestKickDispatch_ForbiddenIsReauthWithScope(t *testing.T) {
	api := &fakeKickAPI{results: []error{clients.ErrKickForbidden}}
	d := NewKick(&fakeKickTokens{cred: kickCredWith(models.ScopeKickModeration)}, api, zap.NewNop())
	res, err := d.Dispatch(context.Background(), "u1", models.ActionBan, models.DispatchRequest{Platform: "kick", ChannelID: "c", TargetUserID: "42"})
	require.NoError(t, err)
	assert.Equal(t, models.DispatchReauthRequired, res.Outcome)
	assert.Equal(t, []string{models.ScopeKickModeration}, res.MissingScopes)
}
