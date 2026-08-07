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
	"time"

	"github.com/caesar/all-chat/services/moderation-service/models"
	"github.com/caesar/all-chat/services/moderation-service/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// "Channels I moderate" (ADR-0048).
//
// GET /api/v1/overlays is owner-filtered, so this listing is the ONLY route an accepted moderator
// has into an overlay they do not own. Everything here follows from that: it is keyed on the
// caller alone, it is never feature-gated, and it must keep listing a channel whose grant has been
// suspended rather than letting it silently vanish.

const delegationsPath = "/api/v1/moderation/delegations"

// delegationHandlerFor builds a grant handler whose store returns the given delegations.
func delegationHandlerFor(t *testing.T, store *fakeGrantStore, gate FeatureGate) *GrantHandler {
	t.Helper()
	h := NewGrantHandler(&fakeAuthorizer{}, store, zap.NewNop())
	if gate != nil {
		h.SetFeatureGate(gate)
	}
	return h
}

func listDelegations(t *testing.T, store *fakeGrantStore, gate FeatureGate) models.DelegationList {
	t.Helper()
	h := delegationHandlerFor(t, store, gate)
	resp := do(grantRouter(h, modUserID), http.MethodGet, delegationsPath, "")
	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())

	var out models.DelegationList
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &out))
	return out
}

func sampleDelegation() repository.Delegation {
	accepted := time.Date(2026, 8, 7, 10, 0, 0, 0, time.UTC)
	return repository.Delegation{
		GrantID:          "grant-1",
		OverlayID:        overlayID,
		OverlayName:      "Main overlay",
		OwnerUserID:      ownerID,
		OwnerDisplayName: "SomeStreamer",
		Status:           models.GrantStatusActive,
		Actions:          []string{"delete", "timeout"},
		AcceptedAt:       &accepted,
		Platforms: []repository.GrantLeg{
			{Platform: "twitch", Enabled: true, Verification: "unverified"},
		},
	}
}

// The moderator gets the overlay id — without it the grant is unreachable — plus enough context to
// know whose channel it is.
func TestDelegations_ListsTheOverlaysAModeratorCanReach(t *testing.T) {
	store := &fakeGrantStore{delegations: []repository.Delegation{sampleDelegation()}}

	out := listDelegations(t, store, nil)

	require.Len(t, out.Delegations, 1)
	d := out.Delegations[0]
	assert.Equal(t, overlayID, d.OverlayID, "without the overlay id the grant cannot be reached")
	assert.Equal(t, "Main overlay", d.OverlayName)
	assert.Equal(t, "SomeStreamer", d.OwnerDisplayName)
	assert.Equal(t, []string{"delete", "timeout"}, d.Actions)
	require.Len(t, d.Platforms, 1)
	assert.Equal(t, "twitch", d.Platforms[0].Platform)
	assert.True(t, d.Available)
}

// The listing is keyed on the caller's own id and nothing else: there is no path by which one
// moderator could enumerate another's channels.
func TestDelegations_KeyedOnTheCallerAlone(t *testing.T) {
	store := &fakeGrantStore{}

	listDelegations(t, store, nil)

	assert.Equal(t, []string{modUserID}, store.listedFor)
}

// An unauthenticated caller has no delegations to look up.
func TestDelegations_RequiresAUser(t *testing.T) {
	h := delegationHandlerFor(t, &fakeGrantStore{}, nil)

	resp := do(grantRouter(h, ""), http.MethodGet, delegationsPath, "")

	assert.Equal(t, http.StatusUnauthorized, resp.Code)
}

// An empty list is a list, not a null: the page renders "no channels yet" rather than crashing on
// a missing array.
func TestDelegations_EmptyListSerializesAsAnArray(t *testing.T) {
	h := delegationHandlerFor(t, &fakeGrantStore{}, nil)

	resp := do(grantRouter(h, modUserID), http.MethodGet, delegationsPath, "")

	require.Equal(t, http.StatusOK, resp.Code)
	assert.JSONEq(t, `{"delegations":[]}`, resp.Body.String())
}

// The streamer's plan lapsing must not make the channel disappear. It is reported as unavailable,
// so the moderator can see why they cannot act instead of assuming they were removed.
func TestDelegations_OwnerOutsideTheCohortIsListedAsUnavailable(t *testing.T) {
	store := &fakeGrantStore{delegations: []repository.Delegation{sampleDelegation()}}

	out := listDelegations(t, store, delegationGateFor{}) // nobody is entitled

	require.Len(t, out.Delegations, 1)
	assert.False(t, out.Delegations[0].Available)
	assert.Equal(t, overlayID, out.Delegations[0].OverlayID,
		"an unavailable delegation is still listed — vanishing reads as revocation")
}

// Availability is the OWNER's entitlement. A moderator's own premium buys them nothing here, and a
// premium streamer's moderators are available without any of their own.
func TestDelegations_AvailabilityIsKeyedOnTheOwner(t *testing.T) {
	store := &fakeGrantStore{delegations: []repository.Delegation{sampleDelegation()}}

	out := listDelegations(t, store, delegationGateFor{ownerID: true})
	require.Len(t, out.Delegations, 1)
	assert.True(t, out.Delegations[0].Available, "a moderator needs no entitlement of their own")

	out = listDelegations(t, store, delegationGateFor{modUserID: true})
	require.Len(t, out.Delegations, 1)
	assert.False(t, out.Delegations[0].Available,
		"the moderator's own premium must not substitute for the streamer's")
}

// A gate lookup that fails must not promise a capability the action path would then refuse.
func TestDelegations_GateErrorReportsUnavailable(t *testing.T) {
	store := &fakeGrantStore{delegations: []repository.Delegation{sampleDelegation()}}

	out := listDelegations(t, store, erroringGate{})

	require.Len(t, out.Delegations, 1)
	assert.False(t, out.Delegations[0].Available)
}

type erroringGate struct{}

func (erroringGate) ModerationEnabled(_ context.Context, _ string) (bool, error) {
	return false, errors.New("database unavailable")
}

func (erroringGate) DelegationEnabled(_ context.Context, _ string) (bool, error) {
	return false, errors.New("database unavailable")
}

// One gate lookup per distinct owner, not per row. A volunteer moderating several overlays for the
// same streamer must not turn one page load into one premium query per overlay.
func TestDelegations_GateIsAskedOncePerOwner(t *testing.T) {
	second := sampleDelegation()
	second.GrantID = "grant-2"
	second.OverlayID = "aaaaaaaa-2222-2222-2222-222222222222"
	second.OverlayName = "Second overlay"
	store := &fakeGrantStore{delegations: []repository.Delegation{sampleDelegation(), second}}
	gate := &countingGate{enabled: true}

	h := delegationHandlerFor(t, store, gate)
	resp := do(grantRouter(h, modUserID), http.MethodGet, delegationsPath, "")

	require.Equal(t, http.StatusOK, resp.Code)
	assert.Equal(t, 1, gate.calls, "both overlays belong to the same streamer")
}

type countingGate struct {
	enabled bool
	calls   int
}

func (g *countingGate) ModerationEnabled(_ context.Context, _ string) (bool, error) {
	return g.enabled, nil
}

func (g *countingGate) DelegationEnabled(_ context.Context, _ string) (bool, error) {
	g.calls++
	return g.enabled, nil
}

// A suspended grant is listed with its status. Hiding it would look identical to revocation, and
// the moderator would have nothing to ask the streamer about.
func TestDelegations_SuspendedGrantIsListedWithItsStatus(t *testing.T) {
	suspended := sampleDelegation()
	suspended.Status = models.GrantStatusSuspended
	store := &fakeGrantStore{delegations: []repository.Delegation{suspended}}

	out := listDelegations(t, store, nil)

	require.Len(t, out.Delegations, 1)
	assert.Equal(t, models.GrantStatusSuspended, out.Delegations[0].Status)
}

// The moderator learns who delegated to them, not who else did — the payload carries the owner's
// display name and no user ids at all.
func TestDelegations_DoesNotLeakUserIdentifiers(t *testing.T) {
	store := &fakeGrantStore{delegations: []repository.Delegation{sampleDelegation()}}
	h := delegationHandlerFor(t, store, nil)

	resp := do(grantRouter(h, modUserID), http.MethodGet, delegationsPath, "")

	require.Equal(t, http.StatusOK, resp.Code)
	assert.NotContains(t, resp.Body.String(), ownerID,
		"the owner's user id is not the moderator's business")
}

// A read failure is a 500, not an empty list: silently reporting "you moderate nothing" would send
// the moderator to ask the streamer about a revocation that never happened.
func TestDelegations_ReadFailureIsAnError(t *testing.T) {
	store := &fakeGrantStore{delegateErr: errors.New("database unavailable")}
	h := delegationHandlerFor(t, store, nil)

	resp := do(grantRouter(h, modUserID), http.MethodGet, delegationsPath, "")

	assert.Equal(t, http.StatusInternalServerError, resp.Code)
}
