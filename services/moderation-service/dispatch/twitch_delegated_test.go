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

// The delegated Twitch write path (ADR-0048).
//
// One invariant dominates every test here: a delegated action runs on the MODERATOR's own
// credential against the OWNER's channel, with **no fallback** between them. If the moderator has
// not consented, the action fails — it must never quietly run as the streamer, which would put the
// streamer's name in their own mod log for something they did not do and would make the platform's
// per-call moderator check meaningless.

const (
	modUserID   = "mod-user"
	ownerUserID = "owner-user"
)

// modCredWith builds a moderator credential whose platform id is deliberately different from any
// broadcaster id used in these tests.
func modCredWith(scopes ...string) *tokens.ModCredential {
	return &tokens.ModCredential{
		AccessToken:    "mod-tok",
		RefreshToken:   "mod-ref",
		PlatformUserID: "777",
		GrantedScopes:  scopes,
		ExpiresAt:      time.Now().Add(time.Hour), // far future: no proactive refresh
	}
}

// delegatedTwitch wires a dispatcher with both credential stores.
func delegatedTwitch(t *testing.T, own *fakeTokens, mod *fakeModTokens, api *fakeAPI) *Twitch {
	t.Helper()
	d := NewTwitch(own, api, zap.NewNop())
	if mod != nil {
		d.SetModSource(mod)
	}
	return d
}

func deleteReq() models.DispatchRequest {
	return models.DispatchRequest{Platform: "twitch", ChannelID: "somestreamer", NativeMessageID: "m1"}
}

// The whole point: the moderator's token, the owner's channel, the moderator's id as moderator_id.
func TestDelegated_UsesTheModeratorsOwnCredentialAgainstTheOwnersChannel(t *testing.T) {
	own := &fakeTokens{anchor: "9001", cred: credWith(models.ScopeTwitchManageMessages)}
	mod := &fakeModTokens{cred: modCredWith(models.ScopeTwitchManageMessages)}
	api := &fakeAPI{}
	d := delegatedTwitch(t, own, mod, api)

	res, err := d.Dispatch(context.Background(), moderator(modUserID, ownerUserID), models.ActionDelete, deleteReq())

	require.NoError(t, err)
	assert.Equal(t, models.DispatchPerformed, res.Outcome)
	assert.Equal(t, []string{"mod-tok"}, api.tokensSeen, "the moderator's own token performs the call")
	assert.Equal(t, "9001", api.broadcaster, "the channel is the owner's, from the anchor")
	assert.Equal(t, "777", api.moderator, "moderator_id is the moderator, never the broadcaster")
	assert.Equal(t, []string{modUserID}, mod.resolvedFor)
}

// The owner's credential must not be touched on a delegated action — not as a fallback, not as a
// source of the token, not at all. Only the anchor may consult the owner, and it reads no token.
func TestDelegated_NeverReachesForTheOwnersCredential(t *testing.T) {
	// The owner holds a perfectly good credential; if any code path preferred it, this passes
	// where it must fail.
	own := &fakeTokens{anchor: "9001", cred: credWith(models.ScopeTwitchManageMessages)}
	mod := &fakeModTokens{resolveErr: tokens.ErrNoCredential}
	api := &fakeAPI{}
	d := delegatedTwitch(t, own, mod, api)

	res, err := d.Dispatch(context.Background(), moderator(modUserID, ownerUserID), models.ActionDelete, deleteReq())

	require.NoError(t, err)
	assert.Equal(t, models.DispatchNoCredential, res.Outcome,
		"an unconsented moderator fails; it must not fall back to the streamer's token")
	assert.Zero(t, api.calls, "nothing may be sent to Helix")
}

// The anchor is asked about the OWNER, not the caller. Asking about the moderator would let anyone
// with a grant act on any channel they happen to hold a credential for.
func TestDelegated_AnchorIsResolvedAgainstTheOwner(t *testing.T) {
	own := &fakeTokens{anchor: "9001"}
	mod := &fakeModTokens{cred: modCredWith(models.ScopeTwitchManageMessages)}
	d := delegatedTwitch(t, own, mod, &fakeAPI{})

	_, err := d.Dispatch(context.Background(), moderator(modUserID, ownerUserID), models.ActionDelete, deleteReq())

	require.NoError(t, err)
	assert.Equal(t, []string{ownerUserID + "|somestreamer"}, own.anchorFor)
}

// Delegation never exceeds what the owner could do themselves: if the owner cannot be shown to
// control the channel, there is nothing on it to delegate.
func TestDelegated_OwnerWithoutAnchorBlocksTheAction(t *testing.T) {
	own := &fakeTokens{anchorErr: tokens.ErrOwnerChannelUnverified}
	mod := &fakeModTokens{cred: modCredWith(models.ScopeTwitchManageMessages)}
	api := &fakeAPI{}
	d := delegatedTwitch(t, own, mod, api)

	res, err := d.Dispatch(context.Background(), moderator(modUserID, ownerUserID), models.ActionDelete, deleteReq())

	require.NoError(t, err)
	assert.Equal(t, models.DispatchOwnerUnverified, res.Outcome)
	assert.Zero(t, api.calls)
	assert.Empty(t, mod.resolvedFor, "the anchor fails first; no credential is even read")
}

// The scope pre-check reads the MODERATOR's scopes. Reading the owner's would advertise actions
// the moderator never consented to and burn a Helix call to discover it.
func TestDelegated_ScopePreCheckReadsTheModeratorsScopes(t *testing.T) {
	// The owner holds the delete scope; the moderator does not.
	own := &fakeTokens{anchor: "9001", cred: credWith(models.ScopeTwitchManageMessages)}
	mod := &fakeModTokens{cred: modCredWith(models.ScopeTwitchManageBannedUsers)}
	api := &fakeAPI{}
	d := delegatedTwitch(t, own, mod, api)

	res, err := d.Dispatch(context.Background(), moderator(modUserID, ownerUserID), models.ActionDelete, deleteReq())

	require.NoError(t, err)
	assert.Equal(t, models.DispatchReauthRequired, res.Outcome)
	assert.Equal(t, []string{models.ScopeTwitchManageMessages}, res.MissingScopes)
	assert.Zero(t, api.calls)
}

// A 401 refreshes the MODERATOR's credential and retries with the new token — the same recovery
// the owner path gets, against the moderator's own row.
func TestDelegated_UnauthorizedRefreshesTheModeratorsTokenAndRetries(t *testing.T) {
	own := &fakeTokens{anchor: "9001"}
	mod := &fakeModTokens{
		cred:      modCredWith(models.ScopeTwitchManageMessages),
		onRefresh: func(c *tokens.ModCredential) { c.AccessToken = "mod-tok2" },
	}
	api := &fakeAPI{results: []error{clients.ErrUnauthorized, nil}}
	d := delegatedTwitch(t, own, mod, api)

	res, err := d.Dispatch(context.Background(), moderator(modUserID, ownerUserID), models.ActionDelete, deleteReq())

	require.NoError(t, err)
	assert.Equal(t, models.DispatchPerformed, res.Outcome)
	assert.Equal(t, 1, mod.refreshes)
	assert.Equal(t, []string{"mod-tok", "mod-tok2"}, api.tokensSeen, "the retry uses the refreshed token")
}

// The proof columns are what an auditor reads to confirm no-fallback held. They must name the
// moderator, never the owner.
func TestDelegated_ReportsWhoseCredentialActed(t *testing.T) {
	own := &fakeTokens{anchor: "9001"}
	mod := &fakeModTokens{cred: modCredWith(models.ScopeTwitchManageMessages)}
	d := delegatedTwitch(t, own, mod, &fakeAPI{})

	res, err := d.Dispatch(context.Background(), moderator(modUserID, ownerUserID), models.ActionDelete, deleteReq())

	require.NoError(t, err)
	assert.Equal(t, modUserID, res.CredentialUserID)
	assert.NotEqual(t, ownerUserID, res.CredentialUserID)
	assert.Equal(t, "777", res.PlatformActorID)
}

// An owner action reports its own attribution too, and the ids coincide legitimately there.
func TestOwnerAction_ReportsItsOwnCredential(t *testing.T) {
	own := &fakeTokens{cred: credWith(models.ScopeTwitchManageMessages)}
	d := delegatedTwitch(t, own, &fakeModTokens{}, &fakeAPI{})

	res, err := d.Dispatch(context.Background(), owner("u1"), models.ActionDelete, deleteReq())

	require.NoError(t, err)
	assert.Equal(t, "u1", res.CredentialUserID)
	assert.Equal(t, "9001", res.PlatformActorID, "the streamer is their own moderator")
	assert.Empty(t, own.anchorFor, "an owner action needs no anchor — ownership is the anchor")
}

// A deployment without the moderator credential store refuses delegated actions rather than
// falling through to whatever the owner path would have done.
func TestDelegated_WithoutAModSourceIsRefused(t *testing.T) {
	own := &fakeTokens{anchor: "9001", cred: credWith(models.ScopeTwitchManageMessages)}
	api := &fakeAPI{}
	d := delegatedTwitch(t, own, nil, api)

	res, err := d.Dispatch(context.Background(), moderator(modUserID, ownerUserID), models.ActionDelete, deleteReq())

	require.NoError(t, err)
	assert.Equal(t, models.DispatchDelegationUnsupported, res.Outcome)
	assert.Zero(t, api.calls)
}

// An anchor lookup that errors for an unexpected reason is an error, not a silent allow.
func TestDelegated_AnchorErrorIsNotAnAllow(t *testing.T) {
	own := &fakeTokens{anchorErr: errors.New("database unavailable")}
	mod := &fakeModTokens{cred: modCredWith(models.ScopeTwitchManageMessages)}
	api := &fakeAPI{}
	d := delegatedTwitch(t, own, mod, api)

	_, err := d.Dispatch(context.Background(), moderator(modUserID, ownerUserID), models.ActionDelete, deleteReq())

	require.Error(t, err)
	assert.Zero(t, api.calls)
}

// Every delegatable action carries the split, not just delete.
func TestDelegated_AllActionsCarryTheModeratorID(t *testing.T) {
	cases := []struct {
		action models.Action
		req    models.DispatchRequest
	}{
		{models.ActionDelete, models.DispatchRequest{Platform: "twitch", ChannelID: "c", NativeMessageID: "m1"}},
		{models.ActionTimeout, models.DispatchRequest{Platform: "twitch", ChannelID: "c", TargetUserID: "42", DurationSeconds: 60}},
		{models.ActionBan, models.DispatchRequest{Platform: "twitch", ChannelID: "c", TargetUserID: "42"}},
		{models.ActionUnban, models.DispatchRequest{Platform: "twitch", ChannelID: "c", TargetUserID: "42"}},
	}

	for _, tc := range cases {
		t.Run(string(tc.action), func(t *testing.T) {
			own := &fakeTokens{anchor: "9001"}
			mod := &fakeModTokens{cred: modCredWith(
				models.ScopeTwitchManageMessages, models.ScopeTwitchManageBannedUsers)}
			api := &fakeAPI{}
			d := delegatedTwitch(t, own, mod, api)

			res, err := d.Dispatch(context.Background(), moderator(modUserID, ownerUserID), tc.action, tc.req)

			require.NoError(t, err)
			assert.Equal(t, models.DispatchPerformed, res.Outcome)
			assert.Equal(t, "9001", api.broadcaster)
			assert.Equal(t, "777", api.moderator)
		})
	}
}
