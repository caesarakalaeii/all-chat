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
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/caesar/all-chat/services/moderation-service/invites"
	"github.com/caesar/all-chat/services/moderation-service/models"
	"github.com/caesar/all-chat/services/moderation-service/repository"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// strangerUserID holds no role on the overlay at all.
const strangerUserID = "44444444-4444-4444-8444-444444444444"

// ---------------------------------------------------------------------------
// Fakes
// ---------------------------------------------------------------------------

type fakeGrantStore struct {
	created     []repository.InviteParams
	createErr   error
	grants      []repository.Grant
	listErr     error
	updated     repository.Grant
	updateErr   error
	updateCalls []struct {
		OverlayID, GrantID string
		Actions            []string
		Legs               map[string]bool
	}
	revokedOK   bool
	revokeErr   error
	revokedIDs  []string
	revokedAll  int
	revokeAllOK []string
	preview     repository.InviteDetails
	previewErr  error
	accepted    repository.InviteDetails
	acceptErr   error
	acceptedFor []string
	seenHashes  [][]byte
	delegations []repository.Delegation
	delegateErr error
	// listedFor records whose delegations were asked for, so a test can prove the listing is
	// keyed on the caller and nothing else.
	listedFor []string
}

func (f *fakeGrantStore) CreateInvite(_ context.Context, p repository.InviteParams) (repository.Grant, error) {
	f.created = append(f.created, p)
	if f.createErr != nil {
		return repository.Grant{}, f.createErr
	}
	return repository.Grant{
		ID: "grant-new", OverlayID: p.OverlayID, Status: models.GrantStatusPending,
		Actions: p.Actions, InviteeLabel: p.InviteeLabel, ExpectedPlatform: p.ExpectedPlatform,
		InviteExpiresAt: &p.ExpiresAt,
	}, nil
}

func (f *fakeGrantStore) ListGrants(context.Context, string) ([]repository.Grant, error) {
	return f.grants, f.listErr
}

func (f *fakeGrantStore) UpdateGrant(_ context.Context, overlayID, grantID string, actions []string, legs map[string]bool) (repository.Grant, error) {
	f.updateCalls = append(f.updateCalls, struct {
		OverlayID, GrantID string
		Actions            []string
		Legs               map[string]bool
	}{overlayID, grantID, actions, legs})
	return f.updated, f.updateErr
}

func (f *fakeGrantStore) RevokeGrant(_ context.Context, _, grantID, _ string) (bool, error) {
	f.revokedIDs = append(f.revokedIDs, grantID)
	return f.revokedOK, f.revokeErr
}

func (f *fakeGrantStore) RevokeAllGrants(_ context.Context, overlayID, _ string) (int, error) {
	f.revokeAllOK = append(f.revokeAllOK, overlayID)
	return f.revokedAll, f.revokeErr
}

func (f *fakeGrantStore) PreviewInvite(_ context.Context, hash []byte) (repository.InviteDetails, error) {
	f.seenHashes = append(f.seenHashes, hash)
	return f.preview, f.previewErr
}

func (f *fakeGrantStore) AcceptInvite(_ context.Context, hash []byte, userID string) (repository.InviteDetails, error) {
	f.seenHashes = append(f.seenHashes, hash)
	f.acceptedFor = append(f.acceptedFor, userID)
	if f.acceptErr != nil {
		return f.accepted, f.acceptErr
	}
	return f.accepted, nil
}

func (f *fakeGrantStore) ListDelegationsFor(_ context.Context, moderatorUserID string) ([]repository.Delegation, error) {
	f.listedFor = append(f.listedFor, moderatorUserID)
	return f.delegations, f.delegateErr
}

// delegationGateFor reports delegation enabled only for the listed user ids, so a test can prove
// the gate is asked about the overlay OWNER.
type delegationGateFor map[string]bool

func (g delegationGateFor) ModerationEnabled(_ context.Context, userID string) (bool, error) {
	return g[userID], nil
}

func (g delegationGateFor) DelegationEnabled(_ context.Context, userID string) (bool, error) {
	return g[userID], nil
}

// ---------------------------------------------------------------------------
// Harness
// ---------------------------------------------------------------------------

func grantHandler(t *testing.T, role string, store *fakeGrantStore, gate FeatureGate) *GrantHandler {
	t.Helper()
	access := &repository.OverlayAccess{OwnerUserID: ownerID, OwnerIsPremium: true, Role: role}
	auth := &fakeAuthorizer{access: access}
	h := NewGrantHandler(auth, store, zap.NewNop())
	if gate != nil {
		h.SetFeatureGate(gate)
	}
	return h
}

func grantRouter(h *GrantHandler, userID string, roles ...string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		if userID != "" {
			c.Set("user_id", userID)
		}
		if len(roles) > 0 {
			c.Set("roles", roles)
		}
		c.Next()
	})
	api := r.Group("/api/v1/moderation")
	api.GET("/overlays/:id/moderators", h.HandleListModerators)
	api.POST("/overlays/:id/moderators", h.HandleCreateInvite)
	api.PATCH("/overlays/:id/moderators/:grant_id", h.HandleUpdateGrant)
	api.DELETE("/overlays/:id/moderators/:grant_id", h.HandleRevokeGrant)
	api.DELETE("/overlays/:id/moderators", h.HandleRevokeAll)
	api.POST("/invites/preview", h.HandlePreviewInvite)
	api.POST("/invites/accept", h.HandleAcceptInvite)
	api.GET("/delegations", h.HandleListDelegations)
	return r
}

func modsPath() string  { return "/api/v1/moderation/overlays/" + overlayID + "/moderators" }
func grantPath() string { return modsPath() + "/bbbbbbbb-2222-2222-2222-222222222222" }

func decode(t *testing.T, resp *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
	return body
}

// ---------------------------------------------------------------------------
// Owner-only authorization
// ---------------------------------------------------------------------------

// Managing the mod team is an ownership power, not a moderation power. A delegated moderator must
// not be able to invite more moderators, widen their own grant, or revoke a colleague — and the
// refusal must look exactly like a stranger's, so the endpoints never confirm an overlay exists.
func TestGrantEndpoints_AreOwnerOnlyAndIndistinguishable(t *testing.T) {
	body := `{"actions":["delete"]}`
	requests := []struct {
		name, method, path, body string
	}{
		{"list", http.MethodGet, modsPath(), ""},
		{"invite", http.MethodPost, modsPath(), body},
		{"update", http.MethodPatch, grantPath(), body},
		{"revoke", http.MethodDelete, grantPath(), ""},
		{"revoke all", http.MethodDelete, modsPath(), ""},
	}

	for _, req := range requests {
		t.Run(req.name, func(t *testing.T) {
			store := &fakeGrantStore{}
			asModerator := do(grantRouter(grantHandler(t, repository.RoleModerator, store, nil), modUserID),
				req.method, req.path, req.body)
			asStranger := do(grantRouter(grantHandler(t, repository.RoleNone, store, nil), strangerUserID),
				req.method, req.path, req.body)

			unknown := &fakeAuthorizer{accessErr: repository.ErrOverlayNotFound}
			hUnknown := NewGrantHandler(unknown, store, zap.NewNop())
			onUnknown := do(grantRouter(hUnknown, ownerID), req.method, req.path, req.body)

			assert.Equal(t, http.StatusForbidden, asModerator.Code,
				"a delegated moderator must not manage the mod team")
			assert.Equal(t, http.StatusForbidden, asStranger.Code)
			assert.Equal(t, http.StatusForbidden, onUnknown.Code)
			assert.JSONEq(t, asStranger.Body.String(), onUnknown.Body.String(),
				"a nonexistent overlay must be indistinguishable from one the caller cannot touch")
			assert.JSONEq(t, asStranger.Body.String(), asModerator.Body.String())

			assert.Empty(t, store.created, "a refused request must not reach the database")
			assert.Empty(t, store.updateCalls)
			assert.Empty(t, store.revokedIDs)
			assert.Empty(t, store.revokeAllOK)
		})
	}
}

func TestGrantEndpoints_RequireAuthentication(t *testing.T) {
	store := &fakeGrantStore{}
	resp := do(grantRouter(grantHandler(t, repository.RoleOwner, store, nil), ""), http.MethodGet, modsPath(), "")
	assert.Equal(t, http.StatusUnauthorized, resp.Code)
}

// ---------------------------------------------------------------------------
// Invite creation
// ---------------------------------------------------------------------------

func TestCreateInvite_ReturnsTheSecretExactlyOnce(t *testing.T) {
	store := &fakeGrantStore{}
	h := grantHandler(t, repository.RoleOwner, store, nil)

	resp := do(grantRouter(h, ownerID), http.MethodPost, modsPath(),
		`{"actions":["delete","ban"],"platforms":["twitch"],"invitee_label":"Sarah"}`)
	require.Equal(t, http.StatusCreated, resp.Code, resp.Body.String())

	body := decode(t, resp)
	secret, ok := body["invite_token"].(string)
	require.True(t, ok, "the streamer must receive the secret to pass on")
	assert.NotEmpty(t, secret)
	assert.NotEmpty(t, body["expires_at"])

	require.Len(t, store.created, 1)
	created := store.created[0]
	assert.Equal(t, overlayID, created.OverlayID)
	assert.Equal(t, ownerID, created.GrantedBy)
	assert.Equal(t, []string{"delete", "ban"}, created.Actions)
	assert.Equal(t, []string{"twitch"}, created.Platforms)
	assert.Equal(t, "Sarah", created.InviteeLabel)

	// Only the digest is handed to storage — the plaintext stops at the HTTP response.
	assert.Equal(t, invites.Hash(secret), created.TokenHash)
	assert.NotContains(t, string(created.TokenHash), secret)

	assert.WithinDuration(t, time.Now().Add(invites.TTL), created.ExpiresAt, time.Minute)

	t.Run("the roster never replays a secret", func(t *testing.T) {
		store.grants = []repository.Grant{{ID: "grant-new", Status: models.GrantStatusPending}}
		list := do(grantRouter(h, ownerID), http.MethodGet, modsPath(), "")
		require.Equal(t, http.StatusOK, list.Code)
		assert.NotContains(t, list.Body.String(), secret,
			"a lost invite is re-minted, never re-displayed")
		assert.NotContains(t, list.Body.String(), "invite_token")
	})
}

func TestCreateInvite_ActionDefaultsAndValidation(t *testing.T) {
	t.Run("an absent action list grants the safe default pair", func(t *testing.T) {
		store := &fakeGrantStore{}
		resp := do(grantRouter(grantHandler(t, repository.RoleOwner, store, nil), ownerID),
			http.MethodPost, modsPath(), `{}`)
		require.Equal(t, http.StatusCreated, resp.Code)
		assert.Equal(t, []string{"delete", "timeout"}, store.created[0].Actions)
	})

	t.Run("an explicitly empty list is a bad request, never widened", func(t *testing.T) {
		store := &fakeGrantStore{}
		resp := do(grantRouter(grantHandler(t, repository.RoleOwner, store, nil), ownerID),
			http.MethodPost, modsPath(), `{"actions":[]}`)
		assert.Equal(t, http.StatusBadRequest, resp.Code)
		assert.Empty(t, store.created)
	})

	// "engagement" is accepted by ModerationScopesForActions downstream, where it maps to
	// channel-point and prediction read scopes. A grant must never be able to carry it.
	t.Run("a non-moderation action is refused", func(t *testing.T) {
		for _, action := range []string{"engagement", "send", "rediscover"} {
			store := &fakeGrantStore{}
			resp := do(grantRouter(grantHandler(t, repository.RoleOwner, store, nil), ownerID),
				http.MethodPost, modsPath(), `{"actions":["`+action+`"]}`)
			assert.Equal(t, http.StatusBadRequest, resp.Code, "action %q must not be delegatable", action)
			assert.Empty(t, store.created)
		}
	})

	t.Run("a platform with no moderation API is refused rather than stored dead", func(t *testing.T) {
		for _, platform := range []string{"tiktok", "shared_overlay", "myspace"} {
			store := &fakeGrantStore{}
			resp := do(grantRouter(grantHandler(t, repository.RoleOwner, store, nil), ownerID),
				http.MethodPost, modsPath(), `{"platforms":["`+platform+`"]}`)
			assert.Equal(t, http.StatusUnprocessableEntity, resp.Code, "platform %q", platform)
			assert.Empty(t, store.created)
		}
	})

	t.Run("no platform enabled is allowed and delegates nothing", func(t *testing.T) {
		store := &fakeGrantStore{}
		resp := do(grantRouter(grantHandler(t, repository.RoleOwner, store, nil), ownerID),
			http.MethodPost, modsPath(), `{}`)
		require.Equal(t, http.StatusCreated, resp.Code)
		assert.Empty(t, store.created[0].Platforms, "absence is disablement; Discord stays off")
	})
}

// A binding we cannot verify at accept time would be a constraint that silently does nothing, so
// the API refuses to store one instead of pretending it protects anybody.
func TestCreateInvite_PreBindingOnlyWherePlatformIdentityIsProvable(t *testing.T) {
	t.Run("twitch is accepted", func(t *testing.T) {
		store := &fakeGrantStore{}
		resp := do(grantRouter(grantHandler(t, repository.RoleOwner, store, nil), ownerID),
			http.MethodPost, modsPath(),
			`{"expected_platform":"twitch","expected_platform_user_id":"77777"}`)
		require.Equal(t, http.StatusCreated, resp.Code)
		assert.Equal(t, "twitch", store.created[0].ExpectedPlatform)
		assert.Equal(t, "77777", store.created[0].ExpectedPlatformUserID)
	})

	t.Run("other platforms are refused", func(t *testing.T) {
		for _, platform := range []string{"kick", "youtube", "discord"} {
			store := &fakeGrantStore{}
			resp := do(grantRouter(grantHandler(t, repository.RoleOwner, store, nil), ownerID),
				http.MethodPost, modsPath(),
				`{"expected_platform":"`+platform+`","expected_platform_user_id":"1"}`)
			assert.Equal(t, http.StatusUnprocessableEntity, resp.Code, "platform %q", platform)
			assert.Empty(t, store.created)
		}
	})

	t.Run("a platform without an id is refused", func(t *testing.T) {
		store := &fakeGrantStore{}
		resp := do(grantRouter(grantHandler(t, repository.RoleOwner, store, nil), ownerID),
			http.MethodPost, modsPath(), `{"expected_platform":"twitch"}`)
		assert.Equal(t, http.StatusBadRequest, resp.Code)
		assert.Empty(t, store.created)
	})

	// The column is VARCHAR(100); without a bound, Postgres raises 22001 and the client gets a 500
	// for what is plainly a bad request.
	t.Run("an over-long platform id is a bad request, not a 500", func(t *testing.T) {
		store := &fakeGrantStore{}
		resp := do(grantRouter(grantHandler(t, repository.RoleOwner, store, nil), ownerID),
			http.MethodPost, modsPath(),
			`{"expected_platform":"twitch","expected_platform_user_id":"`+strings.Repeat("9", 101)+`"}`)
		assert.Equal(t, http.StatusBadRequest, resp.Code)
		assert.Empty(t, store.created)
	})
}

// The label is stored in a VARCHAR(120), and a streamer pasting something long must not produce an
// error — the label is decoration, so it is trimmed.
func TestCreateInvite_AnOverLongLabelIsTrimmedNotRejected(t *testing.T) {
	store := &fakeGrantStore{}
	resp := do(grantRouter(grantHandler(t, repository.RoleOwner, store, nil), ownerID),
		http.MethodPost, modsPath(), `{"invitee_label":"`+strings.Repeat("a", 500)+`"}`)

	require.Equal(t, http.StatusCreated, resp.Code)
	require.Len(t, store.created, 1)
	assert.LessOrEqual(t, len(store.created[0].InviteeLabel), 120)
}

func TestCreateInvite_CapIsRefusedWithAnExplanation(t *testing.T) {
	store := &fakeGrantStore{createErr: repository.ErrModeratorCapReached}
	resp := do(grantRouter(grantHandler(t, repository.RoleOwner, store, nil), ownerID),
		http.MethodPost, modsPath(), `{}`)

	assert.Equal(t, http.StatusConflict, resp.Code)
	body := decode(t, resp)
	assert.Equal(t, "moderator_cap_reached", body["code"])
	assert.EqualValues(t, models.ModeratorsPerOverlayCap, body["cap"],
		"the UI must be able to say what the limit is")
}

// An admin raising the cap for a big channel is the documented escape hatch.
func TestCreateInvite_AdminBypassesTheCap(t *testing.T) {
	store := &fakeGrantStore{}
	resp := do(grantRouter(grantHandler(t, repository.RoleOwner, store, nil), ownerID, "admin"),
		http.MethodPost, modsPath(), `{}`)

	require.Equal(t, http.StatusCreated, resp.Code)
	require.Len(t, store.created, 1)
	assert.True(t, store.created[0].BypassCap)

	t.Run("an ordinary owner does not", func(t *testing.T) {
		plain := &fakeGrantStore{}
		do(grantRouter(grantHandler(t, repository.RoleOwner, plain, nil), ownerID),
			http.MethodPost, modsPath(), `{}`)
		require.Len(t, plain.created, 1)
		assert.False(t, plain.created[0].BypassCap)
	})
}

// The gate keys on the OWNER, and here the owner IS the caller — but the copy still has to be
// owner-facing, because only the streamer can act on it.
func TestCreateInvite_GateClosedIsRefusedWithOwnerFacingCopy(t *testing.T) {
	store := &fakeGrantStore{}
	resp := do(grantRouter(grantHandler(t, repository.RoleOwner, store, delegationGateFor{}), ownerID),
		http.MethodPost, modsPath(), `{}`)

	assert.Equal(t, http.StatusForbidden, resp.Code)
	body := decode(t, resp)
	assert.Equal(t, "delegation_unavailable", body["code"])
	assert.Contains(t, body["error"], "premium")
	assert.Empty(t, store.created)
}

// Rolling delegation back must never trap a streamer with moderators they cannot remove.
func TestRevocation_IsNeverGated(t *testing.T) {
	t.Run("revoke one", func(t *testing.T) {
		store := &fakeGrantStore{revokedOK: true}
		resp := do(grantRouter(grantHandler(t, repository.RoleOwner, store, delegationGateFor{}), ownerID),
			http.MethodDelete, grantPath(), "")
		assert.Equal(t, http.StatusOK, resp.Code)
		assert.Len(t, store.revokedIDs, 1)
	})

	t.Run("revoke all", func(t *testing.T) {
		store := &fakeGrantStore{revokedAll: 3}
		resp := do(grantRouter(grantHandler(t, repository.RoleOwner, store, delegationGateFor{}), ownerID),
			http.MethodDelete, modsPath(), "")
		require.Equal(t, http.StatusOK, resp.Code)
		assert.EqualValues(t, 3, decode(t, resp)["revoked"])
	})

	t.Run("and neither is listing", func(t *testing.T) {
		store := &fakeGrantStore{}
		resp := do(grantRouter(grantHandler(t, repository.RoleOwner, store, delegationGateFor{}), ownerID),
			http.MethodGet, modsPath(), "")
		assert.Equal(t, http.StatusOK, resp.Code,
			"a streamer must always be able to see who can moderate for them")
	})
}

// ---------------------------------------------------------------------------
// Listing
// ---------------------------------------------------------------------------

func TestListModerators_ReportsRosterWithCapUsage(t *testing.T) {
	accepted := time.Now().Add(-time.Hour)
	store := &fakeGrantStore{grants: []repository.Grant{
		{ID: "g1", Status: models.GrantStatusActive, ModeratorUserID: modUserID,
			ModeratorDisplayName: "Sarah", Actions: []string{"delete", "timeout"},
			AcceptedAt: &accepted,
			Platforms: []repository.GrantLeg{
				{Platform: "twitch", Enabled: true, Verification: "verified"},
				{Platform: "discord", Enabled: false, Verification: "unverified"},
			}},
		{ID: "g2", Status: models.GrantStatusPending, InviteeLabel: "Bob", Actions: []string{"delete"}},
	}}

	resp := do(grantRouter(grantHandler(t, repository.RoleOwner, store, nil), ownerID),
		http.MethodGet, modsPath(), "")
	require.Equal(t, http.StatusOK, resp.Code)

	var list models.ModeratorList
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &list))
	require.Len(t, list.Moderators, 2)
	assert.Equal(t, models.ModeratorsPerOverlayCap, list.Cap)
	assert.Equal(t, 2, list.Used, "pending invites occupy a seat, so the count must include them")

	assert.Equal(t, "Sarah", list.Moderators[0].DisplayName)
	require.Len(t, list.Moderators[0].Platforms, 2)
	assert.True(t, list.Moderators[0].Platforms[0].Enabled)
	assert.Equal(t, "verified", list.Moderators[0].Platforms[0].Verification)
	assert.Equal(t, "Bob", list.Moderators[1].InviteeLabel)
	assert.Empty(t, list.Moderators[1].DisplayName)

	t.Run("an empty roster serializes as a list, not null", func(t *testing.T) {
		empty := &fakeGrantStore{grants: []repository.Grant{}}
		resp := do(grantRouter(grantHandler(t, repository.RoleOwner, empty, nil), ownerID),
			http.MethodGet, modsPath(), "")
		assert.Contains(t, resp.Body.String(), `"moderators":[]`)
	})
}

// ---------------------------------------------------------------------------
// Update
// ---------------------------------------------------------------------------

func TestUpdateGrant(t *testing.T) {
	t.Run("actions and legs are passed through", func(t *testing.T) {
		store := &fakeGrantStore{updated: repository.Grant{ID: "g1", Status: models.GrantStatusActive,
			Actions: []string{"delete"}}}
		resp := do(grantRouter(grantHandler(t, repository.RoleOwner, store, nil), ownerID),
			http.MethodPatch, grantPath(), `{"actions":["delete"],"platforms":{"twitch":true,"discord":false}}`)

		require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
		require.Len(t, store.updateCalls, 1)
		call := store.updateCalls[0]
		assert.Equal(t, overlayID, call.OverlayID, "the overlay must scope the write, not just the check")
		assert.Equal(t, []string{"delete"}, call.Actions)
		assert.Equal(t, map[string]bool{"twitch": true, "discord": false}, call.Legs)
	})

	t.Run("an absent field leaves that dimension alone", func(t *testing.T) {
		store := &fakeGrantStore{updated: repository.Grant{ID: "g1", Status: models.GrantStatusActive}}
		do(grantRouter(grantHandler(t, repository.RoleOwner, store, nil), ownerID),
			http.MethodPatch, grantPath(), `{"platforms":{"twitch":true}}`)
		require.Len(t, store.updateCalls, 1)
		assert.Nil(t, store.updateCalls[0].Actions, "nil means untouched, not cleared")
	})

	t.Run("an explicitly empty action list is refused", func(t *testing.T) {
		store := &fakeGrantStore{}
		resp := do(grantRouter(grantHandler(t, repository.RoleOwner, store, nil), ownerID),
			http.MethodPatch, grantPath(), `{"actions":[]}`)
		assert.Equal(t, http.StatusBadRequest, resp.Code)
		assert.Empty(t, store.updateCalls)
	})

	t.Run("a non-delegatable action is refused", func(t *testing.T) {
		store := &fakeGrantStore{}
		resp := do(grantRouter(grantHandler(t, repository.RoleOwner, store, nil), ownerID),
			http.MethodPatch, grantPath(), `{"actions":["engagement"]}`)
		assert.Equal(t, http.StatusBadRequest, resp.Code)
		assert.Empty(t, store.updateCalls)
	})

	t.Run("a leg for an unmoderatable platform is refused", func(t *testing.T) {
		store := &fakeGrantStore{}
		resp := do(grantRouter(grantHandler(t, repository.RoleOwner, store, nil), ownerID),
			http.MethodPatch, grantPath(), `{"platforms":{"tiktok":true}}`)
		assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)
		assert.Empty(t, store.updateCalls)
	})

	t.Run("an unknown grant", func(t *testing.T) {
		store := &fakeGrantStore{updateErr: repository.ErrGrantNotFound}
		resp := do(grantRouter(grantHandler(t, repository.RoleOwner, store, nil), ownerID),
			http.MethodPatch, grantPath(), `{"actions":["delete"]}`)
		assert.Equal(t, http.StatusNotFound, resp.Code)
		assert.Equal(t, "grant_not_found", decode(t, resp)["code"])
	})
}

// ---------------------------------------------------------------------------
// Revoke
// ---------------------------------------------------------------------------

func TestRevokeGrant(t *testing.T) {
	t.Run("a live grant", func(t *testing.T) {
		store := &fakeGrantStore{revokedOK: true}
		resp := do(grantRouter(grantHandler(t, repository.RoleOwner, store, nil), ownerID),
			http.MethodDelete, grantPath(), "")
		require.Equal(t, http.StatusOK, resp.Code)
		assert.Equal(t, true, decode(t, resp)["revoked"])
	})

	t.Run("nothing to revoke", func(t *testing.T) {
		store := &fakeGrantStore{revokedOK: false}
		resp := do(grantRouter(grantHandler(t, repository.RoleOwner, store, nil), ownerID),
			http.MethodDelete, grantPath(), "")
		assert.Equal(t, http.StatusNotFound, resp.Code)
		assert.Equal(t, "grant_not_found", decode(t, resp)["code"])
	})
}

// ---------------------------------------------------------------------------
// Preview
// ---------------------------------------------------------------------------

func TestPreviewInvite(t *testing.T) {
	expires := time.Now().Add(invites.TTL)
	details := repository.InviteDetails{
		Grant: repository.Grant{
			ID: "g1", OverlayID: overlayID, Status: models.GrantStatusPending,
			Actions: []string{"delete", "timeout"}, InviteeLabel: "@sarah",
			InviteExpiresAt: &expires,
			Platforms:       []repository.GrantLeg{{Platform: "twitch", Enabled: true, Verification: "unverified"}},
		},
		OverlayName:      "Main Overlay",
		OwnerDisplayName: "The Streamer",
	}

	t.Run("says who is asking and for what, without needing an account of any kind", func(t *testing.T) {
		store := &fakeGrantStore{preview: details}
		resp := do(grantRouter(grantHandler(t, repository.RoleNone, store, nil), modUserID),
			http.MethodPost, "/api/v1/moderation/invites/preview", `{"token":"a-secret"}`)

		require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
		body := decode(t, resp)
		assert.Equal(t, "Main Overlay", body["overlay_name"])
		assert.Equal(t, "The Streamer", body["owner_display_name"])
		assert.Equal(t, []any{"delete", "timeout"}, body["actions"])

		// An overlay UUID already grants chat READ to anyone holding it, so it is disclosed on
		// acceptance rather than to everyone who merely opens the link.
		assert.NotContains(t, resp.Body.String(), overlayID)
		assert.NotContains(t, body, "overlay_id")

		require.Len(t, store.seenHashes, 1)
		assert.Equal(t, invites.Hash("a-secret"), store.seenHashes[0])
	})

	t.Run("a pasted secret is trimmed before hashing", func(t *testing.T) {
		store := &fakeGrantStore{preview: details}
		do(grantRouter(grantHandler(t, repository.RoleNone, store, nil), modUserID),
			http.MethodPost, "/api/v1/moderation/invites/preview", `{"token":"  a-secret\n"}`)
		require.Len(t, store.seenHashes, 1)
		assert.Equal(t, invites.Hash("a-secret"), store.seenHashes[0],
			"a copy-paste newline must not read as a different secret")
	})

	t.Run("an unknown secret", func(t *testing.T) {
		store := &fakeGrantStore{previewErr: repository.ErrInviteNotFound}
		resp := do(grantRouter(grantHandler(t, repository.RoleNone, store, nil), modUserID),
			http.MethodPost, "/api/v1/moderation/invites/preview", `{"token":"x"}`)
		assert.Equal(t, http.StatusNotFound, resp.Code)
		assert.Equal(t, "invite_not_found", decode(t, resp)["code"])
	})

	t.Run("an expired secret says so, because the holder already knows it was real", func(t *testing.T) {
		store := &fakeGrantStore{previewErr: repository.ErrInviteExpired}
		resp := do(grantRouter(grantHandler(t, repository.RoleNone, store, nil), modUserID),
			http.MethodPost, "/api/v1/moderation/invites/preview", `{"token":"x"}`)
		assert.Equal(t, http.StatusGone, resp.Code)
		assert.Equal(t, "invite_expired", decode(t, resp)["code"])
	})

	t.Run("a missing token is a bad request", func(t *testing.T) {
		store := &fakeGrantStore{}
		resp := do(grantRouter(grantHandler(t, repository.RoleNone, store, nil), modUserID),
			http.MethodPost, "/api/v1/moderation/invites/preview", `{}`)
		assert.Equal(t, http.StatusBadRequest, resp.Code)
		assert.Empty(t, store.seenHashes)
	})
}

// ---------------------------------------------------------------------------
// Accept
// ---------------------------------------------------------------------------

func TestAcceptInvite(t *testing.T) {
	accepted := repository.InviteDetails{
		Grant: repository.Grant{
			ID: "g1", OverlayID: overlayID, Status: models.GrantStatusActive,
			ModeratorUserID: modUserID, Actions: []string{"delete", "timeout"},
			Platforms: []repository.GrantLeg{{Platform: "twitch", Enabled: true, Verification: "unverified"}},
		},
		OverlayName:      "Main Overlay",
		OwnerDisplayName: "The Streamer",
	}

	t.Run("binds to the signed-in account and hands over the overlay", func(t *testing.T) {
		store := &fakeGrantStore{accepted: accepted}
		resp := do(grantRouter(grantHandler(t, repository.RoleNone, store, nil), modUserID),
			http.MethodPost, "/api/v1/moderation/invites/accept", `{"token":"a-secret"}`)

		require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
		body := decode(t, resp)
		assert.Equal(t, overlayID, body["overlay_id"], "acceptance is where the overlay is disclosed")
		assert.Equal(t, "Main Overlay", body["overlay_name"])
		assert.Equal(t, []any{"delete", "timeout"}, body["actions"])
		assert.Equal(t, []string{modUserID}, store.acceptedFor)
	})

	t.Run("an anonymous caller cannot accept", func(t *testing.T) {
		store := &fakeGrantStore{accepted: accepted}
		resp := do(grantRouter(grantHandler(t, repository.RoleNone, store, nil), ""),
			http.MethodPost, "/api/v1/moderation/invites/accept", `{"token":"a-secret"}`)
		assert.Equal(t, http.StatusUnauthorized, resp.Code)
		assert.Empty(t, store.acceptedFor)
	})

	refusals := []struct {
		name string
		err  error
		code int
		want string
	}{
		{"unknown", repository.ErrInviteNotFound, http.StatusNotFound, "invite_not_found"},
		{"expired", repository.ErrInviteExpired, http.StatusGone, "invite_expired"},
		{"already a moderator", repository.ErrAlreadyModerator, http.StatusConflict, "already_moderator"},
		{"the owner themselves", repository.ErrOwnerCannotAccept, http.StatusConflict, "owner_cannot_accept"},
	}
	for _, tc := range refusals {
		t.Run(tc.name, func(t *testing.T) {
			store := &fakeGrantStore{acceptErr: tc.err}
			resp := do(grantRouter(grantHandler(t, repository.RoleNone, store, nil), modUserID),
				http.MethodPost, "/api/v1/moderation/invites/accept", `{"token":"x"}`)
			assert.Equal(t, tc.code, resp.Code)
			assert.Equal(t, tc.want, decode(t, resp)["code"])
		})
	}

	// The whole point of pre-binding: tell Bob the invite is Sarah's instead of silently making
	// him a moderator or failing with something unhelpful.
	t.Run("the wrong account is told whose invite it is", func(t *testing.T) {
		store := &fakeGrantStore{
			acceptErr: repository.ErrInviteBoundToOtherAccount,
			accepted: repository.InviteDetails{Grant: repository.Grant{
				InviteeLabel: "@sarah", ExpectedPlatform: "twitch",
			}},
		}
		resp := do(grantRouter(grantHandler(t, repository.RoleNone, store, nil), modUserID),
			http.MethodPost, "/api/v1/moderation/invites/accept", `{"token":"x"}`)

		assert.Equal(t, http.StatusConflict, resp.Code)
		body := decode(t, resp)
		assert.Equal(t, "invite_bound_to_other_account", body["code"])
		assert.Equal(t, "@sarah", body["expected_account"])
		assert.Equal(t, "twitch", body["expected_platform"])
	})
}

// The secret is a bearer credential for a moderation grant, so it must not be reachable through a
// URL, where it would land in access logs, proxy logs and Referer headers.
func TestInviteEndpoints_TakeTheSecretInABodyNotAPath(t *testing.T) {
	store := &fakeGrantStore{preview: repository.InviteDetails{}}
	router := grantRouter(grantHandler(t, repository.RoleNone, store, nil), modUserID)

	for _, path := range []string{
		"/api/v1/moderation/invites/preview/a-secret",
		"/api/v1/moderation/invites/a-secret",
		"/api/v1/moderation/invites/accept/a-secret",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		assert.Equal(t, http.StatusNotFound, resp.Code, "no route may carry a secret in its path: %s", path)
	}
	assert.Empty(t, store.seenHashes)
}

func TestGrantHandler_StoreFailuresAreInternalErrors(t *testing.T) {
	boom := errors.New("database on fire")
	cases := []struct {
		name, method, path, body string
		store                    *fakeGrantStore
	}{
		{"list", http.MethodGet, modsPath(), "", &fakeGrantStore{listErr: boom}},
		{"invite", http.MethodPost, modsPath(), `{}`, &fakeGrantStore{createErr: boom}},
		{"update", http.MethodPatch, grantPath(), `{"actions":["delete"]}`, &fakeGrantStore{updateErr: boom}},
		{"revoke", http.MethodDelete, grantPath(), "", &fakeGrantStore{revokeErr: boom}},
		{"revoke all", http.MethodDelete, modsPath(), "", &fakeGrantStore{revokeErr: boom}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := do(grantRouter(grantHandler(t, repository.RoleOwner, tc.store, nil), ownerID),
				tc.method, tc.path, tc.body)
			assert.Equal(t, http.StatusInternalServerError, resp.Code)
			assert.NotContains(t, resp.Body.String(), "on fire", "internals must not leak to the client")
		})
	}
}
