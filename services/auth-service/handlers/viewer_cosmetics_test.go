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

// Tests for PATCH /viewer/cosmetics handler (Tasks 2+, plans 28-02 and 29-01).
// Tests use handlePatchCosmeticsLogic directly with a mock cosmeticsUpsertRepo
// to avoid DB and Redis dependency.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/caesar/all-chat/services/auth-service/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"go.uber.org/zap/zaptest/observer"
)

// mockCosmeticsUpsertRepo implements cosmeticsUpsertRepo for testing. It records
// each partial-update call AND maintains an in-memory `stored` row that applies the
// same per-column semantics as the real UPSERT — so the handler's response (built
// from the returned row) reflects exactly what a column-selective write persists.
type mockCosmeticsUpsertRepo struct {
	upsertCalls []struct {
		viewerID uuid.UUID
		update   repository.CosmeticsUpdate
	}
	upsertErr error
	getErr    error
	stored    *repository.ViewerCosmetics // simulates the persisted row (nil = no row)
}

func (m *mockCosmeticsUpsertRepo) UpsertViewerCosmetics(ctx context.Context, viewerID uuid.UUID, u repository.CosmeticsUpdate) (*repository.ViewerCosmetics, error) {
	m.upsertCalls = append(m.upsertCalls, struct {
		viewerID uuid.UUID
		update   repository.CosmeticsUpdate
	}{viewerID, u})
	if m.upsertErr != nil {
		return nil, m.upsertErr
	}
	if m.stored == nil {
		m.stored = &repository.ViewerCosmetics{}
	}
	// Apply only the addressed column groups (mirrors the SQL CASE-per-flag).
	if u.SetName {
		m.stored.NameColor = u.NameColor
		m.stored.NameGradient = u.NameGradient
	}
	if u.SetFrame {
		m.stored.AvatarFrameID = u.AvatarFrameID
	}
	if u.SetFlair {
		m.stored.AvatarFlairID = u.AvatarFlairID
	}
	cp := *m.stored // return a copy, as RETURNING yields a snapshot
	return &cp, nil
}

func (m *mockCosmeticsUpsertRepo) GetFullCosmetics(ctx context.Context, viewerID uuid.UUID) (*repository.ViewerCosmetics, error) {
	return m.stored, m.getErr
}

// lastUpdate returns the CosmeticsUpdate from the single recorded upsert call,
// failing the test if the number of calls is not exactly one.
func (m *mockCosmeticsUpsertRepo) lastUpdate(t *testing.T) repository.CosmeticsUpdate {
	t.Helper()
	if len(m.upsertCalls) != 1 {
		t.Fatalf("expected exactly 1 upsert call, got %d", len(m.upsertCalls))
	}
	return m.upsertCalls[0].update
}

// testCosmeticsHandler wraps handlePatchCosmeticsLogic with a mock repo for unit testing.
type testCosmeticsHandler struct {
	repo   cosmeticsUpsertRepo
	logger *zap.Logger
}

func (h *testCosmeticsHandler) Handle(c *gin.Context) {
	handlePatchCosmeticsLogic(c, h.repo, h.logger)
}

// setupCosmeticsTest creates a gin router that simulates the JWT middleware setting claims.
func setupCosmeticsTest(t *testing.T, viewerIDStr, platform, platformUserID string) (*gin.Engine, *mockCosmeticsUpsertRepo) {
	t.Helper()
	return setupCosmeticsTestWithPremium(t, viewerIDStr, platform, platformUserID, false)
}

// setupCosmeticsTestWithPremium creates a gin router with configurable is_premium flag.
func setupCosmeticsTestWithPremium(t *testing.T, viewerIDStr, platform, platformUserID string, isPremium bool) (*gin.Engine, *mockCosmeticsUpsertRepo) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	mock := &mockCosmeticsUpsertRepo{}
	h := &testCosmeticsHandler{repo: mock, logger: zaptest.NewLogger(t)}

	router := gin.New()
	router.PATCH("/viewer/cosmetics", func(c *gin.Context) {
		// Simulate JWT middleware: always set viewer_id (even if empty string)
		c.Set("viewer_id", viewerIDStr)
		c.Set("is_premium", isPremium)
		if platform != "" {
			c.Set("platform", platform)
		}
		if platformUserID != "" {
			c.Set("platform_user_id", platformUserID)
		}
		c.Next()
	}, h.Handle)

	return router, mock
}

func doPatch(t *testing.T, router *gin.Engine, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPatch, "/viewer/cosmetics", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func parseBody(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	return resp
}

func TestPatchCosmetics_ValidColor(t *testing.T) {
	viewerID := uuid.New()
	router, mock := setupCosmeticsTest(t, viewerID.String(), "twitch", "12345")

	w := doPatch(t, router, `{"name_color":"#ff6600"}`)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	if resp := parseBody(t, w); resp["name_color"] != "#ff6600" {
		t.Errorf("name_color = %v, want #ff6600", resp["name_color"])
	}

	if mock.upsertCalls[0].viewerID != viewerID {
		t.Errorf("upsert called with wrong viewerID")
	}
	u := mock.lastUpdate(t)
	if !u.SetName || u.NameColor == nil || *u.NameColor != "#ff6600" {
		t.Errorf("expected SetName with nameColor #ff6600, got %+v", u)
	}
}

func TestPatchCosmetics_NullColor(t *testing.T) {
	viewerID := uuid.New()
	router, mock := setupCosmeticsTest(t, viewerID.String(), "twitch", "12345")

	w := doPatch(t, router, `{"name_color":null}`)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	if resp := parseBody(t, w); resp["name_color"] != nil {
		t.Errorf("expected name_color null, got %v", resp["name_color"])
	}
	if u := mock.lastUpdate(t); !u.SetName || u.NameColor != nil {
		t.Errorf("expected SetName with nil nameColor, got %+v", u)
	}
}

func TestPatchCosmetics_InvalidHex(t *testing.T) {
	viewerID := uuid.New()
	router, _ := setupCosmeticsTest(t, viewerID.String(), "twitch", "12345")
	if w := doPatch(t, router, `{"name_color":"notahex"}`); w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid hex, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestPatchCosmetics_MissingViewerID(t *testing.T) {
	// Empty viewer_id simulates a pre-Phase-28 token (old token without viewer_id claim)
	router, _ := setupCosmeticsTest(t, "", "twitch", "12345")
	if w := doPatch(t, router, `{"name_color":"#ff6600"}`); w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for missing viewer_id, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestPatchCosmetics_InvalidViewerIDFormat(t *testing.T) {
	router, _ := setupCosmeticsTest(t, "not-a-uuid", "twitch", "12345")
	if w := doPatch(t, router, `{"name_color":"#ff6600"}`); w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for invalid viewer_id format, got %d body=%s", w.Code, w.Body.String())
	}
}

// Phase 29: gradient tests

func TestPatchCosmetics_GradientAccepted(t *testing.T) {
	viewerID := uuid.New()
	router, mock := setupCosmeticsTestWithPremium(t, viewerID.String(), "twitch", "12345", true)

	w := doPatch(t, router, `{"name_gradient":{"type":"linear","colors":["#ff0000","#0000ff"],"angle":90}}`)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for premium gradient, got %d body=%s", w.Code, w.Body.String())
	}
	u := mock.lastUpdate(t)
	// A gradient-only PATCH addresses only the name group; avatar columns are untouched.
	if !u.SetName || u.SetFrame || u.SetFlair {
		t.Errorf("expected SetName only, got %+v", u)
	}
	if u.NameColor != nil {
		t.Errorf("name_color should be nil when gradient is set (mutual exclusion)")
	}
	if u.NameGradient == nil {
		t.Errorf("expected nameGradient bytes to be set")
	}
}

func TestPatchCosmetics_GradientRejectedNonPremium(t *testing.T) {
	viewerID := uuid.New()
	router, _ := setupCosmeticsTestWithPremium(t, viewerID.String(), "twitch", "12345", false)
	w := doPatch(t, router, `{"name_gradient":{"type":"linear","colors":["#ff0000","#0000ff"],"angle":90}}`)
	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for non-premium gradient attempt, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestPatchCosmetics_GradientValidation(t *testing.T) {
	viewerID := uuid.New()
	router, _ := setupCosmeticsTestWithPremium(t, viewerID.String(), "twitch", "12345", true)
	w := doPatch(t, router, `{"name_gradient":{"type":"linear","colors":["#ff0000"],"angle":400}}`)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid gradient (1 color), got %d body=%s", w.Code, w.Body.String())
	}
}

func TestPatchCosmetics_GradientValidation_BadAngle(t *testing.T) {
	viewerID := uuid.New()
	router, _ := setupCosmeticsTestWithPremium(t, viewerID.String(), "twitch", "12345", true)
	w := doPatch(t, router, `{"name_gradient":{"type":"linear","colors":["#ff0000","#00ff00"],"angle":400}}`)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for angle 400, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestPatchCosmetics_MutualExclusion(t *testing.T) {
	viewerID := uuid.New()
	router, mock := setupCosmeticsTestWithPremium(t, viewerID.String(), "twitch", "12345", true)

	w := doPatch(t, router, `{"name_gradient":{"type":"linear","colors":["#aabbcc","#112233"],"angle":45}}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	if u := mock.lastUpdate(t); u.NameColor != nil {
		t.Errorf("mutual exclusion: name_color should be nil when gradient is set, got %v", *u.NameColor)
	}
	if resp := parseBody(t, w); resp["name_color"] != nil {
		t.Errorf("response name_color should be null when gradient is set, got %v", resp["name_color"])
	}
}

// Phase 30: avatar frame / flair tests

func TestPatchCosmetics_AvatarFrameID_PremiumAccepted(t *testing.T) {
	viewerID := uuid.New()
	router, mock := setupCosmeticsTestWithPremium(t, viewerID.String(), "twitch", "12345", true)

	frameID := uuid.New()
	w := doPatch(t, router, `{"avatar_frame_id":"`+frameID.String()+`"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for premium avatar_frame_id, got %d body=%s", w.Code, w.Body.String())
	}
	u := mock.lastUpdate(t)
	// Frame-only PATCH addresses only the frame; name and flair stay untouched.
	if u.SetName || u.SetFlair || !u.SetFrame {
		t.Errorf("expected SetFrame only, got %+v", u)
	}
	if u.AvatarFrameID == nil || *u.AvatarFrameID != frameID {
		t.Errorf("expected avatarFrameID=%v, got %v", frameID, u.AvatarFrameID)
	}
}

func TestPatchCosmetics_AvatarFrameID_NonPremiumRejected(t *testing.T) {
	viewerID := uuid.New()
	router, _ := setupCosmeticsTestWithPremium(t, viewerID.String(), "twitch", "12345", false)
	frameID := uuid.New()
	if w := doPatch(t, router, `{"avatar_frame_id":"`+frameID.String()+`"}`); w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for non-premium avatar_frame_id, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestPatchCosmetics_AvatarFlairID_NonPremiumRejected(t *testing.T) {
	viewerID := uuid.New()
	router, _ := setupCosmeticsTestWithPremium(t, viewerID.String(), "twitch", "12345", false)
	flairID := uuid.New()
	if w := doPatch(t, router, `{"avatar_flair_id":"`+flairID.String()+`"}`); w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for non-premium avatar_flair_id, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestPatchCosmetics_NonPremium_DowngradeClears(t *testing.T) {
	// Non-premium viewer sending only name_color: the avatar columns must be cleared
	// with NIL POINTERS (→ SQL NULL), NOT &uuid.Nil. A pointer to the zero UUID is
	// encoded as the literal '00000000-...' value, which violates the avatar FKs and
	// 500s the request against a real database (prod bug reported 2026-07-17).
	viewerID := uuid.New()
	router, mock := setupCosmeticsTestWithPremium(t, viewerID.String(), "twitch", "12345", false)

	w := doPatch(t, router, `{"name_color":"#00ff00"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	u := mock.lastUpdate(t)
	// Downgrade enforcement: non-premium forces both avatar columns cleared to nil.
	if !u.SetFrame || u.AvatarFrameID != nil {
		t.Errorf("expected SetFrame with nil avatarFrameID (→ SQL NULL), got %+v", u)
	}
	if !u.SetFlair || u.AvatarFlairID != nil {
		t.Errorf("expected SetFlair with nil avatarFlairID (→ SQL NULL), got %+v", u)
	}
}

func TestPatchCosmetics_NonPremium_AvatarOnlyClearsWithNil(t *testing.T) {
	// The frontend avatar card sends {"avatar_frame_id":null,"avatar_flair_id":null}
	// for "None". For a non-premium viewer this must pass nil pointers (→ SQL NULL),
	// not &uuid.Nil. This is the exact avatar-save path that 500'd in prod.
	viewerID := uuid.New()
	router, mock := setupCosmeticsTestWithPremium(t, viewerID.String(), "twitch", "12345", false)

	w := doPatch(t, router, `{"avatar_frame_id":null,"avatar_flair_id":null}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	u := mock.lastUpdate(t)
	if !u.SetFrame || u.AvatarFrameID != nil || !u.SetFlair || u.AvatarFlairID != nil {
		t.Errorf("expected both avatar columns cleared to nil, got %+v", u)
	}
}

func TestPatchCosmetics_Premium_ZeroUUIDNormalizedToNil(t *testing.T) {
	// Defensive: even a premium viewer sending the literal zero UUID must have it
	// normalized to nil (→ SQL NULL) so it clears the slot rather than tripping the FK.
	viewerID := uuid.New()
	router, mock := setupCosmeticsTestWithPremium(t, viewerID.String(), "twitch", "12345", true)

	w := doPatch(t, router, `{"avatar_frame_id":"00000000-0000-0000-0000-000000000000","avatar_flair_id":"00000000-0000-0000-0000-000000000000"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	if u := mock.lastUpdate(t); u.AvatarFrameID != nil || u.AvatarFlairID != nil {
		t.Errorf("expected zero-UUID avatar ids normalized to nil, got %+v", u)
	}
}

func TestPatchCosmetics_UpsertError_Returns500AndLogs(t *testing.T) {
	// When the repository upsert fails, the handler must return 500 AND log the
	// underlying error (previously it swallowed the error, leaving only the
	// middleware access log — impossible to diagnose in prod).
	viewerID := uuid.New()
	gin.SetMode(gin.TestMode)

	core, logs := observer.New(zap.ErrorLevel)
	mock := &mockCosmeticsUpsertRepo{upsertErr: fmt.Errorf("boom: FK violation")}
	h := &testCosmeticsHandler{repo: mock, logger: zap.New(core)}

	router := gin.New()
	router.PATCH("/viewer/cosmetics", func(c *gin.Context) {
		c.Set("viewer_id", viewerID.String())
		c.Set("is_premium", false)
		c.Next()
	}, h.Handle)

	if w := doPatch(t, router, `{"name_color":"#00ff00"}`); w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d body=%s", w.Code, w.Body.String())
	}
	if logs.Len() == 0 {
		t.Error("expected the upsert failure to be logged, got no error logs")
	}
}

func TestPatchCosmetics_Premium_NameOnly_PreservesAvatar(t *testing.T) {
	// Regression ([0]): a name-color/gradient-only PATCH must NOT clear a premium
	// viewer's saved avatar frame/flair. It addresses only the name group, so the
	// avatar columns are left untouched and reported unchanged.
	viewerID := uuid.New()
	router, mock := setupCosmeticsTestWithPremium(t, viewerID.String(), "twitch", "12345", true)

	savedFrame := uuid.New()
	mock.stored = &repository.ViewerCosmetics{AvatarFrameID: &savedFrame}

	w := doPatch(t, router, `{"name_color":"#00ff00","name_gradient":null}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	if u := mock.lastUpdate(t); !u.SetName || u.SetFrame || u.SetFlair {
		t.Errorf("name-only PATCH must address only the name group, got %+v", u)
	}
	resp := parseBody(t, w)
	if resp["avatar_frame_id"] != savedFrame.String() {
		t.Errorf("expected preserved avatar_frame_id=%s, got %v", savedFrame, resp["avatar_frame_id"])
	}
	if resp["name_color"] != "#00ff00" {
		t.Errorf("expected name_color=#00ff00, got %v", resp["name_color"])
	}
}

func TestPatchCosmetics_Premium_FlairOnly_PreservesFrame(t *testing.T) {
	// Regression (re-review): a PATCH touching only one avatar column must not clear
	// the sibling column. Here a flair-only change must preserve the saved frame.
	viewerID := uuid.New()
	router, mock := setupCosmeticsTestWithPremium(t, viewerID.String(), "twitch", "12345", true)

	savedFrame := uuid.New()
	mock.stored = &repository.ViewerCosmetics{AvatarFrameID: &savedFrame}

	newFlair := uuid.New()
	w := doPatch(t, router, `{"avatar_flair_id":"`+newFlair.String()+`"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	if u := mock.lastUpdate(t); u.SetFrame || !u.SetFlair {
		t.Errorf("flair-only PATCH must set flair and leave frame untouched, got %+v", u)
	}
	resp := parseBody(t, w)
	if resp["avatar_frame_id"] != savedFrame.String() {
		t.Errorf("expected preserved avatar_frame_id=%s, got %v", savedFrame, resp["avatar_frame_id"])
	}
	if resp["avatar_flair_id"] != newFlair.String() {
		t.Errorf("expected avatar_flair_id=%s, got %v", newFlair, resp["avatar_flair_id"])
	}
}

func TestPatchCosmetics_AvatarOnly_ResponseReflectsPreservedName(t *testing.T) {
	// Regression ([4]): an avatar-only PATCH preserves the stored name color, and the
	// response must report that preserved value — not echo null from the (absent)
	// request name field.
	viewerID := uuid.New()
	router, mock := setupCosmeticsTestWithPremium(t, viewerID.String(), "twitch", "12345", true)

	savedColor := "#ff0000"
	mock.stored = &repository.ViewerCosmetics{NameColor: &savedColor}

	frameID := uuid.New()
	w := doPatch(t, router, `{"avatar_frame_id":"`+frameID.String()+`"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	resp := parseBody(t, w)
	if resp["name_color"] != savedColor {
		t.Errorf("avatar-only PATCH must report preserved name_color=%s, got %v", savedColor, resp["name_color"])
	}
	if resp["avatar_frame_id"] != frameID.String() {
		t.Errorf("expected avatar_frame_id=%s, got %v", frameID, resp["avatar_frame_id"])
	}
}

func TestPatchCosmetics_Premium_ZeroUUID_ResponseShowsNull(t *testing.T) {
	// Regression ([7]): a zero-UUID avatar selection is normalized to NULL before the
	// write, so the response must report null — not echo the zero UUID.
	viewerID := uuid.New()
	router, _ := setupCosmeticsTestWithPremium(t, viewerID.String(), "twitch", "12345", true)

	w := doPatch(t, router, `{"avatar_frame_id":"00000000-0000-0000-0000-000000000000","avatar_flair_id":"00000000-0000-0000-0000-000000000000"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	resp := parseBody(t, w)
	if resp["avatar_frame_id"] != nil {
		t.Errorf("expected avatar_frame_id null (zero-UUID normalized), got %v", resp["avatar_frame_id"])
	}
	if resp["avatar_flair_id"] != nil {
		t.Errorf("expected avatar_flair_id null (zero-UUID normalized), got %v", resp["avatar_flair_id"])
	}
}

func TestPatchCosmetics_AvatarOnly_DoesNotClearNameColor(t *testing.T) {
	// Regression test: saving avatar cosmetics (no name_color/name_gradient in body)
	// must not address the name group (which would NULL a stored name color).
	viewerID := uuid.New()
	router, mock := setupCosmeticsTestWithPremium(t, viewerID.String(), "twitch", "12345", true)

	frameID := uuid.New()
	flairID := uuid.New()
	w := doPatch(t, router, `{"avatar_frame_id":"`+frameID.String()+`","avatar_flair_id":"`+flairID.String()+`"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	u := mock.lastUpdate(t)
	if u.SetName {
		t.Errorf("avatar-only PATCH must not address the name group, got %+v", u)
	}
	if u.AvatarFrameID == nil || *u.AvatarFrameID != frameID {
		t.Errorf("expected avatarFrameID=%v, got %v", frameID, u.AvatarFrameID)
	}
	if u.AvatarFlairID == nil || *u.AvatarFlairID != flairID {
		t.Errorf("expected avatarFlairID=%v, got %v", flairID, u.AvatarFlairID)
	}
}

// mockLinkedPlatformsGetter implements linkedPlatformsGetter for testing.
type mockLinkedPlatformsGetter struct {
	platforms []repository.LinkedPlatform
	err       error
}

func (m *mockLinkedPlatformsGetter) GetLinkedPlatforms(_ context.Context, _ uuid.UUID) ([]repository.LinkedPlatform, error) {
	return m.platforms, m.err
}

// TestPatchCosmetics_CacheInvalidation_AllLinkedPlatforms verifies that updating cosmetics
// invalidates the Redis identity cache for ALL linked platforms, not just the current session's
// platform. This is the fix for the username gradient cross-platform display bug.
func TestPatchCosmetics_CacheInvalidation_AllLinkedPlatforms(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer mr.Close()

	redisClient := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer redisClient.Close()

	viewerID := uuid.New()
	twitchUserID := "twitch-user-123"
	youtubeUserID := "yt-user-456"
	kickUserID := "kick-user-789"

	// Seed Redis with identity cache entries for all three platforms.
	gradientJSON := `{"type":"linear","colors":["#ff0000","#0000ff"],"angle":90}`
	cachedWithGradient := fmt.Sprintf(`{"viewer_id":"%s","name_color":null,"name_gradient":%s}`, viewerID, gradientJSON)
	nullSentinel := "null"
	redisClient.Set(context.Background(), fmt.Sprintf("viewer:identity:twitch:%s", twitchUserID), cachedWithGradient, 0)
	redisClient.Set(context.Background(), fmt.Sprintf("viewer:identity:youtube:%s", youtubeUserID), nullSentinel, 0)
	redisClient.Set(context.Background(), fmt.Sprintf("viewer:identity:kick:%s", kickUserID), nullSentinel, 0)

	mockRepo := &mockCosmeticsUpsertRepo{}
	mockLinked := &mockLinkedPlatformsGetter{
		platforms: []repository.LinkedPlatform{
			{Platform: "twitch", PlatformUserID: twitchUserID},
			{Platform: "youtube", PlatformUserID: youtubeUserID},
			{Platform: "kick", PlatformUserID: kickUserID},
		},
	}

	gin.SetMode(gin.TestMode)
	logger := zaptest.NewLogger(t)
	h := &ViewerCosmeticsHandler{
		identityRepo:    nil, // not needed; upsert goes through handlePatchCosmeticsLogic(c, mockRepo)
		linkedPlatforms: mockLinked,
		redis:           redisClient,
		logger:          logger,
	}

	router := gin.New()
	router.PATCH("/viewer/cosmetics", func(c *gin.Context) {
		c.Set("viewer_id", viewerID.String())
		c.Set("is_premium", true)
		c.Set("platform", "twitch")
		c.Set("platform_user_id", twitchUserID)
		c.Next()
	}, func(c *gin.Context) {
		handlePatchCosmeticsLogic(c, mockRepo, logger)
	}, func(c *gin.Context) {
		// Simulate the cache invalidation portion of HandlePatchCosmetics.
		if c.Writer.Status() == http.StatusOK {
			vidVal, _ := c.Get("viewer_id")
			vidStr := vidVal.(string)
			vid, _ := uuid.Parse(vidStr)
			linked, _ := h.linkedPlatforms.GetLinkedPlatforms(context.Background(), vid)
			for _, lp := range linked {
				key := fmt.Sprintf("viewer:identity:%s:%s", lp.Platform, lp.PlatformUserID)
				h.redis.Del(context.Background(), key)
			}
		}
	})

	if w := doPatch(t, router, `{"name_color":"#aabbcc"}`); w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	for _, tc := range []struct {
		platform string
		userID   string
	}{
		{"twitch", twitchUserID},
		{"youtube", youtubeUserID},
		{"kick", kickUserID},
	} {
		key := fmt.Sprintf("viewer:identity:%s:%s", tc.platform, tc.userID)
		if mr.Exists(key) {
			t.Errorf("cache key %q should have been deleted but still exists", key)
		}
	}
}

func TestPatchCosmetics_AvatarFrameID_ResponseIncludes(t *testing.T) {
	// Response JSON must include avatar_frame_id and avatar_flair_id fields.
	viewerID := uuid.New()
	router, _ := setupCosmeticsTestWithPremium(t, viewerID.String(), "twitch", "12345", true)

	frameID := uuid.New()
	w := doPatch(t, router, `{"avatar_frame_id":"`+frameID.String()+`"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	resp := parseBody(t, w)
	if _, exists := resp["avatar_frame_id"]; !exists {
		t.Error("response missing avatar_frame_id field")
	}
	if _, exists := resp["avatar_flair_id"]; !exists {
		t.Error("response missing avatar_flair_id field")
	}
}
