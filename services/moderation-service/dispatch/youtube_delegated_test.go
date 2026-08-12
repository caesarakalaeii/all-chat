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
)

// The delegated YouTube write path (ADR-0048).
//
// YouTube is the simplest of the three OAuth platforms and the strictest: its liveChatBans request
// carries no actor field at all, so the moderator's token IS the entire claim of who is acting, and
// YouTube re-checks on every call that the token's account owns or moderates that live chat. All-Chat
// caches no moderator status; it only has to make sure the right token is used.

func ytModCred(scopes ...string) *tokens.ModCredential {
	return &tokens.ModCredential{
		AccessToken:    "yt-mod-tok",
		RefreshToken:   "yt-mod-ref",
		PlatformUserID: "google-9001",
		GrantedScopes:  scopes,
		ExpiresAt:      time.Now().Add(time.Hour), // far future: no proactive refresh
	}
}

func delegatedYT(t *testing.T, own *fakeYTTokens, mod *fakeModTokens, api *fakeYTAPI, q *fakeQuota) *YouTube {
	t.Helper()
	d := newYT(own, api, fakeLiveChat{id: "lc-1"}, q)
	if mod != nil {
		d.SetModSource(mod)
	}
	return d
}

// The moderator's own token performs the call, and the audit records whose it was.
func TestYouTubeDelegated_UsesTheModeratorsOwnCredential(t *testing.T) {
	own := &fakeYTTokens{cred: ytCredWith(models.ScopeYouTubeModeration)}
	mod := &fakeModTokens{cred: ytModCred(models.ScopeYouTubeModeration)}
	api, q := &fakeYTAPI{}, &fakeQuota{reserveOK: true}
	d := delegatedYT(t, own, mod, api, q)

	res, err := d.Dispatch(context.Background(), moderator(modUserID, ownerUserID), models.ActionBan, banReq())

	require.NoError(t, err)
	assert.Equal(t, models.DispatchPerformed, res.Outcome)
	assert.Equal(t, []string{"yt-mod-tok"}, api.tokensSeen, "the moderator's own token performs the call")
	assert.Equal(t, modUserID, res.CredentialUserID)
	assert.Equal(t, "google-9001", res.PlatformActorID)
	assert.Equal(t, 1, q.confirms)
}

// No fallback: the owner holds a working credential, and it must not be reached for.
func TestYouTubeDelegated_NeverReachesForTheOwnersCredential(t *testing.T) {
	own := &fakeYTTokens{cred: ytCredWith(models.ScopeYouTubeModeration)}
	mod := &fakeModTokens{resolveErr: tokens.ErrNoCredential}
	api, q := &fakeYTAPI{}, &fakeQuota{reserveOK: true}
	d := delegatedYT(t, own, mod, api, q)

	res, err := d.Dispatch(context.Background(), moderator(modUserID, ownerUserID), models.ActionBan, banReq())

	require.NoError(t, err)
	assert.Equal(t, models.DispatchNoCredential, res.Outcome)
	assert.Zero(t, api.calls)
	assert.Zero(t, own.resolves, "the owner's token store must not even be consulted")
	assert.Zero(t, q.reserves, "no quota is spent on an action that cannot happen")
}

// The anchor is asked about the OWNER, and it gates the action before any credential work.
func TestYouTubeDelegated_OwnerWithoutAnAnchorIsRefused(t *testing.T) {
	own := &fakeYTTokens{anchorErr: tokens.ErrOwnerChannelUnverified}
	mod := &fakeModTokens{cred: ytModCred(models.ScopeYouTubeModeration)}
	api, q := &fakeYTAPI{}, &fakeQuota{reserveOK: true}
	d := delegatedYT(t, own, mod, api, q)

	res, err := d.Dispatch(context.Background(), moderator(modUserID, ownerUserID), models.ActionBan, banReq())

	require.NoError(t, err)
	assert.Equal(t, models.DispatchOwnerUnverified, res.Outcome)
	assert.Equal(t, []string{ownerUserID + "|UCchan"}, own.anchorFor)
	assert.Empty(t, mod.resolvedFor, "the anchor is checked first")
	assert.Zero(t, q.reserves)
}

// The anchor applies to the OWNER path too, which is what YouTube specifically needs: its credential
// lookup falls back to a channel-agnostic users row, so an unanchored owner path would ask YouTube
// about channels the streamer merely added as a read-only source.
func TestYouTubeOwner_WithoutAnAnchorIsRefused(t *testing.T) {
	own := &fakeYTTokens{anchorErr: tokens.ErrOwnerChannelUnverified, cred: ytCredWith(models.ScopeYouTubeModeration)}
	api, q := &fakeYTAPI{}, &fakeQuota{reserveOK: true}
	d := newYT(own, api, fakeLiveChat{id: "lc"}, q)

	res, err := d.Dispatch(context.Background(), owner("u1"), models.ActionBan, banReq())

	require.NoError(t, err)
	assert.Equal(t, models.DispatchOwnerUnverified, res.Outcome)
	assert.Equal(t, []string{"u1|UCchan"}, own.anchorFor, "an owner anchors against themselves")
	assert.Zero(t, api.calls)
	assert.Zero(t, own.resolves)
}

// The scope pre-check reads the MODERATOR's scopes, not the streamer's.
func TestYouTubeDelegated_ScopePreCheckUsesTheModeratorsScopes(t *testing.T) {
	own := &fakeYTTokens{cred: ytCredWith(models.ScopeYouTubeModeration)}
	mod := &fakeModTokens{cred: ytModCred("https://www.googleapis.com/auth/userinfo.profile")}
	api, q := &fakeYTAPI{}, &fakeQuota{reserveOK: true}
	d := delegatedYT(t, own, mod, api, q)

	res, err := d.Dispatch(context.Background(), moderator(modUserID, ownerUserID), models.ActionBan, banReq())

	require.NoError(t, err)
	assert.Equal(t, models.DispatchReauthRequired, res.Outcome)
	assert.Equal(t, []string{models.ScopeYouTubeModeration}, res.MissingScopes)
	assert.Zero(t, api.calls)
	assert.Zero(t, q.reserves)
}

// A 403 after a passing scope pre-check is YouTube saying this account does not moderate that live
// chat. Reporting it as a re-consent would loop a volunteer through a screen that cannot help.
func TestYouTubeDelegated_ForbiddenReportsNotAPlatformModerator(t *testing.T) {
	own := &fakeYTTokens{}
	mod := &fakeModTokens{cred: ytModCred(models.ScopeYouTubeModeration)}
	api := &fakeYTAPI{results: []error{clients.ErrYouTubeForbidden}}
	q := &fakeQuota{reserveOK: true}
	d := delegatedYT(t, own, mod, api, q)

	res, err := d.Dispatch(context.Background(), moderator(modUserID, ownerUserID), models.ActionBan, banReq())

	require.NoError(t, err)
	assert.Equal(t, models.DispatchNotPlatformModerator, res.Outcome)
	assert.Empty(t, res.MissingScopes)
	assert.Equal(t, modUserID, res.CredentialUserID, "the attribution survives a refusal")
	assert.Equal(t, 1, q.rollbacks, "the reservation is released")
}

// The owner keeps the re-consent answer on a 403: a broadcaster moderates their own chat, so there
// the likely cause really is the grant.
func TestYouTubeOwner_ForbiddenStaysAReauthPrompt(t *testing.T) {
	own := &fakeYTTokens{cred: ytCredWith(models.ScopeYouTubeModeration)}
	api := &fakeYTAPI{results: []error{clients.ErrYouTubeForbidden}}
	q := &fakeQuota{reserveOK: true}
	d := newYT(own, api, fakeLiveChat{id: "lc"}, q)

	res, err := d.Dispatch(context.Background(), owner("u1"), models.ActionBan, banReq())

	require.NoError(t, err)
	assert.Equal(t, models.DispatchReauthRequired, res.Outcome)
	assert.Equal(t, []string{models.ScopeYouTubeModeration}, res.MissingScopes)
}

// A moderator's timeout must carry the duration and run on their token — a dropped duration would
// turn a volunteer's timeout into a permanent ban.
func TestYouTubeDelegated_TimeoutKeepsItsDuration(t *testing.T) {
	own := &fakeYTTokens{}
	mod := &fakeModTokens{cred: ytModCred(models.ScopeYouTubeModeration)}
	api, q := &fakeYTAPI{}, &fakeQuota{reserveOK: true}
	d := delegatedYT(t, own, mod, api, q)

	req := banReq()
	req.DurationSeconds = 600
	res, err := d.Dispatch(context.Background(), moderator(modUserID, ownerUserID), models.ActionTimeout, req)

	require.NoError(t, err)
	assert.Equal(t, models.DispatchPerformed, res.Outcome)
	assert.Equal(t, "timeout", api.method)
	assert.Equal(t, 600, api.duration)
	assert.Equal(t, []string{"yt-mod-tok"}, api.tokensSeen)
}

// The refresh must write back onto the resolved call: the retry reads the token from there.
func TestYouTubeDelegated_RefreshWritesBackOntoTheCall(t *testing.T) {
	own := &fakeYTTokens{}
	mod := &fakeModTokens{
		cred:      ytModCred(models.ScopeYouTubeModeration),
		onRefresh: func(c *tokens.ModCredential) { c.AccessToken = "yt-mod-tok2" },
	}
	api := &fakeYTAPI{results: []error{clients.ErrYouTubeUnauthorized, nil}}
	q := &fakeQuota{reserveOK: true}
	d := delegatedYT(t, own, mod, api, q)

	res, err := d.Dispatch(context.Background(), moderator(modUserID, ownerUserID), models.ActionBan, banReq())

	require.NoError(t, err)
	assert.Equal(t, models.DispatchPerformed, res.Outcome)
	assert.Equal(t, 1, mod.refreshes)
	assert.Equal(t, []string{"yt-mod-tok", "yt-mod-tok2"}, api.tokensSeen)
	assert.Equal(t, 1, q.reserves, "one reservation covers the attempt and its single retry")
	assert.Equal(t, 1, q.confirms)
}

// Without a moderator credential store the delegated path must refuse rather than fall through to
// the owner's token — the refusal is the invariant, pinned even now that production wires the store.
func TestYouTubeDelegated_RefusedWithoutAModSource(t *testing.T) {
	own := &fakeYTTokens{cred: ytCredWith(models.ScopeYouTubeModeration)}
	api, q := &fakeYTAPI{}, &fakeQuota{reserveOK: true}
	d := newYT(own, api, fakeLiveChat{id: "lc"}, q)

	res, err := d.Dispatch(context.Background(), moderator(modUserID, ownerUserID), models.ActionBan, banReq())

	require.NoError(t, err)
	assert.Equal(t, models.DispatchDelegationUnsupported, res.Outcome)
	assert.Zero(t, api.calls)
	assert.Zero(t, own.resolves)
	assert.Zero(t, q.reserves)
}
