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

type fakeTokens struct {
	cred       *tokens.TwitchCredential
	resolveErr error
	refreshErr error
	refreshes  int
	onRefresh  func(*tokens.TwitchCredential) // mutate cred to simulate a successful refresh
	// anchor is the broadcaster id the owner-reach anchor resolves; anchorErr overrides it.
	anchor    string
	anchorErr error
	anchorFor []string // the (owner, channel) pairs the anchor was asked about
}

func (f *fakeTokens) OwnerTwitchAnchor(_ context.Context, ownerUserID, channelID string) (string, error) {
	f.anchorFor = append(f.anchorFor, ownerUserID+"|"+channelID)
	if f.anchorErr != nil {
		return "", f.anchorErr
	}
	return f.anchor, nil
}

// fakeModTokens stands in for a delegated moderator's OWN credential store.
type fakeModTokens struct {
	cred        *tokens.ModCredential
	resolveErr  error
	refreshErr  error
	refreshes   int
	resolvedFor []string
	onRefresh   func(*tokens.ModCredential)
}

func (f *fakeModTokens) Resolve(_ context.Context, userID string) (*tokens.ModCredential, error) {
	f.resolvedFor = append(f.resolvedFor, userID)
	if f.resolveErr != nil {
		return nil, f.resolveErr
	}
	return f.cred, nil
}

func (f *fakeModTokens) Refresh(_ context.Context, _ string, cred *tokens.ModCredential) error {
	f.refreshes++
	if f.refreshErr != nil {
		return f.refreshErr
	}
	if f.onRefresh != nil {
		f.onRefresh(cred)
	}
	return nil
}

func (f *fakeTokens) Resolve(context.Context, string, string) (*tokens.TwitchCredential, error) {
	if f.resolveErr != nil {
		return nil, f.resolveErr
	}
	return f.cred, nil
}

func (f *fakeTokens) Refresh(_ context.Context, cred *tokens.TwitchCredential) error {
	f.refreshes++
	if f.refreshErr != nil {
		return f.refreshErr
	}
	if f.onRefresh != nil {
		f.onRefresh(cred)
	}
	return nil
}

// fakeAPI returns the configured error per call (in order) and records arguments.
type fakeAPI struct {
	results     []error
	calls       int
	tokensSeen  []string
	method      string
	broadcaster string
	moderator   string
	target      string
	msgID       string
	duration    int
}

func (f *fakeAPI) next(token, broadcaster, moderator string) error {
	f.tokensSeen = append(f.tokensSeen, token)
	f.broadcaster, f.moderator = broadcaster, moderator
	var err error
	if f.calls < len(f.results) {
		err = f.results[f.calls]
	}
	f.calls++
	return err
}

func (f *fakeAPI) DeleteMessage(_ context.Context, token, b, m, msgID string) error {
	f.method, f.msgID = "delete", msgID
	return f.next(token, b, m)
}
func (f *fakeAPI) TimeoutUser(_ context.Context, token, b, m, target string, dur int, _ string) error {
	f.method, f.target, f.duration = "timeout", target, dur
	return f.next(token, b, m)
}
func (f *fakeAPI) BanUser(_ context.Context, token, b, m, target, _ string) error {
	f.method, f.target = "ban", target
	return f.next(token, b, m)
}
func (f *fakeAPI) UnbanUser(_ context.Context, token, b, m, target string) error {
	f.method, f.target = "unban", target
	return f.next(token, b, m)
}

func credWith(scopes ...string) *tokens.TwitchCredential {
	return &tokens.TwitchCredential{
		AccessToken:   "tok",
		RefreshToken:  "ref",
		BroadcasterID: "9001",
		GrantedScopes: scopes,
		ExpiresAt:     time.Now().Add(time.Hour), // far future: no proactive refresh
	}
}

func TestDispatch_NonTwitchIsDryRun(t *testing.T) {
	api := &fakeAPI{}
	d := NewTwitch(&fakeTokens{}, api, zap.NewNop())
	res, err := d.Dispatch(context.Background(), owner("u1"), models.ActionDelete, models.DispatchRequest{Platform: "kick", ChannelID: "c"})
	require.NoError(t, err)
	assert.Equal(t, models.DispatchDryRun, res.Outcome)
	assert.Zero(t, api.calls, "no platform call for an unimplemented platform")
}

func TestDispatch_NoCredential(t *testing.T) {
	api := &fakeAPI{}
	d := NewTwitch(&fakeTokens{resolveErr: tokens.ErrNoCredential}, api, zap.NewNop())
	res, err := d.Dispatch(context.Background(), owner("u1"), models.ActionDelete, models.DispatchRequest{Platform: "twitch", ChannelID: "c"})
	require.NoError(t, err)
	assert.Equal(t, models.DispatchNoCredential, res.Outcome)
	assert.Zero(t, api.calls)
}

func TestDispatch_MissingScopePreCheckSkipsAPICall(t *testing.T) {
	api := &fakeAPI{}
	// Token has banned-users scope but the action is delete (needs manage:chat_messages).
	d := NewTwitch(&fakeTokens{cred: credWith(models.ScopeTwitchManageBannedUsers)}, api, zap.NewNop())
	res, err := d.Dispatch(context.Background(), owner("u1"), models.ActionDelete, models.DispatchRequest{Platform: "twitch", ChannelID: "c", NativeMessageID: "m1"})
	require.NoError(t, err)
	assert.Equal(t, models.DispatchReauthRequired, res.Outcome)
	assert.Equal(t, []string{models.ScopeTwitchManageMessages}, res.MissingScopes)
	assert.Zero(t, api.calls, "must not call Helix when the scope is absent")
}

func TestDispatch_DeletePerformed(t *testing.T) {
	api := &fakeAPI{results: []error{nil}}
	d := NewTwitch(&fakeTokens{cred: credWith(models.ScopeTwitchManageMessages)}, api, zap.NewNop())
	res, err := d.Dispatch(context.Background(), owner("u1"), models.ActionDelete, models.DispatchRequest{Platform: "twitch", ChannelID: "c", NativeMessageID: "m1"})
	require.NoError(t, err)
	assert.Equal(t, models.DispatchPerformed, res.Outcome)
	assert.Equal(t, 1, api.calls)
	assert.Equal(t, "delete", api.method)
	assert.Equal(t, "m1", api.msgID)
	assert.Equal(t, "9001", api.broadcaster, "broadcaster_id comes from the resolved credential")
	assert.Equal(t, []string{"tok"}, api.tokensSeen)
}

func TestDispatch_TimeoutThreadsDuration(t *testing.T) {
	api := &fakeAPI{results: []error{nil}}
	d := NewTwitch(&fakeTokens{cred: credWith(models.ScopeTwitchManageBannedUsers)}, api, zap.NewNop())
	res, err := d.Dispatch(context.Background(), owner("u1"), models.ActionTimeout,
		models.DispatchRequest{Platform: "twitch", ChannelID: "c", TargetUserID: "42", DurationSeconds: 600})
	require.NoError(t, err)
	assert.Equal(t, models.DispatchPerformed, res.Outcome)
	assert.Equal(t, "timeout", api.method)
	assert.Equal(t, "42", api.target)
	assert.Equal(t, 600, api.duration)
}

func TestDispatch_UnauthorizedRefreshesAndRetries(t *testing.T) {
	api := &fakeAPI{results: []error{clients.ErrUnauthorized, nil}} // 401 then success
	src := &fakeTokens{
		cred:      credWith(models.ScopeTwitchManageMessages),
		onRefresh: func(c *tokens.TwitchCredential) { c.AccessToken = "tok2" },
	}
	d := NewTwitch(src, api, zap.NewNop())
	res, err := d.Dispatch(context.Background(), owner("u1"), models.ActionDelete, models.DispatchRequest{Platform: "twitch", ChannelID: "c", NativeMessageID: "m1"})
	require.NoError(t, err)
	assert.Equal(t, models.DispatchPerformed, res.Outcome)
	assert.Equal(t, 1, src.refreshes, "a 401 triggers exactly one reactive refresh")
	assert.Equal(t, 2, api.calls, "the action is retried once after refresh")
	assert.Equal(t, []string{"tok", "tok2"}, api.tokensSeen, "the retry uses the refreshed token")
}

func TestDispatch_UnauthorizedRefreshFailsIsReauth(t *testing.T) {
	api := &fakeAPI{results: []error{clients.ErrUnauthorized}}
	src := &fakeTokens{cred: credWith(models.ScopeTwitchManageMessages), refreshErr: assertErr("refresh dead")}
	d := NewTwitch(src, api, zap.NewNop())
	res, err := d.Dispatch(context.Background(), owner("u1"), models.ActionDelete, models.DispatchRequest{Platform: "twitch", ChannelID: "c", NativeMessageID: "m1"})
	require.NoError(t, err)
	assert.Equal(t, models.DispatchReauthRequired, res.Outcome)
	assert.Equal(t, 1, api.calls, "no retry when the refresh fails")
}

func TestDispatch_ForbiddenIsReauthWithScope(t *testing.T) {
	api := &fakeAPI{results: []error{clients.ErrForbidden}}
	// Token claims the scope locally, but Twitch rejects (e.g. not actually a moderator).
	d := NewTwitch(&fakeTokens{cred: credWith(models.ScopeTwitchManageBannedUsers)}, api, zap.NewNop())
	res, err := d.Dispatch(context.Background(), owner("u1"), models.ActionBan, models.DispatchRequest{Platform: "twitch", ChannelID: "c", TargetUserID: "42"})
	require.NoError(t, err)
	assert.Equal(t, models.DispatchReauthRequired, res.Outcome)
	assert.Equal(t, []string{models.ScopeTwitchManageBannedUsers}, res.MissingScopes)
}

func TestDispatch_ProactiveRefreshNearExpiry(t *testing.T) {
	api := &fakeAPI{results: []error{nil}}
	cred := credWith(models.ScopeTwitchManageMessages)
	cred.ExpiresAt = time.Now().Add(time.Minute) // within the lead time
	src := &fakeTokens{cred: cred}
	d := NewTwitch(src, api, zap.NewNop())
	res, err := d.Dispatch(context.Background(), owner("u1"), models.ActionDelete, models.DispatchRequest{Platform: "twitch", ChannelID: "c", NativeMessageID: "m1"})
	require.NoError(t, err)
	assert.Equal(t, models.DispatchPerformed, res.Outcome)
	assert.Equal(t, 1, src.refreshes, "an imminent expiry triggers a proactive refresh")
	assert.Equal(t, 1, api.calls, "the call still happens once after the proactive refresh")
}

type assertErr string

func (e assertErr) Error() string { return string(e) }
