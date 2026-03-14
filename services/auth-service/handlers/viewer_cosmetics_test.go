package handlers

// Tests for PATCH /viewer/cosmetics handler (Task 2, plan 28-02).
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
		viewerID  uuid.UUID
		nameColor *string
	}
	upsertErr error
}

func (m *mockCosmeticsUpsertRepo) UpsertViewerCosmetics(ctx context.Context, viewerID uuid.UUID, nameColor *string) error {
	m.upsertCalls = append(m.upsertCalls, struct {
		viewerID  uuid.UUID
		nameColor *string
	}{viewerID, nameColor})
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
	_ = zaptest.NewLogger(t) // ensure logger is available
	gin.SetMode(gin.TestMode)

	mock := &mockCosmeticsUpsertRepo{}
	h := &testCosmeticsHandler{repo: mock}

	router := gin.New()
	router.PATCH("/viewer/cosmetics", func(c *gin.Context) {
		// Simulate JWT middleware: always set viewer_id (even if empty string)
		c.Set("viewer_id", viewerIDStr)
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
