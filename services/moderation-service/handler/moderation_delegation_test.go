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

	"github.com/caesar/all-chat/services/moderation-service/audit"
	"github.com/caesar/all-chat/services/moderation-service/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

const modUserID = "33333333-3333-4333-8333-333333333333"

// gateFor reports enabled only for the listed user ids, so a test can assert the gate is asked
// about the OWNER rather than the caller.
type gateFor map[string]bool

func (g gateFor) ModerationEnabled(_ context.Context, userID string) (bool, error) {
	return g[userID], nil
}

func (g gateFor) DelegationEnabled(_ context.Context, userID string) (bool, error) {
	return g[userID], nil
}

func delegationHandler(t *testing.T, access repository.OverlayAccess, gate FeatureGate) (*Handler, *fakeEmitter, *fakeRecorder) {
	t.Helper()
	auth := &fakeAuthorizer{
		access:   &access,
		isSource: map[string]bool{"twitch|somestreamer": true},
	}
	emitter := &fakeEmitter{}
	rec := &fakeRecorder{}
	h := New(auth, emitter, rec, NoScopeChecker{}, DryRunDispatcher{}, zap.NewNop())
	if gate != nil {
		h.SetFeatureGate(gate)
	}
	return h, emitter, rec
}

const deleteBody = `{"platform":"twitch","channel_id":"somestreamer","native_message_id":"nm1"}`

func deletePath() string {
	return "/api/v1/moderation/overlays/" + overlayID + "/delete"
}

// An active grant for a delegated action lets a non-owner moderate.
func TestDelete_DelegatedModeratorIsAuthorized(t *testing.T) {
	h, emitter, rec := delegationHandler(t, repository.OverlayAccess{
		OwnerUserID:    ownerID,
		OwnerIsPremium: true,
		Role:           repository.RoleModerator,
		GrantID:        "grant-1",
		Actions:        []string{"delete", "timeout"},
	}, nil)

	resp := do(newTestRouter(h, modUserID, ""), http.MethodPost, deletePath(), deleteBody)

	assert.Equal(t, http.StatusOK, resp.Code)
	assert.NotEmpty(t, emitter.published, "a successful moderation must reflect back")
	require.Len(t, rec.entries, 1)
	assert.Equal(t, modUserID, rec.entries[0].ActorUserID,
		"the audit row records the human who pressed the button")
}

// last_action_at is what the 90-day dormancy rule reads. Stamping it from the first delegated
// action onwards is what stops a dormancy job introduced later from reading a working mod team as
// idle since the day their grants were created.
func TestDelete_DelegatedActionStampsGrantActivity(t *testing.T) {
	auth := &fakeAuthorizer{
		access: &repository.OverlayAccess{
			OwnerUserID: ownerID, OwnerIsPremium: true, Role: repository.RoleModerator,
			GrantID: "grant-1", Actions: []string{"delete"},
		},
		isSource: map[string]bool{"twitch|somestreamer": true},
	}
	h := New(auth, &fakeEmitter{}, &fakeRecorder{}, NoScopeChecker{}, DryRunDispatcher{}, zap.NewNop())

	resp := do(newTestRouter(h, modUserID, ""), http.MethodPost, deletePath(), deleteBody)

	require.Equal(t, http.StatusOK, resp.Code)
	assert.Equal(t, []string{"grant-1"}, auth.touchedGrants)
}

// An owner acts by ownership, not by a grant, so there is nothing to stamp — and no dormancy
// clock that could ever suspend them.
func TestDelete_OwnerActionStampsNothing(t *testing.T) {
	auth := &fakeAuthorizer{owns: true, isSource: map[string]bool{"twitch|somestreamer": true}}
	h := New(auth, &fakeEmitter{}, &fakeRecorder{}, NoScopeChecker{}, DryRunDispatcher{}, zap.NewNop())

	resp := do(newTestRouter(h, ownerID, ""), http.MethodPost, deletePath(), deleteBody)

	require.Equal(t, http.StatusOK, resp.Code)
	assert.Empty(t, auth.touchedGrants)
}

// A failed stamp is bookkeeping, not the action: the moderation already happened on the platform,
// so the request must still succeed.
func TestDelete_AStampFailureDoesNotFailTheAction(t *testing.T) {
	auth := &fakeAuthorizer{
		access: &repository.OverlayAccess{
			OwnerUserID: ownerID, OwnerIsPremium: true, Role: repository.RoleModerator,
			GrantID: "grant-1", Actions: []string{"delete"},
		},
		isSource: map[string]bool{"twitch|somestreamer": true},
		touchErr: errors.New("database unavailable"),
	}
	h := New(auth, &fakeEmitter{}, &fakeRecorder{}, NoScopeChecker{}, DryRunDispatcher{}, zap.NewNop())

	resp := do(newTestRouter(h, modUserID, ""), http.MethodPost, deletePath(), deleteBody)

	assert.Equal(t, http.StatusOK, resp.Code)
}

// The action set on the grant is enforced server-side, not merely hidden in the UI.
func TestDelete_ActionNotDelegatedIsRefused(t *testing.T) {
	h, emitter, rec := delegationHandler(t, repository.OverlayAccess{
		OwnerUserID:    ownerID,
		OwnerIsPremium: true,
		Role:           repository.RoleModerator,
		Actions:        []string{"timeout"}, // delete withheld
	}, nil)

	resp := do(newTestRouter(h, modUserID, ""), http.MethodPost, deletePath(), deleteBody)

	assert.Equal(t, http.StatusForbidden, resp.Code)
	assert.Empty(t, emitter.published)
	require.Len(t, rec.entries, 1)
	assert.Equal(t, audit.OutcomeDenied, rec.entries[0].Outcome)
}

// The premium gate must be asked about the OVERLAY OWNER. A moderator with no entitlement of
// their own moderates freely on a premium streamer's overlay.
func TestDelete_GateIsKeyedOnTheOwnerNotTheCaller(t *testing.T) {
	h, emitter, _ := delegationHandler(t, repository.OverlayAccess{
		OwnerUserID:    ownerID,
		OwnerIsPremium: true,
		Role:           repository.RoleModerator,
		Actions:        []string{"delete"},
	}, gateFor{ownerID: true}) // the moderator is deliberately absent

	resp := do(newTestRouter(h, modUserID, ""), http.MethodPost, deletePath(), deleteBody)

	assert.Equal(t, http.StatusOK, resp.Code,
		"a moderator must not need premium of their own")
	assert.NotEmpty(t, emitter.published)
}

// Conversely, an entitled moderator gains nothing on a non-entitled streamer's overlay.
func TestDelete_ModeratorCannotSubstituteTheirOwnEntitlement(t *testing.T) {
	h, emitter, rec := delegationHandler(t, repository.OverlayAccess{
		OwnerUserID:    ownerID,
		OwnerIsPremium: false,
		Role:           repository.RoleModerator,
		Actions:        []string{"delete"},
	}, gateFor{modUserID: true}) // only the moderator is "premium"

	resp := do(newTestRouter(h, modUserID, ""), http.MethodPost, deletePath(), deleteBody)

	assert.Equal(t, http.StatusForbidden, resp.Code)
	assert.Empty(t, emitter.published)

	// The copy must not send a volunteer to buy a plan that is not theirs to buy.
	var body map[string]any
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
	msg, _ := body["error"].(string)
	assert.Contains(t, msg, "streamer")
	assert.NotContains(t, msg, "premium",
		"moderator-facing copy must not read as an upgrade prompt")

	require.Len(t, rec.entries, 1)
	assert.Equal(t, audit.OutcomeDenied, rec.entries[0].Outcome,
		"an entitlement denial must be audited like any other denial")
}

// An owner outside the rollout cohort still gets owner-appropriate copy.
func TestDelete_OwnerOutsideCohortGetsUpgradeCopy(t *testing.T) {
	h, _, _ := delegationHandler(t, repository.OverlayAccess{
		OwnerUserID:    ownerID,
		OwnerIsPremium: false,
		Role:           repository.RoleOwner,
	}, gateFor{})

	resp := do(newTestRouter(h, ownerID, ""), http.MethodPost, deletePath(), deleteBody)

	assert.Equal(t, http.StatusForbidden, resp.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
	assert.Contains(t, body["error"], "premium")
}

// A pending, suspended or revoked grant resolves to RoleNone upstream, and must be refused with
// the same response a stranger gets.
func TestDelete_NoRoleIsRefused(t *testing.T) {
	h, emitter, _ := delegationHandler(t, repository.OverlayAccess{
		OwnerUserID:    ownerID,
		OwnerIsPremium: true,
		Role:           repository.RoleNone,
	}, nil)

	resp := do(newTestRouter(h, modUserID, ""), http.MethodPost, deletePath(), deleteBody)

	assert.Equal(t, http.StatusForbidden, resp.Code)
	assert.Empty(t, emitter.published)
}

// An unknown overlay must be indistinguishable from an unauthorized one — same status, same
// message — or the endpoint becomes an overlay-existence oracle for any valid token holder.
func TestDelete_UnknownOverlayIsIndistinguishableFromUnauthorized(t *testing.T) {
	unknown := &fakeAuthorizer{
		accessErr: repository.ErrOverlayNotFound,
		isSource:  map[string]bool{"twitch|somestreamer": true},
	}
	hUnknown := New(unknown, &fakeEmitter{}, &fakeRecorder{}, NoScopeChecker{}, DryRunDispatcher{}, zap.NewNop())
	respUnknown := do(newTestRouter(hUnknown, modUserID, ""), http.MethodPost, deletePath(), deleteBody)

	hStranger, _, _ := delegationHandler(t, repository.OverlayAccess{
		OwnerUserID:    ownerID,
		OwnerIsPremium: true,
		Role:           repository.RoleNone,
	}, nil)
	respStranger := do(newTestRouter(hStranger, modUserID, ""), http.MethodPost, deletePath(), deleteBody)

	assert.Equal(t, respStranger.Code, respUnknown.Code)
	assert.JSONEq(t, respStranger.Body.String(), respUnknown.Body.String(),
		"a nonexistent overlay must not be distinguishable from one the caller cannot touch")
}

// The shared_overlay exclusion was true by construction under owner-only authorization. Under
// role-based authorization only the source predicate carries it, so it gets its own test.
func TestDelete_DelegatedModeratorCannotReachANonSource(t *testing.T) {
	auth := &fakeAuthorizer{
		access: &repository.OverlayAccess{
			OwnerUserID:    ownerID,
			OwnerIsPremium: true,
			Role:           repository.RoleModerator,
			Actions:        []string{"delete"},
		},
		isSource: map[string]bool{}, // nothing is a moderatable source on this overlay
	}
	emitter := &fakeEmitter{}
	h := New(auth, emitter, &fakeRecorder{}, NoScopeChecker{}, DryRunDispatcher{}, zap.NewNop())

	resp := do(newTestRouter(h, modUserID, ""), http.MethodPost, deletePath(), deleteBody)

	assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
	assert.Empty(t, emitter.published)
}
