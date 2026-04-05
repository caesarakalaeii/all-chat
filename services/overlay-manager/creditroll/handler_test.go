package creditroll

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/caesar/all-chat/services/overlay-manager/models"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// mockConfigRepo is a test double for ConfigRepository
type mockConfigRepo struct {
	config    *models.CreditRollConfig
	getErr    error
	createErr error
}

func (m *mockConfigRepo) GetByOverlayID(_ context.Context, _ string) (*models.CreditRollConfig, error) {
	return m.config, m.getErr
}

func (m *mockConfigRepo) GetOrCreate(_ context.Context, _ string) (*models.CreditRollConfig, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	if m.config != nil {
		return m.config, nil
	}
	// Simulate auto-create: return a default config
	return &models.CreditRollConfig{
		ID:        "default-id",
		OverlayID: "test-overlay-id",
		Enabled:   true,
	}, nil
}

func (m *mockConfigRepo) Update(_ context.Context, _ *models.CreditRollConfig) error {
	return nil
}

func (m *mockConfigRepo) GetMostRecentCompletedSession(_ context.Context, _ string) (*models.SessionInfo, error) {
	return nil, errors.New("no session")
}

// mockOverlayRepo is a test double for OverlayRepository
type mockOverlayRepo struct {
	overlay *models.Overlay
	err     error
}

func (m *mockOverlayRepo) GetByID(_ context.Context, _ string) (*models.Overlay, error) {
	return m.overlay, m.err
}

func (m *mockOverlayRepo) GetByIDAndUserID(_ context.Context, _, _ string) (*models.Overlay, error) {
	return m.overlay, m.err
}

// mockSourceRepo is a test double for SourceRepository
type mockSourceRepo struct{}

func (m *mockSourceRepo) ListByOverlayID(_ context.Context, _ string) ([]*models.ChatSource, error) {
	return []*models.ChatSource{}, nil
}

func newTestHandler(configRepo ConfigRepository, overlayRepo OverlayRepository) *Handler {
	return &Handler{
		configRepo:  configRepo,
		overlayRepo: overlayRepo,
		sourceRepo:  &mockSourceRepo{},
		logger:      zap.NewNop(),
	}
}

func TestHandleGetPublicConfig_MissingConfigRow_AutoCreates(t *testing.T) {
	gin.SetMode(gin.TestMode)

	overlay := &models.Overlay{ID: "test-overlay-id"}
	configRepo := &mockConfigRepo{config: nil} // no config row exists
	overlayRepo := &mockOverlayRepo{overlay: overlay}

	h := newTestHandler(configRepo, overlayRepo)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/public/test-overlay-id/creditroll", nil)
	c.Params = gin.Params{{Key: "id", Value: "test-overlay-id"}}

	h.HandleGetPublicConfig(c)

	if w.Code != http.StatusOK {
		t.Errorf("HandleGetPublicConfig() with missing config row: got status %d, want %d", w.Code, http.StatusOK)
	}

	var resp models.CreditRollConfig
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("HandleGetPublicConfig() response not valid JSON: %v", err)
	}

	if resp.OverlayID != "test-overlay-id" {
		t.Errorf("HandleGetPublicConfig() auto-created config has overlay_id=%q, want %q", resp.OverlayID, "test-overlay-id")
	}
}

func TestHandleGetPublicConfig_ExistingConfig_ReturnsIt(t *testing.T) {
	gin.SetMode(gin.TestMode)

	overlay := &models.Overlay{ID: "test-overlay-id"}
	existing := &models.CreditRollConfig{
		ID:        "existing-config-id",
		OverlayID: "test-overlay-id",
		Enabled:   true,
		Theme:     "cinematic",
	}
	configRepo := &mockConfigRepo{config: existing}
	overlayRepo := &mockOverlayRepo{overlay: overlay}

	h := newTestHandler(configRepo, overlayRepo)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/public/test-overlay-id/creditroll", nil)
	c.Params = gin.Params{{Key: "id", Value: "test-overlay-id"}}

	h.HandleGetPublicConfig(c)

	if w.Code != http.StatusOK {
		t.Errorf("HandleGetPublicConfig() with existing config: got status %d, want %d", w.Code, http.StatusOK)
	}

	var resp models.CreditRollConfig
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("HandleGetPublicConfig() response not valid JSON: %v", err)
	}

	if resp.ID != "existing-config-id" {
		t.Errorf("HandleGetPublicConfig() got config ID %q, want %q", resp.ID, "existing-config-id")
	}
}

func TestHandleGetPublicConfig_OverlayNotFound_Returns404(t *testing.T) {
	gin.SetMode(gin.TestMode)

	configRepo := &mockConfigRepo{}
	overlayRepo := &mockOverlayRepo{err: errors.New("not found")}

	h := newTestHandler(configRepo, overlayRepo)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/public/nonexistent/creditroll", nil)
	c.Params = gin.Params{{Key: "id", Value: "nonexistent"}}

	h.HandleGetPublicConfig(c)

	if w.Code != http.StatusNotFound {
		t.Errorf("HandleGetPublicConfig() with missing overlay: got status %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestHandleGetPublicConfig_GetOrCreateFails_Returns500(t *testing.T) {
	gin.SetMode(gin.TestMode)

	overlay := &models.Overlay{ID: "test-overlay-id"}
	configRepo := &mockConfigRepo{createErr: errors.New("db connection refused")}
	overlayRepo := &mockOverlayRepo{overlay: overlay}

	h := newTestHandler(configRepo, overlayRepo)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/public/test-overlay-id/creditroll", nil)
	c.Params = gin.Params{{Key: "id", Value: "test-overlay-id"}}

	h.HandleGetPublicConfig(c)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("HandleGetPublicConfig() with DB error: got status %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestParseSessionTime(t *testing.T) {
	tests := []struct {
		name      string
		timeStr   string
		fieldName string
		wantError bool
	}{
		{
			name:      "valid RFC3339 time",
			timeStr:   "2026-02-07T10:00:00Z",
			fieldName: "started_at",
			wantError: false,
		},
		{
			name:      "empty string",
			timeStr:   "",
			fieldName: "started_at",
			wantError: true,
		},
		{
			name:      "invalid format",
			timeStr:   "not-a-time",
			fieldName: "started_at",
			wantError: true,
		},
		{
			name:      "zero time",
			timeStr:   "0001-01-01T00:00:00Z",
			fieldName: "started_at",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseSessionTime(tt.timeStr, tt.fieldName)
			if tt.wantError {
				if err == nil {
					t.Errorf("parseSessionTime() expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("parseSessionTime() unexpected error: %v", err)
				}
				if result.IsZero() {
					t.Errorf("parseSessionTime() returned zero time for valid input")
				}
			}
		})
	}
}

func TestValidateStartedAt(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name      string
		time      time.Time
		wantError bool
	}{
		{
			name:      "valid current time",
			time:      now,
			wantError: false,
		},
		{
			name:      "valid time 1 hour ago",
			time:      now.Add(-1 * time.Hour),
			wantError: false,
		},
		{
			name:      "zero time",
			time:      time.Time{},
			wantError: true,
		},
		{
			name:      "time before 2020",
			time:      time.Date(2019, 12, 31, 23, 59, 59, 0, time.UTC),
			wantError: true,
		},
		{
			name:      "time in future (2 hours)",
			time:      now.Add(2 * time.Hour),
			wantError: true,
		},
		{
			name:      "time at year 0001 (common parse error)",
			time:      time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC),
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateStartedAt(tt.time)
			if tt.wantError {
				if err == nil {
					t.Errorf("validateStartedAt() expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("validateStartedAt() unexpected error: %v", err)
				}
			}
		})
	}
}

func TestCalculateSessionDuration(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name      string
		startedAt time.Time
		wantMin   int
		wantMax   int
	}{
		{
			name:      "zero time returns 0",
			startedAt: time.Time{},
			wantMin:   0,
			wantMax:   0,
		},
		{
			name:      "1 hour ago",
			startedAt: now.Add(-1 * time.Hour),
			wantMin:   3500,  // Allow some variance
			wantMax:   3700,
		},
		{
			name:      "5 minutes ago",
			startedAt: now.Add(-5 * time.Minute),
			wantMin:   290,
			wantMax:   310,
		},
		{
			name:      "future time returns 0",
			startedAt: now.Add(1 * time.Hour),
			wantMin:   0,
			wantMax:   0,
		},
		{
			name:      "40 days ago (capped at 30 days)",
			startedAt: now.Add(-40 * 24 * time.Hour),
			wantMin:   30 * 24 * 60 * 60,      // Exactly 30 days
			wantMax:   30 * 24 * 60 * 60,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateSessionDuration(tt.startedAt)
			if result < tt.wantMin || result > tt.wantMax {
				t.Errorf("calculateSessionDuration() = %d, want between %d and %d", result, tt.wantMin, tt.wantMax)
			}
		})
	}
}
