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

package handlers

// Tests for the dashboard half of device linking: approve, the paired-devices list, and
// revoke.
//
// Two properties are load-bearing here and each has its own test:
//
//   - SESSION ONLY. Every route refuses a token-authenticated request, so a device token
//     cannot mint more devices or revoke the streamer's other devices. This mirrors
//     api_tokens.go's requireSelf and middleware.AdminOnly.
//   - NO SECRET EVER REACHES THE BROWSER. Unlike the PAT surface there is no "shown once"
//     plaintext at all: a device token's secret goes to the plugin over the loopback. A
//     response here carrying one would be the bug.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/caesar/all-chat/services/auth-service/repository"
	"github.com/caesar/all-chat/shared/middleware"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// deviceRouter mounts the four dashboard routes behind a context-injecting stub standing
// in for JWTAuthWithRevocation.
func devicesRouter(store deviceStore, owner overlayOwnershipChecker, ctxValues map[string]string) *gin.Engine {
	router := setupTestRouter()
	h := newDeviceHandlerWithStore(store, owner, zap.NewNop())
	inject := func(c *gin.Context) {
		for k, v := range ctxValues {
			c.Set(k, v)
		}
		c.Next()
	}
	router.GET("/me/devices", inject, h.HandleListDevices)
	router.GET("/me/devices/pending", inject, h.HandleGetPendingLink)
	router.POST("/me/devices/approve", inject, h.HandleApproveDevice)
	router.DELETE("/me/devices/:id", inject, h.HandleRevokeDevice)
	return router
}

func pendingLoopbackRequest() *repository.LinkRequest {
	return &repository.LinkRequest{
		ID:              testLinkRequestID,
		Flow:            repository.FlowLoopback,
		DeviceName:      "Stream Deck (self-reported)",
		RequestedScopes: []string{middleware.ScopeChatWrite, middleware.ScopeEngagementWrite},
		PKCEChallenge:   testChallenge(),
		PKCEMethod:      "S256",
		RedirectURI:     "http://127.0.0.1:51234" + LoopbackPath,
		ExpiresAt:       time.Now().Add(repository.LinkRequestTTL),
	}
}

func tokenCtx() map[string]string {
	return map[string]string{
		"user_id":                testUserID,
		middleware.CtxAuthMethod: middleware.AuthMethodAPIToken,
		middleware.CtxTokenKind:  middleware.TokenKindDevice,
	}
}

func doJSON(router *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	}
	router.ServeHTTP(w, req)
	return w
}

func TestDeviceRoutes_RefuseTokenAuthenticatedRequests(t *testing.T) {
	// A device token authenticating against the device-management surface would be a
	// self-renewing foothold: mint a fresh device, then revoke the streamer's others so
	// they cannot lock it out. Every route says no, in one place per route.
	store := &fakeDeviceLinkStore{pending: pendingLoopbackRequest()}
	router := devicesRouter(store, fakeOverlayOwner{owns: true}, tokenCtx())

	for _, tc := range []struct{ method, path, body string }{
		{http.MethodGet, "/me/devices", ""},
		{http.MethodGet, "/me/devices/pending?request_id=" + testLinkRequestID, ""},
		{http.MethodPost, "/me/devices/approve", `{"request_id":"` + testLinkRequestID +
			`","overlay_id":"` + testDeviceOverlayID + `","scopes":["chat:write"]}`},
		{http.MethodDelete, "/me/devices/" + testDeviceID, ""},
	} {
		w := doJSON(router, tc.method, tc.path, tc.body)
		if w.Code != http.StatusForbidden {
			t.Errorf("%s %s: status = %d, want 403 for a token-authenticated request; body=%s",
				tc.method, tc.path, w.Code, w.Body.String())
		}
	}
}

func TestDeviceRoutes_RefuseImpersonation(t *testing.T) {
	// An admin acting as a user must not walk away with a credential that outlives the
	// impersonation session, and must not be able to cut the user's devices off either.
	store := &fakeDeviceLinkStore{pending: pendingLoopbackRequest()}
	ctx := sessionCtx()
	ctx["impersonated_by"] = "admin-user-id"
	router := devicesRouter(store, fakeOverlayOwner{owns: true}, ctx)

	w := doJSON(router, http.MethodPost, "/me/devices/approve",
		`{"request_id":"`+testLinkRequestID+`","overlay_id":"`+testDeviceOverlayID+`","scopes":["chat:write"]}`)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 while impersonating; body=%s", w.Code, w.Body.String())
	}
	w = doJSON(router, http.MethodDelete, "/me/devices/"+testDeviceID, "")
	if w.Code != http.StatusForbidden {
		t.Fatalf("revoke status = %d, want 403 while impersonating", w.Code)
	}
}

func TestGetPendingLink_LabelsTheDeviceNameAsSelfReported(t *testing.T) {
	// The name comes from the plugin, so it is a claim, not a fact. The JSON field name
	// carries that so no client author can render it as trustworthy by accident.
	store := &fakeDeviceLinkStore{pending: pendingLoopbackRequest()}
	w := doJSON(devicesRouter(store, fakeOverlayOwner{owns: true}, sessionCtx()),
		http.MethodGet, "/me/devices/pending?request_id="+testLinkRequestID, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "device_name_self_reported") {
		t.Errorf("pending response does not label the device name as self-reported: %s", w.Body.String())
	}
	var resp pendingLinkResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.RequestedScopes) != 2 {
		t.Errorf("requested_scopes = %v, want what the plugin asked for so the streamer can see it",
			resp.RequestedScopes)
	}
}

func TestApproveDevice_BindsOverlayAndReturnsARedirect(t *testing.T) {
	store := &fakeDeviceLinkStore{pending: pendingLoopbackRequest()}
	w := doJSON(devicesRouter(store, fakeOverlayOwner{owns: true}, sessionCtx()),
		http.MethodPost, "/me/devices/approve",
		`{"request_id":"`+testLinkRequestID+`","overlay_id":"`+testDeviceOverlayID+
			`","scopes":["engagement:write"],"device_name":"Studio deck"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}

	var resp approveDeviceResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.OverlayID != testDeviceOverlayID {
		t.Errorf("overlay_id = %q, want the approved overlay", resp.OverlayID)
	}
	if !strings.Contains(resp.RedirectTo, "/device/link/callback?request_id=") {
		t.Errorf("redirect_to = %q, want the server-side callback that emits the Location header",
			resp.RedirectTo)
	}
	// The one-time code's DIGEST is what was stored.
	if len(store.lastApproveHash) != 32 {
		t.Errorf("stored auth code hash is %d bytes, want a 32-byte SHA-256", len(store.lastApproveHash))
	}
	if store.lastApproveName != "Studio deck" {
		t.Errorf("device_name = %q, want the streamer's override of the self-reported name",
			store.lastApproveName)
	}
	// No secret in the response body other than the one-time code in the redirect, which
	// is not the token. A device token must never appear here.
	if strings.Contains(w.Body.String(), middleware.DeviceTokenPrefix) {
		t.Fatalf("the approve response carries a device token: %s", w.Body.String())
	}
}

func TestApproveDevice_CodeFlowHasNothingToRedirectTo(t *testing.T) {
	// The fallback's plugin is polling, so there is nowhere for the browser to go. An
	// empty redirect_to is the signal the dashboard uses to say "return to your plugin".
	pending := pendingLoopbackRequest()
	pending.Flow = repository.FlowCode
	pending.RedirectURI = ""
	store := &fakeDeviceLinkStore{pending: pending}
	w := doJSON(devicesRouter(store, fakeOverlayOwner{owns: true}, sessionCtx()),
		http.MethodPost, "/me/devices/approve",
		`{"request_id":"`+testLinkRequestID+`","overlay_id":"`+testDeviceOverlayID+
			`","scopes":["chat:write"]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	var resp approveDeviceResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.RedirectTo != "" {
		t.Errorf("redirect_to = %q, want empty for the code flow", resp.RedirectTo)
	}
}

func TestApproveDevice_CannotGrantAnUnrequestedScope(t *testing.T) {
	// A dashboard bug must not be able to hand a device more than it asked for. The
	// plugin's request is the ceiling; the streamer may only narrow it.
	pending := pendingLoopbackRequest()
	pending.RequestedScopes = []string{middleware.ScopeEngagementWrite}
	store := &fakeDeviceLinkStore{pending: pending}
	w := doJSON(devicesRouter(store, fakeOverlayOwner{owns: true}, sessionCtx()),
		http.MethodPost, "/me/devices/approve",
		`{"request_id":"`+testLinkRequestID+`","overlay_id":"`+testDeviceOverlayID+
			`","scopes":["chat:write","engagement:write"]}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 when granting a scope the device never requested; body=%s",
			w.Code, w.Body.String())
	}
	if store.lastApproveHash != nil {
		t.Fatal("an over-broad grant reached persistence")
	}
}

func TestApproveDevice_RefusesAnOverlayTheUserDoesNotOwn(t *testing.T) {
	// 404, not 403: whether an overlay id exists is not something this endpoint should
	// teach an unrelated session. And this is the last gate before the binding that the
	// whole per-overlay property rests on.
	store := &fakeDeviceLinkStore{pending: pendingLoopbackRequest()}
	w := doJSON(devicesRouter(store, fakeOverlayOwner{owns: false}, sessionCtx()),
		http.MethodPost, "/me/devices/approve",
		`{"request_id":"`+testLinkRequestID+`","overlay_id":"`+testDeviceOverlayID+
			`","scopes":["chat:write"]}`)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body=%s", w.Code, w.Body.String())
	}
	if store.lastApproveHash != nil {
		t.Fatal("an unowned overlay reached persistence as a device binding")
	}
}

func TestApproveDevice_DenyTerminatesTheRequest(t *testing.T) {
	// Deny must end the row, or the plugin polls until the TTL expires with no idea the
	// streamer said no.
	store := &fakeDeviceLinkStore{pending: pendingLoopbackRequest()}
	w := doJSON(devicesRouter(store, fakeOverlayOwner{owns: true}, sessionCtx()),
		http.MethodPost, "/me/devices/approve",
		`{"request_id":"`+testLinkRequestID+`","overlay_id":"`+testDeviceOverlayID+
			`","scopes":["chat:write"],"deny":true}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if !store.denied {
		t.Fatal("Deny did not terminate the link request")
	}
	if store.lastApproveHash != nil {
		t.Fatal("Deny also approved the request")
	}
}

func TestApproveDevice_Validation(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"no identifier", `{"overlay_id":"` + testDeviceOverlayID + `","scopes":["chat:write"]}`},
		{"bad overlay id", `{"request_id":"` + testLinkRequestID + `","overlay_id":"nope","scopes":["chat:write"]}`},
		{"missing scopes", `{"request_id":"` + testLinkRequestID + `","overlay_id":"` + testDeviceOverlayID + `"}`},
		{"unknown scope", `{"request_id":"` + testLinkRequestID + `","overlay_id":"` + testDeviceOverlayID +
			`","scopes":["admin:*"]}`},
		{"bad request id", `{"request_id":"nope","overlay_id":"` + testDeviceOverlayID + `","scopes":["chat:write"]}`},
		{"garbage body", `not json`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := &fakeDeviceLinkStore{pending: pendingLoopbackRequest()}
			w := doJSON(devicesRouter(store, fakeOverlayOwner{owns: true}, sessionCtx()),
				http.MethodPost, "/me/devices/approve", tc.body)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body=%s", w.Code, w.Body.String())
			}
			if store.lastApproveHash != nil {
				t.Fatal("an invalid approve reached persistence")
			}
		})
	}
}

func TestApproveDevice_ExpiredRequestIs404(t *testing.T) {
	store := &fakeDeviceLinkStore{pendingErr: repository.ErrLinkRequestNotFound}
	w := doJSON(devicesRouter(store, fakeOverlayOwner{owns: true}, sessionCtx()),
		http.MethodPost, "/me/devices/approve",
		`{"request_id":"`+testLinkRequestID+`","overlay_id":"`+testDeviceOverlayID+`","scopes":["chat:write"]}`)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 for an expired request; body=%s", w.Code, w.Body.String())
	}
}

func TestApproveDevice_LookupByTypedCode(t *testing.T) {
	// The fallback's approve: the streamer is on another machine and types the code into
	// /link, so the request is identified by code rather than by an id in the URL.
	pending := pendingLoopbackRequest()
	pending.Flow = repository.FlowCode
	pending.RedirectURI = ""
	store := &fakeDeviceLinkStore{byCode: pending}
	w := doJSON(devicesRouter(store, fakeOverlayOwner{owns: true}, sessionCtx()),
		http.MethodPost, "/me/devices/approve",
		`{"user_code":"abcd-efgh","overlay_id":"`+testDeviceOverlayID+`","scopes":["chat:write"]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if store.lastApproveUser != testUserID {
		t.Errorf("approved as %q, want the signed-in streamer", store.lastApproveUser)
	}
}

func TestApproveDevice_UnknownTypedCodeIs404(t *testing.T) {
	store := &fakeDeviceLinkStore{byCodeErr: repository.ErrLinkRequestNotFound}
	w := doJSON(devicesRouter(store, fakeOverlayOwner{owns: true}, sessionCtx()),
		http.MethodPost, "/me/devices/approve",
		`{"user_code":"ABCD-EFGH","overlay_id":"`+testDeviceOverlayID+`","scopes":["chat:write"]}`)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body=%s", w.Code, w.Body.String())
	}
}

func TestListDevices_ExposesMetadataAndNeverASecret(t *testing.T) {
	last := time.Now().Add(-2 * time.Hour)
	store := &fakeDeviceLinkStore{listed: []repository.DeviceToken{{
		ID: testDeviceID, Name: "Stream Deck", OverlayID: testDeviceOverlayID,
		OverlayName: "Main overlay", Scopes: []string{middleware.ScopeEngagementWrite},
		CreatedAt: time.Now().Add(-72 * time.Hour), LastUsedAt: &last,
		ExpiresAt: time.Now().Add(middleware.DeviceTokenLifetime),
	}}}
	w := doJSON(devicesRouter(store, fakeOverlayOwner{owns: true}, sessionCtx()),
		http.MethodGet, "/me/devices", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	// Everything the paired-devices list needs, and nothing that could be a credential.
	for _, want := range []string{"last_used_at", "expires_at", "overlay_name", "scopes"} {
		if !strings.Contains(body, want) {
			t.Errorf("list response is missing %q: %s", want, body)
		}
	}
	for _, forbidden := range []string{"token_hash", middleware.DeviceTokenPrefix, `"token"`} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("the paired-devices list exposes %q. There is no plaintext for this "+
				"surface to render: a device token goes to the plugin over the loopback, never "+
				"to the browser. Body: %s", forbidden, body)
		}
	}
}

func TestRevokeDevice_RevokesAndRejectsAMalformedID(t *testing.T) {
	store := &fakeDeviceLinkStore{}
	router := devicesRouter(store, fakeOverlayOwner{owns: true}, sessionCtx())

	w := doJSON(router, http.MethodDelete, "/me/devices/"+testDeviceID, "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if store.revokedForID != testDeviceID {
		t.Errorf("revoked %q, want %q", store.revokedForID, testDeviceID)
	}

	// Rejected before the query, so a malformed id is a clean 400 rather than a
	// PostgreSQL cast error surfacing as a 500.
	w = doJSON(router, http.MethodDelete, "/me/devices/not-a-uuid", "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("malformed id status = %d, want 400", w.Code)
	}
}

func TestRevokeDevice_AnotherUsersDeviceIs404(t *testing.T) {
	// Indistinguishable from a nonexistent id, so this cannot enumerate other users'
	// devices.
	store := &fakeDeviceLinkStore{revokeErr: repository.ErrNotFound}
	w := doJSON(devicesRouter(store, fakeOverlayOwner{owns: true}, sessionCtx()),
		http.MethodDelete, "/me/devices/"+testDeviceID, "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body=%s", w.Code, w.Body.String())
	}
}

func TestDeviceRoutes_RejectAnonymous(t *testing.T) {
	store := &fakeDeviceLinkStore{pending: pendingLoopbackRequest()}
	router := devicesRouter(store, fakeOverlayOwner{owns: true}, nil)
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/me/devices"},
		{http.MethodGet, "/me/devices/pending?request_id=" + testLinkRequestID},
		{http.MethodDelete, "/me/devices/" + testDeviceID},
	} {
		if w := doJSON(router, tc.method, tc.path, ""); w.Code != http.StatusUnauthorized {
			t.Errorf("%s %s: status = %d, want 401", tc.method, tc.path, w.Code)
		}
	}
}
