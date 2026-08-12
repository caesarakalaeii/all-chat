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

// The delegated Kick write path (ADR-0048).
//
// Same invariant as Twitch — the moderator's own credential against the owner's channel, with no
// fallback — but Kick raises the stakes on the second half. Its endpoints carry no moderator
// field, so `broadcaster_user_id` is the ONLY id in the request: get it from the acting user
// instead of the anchor and the call silently moderates the moderator's own channel.

// kickModCred builds a moderator credential whose Kick id is deliberately different from any
// broadcaster id used here, so a swapped id cannot pass unnoticed.
func kickModCred(scopes ...string) *tokens.ModCredential {
	return &tokens.ModCredential{
		AccessToken:    "kmod-tok",
		RefreshToken:   "kmod-ref",
		PlatformUserID: "9001",
		GrantedScopes:  scopes,
		ExpiresAt:      time.Now().Add(time.Hour), // far future: no proactive refresh
	}
}

func delegatedKick(t *testing.T, own *fakeKickTokens, mod *fakeModTokens, api *fakeKickAPI) *Kick {
	t.Helper()
	d := NewKick(own, api, zap.NewNop())
	if mod != nil {
		d.SetModSource(mod)
	}
	return d
}

func kickBanReq() models.DispatchRequest {
	return models.DispatchRequest{Platform: "kick", ChannelID: "kickstreamer", TargetUserID: "42"}
}

// The whole point: the moderator's token, the owner's channel id from the anchor, and the
// moderator's own Kick account recorded as the actor.
func TestKickDelegated_UsesTheModeratorsOwnCredentialAgainstTheOwnersChannel(t *testing.T) {
	own := &fakeKickTokens{anchor: "555", cred: kickCredWith(models.ScopeKickModeration)}
	mod := &fakeModTokens{cred: kickModCred(models.ScopeKickModeration)}
	api := &fakeKickAPI{}
	d := delegatedKick(t, own, mod, api)

	res, err := d.Dispatch(context.Background(), moderator(modUserID, ownerUserID), models.ActionBan, kickBanReq())

	require.NoError(t, err)
	assert.Equal(t, models.DispatchPerformed, res.Outcome)
	assert.Equal(t, []string{"kmod-tok"}, api.tokensSeen, "the moderator's own token performs the call")
	assert.Equal(t, "555", api.broadcaster, "broadcaster_user_id is the owner's, from the anchor")
	assert.Equal(t, []string{ownerUserID + "|kickstreamer"}, own.anchorFor,
		"the anchor is asked about the OWNER, never the caller")
	assert.Equal(t, modUserID, res.CredentialUserID, "the audit proof of whose token acted")
	assert.Equal(t, "9001", res.PlatformActorID, "the Kick account Kick saw acting")
}

// The owner's credential must not be touched on a delegated action — not as a fallback, not as a
// source of the token, not at all. Only the anchor may consult the owner, and it reads no token.
func TestKickDelegated_NeverReachesForTheOwnersCredential(t *testing.T) {
	own := &fakeKickTokens{anchor: "555", cred: kickCredWith(models.ScopeKickModeration)}
	mod := &fakeModTokens{resolveErr: tokens.ErrNoCredential}
	api := &fakeKickAPI{}
	d := delegatedKick(t, own, mod, api)

	res, err := d.Dispatch(context.Background(), moderator(modUserID, ownerUserID), models.ActionBan, kickBanReq())

	require.NoError(t, err)
	assert.Equal(t, models.DispatchNoCredential, res.Outcome,
		"no consent yet is the normal state of a fresh grant, not a fallback trigger")
	assert.Zero(t, api.calls, "nothing may be performed on the owner's behalf")
	assert.Zero(t, own.resolves, "the owner's token store must not even be consulted")
}

// Delegation never exceeds what the owner could do themselves. An owner who cannot be shown to
// control the channel has nothing to delegate on it, and only they can fix that.
func TestKickDelegated_OwnerWithoutAnAnchorIsRefused(t *testing.T) {
	own := &fakeKickTokens{anchorErr: tokens.ErrOwnerChannelUnverified}
	mod := &fakeModTokens{cred: kickModCred(models.ScopeKickModeration)}
	api := &fakeKickAPI{}
	d := delegatedKick(t, own, mod, api)

	res, err := d.Dispatch(context.Background(), moderator(modUserID, ownerUserID), models.ActionBan, kickBanReq())

	require.NoError(t, err)
	assert.Equal(t, models.DispatchOwnerUnverified, res.Outcome)
	assert.Zero(t, api.calls)
	assert.Empty(t, mod.resolvedFor, "the anchor is checked before the moderator's credential")
}

// The scope pre-check reads the MODERATOR's scopes. Checking the owner's would let a streamer's
// broad grant stand in for consent the volunteer never gave.
func TestKickDelegated_ScopePreCheckUsesTheModeratorsScopes(t *testing.T) {
	own := &fakeKickTokens{anchor: "555", cred: kickCredWith(models.ScopeKickModeration)}
	mod := &fakeModTokens{cred: kickModCred("user:read")} // consented to identity only
	api := &fakeKickAPI{}
	d := delegatedKick(t, own, mod, api)

	res, err := d.Dispatch(context.Background(), moderator(modUserID, ownerUserID), models.ActionBan, kickBanReq())

	require.NoError(t, err)
	assert.Equal(t, models.DispatchReauthRequired, res.Outcome)
	assert.Equal(t, []string{models.ScopeKickModeration}, res.MissingScopes)
	assert.Zero(t, api.calls)
}

// Kick's two moderation scopes are independent, so a moderator who consented for ban alone cannot
// delete — even where the streamer delegated delete.
func TestKickDelegated_DeleteNeedsTheModeratorsMessageScope(t *testing.T) {
	own := &fakeKickTokens{anchor: "555"}
	mod := &fakeModTokens{cred: kickModCred(models.ScopeKickModeration)}
	api := &fakeKickAPI{}
	d := delegatedKick(t, own, mod, api)

	res, err := d.Dispatch(context.Background(), moderator(modUserID, ownerUserID), models.ActionDelete,
		models.DispatchRequest{Platform: "kick", ChannelID: "kickstreamer", NativeMessageID: "kick-msg-1"})

	require.NoError(t, err)
	assert.Equal(t, models.DispatchReauthRequired, res.Outcome)
	assert.Equal(t, []string{models.ScopeKickChatMessageManage}, res.MissingScopes)
	assert.Zero(t, api.calls)
}

// Delete carries no broadcaster id, so the anchor's VALUE is unused — but the anchor must still
// gate the action, because "does the owner control this channel at all?" is exactly as necessary
// for a delete as for a ban.
func TestKickDelegated_DeleteStillRequiresTheAnchor(t *testing.T) {
	own := &fakeKickTokens{anchorErr: tokens.ErrOwnerChannelUnverified}
	mod := &fakeModTokens{cred: kickModCred(models.ScopeKickChatMessageManage)}
	api := &fakeKickAPI{}
	d := delegatedKick(t, own, mod, api)

	res, err := d.Dispatch(context.Background(), moderator(modUserID, ownerUserID), models.ActionDelete,
		models.DispatchRequest{Platform: "kick", ChannelID: "kickstreamer", NativeMessageID: "kick-msg-1"})

	require.NoError(t, err)
	assert.Equal(t, models.DispatchOwnerUnverified, res.Outcome)
	assert.Zero(t, api.calls)
}

func TestKickDelegated_DeletePerformedOnTheModeratorsToken(t *testing.T) {
	own := &fakeKickTokens{anchor: "555"}
	mod := &fakeModTokens{cred: kickModCred(models.ScopeKickChatMessageManage)}
	api := &fakeKickAPI{}
	d := delegatedKick(t, own, mod, api)

	res, err := d.Dispatch(context.Background(), moderator(modUserID, ownerUserID), models.ActionDelete,
		models.DispatchRequest{Platform: "kick", ChannelID: "kickstreamer", NativeMessageID: "kick-msg-1"})

	require.NoError(t, err)
	assert.Equal(t, models.DispatchPerformed, res.Outcome)
	assert.Equal(t, "delete", api.method)
	assert.Equal(t, "kick-msg-1", api.messageID)
	assert.Equal(t, []string{"kmod-tok"}, api.tokensSeen)
}

// A 403 after a passing scope pre-check is Kick answering the question the design defers to it:
// this person does not moderate this channel. Sending them to a consent screen would loop a
// volunteer through a flow that cannot fix it — the streamer has to mod them on Kick.
func TestKickDelegated_ForbiddenReportsNotAPlatformModerator(t *testing.T) {
	own := &fakeKickTokens{anchor: "555"}
	mod := &fakeModTokens{cred: kickModCred(models.ScopeKickModeration)}
	api := &fakeKickAPI{results: []error{clients.ErrKickForbidden}}
	d := delegatedKick(t, own, mod, api)

	res, err := d.Dispatch(context.Background(), moderator(modUserID, ownerUserID), models.ActionBan, kickBanReq())

	require.NoError(t, err)
	assert.Equal(t, models.DispatchNotPlatformModerator, res.Outcome)
	assert.Empty(t, res.MissingScopes, "naming a scope here would send them somewhere that cannot help")
	assert.Equal(t, modUserID, res.CredentialUserID, "the attribution survives a refusal")
}

// Insurance against Kick's undocumented 401-vs-403 mapping (ADR-0048's empirical unknown): a 401
// that survives a SUCCESSFUL refresh cannot be about token validity — the token is seconds old and
// its scope was pre-checked — so on the delegated path it means the same thing a 403 does.
func TestKickDelegated_UnauthorizedAfterASuccessfulRefreshReportsNotAPlatformModerator(t *testing.T) {
	own := &fakeKickTokens{anchor: "555"}
	mod := &fakeModTokens{
		cred:      kickModCred(models.ScopeKickModeration),
		onRefresh: func(c *tokens.ModCredential) { c.AccessToken = "kmod-tok2" },
	}
	api := &fakeKickAPI{results: []error{clients.ErrKickUnauthorized, clients.ErrKickUnauthorized}}
	d := delegatedKick(t, own, mod, api)

	res, err := d.Dispatch(context.Background(), moderator(modUserID, ownerUserID), models.ActionBan, kickBanReq())

	require.NoError(t, err)
	assert.Equal(t, models.DispatchNotPlatformModerator, res.Outcome)
	assert.Equal(t, 1, mod.refreshes, "exactly one reactive refresh, then the conclusion")
	assert.Equal(t, []string{"kmod-tok", "kmod-tok2"}, api.tokensSeen, "the retry used the fresh token")
}

// ...but a 401 whose refresh FAILED says nothing about moderator status: the token really is
// unusable, and reconnecting is the fix.
func TestKickDelegated_UnauthorizedWithAFailedRefreshIsReauth(t *testing.T) {
	own := &fakeKickTokens{anchor: "555"}
	mod := &fakeModTokens{
		cred:       kickModCred(models.ScopeKickModeration),
		refreshErr: errors.New("kick refused the refresh grant"),
	}
	api := &fakeKickAPI{results: []error{clients.ErrKickUnauthorized}}
	d := delegatedKick(t, own, mod, api)

	res, err := d.Dispatch(context.Background(), moderator(modUserID, ownerUserID), models.ActionBan, kickBanReq())

	require.NoError(t, err)
	assert.Equal(t, models.DispatchReauthRequired, res.Outcome)
	assert.Equal(t, "token refresh failed", res.PlatformStatus)
}

// The refresh must write back onto the resolved call, not just the credential: the retry reads the
// token from the call, so a snapshot taken at construction would retry with the same dead token.
func TestKickDelegated_RefreshWritesBackOntoTheCall(t *testing.T) {
	own := &fakeKickTokens{anchor: "555"}
	mod := &fakeModTokens{
		cred:      kickModCred(models.ScopeKickModeration),
		onRefresh: func(c *tokens.ModCredential) { c.AccessToken = "kmod-tok2" },
	}
	api := &fakeKickAPI{results: []error{clients.ErrKickUnauthorized, nil}}
	d := delegatedKick(t, own, mod, api)

	res, err := d.Dispatch(context.Background(), moderator(modUserID, ownerUserID), models.ActionBan, kickBanReq())

	require.NoError(t, err)
	assert.Equal(t, models.DispatchPerformed, res.Outcome)
	assert.Equal(t, []string{"kmod-tok", "kmod-tok2"}, api.tokensSeen)
}

// An expiring moderator credential is refreshed before the first attempt, so a foreseeable expiry
// does not cost the volunteer a failed action.
func TestKickDelegated_ProactiveRefreshNearExpiry(t *testing.T) {
	cred := kickModCred(models.ScopeKickModeration)
	cred.ExpiresAt = time.Now().Add(30 * time.Second) // inside refreshLeadTime
	own := &fakeKickTokens{anchor: "555"}
	mod := &fakeModTokens{
		cred:      cred,
		onRefresh: func(c *tokens.ModCredential) { c.AccessToken = "kmod-tok2" },
	}
	api := &fakeKickAPI{}
	d := delegatedKick(t, own, mod, api)

	res, err := d.Dispatch(context.Background(), moderator(modUserID, ownerUserID), models.ActionBan, kickBanReq())

	require.NoError(t, err)
	assert.Equal(t, models.DispatchPerformed, res.Outcome)
	assert.Equal(t, 1, mod.refreshes)
	assert.Equal(t, []string{"kmod-tok2"}, api.tokensSeen, "the first attempt already uses the fresh token")
}

// Timeout and unban travel the same resolved call, so the anchor's broadcaster id must reach them
// too — a regression here would ban on the wrong channel rather than fail loudly.
func TestKickDelegated_TimeoutAndUnbanUseTheAnchoredChannel(t *testing.T) {
	for _, tc := range []struct {
		action models.Action
		method string
	}{
		{models.ActionTimeout, "timeout"},
		{models.ActionUnban, "unban"},
	} {
		own := &fakeKickTokens{anchor: "555"}
		mod := &fakeModTokens{cred: kickModCred(models.ScopeKickModeration)}
		api := &fakeKickAPI{}
		d := delegatedKick(t, own, mod, api)

		req := kickBanReq()
		req.DurationSeconds = 600
		res, err := d.Dispatch(context.Background(), moderator(modUserID, ownerUserID), tc.action, req)

		require.NoError(t, err)
		assert.Equal(t, models.DispatchPerformed, res.Outcome)
		assert.Equal(t, tc.method, api.method)
		assert.Equal(t, "555", api.broadcaster, "%s must act on the owner's channel", tc.action)
		assert.Equal(t, []string{"kmod-tok"}, api.tokensSeen)
	}
}
