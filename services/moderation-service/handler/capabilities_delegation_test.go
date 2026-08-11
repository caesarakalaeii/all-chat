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
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/caesar/all-chat/services/moderation-service/models"
	"github.com/caesar/all-chat/services/moderation-service/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// Capabilities for a DELEGATED MODERATOR (ADR-0048).
//
// The moderator's answer is computed from a different authority than the owner's: what the
// streamer delegated, intersected with the scopes on the moderator's OWN credential. These tests
// pin that the two never get crossed — a moderator must never inherit the streamer's scopes, and
// the streamer's grant must never be widened by the moderator's consent.

const capsPath = "/api/v1/moderation/overlays/" + overlayID + "/capabilities"

// twitchModScopes is a moderator credential granting the full Twitch moderation set.
var twitchModScopes = []string{
	models.ScopeTwitchManageMessages, models.ScopeTwitchManageBannedUsers,
}

func modCapsHandler(t *testing.T, auth *fakeAuthorizer) *Handler {
	t.Helper()
	// fakeScopes stands in for the OWNER's broadcaster scopes and deliberately reports the full
	// set: any of it leaking into a moderator's answer is the bug these tests exist to catch.
	owner := fakeScopes{actions: []models.Action{
		models.ActionDelete, models.ActionTimeout, models.ActionBan, models.ActionUnban,
	}}
	return New(auth, &fakeEmitter{}, &fakeRecorder{}, owner, DryRunDispatcher{}, zap.NewNop())
}

func modCaps(t *testing.T, auth *fakeAuthorizer) models.Capabilities {
	t.Helper()
	resp := do(newTestRouter(modCapsHandler(t, auth), modUserID, ""), http.MethodGet, capsPath, "")
	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())

	var caps models.Capabilities
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &caps))
	return caps
}

func moderatorAccess(actions, platforms []string) *repository.OverlayAccess {
	return &repository.OverlayAccess{
		OwnerUserID:    ownerID,
		OwnerIsPremium: true,
		Role:           repository.RoleModerator,
		GrantID:        "grant-1",
		Actions:        actions,
		Platforms:      platforms,
	}
}

func twitchSource() []repository.Source {
	return []repository.Source{{Platform: "twitch", ChannelID: "somestreamer", ChannelName: "SomeStreamer"}}
}

// A moderator who has consented sees exactly the intersection of their own scopes and the
// streamer's grant — never more.
func TestCapabilities_ModeratorSeesDelegatedIntersection(t *testing.T) {
	auth := &fakeAuthorizer{
		access:    moderatorAccess([]string{"delete", "timeout"}, []string{"twitch"}),
		sources:   twitchSource(),
		modScopes: map[string][]string{"twitch": twitchModScopes},
	}

	caps := modCaps(t, auth)

	assert.Equal(t, repository.RoleModerator, caps.Role)
	assert.False(t, caps.IsOwner, "a moderator is not an owner and the UI branches on this")
	assert.True(t, caps.CanModerate)
	require.Len(t, caps.Sources, 1)
	assert.True(t, caps.Sources[0].Moderatable)
	assert.ElementsMatch(t,
		[]models.Action{models.ActionDelete, models.ActionTimeout}, caps.Sources[0].Actions,
		"the moderator's own scopes allow ban/unban too, but the streamer did not delegate them")
}

// The owner's broadcaster scopes must never reach a moderator's answer. If they did, a moderator
// would be shown controls backed by the streamer's credential — the exact fallback ADR-0048 exists
// to forbid.
func TestCapabilities_ModeratorDoesNotInheritOwnerScopes(t *testing.T) {
	auth := &fakeAuthorizer{
		access:  moderatorAccess([]string{"delete", "timeout", "ban", "unban"}, []string{"twitch"}),
		sources: twitchSource(),
		// No moderator credential at all, while the owner (fakeScopes) holds everything.
		modScopes: map[string][]string{},
	}

	caps := modCaps(t, auth)

	require.Len(t, caps.Sources, 1)
	assert.False(t, caps.Sources[0].Moderatable)
	assert.Equal(t, models.ReasonNeedsConsent, caps.Sources[0].Reason)
	assert.Empty(t, caps.Sources[0].Actions)
}

// A platform the streamer did not enable reads as "ask the streamer", not "connect your account":
// only the streamer can clear it, so pointing the moderator at a consent screen would be a dead
// end.
func TestCapabilities_PlatformNotDelegatedReadsAsNotDelegated(t *testing.T) {
	auth := &fakeAuthorizer{
		access:    moderatorAccess([]string{"delete"}, []string{"kick"}),
		sources:   twitchSource(),
		modScopes: map[string][]string{"twitch": twitchModScopes},
	}

	caps := modCaps(t, auth)

	require.Len(t, caps.Sources, 1)
	assert.False(t, caps.Sources[0].Moderatable)
	assert.Equal(t, models.ReasonNotDelegated, caps.Sources[0].Reason)
}

// Delegating only actions the platform cannot perform is the same dead end as not delegating the
// platform at all, and must read the same way.
func TestCapabilities_ActionsUnsupportedByPlatformReadAsNotDelegated(t *testing.T) {
	auth := &fakeAuthorizer{
		// Kick has no single-message delete, so a delete-only grant delegates nothing there.
		access:    moderatorAccess([]string{"delete"}, []string{"kick"}),
		sources:   []repository.Source{{Platform: "kick", ChannelID: "kickstreamer", ChannelName: "KickStreamer"}},
		modScopes: map[string][]string{"kick": {models.ScopeKickModeration}},
	}

	caps := modCaps(t, auth)

	require.Len(t, caps.Sources, 1)
	assert.Equal(t, models.ReasonNotDelegated, caps.Sources[0].Reason)
}

// Consent granted for a narrower set than the streamer delegated still reads as "connect": the
// moderator may have consented for another streamer who delegated less, and re-running consent is
// the fix they can actually perform.
func TestCapabilities_ConsentNarrowerThanTheGrantReadsAsNeedsConsent(t *testing.T) {
	auth := &fakeAuthorizer{
		access:  moderatorAccess([]string{"ban", "unban"}, []string{"twitch"}),
		sources: twitchSource(),
		// Delete-only consent: nothing here overlaps ban/unban.
		modScopes: map[string][]string{"twitch": {models.ScopeTwitchManageMessages}},
	}

	caps := modCaps(t, auth)

	require.Len(t, caps.Sources, 1)
	assert.False(t, caps.Sources[0].Moderatable)
	assert.Equal(t, models.ReasonNeedsConsent, caps.Sources[0].Reason)
}

func discordSource() []repository.Source {
	return []repository.Source{{Platform: "discord", ChannelID: "123", ChannelName: "#general"}}
}

// Discord has no per-user moderation API, so what a moderator must supply is not a consent but an
// identity: the shared bot writes, and All-Chat checks their own server permissions. Saying
// "connect your account" would point them at an OAuth flow that grants nothing usable.
func TestCapabilities_DiscordReadsAsNeedsLinkNotConsent(t *testing.T) {
	auth := &fakeAuthorizer{
		access:    moderatorAccess([]string{"delete"}, []string{"discord"}),
		sources:   discordSource(),
		modScopes: map[string][]string{},
	}

	caps := modCaps(t, auth)

	require.Len(t, caps.Sources, 1)
	assert.Equal(t, models.ReasonNeedsDiscordLink, caps.Sources[0].Reason)
}

// The same missing thing — a Discord account link — has two subjects, and only one of them is the
// person reading this. A moderator told to link their account when it is the STREAMER who has not
// would link it and see nothing change.
func TestCapabilities_UnlinkedOwnerIsNotTheModeratorsProblemToFix(t *testing.T) {
	auth := &fakeAuthorizer{
		access:            moderatorAccess([]string{"delete"}, []string{"discord"}),
		sources:           discordSource(),
		discordIdentities: map[string]string{modUserID: "200"}, // the moderator linked; the owner did not
	}

	caps := modCaps(t, auth)

	require.Len(t, caps.Sources, 1)
	assert.Equal(t, models.ReasonOwnerChannelUnverified, caps.Sources[0].Reason)
	assert.False(t, caps.Sources[0].Moderatable)
}

// With both linked, the answer is what the BOT can do in that guild narrowed to what the streamer
// delegated. The moderator's own server permissions are checked at action time, not here — exactly
// as on Twitch, where capabilities checks the scope and the platform decides the rest.
func TestCapabilities_LinkedDiscordReportsTheDelegatedBotIntersection(t *testing.T) {
	auth := &fakeAuthorizer{
		access:            moderatorAccess([]string{"delete", "ban"}, []string{"discord"}),
		sources:           discordSource(),
		discordIdentities: map[string]string{modUserID: "200", ownerID: "100"},
	}

	caps := modCaps(t, auth)

	require.Len(t, caps.Sources, 1)
	assert.True(t, caps.Sources[0].Moderatable)
	assert.Empty(t, caps.Sources[0].Reason)
	// modCapsHandler's scope checker reports the full set as the bot's permissions, so the grant
	// is the only narrowing left.
	assert.ElementsMatch(t, []models.Action{models.ActionDelete, models.ActionBan}, caps.Sources[0].Actions)
}

// A bot invited without the permissions the grant covers cannot lend them to anyone, and only the
// streamer can fix it — by re-inviting the bot, never by any re-consent.
func TestCapabilities_DiscordBotWithoutThePermissionsReadsAsReinvite(t *testing.T) {
	auth := &fakeAuthorizer{
		access:            moderatorAccess([]string{"ban"}, []string{"discord"}),
		sources:           discordSource(),
		discordIdentities: map[string]string{modUserID: "200", ownerID: "100"},
	}
	// A bot holding only MANAGE_MESSAGES: it can delete, and the grant covers only ban.
	h := New(auth, &fakeEmitter{}, &fakeRecorder{}, fakeScopes{actions: []models.Action{models.ActionDelete}},
		DryRunDispatcher{}, zap.NewNop())

	resp := do(newTestRouter(h, modUserID, ""), http.MethodGet, capsPath, "")
	require.Equal(t, http.StatusOK, resp.Code)
	var caps models.Capabilities
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &caps))

	require.Len(t, caps.Sources, 1)
	assert.Equal(t, models.ReasonBotMissingPermission, caps.Sources[0].Reason)
	assert.False(t, caps.Sources[0].Moderatable)
}

// A lookup failure must not advertise a capability the action path would refuse. On Discord that
// matters more than elsewhere: nothing downstream re-checks it on our behalf.
func TestCapabilities_DiscordIdentityLookupErrorFailsClosed(t *testing.T) {
	auth := &fakeAuthorizer{
		access:          moderatorAccess([]string{"delete"}, []string{"discord"}),
		sources:         discordSource(),
		discordIdentErr: errors.New("db down"),
	}

	caps := modCaps(t, auth)

	require.Len(t, caps.Sources, 1)
	assert.False(t, caps.Sources[0].Moderatable)
	assert.Equal(t, models.ReasonNeedsDiscordLink, caps.Sources[0].Reason)
}

// Chat-send is owner-only in v1. The send bar reads can_send straight off the payload, so a true
// here would hand a volunteer the streamer's voice.
func TestCapabilities_ModeratorNeverReportsCanSend(t *testing.T) {
	auth := &fakeAuthorizer{
		access:    moderatorAccess([]string{"delete"}, []string{"twitch"}),
		sources:   twitchSource(),
		modScopes: map[string][]string{"twitch": twitchModScopes},
	}
	h := modCapsHandler(t, auth)
	// A send checker that says yes to everything: only the role may stop it.
	h.SetSendChecker(alwaysSendable{})

	resp := do(newTestRouter(h, modUserID, ""), http.MethodGet, capsPath, "")
	require.Equal(t, http.StatusOK, resp.Code)

	var caps models.Capabilities
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &caps))
	require.Len(t, caps.Sources, 1)
	assert.False(t, caps.Sources[0].CanSend, "moderators get no chat-send in v1")
}

type alwaysSendable struct{}

func (alwaysSendable) CanSend(_ context.Context, _, _, _ string) (bool, error) { return true, nil }

// A scope-lookup failure must not advertise a capability the action path would refuse.
func TestCapabilities_ModeratorScopeLookupErrorFailsClosed(t *testing.T) {
	auth := &fakeAuthorizer{
		access:       moderatorAccess([]string{"delete"}, []string{"twitch"}),
		sources:      twitchSource(),
		modScopesErr: errors.New("database unavailable"),
	}

	caps := modCaps(t, auth)

	require.Len(t, caps.Sources, 1)
	assert.False(t, caps.Sources[0].Moderatable)
	assert.Equal(t, models.ReasonNeedsConsent, caps.Sources[0].Reason)
}

// The gate is asked about the OWNER here exactly as it is on the action path, so a moderator with
// no entitlement of their own still gets controls on a premium streamer's overlay.
func TestCapabilities_GateIsKeyedOnTheOwner(t *testing.T) {
	auth := &fakeAuthorizer{
		access:    moderatorAccess([]string{"delete"}, []string{"twitch"}),
		sources:   twitchSource(),
		modScopes: map[string][]string{"twitch": twitchModScopes},
	}
	h := modCapsHandler(t, auth)
	h.SetFeatureGate(gateFor{ownerID: true}) // the moderator is deliberately absent

	resp := do(newTestRouter(h, modUserID, ""), http.MethodGet, capsPath, "")
	require.Equal(t, http.StatusOK, resp.Code)

	var caps models.Capabilities
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &caps))
	assert.True(t, caps.Enabled)
	assert.True(t, caps.CanModerate)
}

// Closing `delegated_moderation` must close the moderator's controls too, or the capability
// payload would keep advertising actions the action path now refuses.
func TestCapabilities_DelegationGateClosedReportsCannotModerate(t *testing.T) {
	auth := &fakeAuthorizer{
		access:    moderatorAccess([]string{"delete"}, []string{"twitch"}),
		sources:   twitchSource(),
		modScopes: map[string][]string{"twitch": twitchModScopes},
	}
	h := modCapsHandler(t, auth)
	h.SetFeatureGate(splitGate{moderation: map[string]bool{ownerID: true}})

	resp := do(newTestRouter(h, modUserID, ""), http.MethodGet, capsPath, "")
	require.Equal(t, http.StatusOK, resp.Code)

	var caps models.Capabilities
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &caps))
	assert.False(t, caps.Enabled)
	assert.False(t, caps.CanModerate)
}

// An overlay that does not exist and one the caller holds no role on must produce the identical
// body, or capabilities becomes the overlay-existence oracle the action path refuses to be.
func TestCapabilities_UnknownOverlayMatchesNoRole(t *testing.T) {
	unknown := &fakeAuthorizer{accessErr: repository.ErrOverlayNotFound, sources: twitchSource()}
	respUnknown := do(newTestRouter(modCapsHandler(t, unknown), modUserID, ""), http.MethodGet, capsPath, "")

	stranger := &fakeAuthorizer{
		access:  &repository.OverlayAccess{OwnerUserID: ownerID, Role: repository.RoleNone},
		sources: twitchSource(),
	}
	respStranger := do(newTestRouter(modCapsHandler(t, stranger), modUserID, ""), http.MethodGet, capsPath, "")

	assert.Equal(t, respStranger.Code, respUnknown.Code)
	assert.JSONEq(t, respStranger.Body.String(), respUnknown.Body.String())

	var caps models.Capabilities
	require.NoError(t, json.Unmarshal(respStranger.Body.Bytes(), &caps))
	assert.Equal(t, repository.RoleNone, caps.Role)
	assert.False(t, caps.CanModerate)
	assert.Empty(t, caps.Sources, "a caller with no role must not learn the overlay's sources")
}
