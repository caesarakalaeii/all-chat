package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/caesar/all-chat/services/overlay-manager/models"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// mockSourceRepository is used in source handler tests.
// mockOverlayRepository is already defined in overlay_test.go (same package).
type mockSourceRepository struct {
	createFunc        func(context.Context, *models.ChatSource) error
	listByOverlayFunc func(context.Context, string) ([]*models.ChatSource, error)
	getByIDFunc       func(context.Context, string) (*models.ChatSource, error)
	deleteFunc        func(context.Context, string) error
}

func (m *mockSourceRepository) Create(ctx context.Context, source *models.ChatSource) error {
	if m.createFunc != nil {
		return m.createFunc(ctx, source)
	}
	return nil
}

func (m *mockSourceRepository) ListByOverlayID(ctx context.Context, overlayID string) ([]*models.ChatSource, error) {
	if m.listByOverlayFunc != nil {
		return m.listByOverlayFunc(ctx, overlayID)
	}
	return nil, nil
}

func (m *mockSourceRepository) GetByID(ctx context.Context, id string) (*models.ChatSource, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(ctx, id)
	}
	return nil, nil
}

func (m *mockSourceRepository) Delete(ctx context.Context, id string) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, id)
	}
	return nil
}

// TestHandleAddSource_SharedOverlay_Forbidden tests that posting platform=shared_overlay
// with a channel_id that has no accepted share returns 403.
//
// RED state (Wave 0): HandleAddSource has no shared_overlay branch, so validPlatforms
// rejects "shared_overlay" with a 400 "invalid platform" error. The assertion checks for
// 403, so the test FAILS RED.
//
// GREEN state (Wave 1+): HandleAddSource adds a shared_overlay branch that queries
// share_requests and returns 403 when no accepted share exists for channel_id. Test PASSES.
func TestHandleAddSource_SharedOverlay_Forbidden(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// handler with nil db — the shared_overlay branch (once implemented) uses db,
	// but the forbidden case should return 403 BEFORE querying db for a missing share.
	// At Wave 0 there is no shared_overlay branch, so the handler returns 400 (invalid platform),
	// causing this assertion to FAIL (RED state).
	h := &SourcesHandler{
		sourceRepo: &mockSourceRepository{},
		overlayRepo: &mockOverlayRepository{
			getByIDAndUserIDFunc: func(ctx context.Context, id, userID string) (*models.Overlay, error) {
				return &models.Overlay{ID: id, UserID: userID, Name: "Test Overlay"}, nil
			},
		},
		db:     nil, // shared_overlay branch must NOT reach db for the forbidden case
		logger: zap.NewNop(),
	}

	router := gin.New()
	router.POST("/overlays/:id/sources", func(c *gin.Context) {
		c.Set("user_id", "test-user-id")
		h.HandleAddSource(c)
	})

	body := map[string]interface{}{
		"platform":   "shared_overlay",
		"channel_id": "some-uuid-that-has-no-accepted-share",
	}
	bodyBytes, _ := json.Marshal(body)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/overlays/overlay-id/sources", bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	// At Wave 0: no shared_overlay branch exists → handler returns 400 (invalid platform) → FAILS RED
	// After implementation: shared_overlay branch checks share_requests → returns 403 → PASSES GREEN
	assert.Equal(t, http.StatusForbidden, w.Code,
		"Should return 403 when no accepted share exists for shared_overlay platform")
}

// TestHandleAddSource_SharedOverlay_Success tests that posting platform=shared_overlay
// with a valid accepted share channel_id creates the source and returns 201.
// This test requires a real share_requests row in the database for full verification.
func TestHandleAddSource_SharedOverlay_Success(t *testing.T) {
	t.Skip("requires DB for share validation — GREEN verified in integration tests")
}

// TestHandleAddSource_SharedOverlay_FetchesOverlayName asserts that a source with
// platform=shared_overlay stores a human-readable channel_name (the sender's overlay name),
// not a raw UUID as the channel_name.
// This test requires DB to resolve overlay name from channel_id (the sender overlay UUID).
func TestHandleAddSource_SharedOverlay_FetchesOverlayName(t *testing.T) {
	t.Skip("requires DB to resolve overlay name — GREEN verified in integration tests")
}
