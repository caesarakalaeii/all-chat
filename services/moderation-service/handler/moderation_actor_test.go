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
	"encoding/json"
	"net/http"
	"testing"

	"github.com/caesar/all-chat/services/moderation-service/audit"
	"github.com/caesar/all-chat/services/moderation-service/models"
	"github.com/caesar/all-chat/services/moderation-service/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// What the handler hands the dispatcher, and what it records about the result (ADR-0048).
//
// The handler is where the resolved role becomes the Actor the dispatcher branches on, and where
// the dispatcher's report of *whose credential acted* becomes an audit row. Both are load-bearing:
// an Actor missing its owner id would send the anchor looking at the wrong account, and an audit
// row missing credential_user_id would lose the only machine-checkable proof of no-fallback.

// actorHandler wires a handler whose dispatcher records the Actor it was given.
func actorHandler(t *testing.T, access repository.OverlayAccess, res models.DispatchResult) (*Handler, *fakeDispatcher, *fakeRecorder) {
	t.Helper()
	auth := &fakeAuthorizer{
		access:   &access,
		isSource: map[string]bool{"twitch|somestreamer": true},
	}
	disp := &fakeDispatcher{res: res}
	rec := &fakeRecorder{}
	return New(auth, &fakeEmitter{}, rec, NoScopeChecker{}, disp, zap.NewNop()), disp, rec
}

func delegatedAccess() repository.OverlayAccess {
	return repository.OverlayAccess{
		OwnerUserID:    ownerID,
		OwnerIsPremium: true,
		Role:           repository.RoleModerator,
		GrantID:        "grant-1",
		Actions:        []string{"delete"},
		Platforms:      []string{"twitch"},
	}
}

// The Actor carries the OWNER's id, not just the caller's: the owner-reach anchor is resolved
// against it, and pointing that at the moderator would let anyone with a grant act on any channel
// they happen to hold a credential for.
func TestExecute_ActorCarriesRoleOwnerAndGrant(t *testing.T) {
	h, disp, _ := actorHandler(t, delegatedAccess(), models.DispatchResult{Outcome: models.DispatchPerformed})

	resp := do(newTestRouter(h, modUserID, ""), http.MethodPost, deletePath(), deleteBody)

	require.Equal(t, http.StatusOK, resp.Code)
	assert.Equal(t, modUserID, disp.gotActor.UserID, "the human who pressed the button")
	assert.Equal(t, repository.RoleModerator, disp.gotActor.Role)
	assert.Equal(t, ownerID, disp.gotActor.OwnerUserID, "whose channel is being moderated")
	assert.Equal(t, "grant-1", disp.gotActor.GrantID)
}

// An owner action names itself on both sides — there is no third party — and carries no grant.
func TestExecute_OwnerActorIsItsOwnPrincipal(t *testing.T) {
	access := repository.OverlayAccess{OwnerUserID: ownerID, OwnerIsPremium: true, Role: repository.RoleOwner}
	h, disp, _ := actorHandler(t, access, models.DispatchResult{Outcome: models.DispatchPerformed})

	resp := do(newTestRouter(h, ownerID, ""), http.MethodPost, deletePath(), deleteBody)

	require.Equal(t, http.StatusOK, resp.Code)
	assert.Equal(t, ownerID, disp.gotActor.UserID)
	assert.Equal(t, ownerID, disp.gotActor.OwnerUserID)
	assert.Empty(t, disp.gotActor.GrantID)
}

// credential_user_id comes from the dispatcher rather than being asserted here, because only the
// dispatcher knows which credential it actually reached for. Recording an assumption instead
// would turn the proof into a restatement of intent.
func TestExecute_AuditRecordsWhoseCredentialActed(t *testing.T) {
	h, _, rec := actorHandler(t, delegatedAccess(), models.DispatchResult{
		Outcome:          models.DispatchPerformed,
		CredentialUserID: modUserID,
		PlatformActorID:  "777",
	})

	resp := do(newTestRouter(h, modUserID, ""), http.MethodPost, deletePath(), deleteBody)

	require.Equal(t, http.StatusOK, resp.Code)
	require.Len(t, rec.entries, 1)
	e := rec.entries[0]
	assert.Equal(t, modUserID, e.ActorUserID)
	assert.Equal(t, repository.RoleModerator, e.ActorRole)
	assert.Equal(t, ownerID, e.OnBehalfOfUserID)
	assert.Equal(t, modUserID, e.CredentialUserID, "the moderator's own token acted")
	assert.NotEqual(t, ownerID, e.CredentialUserID)
	assert.Equal(t, "777", e.PlatformActorID)
	assert.Equal(t, "grant-1", e.GrantID)
}

// A denial is audited with the same attribution. "This moderator keeps getting refused" is one of
// the signals that a grant has gone wrong, and it is invisible if a denial cannot be told apart
// from an owner's.
func TestDeny_RecordsTheRoleAndTheOwner(t *testing.T) {
	access := delegatedAccess()
	access.Actions = []string{"timeout"} // delete withheld
	h, _, rec := actorHandler(t, access, models.DispatchResult{})

	resp := do(newTestRouter(h, modUserID, ""), http.MethodPost, deletePath(), deleteBody)

	require.Equal(t, http.StatusForbidden, resp.Code)
	require.Len(t, rec.entries, 1)
	assert.Equal(t, audit.OutcomeDenied, rec.entries[0].Outcome)
	assert.Equal(t, repository.RoleModerator, rec.entries[0].ActorRole)
	assert.Equal(t, ownerID, rec.entries[0].OnBehalfOfUserID)
	assert.Equal(t, "grant-1", rec.entries[0].GrantID)
}

// The owner-reach anchor failing is the owner's problem to fix, so the copy names them and gives
// the moderator nothing to attempt. It is audited, not silently dropped.
func TestExecute_OwnerUnverifiedIsRefusedAndAudited(t *testing.T) {
	h, _, rec := actorHandler(t, delegatedAccess(), models.DispatchResult{Outcome: models.DispatchOwnerUnverified})

	resp := do(newTestRouter(h, modUserID, ""), http.MethodPost, deletePath(), deleteBody)

	assert.Equal(t, http.StatusForbidden, resp.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
	assert.Equal(t, codeOwnerUnverified, body["code"])
	assert.Contains(t, body["error"], "streamer")
	require.Len(t, rec.entries, 1)
	assert.Equal(t, audit.OutcomeOwnerUnverified, rec.entries[0].Outcome)
}

// The platform's own refusal is surfaced as something the streamer can act on, not as a consent
// prompt the moderator would run in circles on.
func TestExecute_NotAPlatformModeratorPointsAtTheStreamer(t *testing.T) {
	h, _, rec := actorHandler(t, delegatedAccess(), models.DispatchResult{
		Outcome: models.DispatchNotPlatformModerator, PlatformStatus: "forbidden",
	})

	resp := do(newTestRouter(h, modUserID, ""), http.MethodPost, deletePath(), deleteBody)

	assert.Equal(t, http.StatusForbidden, resp.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
	assert.Equal(t, codeNotPlatformModerator, body["code"])
	assert.NotContains(t, body, "requires_reauth", "re-consent cannot fix this")
	require.Len(t, rec.entries, 1)
	assert.Equal(t, audit.OutcomeNotPlatformModerator, rec.entries[0].Outcome)
}

// A platform whose delegated leg is not built must refuse rather than report success. Audited
// too, so a moderator hitting the wall is visible rather than just looking broken.
func TestExecute_DelegationUnsupportedIsRefusedAndAudited(t *testing.T) {
	h, _, rec := actorHandler(t, delegatedAccess(), models.DispatchResult{Outcome: models.DispatchDelegationUnsupported})

	resp := do(newTestRouter(h, modUserID, ""), http.MethodPost, deletePath(), deleteBody)

	assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
	assert.Equal(t, codeDelegationUnsupported, body["code"])
	require.Len(t, rec.entries, 1)
	assert.Equal(t, audit.OutcomeDelegationUnsupported, rec.entries[0].Outcome)
}

// "No credential" reads differently by role: a streamer is not the broadcaster of this channel,
// whereas a moderator simply has not connected their own account — the expected state of a fresh
// grant, since consent is deferred to first use.
func TestExecute_NoCredentialCopyDiffersByRole(t *testing.T) {
	t.Run("moderator is told to connect", func(t *testing.T) {
		h, _, _ := actorHandler(t, delegatedAccess(), models.DispatchResult{Outcome: models.DispatchNoCredential})

		resp := do(newTestRouter(h, modUserID, ""), http.MethodPost, deletePath(), deleteBody)

		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
		var body map[string]any
		require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
		assert.Equal(t, codeConnectRequired, body["code"])
		assert.Contains(t, body["error"], "connect")
	})

	t.Run("owner is told they are not the broadcaster", func(t *testing.T) {
		access := repository.OverlayAccess{OwnerUserID: ownerID, OwnerIsPremium: true, Role: repository.RoleOwner}
		h, _, _ := actorHandler(t, access, models.DispatchResult{Outcome: models.DispatchNoCredential})

		resp := do(newTestRouter(h, ownerID, ""), http.MethodPost, deletePath(), deleteBody)

		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
		var body map[string]any
		require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
		assert.Contains(t, body["error"], "do not hold moderator credentials")
	})
}

// The five Discord refusals (ADR-0048's platform-attested leg).
//
// Discord is the one platform where All-Chat's own check is the only authority, so its refusals
// have no platform message behind them — this mapping IS the explanation the person gets. What
// each case must get right is WHO is told to do something: a volunteer sent to fix a permission
// only the streamer controls has been given a dead end, which is the failure mode the whole reason
// vocabulary exists to prevent.
func TestExecute_DiscordRefusalsNameTheRightFixer(t *testing.T) {
	cases := []struct {
		name     string
		outcome  models.DispatchOutcome
		status   int
		code     string
		audited  string
		mentions string
	}{
		{
			name: "moderator has not linked Discord", outcome: models.DispatchModNotLinked,
			status: http.StatusUnprocessableEntity, code: codeDiscordLinkRequired,
			audited: audit.OutcomeDiscordLinkRequired, mentions: "your",
		},
		{
			name: "moderator is not in the guild", outcome: models.DispatchModNotInGuild,
			status: http.StatusForbidden, code: codeModNotInGuild,
			audited: audit.OutcomeModNotInGuild, mentions: "streamer",
		},
		{
			name: "moderator lacks the Discord permission", outcome: models.DispatchModLacksPermission,
			status: http.StatusForbidden, code: codeModLacksPermission,
			audited: audit.OutcomeModLacksPermission, mentions: "streamer",
		},
		{
			name: "role hierarchy refuses", outcome: models.DispatchModBelowTarget,
			status: http.StatusForbidden, code: codeModBelowTarget,
			audited: audit.OutcomeModBelowTarget, mentions: "role",
		},
		{
			name: "the bot was never given the permission", outcome: models.DispatchBotMissingPermission,
			status: http.StatusForbidden, code: codeBotMissingPermission,
			audited: audit.OutcomeBotMissingPermission, mentions: "re-invite",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, _, rec := actorHandler(t, delegatedAccess(), models.DispatchResult{Outcome: tc.outcome})

			resp := do(newTestRouter(h, modUserID, ""), http.MethodPost, deletePath(), deleteBody)

			assert.Equal(t, tc.status, resp.Code)
			var body map[string]any
			require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
			assert.Equal(t, tc.code, body["code"])
			assert.Contains(t, body["error"], tc.mentions, "the copy must name the person who can fix it")
			assert.NotContains(t, body, "requires_reauth",
				"no Discord refusal is fixed by an OAuth re-consent — the link stores no token and guild permissions are not scopes")
			require.Len(t, rec.entries, 1, "a refusal is audited, never silently dropped")
			assert.Equal(t, tc.audited, rec.entries[0].Outcome)
			assert.Equal(t, repository.RoleModerator, rec.entries[0].ActorRole)
		})
	}
}

// No refusal may emit the reflect-back: the message is still there on Discord, and hiding it in the
// overlay would tell the streamer an action landed that never did.
func TestExecute_DiscordRefusalsEmitNoReflectBack(t *testing.T) {
	for _, outcome := range []models.DispatchOutcome{
		models.DispatchModNotLinked, models.DispatchModNotInGuild, models.DispatchModLacksPermission,
		models.DispatchModBelowTarget, models.DispatchBotMissingPermission,
	} {
		access := delegatedAccess()
		auth := &fakeAuthorizer{access: &access, isSource: map[string]bool{"twitch|somestreamer": true}}
		emitter := &fakeEmitter{}
		h := New(auth, emitter, &fakeRecorder{}, NoScopeChecker{}, &fakeDispatcher{res: models.DispatchResult{Outcome: outcome}}, zap.NewNop())

		resp := do(newTestRouter(h, modUserID, ""), http.MethodPost, deletePath(), deleteBody)

		require.NotEqual(t, http.StatusOK, resp.Code)
		assert.Empty(t, emitter.published, "a refused action must not be reflected back as if it had landed")
	}
}
