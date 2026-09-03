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

// Tests for the device-linking endpoints (ADR-0049 steps 2-3), over a fake store so the
// request shaping, the PKCE rules and the replay behaviour are all exercised without a
// database.
//
// THE PAIRING-CODE FALLBACK IS TESTED DELIBERATELY, not incidentally. ADR-0049 says so
// in as many words:
//
//	Two linking paths mean two paths to keep correct. The fallback will be used rarely,
//	which is exactly why it will rot silently unless it is tested deliberately rather
//	than only when someone reports it.
//
// So the code flow gets its own start/approve/exchange coverage, its own attempt-counter
// assertions and its own normalisation cases, at parity with the loopback path.

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
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

const (
	testDeviceOverlayID = "22222222-2222-2222-2222-222222222222"
	testLinkRequestID   = "33333333-3333-3333-3333-333333333333"
	testDeviceID        = "44444444-4444-4444-4444-444444444444"
	// A 43-character base64url verifier, the RFC 7636 minimum.
	testVerifier = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
)

// testChallenge is S256(testVerifier), which is what a plugin sends at start.
func testChallenge() string {
	sum := sha256.Sum256([]byte(testVerifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// fakeDeviceLinkStore records what the handlers asked for and returns canned results.
// It implements both deviceLinkStore and deviceStore, so one fake serves the plugin-side
// and dashboard-side handlers and a test can drive an end-to-end link.
type fakeDeviceLinkStore struct {
	// created is what CreateLinkRequest returns; the Last* fields record its arguments.
	created          *repository.LinkRequest
	createErr        error
	lastFlow         string
	lastUserCodeHash []byte
	lastChallenge    string
	lastMethod       string
	lastRedirect     string
	lastDeviceName   string
	lastScopes       []string
	createLinkCalled bool

	pending    *repository.LinkRequest
	pendingErr error

	byCode    *repository.LinkRequest
	byCodeErr error

	attemptCalls int
	attemptDead  bool
	attemptErr   error

	approved         *repository.LinkRequest
	approveErr       error
	lastApproveHash  []byte
	lastApproveUser  string
	lastApproveScope []string
	lastApproveName  string

	denied bool

	consumed    *repository.LinkRequest
	consumeErr  error
	lastClaim   []byte
	consumeSeen int

	device       *repository.DeviceToken
	deviceErr    error
	lastTokenHsh []byte
	lastLifetime time.Duration

	revokedByID  string
	revokeIDErr  error
	listed       []repository.DeviceToken
	listErr      error
	revoked      *repository.DeviceToken
	revokeErr    error
	revokedForID string
}

func (f *fakeDeviceLinkStore) CreateLinkRequest(_ context.Context, flow string, userCodeHash []byte,
	challenge, method, redirect, deviceName string, scopes []string, _ time.Duration,
) (*repository.LinkRequest, error) {
	f.createLinkCalled = true
	f.lastFlow, f.lastUserCodeHash, f.lastChallenge = flow, userCodeHash, challenge
	f.lastMethod, f.lastRedirect, f.lastDeviceName, f.lastScopes = method, redirect, deviceName, scopes
	if f.createErr != nil {
		return nil, f.createErr
	}
	if f.created != nil {
		return f.created, nil
	}
	return &repository.LinkRequest{
		ID: testLinkRequestID, Flow: flow, DeviceName: deviceName, RequestedScopes: scopes,
		RedirectURI: redirect, PKCEChallenge: challenge, PKCEMethod: method,
		ExpiresAt: time.Now().Add(repository.LinkRequestTTL),
	}, nil
}

func (f *fakeDeviceLinkStore) GetPendingLinkRequest(_ context.Context, _ string) (*repository.LinkRequest, error) {
	if f.pendingErr != nil {
		return nil, f.pendingErr
	}
	if f.pending == nil {
		return nil, repository.ErrLinkRequestNotFound
	}
	return f.pending, nil
}

func (f *fakeDeviceLinkStore) FindPendingByUserCode(_ context.Context, _ []byte) (*repository.LinkRequest, error) {
	if f.byCodeErr != nil {
		return nil, f.byCodeErr
	}
	if f.byCode == nil {
		return nil, repository.ErrLinkRequestNotFound
	}
	return f.byCode, nil
}

func (f *fakeDeviceLinkStore) RegisterFailedAttempt(_ context.Context, _ string) (bool, error) {
	f.attemptCalls++
	return f.attemptDead, f.attemptErr
}

func (f *fakeDeviceLinkStore) ApproveLinkRequest(_ context.Context, id, userID, overlayID string,
	granted []string, deviceName string, authCodeHash []byte, _ time.Duration,
) (*repository.LinkRequest, error) {
	f.lastApproveHash, f.lastApproveUser = authCodeHash, userID
	f.lastApproveScope, f.lastApproveName = granted, deviceName
	if f.approveErr != nil {
		return nil, f.approveErr
	}
	if f.approved != nil {
		return f.approved, nil
	}
	now := time.Now()
	flow := repository.FlowLoopback
	if f.pending != nil {
		flow = f.pending.Flow
	}
	name := deviceName
	if name == "" && f.pending != nil {
		name = f.pending.DeviceName
	}
	return &repository.LinkRequest{
		ID: id, Flow: flow, DeviceName: name, UserID: userID, OverlayID: overlayID,
		GrantedScopes: granted, ApprovedAt: &now,
		ExpiresAt: time.Now().Add(repository.LinkRequestTTL),
	}, nil
}

func (f *fakeDeviceLinkStore) DenyLinkRequest(_ context.Context, _ string) error {
	f.denied = true
	return nil
}

func (f *fakeDeviceLinkStore) ConsumeAuthCode(_ context.Context, _ string, claim []byte) (*repository.LinkRequest, error) {
	f.consumeSeen++
	f.lastClaim = claim
	if f.consumeErr != nil {
		return f.consumed, f.consumeErr
	}
	if f.consumed == nil {
		return nil, repository.ErrLinkRequestNotFound
	}
	return f.consumed, nil
}

func (f *fakeDeviceLinkStore) CreateDeviceToken(_ context.Context, userID, overlayID, name string,
	tokenHash []byte, scopes []string, lifetime time.Duration, _ string,
) (*repository.DeviceToken, error) {
	f.lastTokenHsh, f.lastLifetime = tokenHash, lifetime
	if f.deviceErr != nil {
		return nil, f.deviceErr
	}
	if f.device != nil {
		return f.device, nil
	}
	return &repository.DeviceToken{
		ID: testDeviceID, Name: name, OverlayID: overlayID, Scopes: scopes,
		CreatedAt: time.Now(), ExpiresAt: time.Now().Add(lifetime),
	}, nil
}

func (f *fakeDeviceLinkStore) RevokeDeviceTokenByID(_ context.Context, deviceID string) error {
	f.revokedByID = deviceID
	return f.revokeIDErr
}

func (f *fakeDeviceLinkStore) ListDeviceTokensByUser(_ context.Context, _ string) ([]repository.DeviceToken, error) {
	return f.listed, f.listErr
}

func (f *fakeDeviceLinkStore) RevokeDeviceToken(_ context.Context, _, deviceID string) (*repository.DeviceToken, error) {
	f.revokedForID = deviceID
	if f.revokeErr != nil {
		return nil, f.revokeErr
	}
	if f.revoked != nil {
		return f.revoked, nil
	}
	now := time.Now()
	return &repository.DeviceToken{ID: deviceID, Name: "revoked", Scopes: []string{},
		CreatedAt: now, ExpiresAt: now.Add(time.Hour), RevokedAt: &now}, nil
}

// fakeOverlayOwner answers the one ownership question the approve handler asks.
type fakeOverlayOwner struct {
	owns bool
	err  error
}

func (f fakeOverlayOwner) UserOwnsOverlay(context.Context, string, string) (bool, error) {
	return f.owns, f.err
}

// deviceLinkRouter mounts the plugin-facing routes (unauthenticated) plus the
// session-only callback behind a context-injecting stub, exactly as main.go wires them.
func deviceLinkRouter(store *fakeDeviceLinkStore, ctxValues map[string]string) *gin.Engine {
	router := setupTestRouter()
	h := newDeviceLinkHandlerWithStore(store, "https://allch.at", zap.NewNop())
	inject := func(c *gin.Context) {
		for k, v := range ctxValues {
			c.Set(k, v)
		}
		c.Next()
	}
	router.POST("/device/link/start", h.HandleStartDeviceLink)
	router.POST("/device/link/exchange", h.HandleExchangeDeviceLink)
	router.GET("/device/link/callback", inject, h.HandleDeviceLinkCallback)
	return router
}

func postJSON(router *gin.Engine, path, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	return w
}

func TestStartDeviceLink_LoopbackFlow(t *testing.T) {
	store := &fakeDeviceLinkStore{}
	body := `{"flow":"loopback","device_name":"Stream Deck (studio)","scopes":["engagement:write"],` +
		`"code_challenge":"` + testChallenge() + `","code_challenge_method":"S256",` +
		`"redirect_uri":"http://127.0.0.1:51234` + LoopbackPath + `"}`
	w := postJSON(deviceLinkRouter(store, nil), "/device/link/start", body)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", w.Code, w.Body.String())
	}

	var resp startLinkResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.RequestID != testLinkRequestID {
		t.Errorf("request_id = %q", resp.RequestID)
	}
	// The loopback flow shows the streamer NOTHING to read. A user_code here would mean
	// the primary path had quietly acquired the fallback's paste-a-value problem.
	if resp.UserCode != "" {
		t.Errorf("loopback flow returned a user_code (%q); nothing should be displayed", resp.UserCode)
	}
	if !strings.Contains(resp.VerificationURI, "/link?request_id=") {
		t.Errorf("verification_uri = %q, want the /link page with the request id", resp.VerificationURI)
	}
	if resp.Interval <= 0 || resp.ExpiresIn <= 0 {
		t.Errorf("interval/expires_in = %d/%d, want positive hints for the poller", resp.Interval, resp.ExpiresIn)
	}
	if store.lastRedirect != "http://127.0.0.1:51234"+LoopbackPath {
		t.Errorf("stored redirect = %q", store.lastRedirect)
	}
	if store.lastUserCodeHash != nil {
		t.Error("loopback flow stored a user_code_hash")
	}
}

func TestStartDeviceLink_CodeFlowShowsAGroupedCode(t *testing.T) {
	store := &fakeDeviceLinkStore{}
	body := `{"flow":"code","device_name":"StreamController","scopes":["chat:write"],` +
		`"code_challenge":"` + testChallenge() + `"}`
	w := postJSON(deviceLinkRouter(store, nil), "/device/link/start", body)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", w.Code, w.Body.String())
	}
	var resp startLinkResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// XXXX-XXXX, from the unambiguous alphabet only.
	if len(resp.UserCode) != userCodeLength+1 || resp.UserCode[4] != '-' {
		t.Fatalf("user_code = %q, want XXXX-XXXX", resp.UserCode)
	}
	for _, r := range strings.ReplaceAll(resp.UserCode, "-", "") {
		if !strings.ContainsRune(userCodeAlphabet, r) {
			t.Errorf("user_code %q contains %q, which is not in the unambiguous alphabet "+
				"(no 0/O, no 1/I/L — a streamer reads this off a screen and types it)", resp.UserCode, r)
		}
	}
	// The code flow sends the streamer to the bare /link page: the whole point is that
	// they are on a different machine, so there is no request id in their URL.
	if strings.Contains(resp.VerificationURI, "request_id") {
		t.Errorf("verification_uri = %q, want the bare /link page for the code flow", resp.VerificationURI)
	}
	// Only the digest is stored, and it is the digest of the code that was displayed.
	if store.lastUserCodeHash == nil {
		t.Fatal("code flow did not store a user_code_hash")
	}
	want := hashUserCode(strings.ReplaceAll(resp.UserCode, "-", ""))
	if string(store.lastUserCodeHash) != string(want) {
		t.Error("stored user_code_hash is not the digest of the code that was returned")
	}
	if store.lastRedirect != "" {
		t.Errorf("code flow stored a redirect_uri (%q)", store.lastRedirect)
	}
}

func TestStartDeviceLink_RejectsPlainPKCE(t *testing.T) {
	// `plain` is refused outright rather than supported. A plain challenge IS the
	// verifier, so anything that can read the start request can complete the exchange —
	// which removes the entire property PKCE exists to provide for a public client.
	store := &fakeDeviceLinkStore{}
	body := `{"flow":"code","scopes":["chat:write"],"code_challenge":"` + testVerifier +
		`","code_challenge_method":"plain"}`
	w := postJSON(deviceLinkRouter(store, nil), "/device/link/start", body)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for code_challenge_method=plain; body=%s", w.Code, w.Body.String())
	}
	if store.createLinkCalled {
		t.Error("a plain-PKCE request reached persistence")
	}
	if !strings.Contains(w.Body.String(), "S256") {
		t.Errorf("the rejection should name S256 so a plugin author can fix it: %s", w.Body.String())
	}
}

func TestStartDeviceLink_Validation(t *testing.T) {
	challenge := testChallenge()
	cases := []struct {
		name string
		body string
	}{
		{"unknown flow", `{"flow":"magic","scopes":["chat:write"],"code_challenge":"` + challenge + `"}`},
		{"missing flow", `{"scopes":["chat:write"],"code_challenge":"` + challenge + `"}`},
		{"missing challenge", `{"flow":"code","scopes":["chat:write"]}`},
		{"short challenge", `{"flow":"code","scopes":["chat:write"],"code_challenge":"tooshort"}`},
		{"challenge with bad characters", `{"flow":"code","scopes":["chat:write"],"code_challenge":"` +
			strings.Repeat("!", 43) + `"}`},
		{"unknown scope", `{"flow":"code","scopes":["admin:*"],"code_challenge":"` + challenge + `"}`},
		{"no scopes", `{"flow":"code","scopes":[],"code_challenge":"` + challenge + `"}`},
		{"loopback without redirect", `{"flow":"loopback","scopes":["chat:write"],"code_challenge":"` + challenge + `"}`},
		{"loopback with localhost", `{"flow":"loopback","scopes":["chat:write"],"code_challenge":"` + challenge +
			`","redirect_uri":"http://localhost:51234` + LoopbackPath + `"}`},
		{"loopback with a public host", `{"flow":"loopback","scopes":["chat:write"],"code_challenge":"` + challenge +
			`","redirect_uri":"http://evil.example.com` + LoopbackPath + `"}`},
		{"code flow with a redirect", `{"flow":"code","scopes":["chat:write"],"code_challenge":"` + challenge +
			`","redirect_uri":"http://127.0.0.1:51234` + LoopbackPath + `"}`},
		{"device name too long", `{"flow":"code","scopes":["chat:write"],"code_challenge":"` + challenge +
			`","device_name":"` + strings.Repeat("x", 121) + `"}`},
		{"garbage body", `not json`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := &fakeDeviceLinkStore{}
			w := postJSON(deviceLinkRouter(store, nil), "/device/link/start", tc.body)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body=%s", w.Code, w.Body.String())
			}
			if store.createLinkCalled {
				t.Fatal("an invalid start request reached persistence")
			}
		})
	}
}

func approvedLoopbackRequest() *repository.LinkRequest {
	now := time.Now()
	return &repository.LinkRequest{
		ID: testLinkRequestID, Flow: repository.FlowLoopback, DeviceName: "Stream Deck",
		UserID: testUserID, OverlayID: testDeviceOverlayID,
		GrantedScopes: []string{middleware.ScopeEngagementWrite},
		PKCEChallenge: testChallenge(), PKCEMethod: "S256",
		RedirectURI: "http://127.0.0.1:51234" + LoopbackPath,
		ApprovedAt:  &now, ExpiresAt: now.Add(repository.LinkRequestTTL),
	}
}

func TestExchangeDeviceLink_ReturnsTheTokenOnceAndStoresOnlyItsDigest(t *testing.T) {
	store := &fakeDeviceLinkStore{consumed: approvedLoopbackRequest()}
	body := `{"request_id":"` + testLinkRequestID + `","code":"one-time-code","code_verifier":"` + testVerifier + `"}`
	w := postJSON(deviceLinkRouter(store, nil), "/device/link/exchange", body)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}

	var resp exchangeResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !middleware.IsDeviceToken(resp.Token) {
		t.Fatalf("exchange returned %q, which is not an allchat_dev_ token", resp.Token)
	}
	if resp.OverlayID != testDeviceOverlayID {
		t.Errorf("overlay_id = %q, want the bound overlay", resp.OverlayID)
	}
	// Only the digest reaches persistence, and it is the digest of what was returned.
	if string(store.lastTokenHsh) != string(middleware.HashDeviceToken(resp.Token)) {
		t.Error("stored hash is not sha256(returned plaintext)")
	}
	if store.lastLifetime != middleware.DeviceTokenLifetime {
		t.Errorf("lifetime = %v, want the 90-day default", store.lastLifetime)
	}
	if strings.Contains(w.Body.String(), "token_hash") {
		t.Errorf("exchange response exposes a digest: %s", w.Body.String())
	}
	// The claim was made with the digest of the presented code, never the code itself.
	if string(store.lastClaim) != string(hashAuthCode("one-time-code")) {
		t.Error("the row was claimed with something other than the digest of the presented code")
	}
}

func TestExchangeDeviceLink_PendingIs428(t *testing.T) {
	// 428 is the plugin's "keep polling". Anything else and either the plugin gives up
	// while the streamer is still reading the approve screen, or it polls forever after
	// a terminal failure.
	store := &fakeDeviceLinkStore{consumeErr: repository.ErrLinkRequestPending}
	body := `{"request_id":"` + testLinkRequestID + `","code":"c","code_verifier":"` + testVerifier + `"}`
	w := postJSON(deviceLinkRouter(store, nil), "/device/link/exchange", body)
	if w.Code != http.StatusPreconditionRequired {
		t.Fatalf("status = %d, want 428 while the streamer has not approved; body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "interval") {
		t.Error("the 428 body should carry the poll interval")
	}
}

func TestExchangeDeviceLink_ReplayRevokesTheMintedToken(t *testing.T) {
	// A replayed code means the code leaked, so the token the FIRST exchange produced is
	// no longer trustworthy either. Losing a working pairing is the correct outcome:
	// re-linking costs one click.
	store := &fakeDeviceLinkStore{
		consumed:   &repository.LinkRequest{ID: testLinkRequestID, MintedTokenID: testDeviceID},
		consumeErr: repository.ErrLinkRequestReplayed,
	}
	body := `{"request_id":"` + testLinkRequestID + `","code":"one-time-code","code_verifier":"` + testVerifier + `"}`
	w := postJSON(deviceLinkRouter(store, nil), "/device/link/exchange", body)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for a replayed code; body=%s", w.Code, w.Body.String())
	}
	if store.revokedByID != testDeviceID {
		t.Fatalf("replay revoked %q, want the token the first exchange minted (%q)",
			store.revokedByID, testDeviceID)
	}
}

func TestExchangeDeviceLink_WrongVerifierIsRejected(t *testing.T) {
	// PKCE is what stands in for the client secret a published plugin cannot hold. A
	// wrong verifier must not produce a token even though the code was valid — and note
	// the code is still burnt by ConsumeAuthCode, which is what one-time has to mean.
	store := &fakeDeviceLinkStore{consumed: approvedLoopbackRequest()}
	other := "SGVsbG8gd29ybGQgdGhpcyBpcyBhIGRpZmZlcmVudCB2ZXJpZmllcg"
	body := `{"request_id":"` + testLinkRequestID + `","code":"one-time-code","code_verifier":"` + other + `"}`
	w := postJSON(deviceLinkRouter(store, nil), "/device/link/exchange", body)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for a wrong code_verifier; body=%s", w.Code, w.Body.String())
	}
	if store.lastTokenHsh != nil {
		t.Fatal("a wrong verifier still minted a device token")
	}
	if store.consumeSeen != 1 {
		t.Errorf("ConsumeAuthCode called %d times, want 1 — the code must be burnt even when "+
			"the verifier turns out to be wrong", store.consumeSeen)
	}
}

func TestExchangeDeviceLink_CodeFlowIdentifiesByTypedCode(t *testing.T) {
	// The fallback path, end to end at the exchange: the plugin knows the code it
	// displayed and may have no request id (it is running on the other machine).
	approved := approvedLoopbackRequest()
	approved.Flow = repository.FlowCode
	approved.RedirectURI = ""
	store := &fakeDeviceLinkStore{
		byCode:   &repository.LinkRequest{ID: testLinkRequestID, Flow: repository.FlowCode},
		consumed: approved,
	}
	body := `{"user_code":"abcd-efgh","code_verifier":"` + testVerifier + `"}`
	w := postJSON(deviceLinkRouter(store, nil), "/device/link/exchange", body)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	// The claim used the digest of the NORMALISED code: upper-cased, hyphen removed.
	if string(store.lastClaim) != string(hashUserCode("ABCDEFGH")) {
		t.Error("the typed code was not normalised before hashing")
	}
}

func TestExchangeDeviceLink_WrongTypedCodeCostsAnAttempt(t *testing.T) {
	// The per-request attempt counter is the actual brute-force bound for the fallback
	// (ADR-0049's security-review item). A wrong code must cost one of the five.
	store := &fakeDeviceLinkStore{
		byCode:     &repository.LinkRequest{ID: testLinkRequestID, Flow: repository.FlowCode},
		consumeErr: repository.ErrLinkRequestNotFound,
	}
	body := `{"request_id":"` + testLinkRequestID + `","user_code":"ABCD-EFGH","code_verifier":"` + testVerifier + `"}`
	w := postJSON(deviceLinkRouter(store, nil), "/device/link/exchange", body)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", w.Code, w.Body.String())
	}
	if store.attemptCalls != 1 {
		t.Fatalf("RegisterFailedAttempt called %d times, want 1. That counter is the "+
			"brute-force bound for the typed-code fallback; without it the only limit is "+
			"the gateway rate limit, which an attacker can spread across addresses.",
			store.attemptCalls)
	}
}

func TestExchangeDeviceLink_LoopbackMissDoesNotBurnAttempts(t *testing.T) {
	// A loopback code is not something a human types, so a miss there is a bug or a
	// probe against an id the attacker already has — not a guess against the five
	// attempts the streamer's own pairing gets.
	store := &fakeDeviceLinkStore{consumeErr: repository.ErrLinkRequestNotFound}
	body := `{"request_id":"` + testLinkRequestID + `","code":"wrong","code_verifier":"` + testVerifier + `"}`
	w := postJSON(deviceLinkRouter(store, nil), "/device/link/exchange", body)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
	if store.attemptCalls != 0 {
		t.Errorf("RegisterFailedAttempt called %d times for a loopback miss, want 0", store.attemptCalls)
	}
}

func TestExchangeDeviceLink_Validation(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"no verifier", `{"request_id":"` + testLinkRequestID + `","code":"c"}`},
		{"no identifier", `{"code_verifier":"` + testVerifier + `"}`},
		{"bad request id", `{"request_id":"not-a-uuid","code":"c","code_verifier":"` + testVerifier + `"}`},
		{"no secret at all", `{"request_id":"` + testLinkRequestID + `","code_verifier":"` + testVerifier + `"}`},
		{"garbage body", `not json`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := &fakeDeviceLinkStore{consumed: approvedLoopbackRequest()}
			w := postJSON(deviceLinkRouter(store, nil), "/device/link/exchange", tc.body)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body=%s", w.Code, w.Body.String())
			}
			if store.lastTokenHsh != nil {
				t.Fatal("an invalid exchange minted a device token")
			}
		})
	}
}

func getCallback(router *gin.Engine, query string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/device/link/callback?"+query, nil))
	return w
}

func TestDeviceLinkCallback_RedirectsToTheStoredLoopback(t *testing.T) {
	store := &fakeDeviceLinkStore{pending: approvedLoopbackRequest()}
	router := deviceLinkRouter(store, sessionCtx())
	w := getCallback(router, "request_id="+testLinkRequestID+"&code=one-time-code&state=plugin-state")
	if w.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302; body=%s", w.Code, w.Body.String())
	}
	got := w.Header().Get("Location")
	want := "http://127.0.0.1:51234" + LoopbackPath + "?code=one-time-code&state=plugin-state"
	if got != want {
		t.Errorf("Location = %q, want %q", got, want)
	}
}

func TestDeviceLinkCallback_RefusesATokenAuthenticatedRequest(t *testing.T) {
	// Session-only: a device token must not be able to walk a linking flow to
	// completion on its own and mint a second credential.
	store := &fakeDeviceLinkStore{pending: approvedLoopbackRequest()}
	router := deviceLinkRouter(store, map[string]string{
		"user_id":                testUserID,
		middleware.CtxAuthMethod: middleware.AuthMethodAPIToken,
	})
	w := getCallback(router, "request_id="+testLinkRequestID+"&code=c")
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 for a token-authenticated callback; body=%s", w.Code, w.Body.String())
	}
}

func TestDeviceLinkCallback_RefusesAnotherUsersRequest(t *testing.T) {
	// 404 rather than 403: whether a request id exists is not something an unrelated
	// session should learn.
	req := approvedLoopbackRequest()
	req.UserID = "88888888-8888-8888-8888-888888888888"
	store := &fakeDeviceLinkStore{pending: req}
	w := getCallback(deviceLinkRouter(store, sessionCtx()), "request_id="+testLinkRequestID+"&code=c")
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body=%s", w.Code, w.Body.String())
	}
}

func TestDeviceLinkCallback_RefusesAnUnapprovedRequest(t *testing.T) {
	req := approvedLoopbackRequest()
	req.ApprovedAt = nil
	store := &fakeDeviceLinkStore{pending: req}
	w := getCallback(deviceLinkRouter(store, sessionCtx()), "request_id="+testLinkRequestID+"&code=c")
	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 before approval; body=%s", w.Code, w.Body.String())
	}
}

func TestDeviceLinkCallback_RefusesACodeFlowRequest(t *testing.T) {
	// There is no redirect for the code flow: the plugin polls. A callback against one
	// would mean somebody is trying to get a code delivered somewhere it was never meant
	// to go.
	req := approvedLoopbackRequest()
	req.Flow = repository.FlowCode
	req.RedirectURI = ""
	store := &fakeDeviceLinkStore{pending: req}
	w := getCallback(deviceLinkRouter(store, sessionCtx()), "request_id="+testLinkRequestID+"&code=c")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", w.Code, w.Body.String())
	}
}

func TestNormalizeUserCode(t *testing.T) {
	// A streamer reading a code off a screen may or may not type the hyphen and may type
	// lower case; neither should cost one of their five attempts.
	for input, want := range map[string]string{
		"ABCD-EFGH":  "ABCDEFGH",
		"abcdefgh":   "ABCDEFGH",
		"abcd efgh":  "ABCDEFGH",
		" ABCDEFGH ": "ABCDEFGH",
		"ABCD-EFG":   "",
		"ABCD-EFGHI": "",
		"":           "",
	} {
		if got := normalizeUserCode(input); got != want {
			t.Errorf("normalizeUserCode(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestUserCodeAlphabetHasNoAmbiguousGlyphs(t *testing.T) {
	// 0/O and 1/I/L are the pairs a human misreads off a screen or a Stream Deck key.
	for _, bad := range "01OIL" {
		if strings.ContainsRune(userCodeAlphabet, bad) {
			t.Errorf("userCodeAlphabet contains the ambiguous glyph %q", bad)
		}
	}
	if len(userCodeAlphabet) < 30 {
		t.Errorf("userCodeAlphabet has %d symbols; 8 characters of it is the entropy of a "+
			"pairing code", len(userCodeAlphabet))
	}
}

func TestVerifyPKCE(t *testing.T) {
	if !verifyPKCE(testChallenge(), testVerifier) {
		t.Error("verifyPKCE rejected a matching S256 pair")
	}
	if verifyPKCE(testChallenge(), "SGVsbG8gd29ybGQgdGhpcyBpcyBhIGRpZmZlcmVudCB2ZXJpZmllcg") {
		t.Error("verifyPKCE accepted a non-matching verifier")
	}
	if verifyPKCE(testVerifier, testVerifier) {
		t.Error("verifyPKCE accepted a plain challenge; only S256 is supported")
	}
	if verifyPKCE("", testVerifier) || verifyPKCE(testChallenge(), "") {
		t.Error("verifyPKCE accepted an empty side")
	}
}

func TestGenerateAuthCodeIsUnpredictableAndHashedSeparately(t *testing.T) {
	a, ha, err := generateAuthCode()
	if err != nil {
		t.Fatalf("generateAuthCode: %v", err)
	}
	b, _, err := generateAuthCode()
	if err != nil {
		t.Fatalf("generateAuthCode: %v", err)
	}
	if a == b {
		t.Fatal("two generated authorization codes collided")
	}
	if len(ha) != 32 {
		t.Errorf("digest length = %d, want 32", len(ha))
	}
	// Domain separation: the same string must not hash to the same digest in the user-code
	// namespace, so a value valid in one cannot be replayed into the other.
	if string(hashAuthCode(a)) == string(hashUserCode(a)) {
		t.Error("hashAuthCode and hashUserCode are not domain-separated")
	}
}
