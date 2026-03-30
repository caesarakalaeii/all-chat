package handlers

// Tests for PATCH /viewer/cosmetics handler (Tasks 2+, plans 28-02 and 29-01).
// Tests use handlePatchCosmeticsLogic directly with a mock cosmeticsUpsertRepo
// to avoid DB and Redis dependency.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap/zaptest"
)

// mockCosmeticsUpsertRepo implements cosmeticsUpsertRepo for testing.
type mockCosmeticsUpsertRepo struct {
	upsertCalls []struct {
		viewerID      uuid.UUID
		nameColor     *string
		nameGradient  []byte
		avatarFrameID *uuid.UUID
		avatarFlairID *uuid.UUID
	}
	avatarUpsertCalls []struct {
		viewerID      uuid.UUID
		avatarFrameID *uuid.UUID
		avatarFlairID *uuid.UUID
	}
	upsertErr error
}

func (m *mockCosmeticsUpsertRepo) UpsertViewerCosmetics(ctx context.Context, viewerID uuid.UUID, nameColor *string, nameGradient []byte, avatarFrameID *uuid.UUID, avatarFlairID *uuid.UUID) error {
	m.upsertCalls = append(m.upsertCalls, struct {
		viewerID      uuid.UUID
		nameColor     *string
		nameGradient  []byte
		avatarFrameID *uuid.UUID
		avatarFlairID *uuid.UUID
	}{viewerID, nameColor, nameGradient, avatarFrameID, avatarFlairID})
	return m.upsertErr
}

func (m *mockCosmeticsUpsertRepo) UpsertAvatarCosmetics(ctx context.Context, viewerID uuid.UUID, avatarFrameID *uuid.UUID, avatarFlairID *uuid.UUID) error {
	m.avatarUpsertCalls = append(m.avatarUpsertCalls, struct {
		viewerID      uuid.UUID
		avatarFrameID *uuid.UUID
		avatarFlairID *uuid.UUID
	}{viewerID, avatarFrameID, avatarFlairID})
	return m.upsertErr
}

// testCosmeticsHandler wraps handlePatchCosmeticsLogic with a mock repo for unit testing.
type testCosmeticsHandler struct {
	repo cosmeticsUpsertRepo
}

func (h *testCosmeticsHandler) Handle(c *gin.Context) {
	handlePatchCosmeticsLogic(c, h.repo)
}

// setupCosmeticsTest creates a gin router that simulates the JWT middleware setting claims.
func setupCosmeticsTest(t *testing.T, viewerIDStr, platform, platformUserID string) (*gin.Engine, *mockCosmeticsUpsertRepo) {
	t.Helper()
	return setupCosmeticsTestWithPremium(t, viewerIDStr, platform, platformUserID, false)
}

// setupCosmeticsTestWithPremium creates a gin router with configurable is_premium flag.
func setupCosmeticsTestWithPremium(t *testing.T, viewerIDStr, platform, platformUserID string, isPremium bool) (*gin.Engine, *mockCosmeticsUpsertRepo) {
	t.Helper()
	_ = zaptest.NewLogger(t) // ensure logger is available
	gin.SetMode(gin.TestMode)

	mock := &mockCosmeticsUpsertRepo{}
	h := &testCosmeticsHandler{repo: mock}

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

func TestPatchCosmetics_ValidColor(t *testing.T) {
	viewerID := uuid.New()
	router, mock := setupCosmeticsTest(t, viewerID.String(), "twitch", "12345")

	body := `{"name_color":"#ff6600"}`
	req := httptest.NewRequest(http.MethodPatch, "/viewer/cosmetics", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp["name_color"] != "#ff6600" {
		t.Errorf("name_color = %v, want #ff6600", resp["name_color"])
	}

	// Verify DB was called with correct args
	if len(mock.upsertCalls) != 1 {
		t.Fatalf("expected 1 upsert call, got %d", len(mock.upsertCalls))
	}
	if mock.upsertCalls[0].viewerID != viewerID {
		t.Errorf("upsert called with wrong viewerID")
	}
	if mock.upsertCalls[0].nameColor == nil || *mock.upsertCalls[0].nameColor != "#ff6600" {
		t.Errorf("upsert called with wrong nameColor")
	}
}

func TestPatchCosmetics_NullColor(t *testing.T) {
	viewerID := uuid.New()
	router, mock := setupCosmeticsTest(t, viewerID.String(), "twitch", "12345")

	body := `{"name_color":null}`
	req := httptest.NewRequest(http.MethodPatch, "/viewer/cosmetics", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	// name_color should be null
	if val, exists := resp["name_color"]; exists && val != nil {
		t.Errorf("expected name_color to be null, got %v", val)
	}

	// Verify DB was called with nil nameColor
	if len(mock.upsertCalls) != 1 {
		t.Fatalf("expected 1 upsert call, got %d", len(mock.upsertCalls))
	}
	if mock.upsertCalls[0].nameColor != nil {
		t.Errorf("expected nil nameColor for null input, got %v", *mock.upsertCalls[0].nameColor)
	}
}

func TestPatchCosmetics_InvalidHex(t *testing.T) {
	viewerID := uuid.New()
	router, _ := setupCosmeticsTest(t, viewerID.String(), "twitch", "12345")

	body := `{"name_color":"notahex"}`
	req := httptest.NewRequest(http.MethodPatch, "/viewer/cosmetics", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid hex, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestPatchCosmetics_MissingViewerID(t *testing.T) {
	// Empty viewer_id simulates a pre-Phase-28 token (old token without viewer_id claim)
	router, _ := setupCosmeticsTest(t, "", "twitch", "12345")

	body := `{"name_color":"#ff6600"}`
	req := httptest.NewRequest(http.MethodPatch, "/viewer/cosmetics", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for missing viewer_id, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestPatchCosmetics_InvalidViewerIDFormat(t *testing.T) {
	// Non-UUID viewer_id value
	router, _ := setupCosmeticsTest(t, "not-a-uuid", "twitch", "12345")

	body := `{"name_color":"#ff6600"}`
	req := httptest.NewRequest(http.MethodPatch, "/viewer/cosmetics", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for invalid viewer_id format, got %d body=%s", w.Code, w.Body.String())
	}
}

// Phase 29: gradient tests

func TestPatchCosmetics_GradientAccepted(t *testing.T) {
	// Premium viewer with a valid gradient should get 200.
	viewerID := uuid.New()
	router, mock := setupCosmeticsTestWithPremium(t, viewerID.String(), "twitch", "12345", true)

	body := `{"name_gradient":{"type":"linear","colors":["#ff0000","#0000ff"],"angle":90}}`
	req := httptest.NewRequest(http.MethodPatch, "/viewer/cosmetics", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for premium gradient, got %d body=%s", w.Code, w.Body.String())
	}

	if len(mock.upsertCalls) != 1 {
		t.Fatalf("expected 1 upsert call, got %d", len(mock.upsertCalls))
	}
	if mock.upsertCalls[0].nameColor != nil {
		t.Errorf("name_color should be nil when gradient is set (mutual exclusion)")
	}
	if mock.upsertCalls[0].nameGradient == nil {
		t.Errorf("expected nameGradient bytes to be set")
	}
}

func TestPatchCosmetics_GradientRejectedNonPremium(t *testing.T) {
	// Non-premium viewer attempting to set gradient should get 403.
	viewerID := uuid.New()
	router, _ := setupCosmeticsTestWithPremium(t, viewerID.String(), "twitch", "12345", false)

	body := `{"name_gradient":{"type":"linear","colors":["#ff0000","#0000ff"],"angle":90}}`
	req := httptest.NewRequest(http.MethodPatch, "/viewer/cosmetics", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for non-premium gradient attempt, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestPatchCosmetics_GradientValidation(t *testing.T) {
	// Invalid gradient (1 color, angle 400) should return 400.
	viewerID := uuid.New()
	router, _ := setupCosmeticsTestWithPremium(t, viewerID.String(), "twitch", "12345", true)

	// Only 1 color — invalid
	body := `{"name_gradient":{"type":"linear","colors":["#ff0000"],"angle":400}}`
	req := httptest.NewRequest(http.MethodPatch, "/viewer/cosmetics", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid gradient (1 color), got %d body=%s", w.Code, w.Body.String())
	}
}

func TestPatchCosmetics_GradientValidation_BadAngle(t *testing.T) {
	// Valid colors but angle out of range should return 400.
	viewerID := uuid.New()
	router, _ := setupCosmeticsTestWithPremium(t, viewerID.String(), "twitch", "12345", true)

	body := `{"name_gradient":{"type":"linear","colors":["#ff0000","#00ff00"],"angle":400}}`
	req := httptest.NewRequest(http.MethodPatch, "/viewer/cosmetics", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for angle 400, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestPatchCosmetics_MutualExclusion(t *testing.T) {
	// Gradient PATCH should nullify name_color in the DB call.
	viewerID := uuid.New()
	router, mock := setupCosmeticsTestWithPremium(t, viewerID.String(), "twitch", "12345", true)

	// Send gradient only — name_color should be nil in upsert call
	body := `{"name_gradient":{"type":"linear","colors":["#aabbcc","#112233"],"angle":45}}`
	req := httptest.NewRequest(http.MethodPatch, "/viewer/cosmetics", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	if len(mock.upsertCalls) != 1 {
		t.Fatalf("expected 1 upsert call, got %d", len(mock.upsertCalls))
	}
	if mock.upsertCalls[0].nameColor != nil {
		t.Errorf("mutual exclusion: name_color should be nil when gradient is set, got %v", *mock.upsertCalls[0].nameColor)
	}

	// Response should have null name_color
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if val, exists := resp["name_color"]; exists && val != nil {
		t.Errorf("response name_color should be null when gradient is set, got %v", val)
	}
}

// Phase 30: avatar frame / flair tests

func TestPatchCosmetics_AvatarFrameID_PremiumAccepted(t *testing.T) {
	// Premium viewer with a valid avatar_frame_id should get 200.
	// Since name_color and name_gradient are absent from the body, the handler
	// routes to UpsertAvatarCosmetics to avoid NULLing out saved name color.
	viewerID := uuid.New()
	router, mock := setupCosmeticsTestWithPremium(t, viewerID.String(), "twitch", "12345", true)

	frameID := uuid.New()
	body := `{"avatar_frame_id":"` + frameID.String() + `"}`
	req := httptest.NewRequest(http.MethodPatch, "/viewer/cosmetics", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for premium avatar_frame_id, got %d body=%s", w.Code, w.Body.String())
	}
	// Avatar-only PATCH routes to UpsertAvatarCosmetics, not UpsertViewerCosmetics.
	if len(mock.upsertCalls) != 0 {
		t.Fatalf("expected 0 full upsert calls for avatar-only PATCH, got %d", len(mock.upsertCalls))
	}
	if len(mock.avatarUpsertCalls) != 1 {
		t.Fatalf("expected 1 avatar upsert call, got %d", len(mock.avatarUpsertCalls))
	}
	if mock.avatarUpsertCalls[0].avatarFrameID == nil {
		t.Error("expected avatarFrameID to be non-nil in avatar upsert call")
	} else if *mock.avatarUpsertCalls[0].avatarFrameID != frameID {
		t.Errorf("expected avatarFrameID=%v, got %v", frameID, *mock.avatarUpsertCalls[0].avatarFrameID)
	}
}

func TestPatchCosmetics_AvatarFrameID_NonPremiumRejected(t *testing.T) {
	// Non-premium viewer attempting to set avatar_frame_id should get 403.
	viewerID := uuid.New()
	router, _ := setupCosmeticsTestWithPremium(t, viewerID.String(), "twitch", "12345", false)

	frameID := uuid.New()
	body := `{"avatar_frame_id":"` + frameID.String() + `"}`
	req := httptest.NewRequest(http.MethodPatch, "/viewer/cosmetics", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for non-premium avatar_frame_id, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestPatchCosmetics_AvatarFlairID_NonPremiumRejected(t *testing.T) {
	// Non-premium viewer attempting to set avatar_flair_id should get 403.
	viewerID := uuid.New()
	router, _ := setupCosmeticsTestWithPremium(t, viewerID.String(), "twitch", "12345", false)

	flairID := uuid.New()
	body := `{"avatar_flair_id":"` + flairID.String() + `"}`
	req := httptest.NewRequest(http.MethodPatch, "/viewer/cosmetics", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for non-premium avatar_flair_id, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestPatchCosmetics_NonPremium_DowngradeClears(t *testing.T) {
	// Non-premium viewer sending only name_color: DB call should pass nil, nil for frame/flair
	// AND the frame/flair clear UPDATE should fire (avatarFrameID = &uuid.Nil in upsert).
	viewerID := uuid.New()
	router, mock := setupCosmeticsTestWithPremium(t, viewerID.String(), "twitch", "12345", false)

	body := `{"name_color":"#00ff00"}`
	req := httptest.NewRequest(http.MethodPatch, "/viewer/cosmetics", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	if len(mock.upsertCalls) != 1 {
		t.Fatalf("expected 1 upsert call, got %d", len(mock.upsertCalls))
	}
	// Downgrade enforcement: avatarFrameID should be &uuid.Nil (clear sentinel)
	if mock.upsertCalls[0].avatarFrameID == nil {
		t.Error("expected avatarFrameID=&uuid.Nil for downgrade clear, got nil pointer")
	} else if *mock.upsertCalls[0].avatarFrameID != uuid.Nil {
		t.Errorf("expected avatarFrameID=uuid.Nil, got %v", *mock.upsertCalls[0].avatarFrameID)
	}
	if mock.upsertCalls[0].avatarFlairID == nil {
		t.Error("expected avatarFlairID=&uuid.Nil for downgrade clear, got nil pointer")
	} else if *mock.upsertCalls[0].avatarFlairID != uuid.Nil {
		t.Errorf("expected avatarFlairID=uuid.Nil, got %v", *mock.upsertCalls[0].avatarFlairID)
	}
}

func TestPatchCosmetics_AvatarOnly_DoesNotClearNameColor(t *testing.T) {
	// Regression test: saving avatar cosmetics (no name_color/name_gradient in body)
	// must NOT call UpsertViewerCosmetics (which would NULL out the stored name color).
	// It must call UpsertAvatarCosmetics instead, leaving name cosmetics untouched.
	viewerID := uuid.New()
	router, mock := setupCosmeticsTestWithPremium(t, viewerID.String(), "twitch", "12345", true)

	frameID := uuid.New()
	flairID := uuid.New()
	body := `{"avatar_frame_id":"` + frameID.String() + `","avatar_flair_id":"` + flairID.String() + `"}`
	req := httptest.NewRequest(http.MethodPatch, "/viewer/cosmetics", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	// Full upsert must NOT be called — it would clear name_color.
	if len(mock.upsertCalls) != 0 {
		t.Fatalf("avatar-only PATCH must not call UpsertViewerCosmetics, got %d call(s)", len(mock.upsertCalls))
	}
	// Avatar-targeted upsert must be called.
	if len(mock.avatarUpsertCalls) != 1 {
		t.Fatalf("expected 1 UpsertAvatarCosmetics call, got %d", len(mock.avatarUpsertCalls))
	}
	c := mock.avatarUpsertCalls[0]
	if c.avatarFrameID == nil || *c.avatarFrameID != frameID {
		t.Errorf("expected avatarFrameID=%v, got %v", frameID, c.avatarFrameID)
	}
	if c.avatarFlairID == nil || *c.avatarFlairID != flairID {
		t.Errorf("expected avatarFlairID=%v, got %v", flairID, c.avatarFlairID)
	}
}

func TestPatchCosmetics_AvatarFrameID_ResponseIncludes(t *testing.T) {
	// Response JSON must include avatar_frame_id field.
	viewerID := uuid.New()
	router, _ := setupCosmeticsTestWithPremium(t, viewerID.String(), "twitch", "12345", true)

	frameID := uuid.New()
	body := `{"avatar_frame_id":"` + frameID.String() + `"}`
	req := httptest.NewRequest(http.MethodPatch, "/viewer/cosmetics", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if _, exists := resp["avatar_frame_id"]; !exists {
		t.Error("response missing avatar_frame_id field")
	}
	if _, exists := resp["avatar_flair_id"]; !exists {
		t.Error("response missing avatar_flair_id field")
	}
}
