package creditroll

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/caesar/all-chat/services/overlay-manager/models"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
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
			wantMin:   3500, // Allow some variance
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
			wantMin:   30 * 24 * 60 * 60, // Exactly 30 days
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

// setupTestHandlerWithRedis creates a Handler wired to a miniredis instance.
func setupTestHandlerWithRedis(t *testing.T, configRepo ConfigRepository, overlayRepo OverlayRepository) (*Handler, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rc.Close() })
	h := &Handler{
		configRepo:  configRepo,
		overlayRepo: overlayRepo,
		sourceRepo:  &mockSourceRepo{},
		redis:       rc,
		logger:      zap.NewNop(),
	}
	return h, mr
}

// writeActiveSession writes a session hash into miniredis as the API Gateway would.
func writeActiveSession(mr *miniredis.Miniredis, overlayID, sessionID string, startedAt time.Time) {
	key := "session:active:" + overlayID
	mr.HSet(key, "session_id", sessionID)
	mr.HSet(key, "started_at", startedAt.UTC().Format(time.RFC3339))
	mr.HSet(key, "state", "ACTIVE")
	mr.HSet(key, "event_count", "0")
}

// TestHandleGetCreditRoll_ActiveSession_ReturnsDuration verifies that when a stream has been
// running for 4 hours, the credit roll response includes a non-zero session_duration_seconds.
// This is the core regression test for the "Duration: 0 minutes" bug.
func TestHandleGetCreditRoll_ActiveSession_ReturnsDuration(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const overlayID = "overlay-duration-test"
	const sessionID = "session-abc"
	startedAt := time.Now().UTC().Add(-4 * time.Hour) // stream started 4 hours ago

	overlay := &models.Overlay{ID: overlayID}
	configRepo := &mockConfigRepo{}
	overlayRepo := &mockOverlayRepo{overlay: overlay}

	h, mr := setupTestHandlerWithRedis(t, configRepo, overlayRepo)
	writeActiveSession(mr, overlayID, sessionID, startedAt)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/public/"+overlayID+"/credit-roll", nil)
	c.Params = gin.Params{{Key: "id", Value: overlayID}}

	h.HandleGetCreditRoll(c)

	if w.Code != http.StatusOK {
		t.Fatalf("HandleGetCreditRoll() with active session: got status %d, want %d\nbody: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp models.CreditRollResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("HandleGetCreditRoll() response not valid JSON: %v", err)
	}

	// session_duration_seconds must reflect ~4 hours (14400s), not 0.
	const minExpected = 4*60*60 - 60  // 4 hours minus 1 minute of tolerance
	const maxExpected = 4*60*60 + 60  // 4 hours plus 1 minute of tolerance
	if resp.SessionDurationSeconds < minExpected || resp.SessionDurationSeconds > maxExpected {
		t.Errorf("HandleGetCreditRoll() session_duration_seconds = %d, want ~%d (4 hours); "+
			"this is the 'Duration: 0 minutes' bug: session_started_at=%s",
			resp.SessionDurationSeconds, 4*60*60, resp.SessionStartedAt.Format(time.RFC3339))
	}

	// session_started_at must match the stored start time (within 1 second due to RFC3339 truncation).
	if resp.SessionStartedAt.IsZero() {
		t.Errorf("HandleGetCreditRoll() session_started_at is zero")
	}
	diff := resp.SessionStartedAt.Sub(startedAt)
	if diff < -time.Second || diff > time.Second {
		t.Errorf("HandleGetCreditRoll() session_started_at = %s, want ~%s",
			resp.SessionStartedAt.Format(time.RFC3339), startedAt.Format(time.RFC3339))
	}
}

// TestHandleGetCreditRoll_NoSession_Returns500 verifies that when there is no active session
// and no completed session in the database, the handler returns 500 (not a 0-duration response).
func TestHandleGetCreditRoll_NoSession_Returns500(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const overlayID = "overlay-no-session"

	overlay := &models.Overlay{ID: overlayID}
	configRepo := &mockConfigRepo{} // GetMostRecentCompletedSession returns "no session" error
	overlayRepo := &mockOverlayRepo{overlay: overlay}

	h, _ := setupTestHandlerWithRedis(t, configRepo, overlayRepo)
	// No session written to Redis

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/public/"+overlayID+"/credit-roll", nil)
	c.Params = gin.Params{{Key: "id", Value: overlayID}}

	h.HandleGetCreditRoll(c)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("HandleGetCreditRoll() with no session: got status %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

// TestHandleGetCreditRoll_CorruptedSession_RepairsAndReturnsDuration verifies that when the
// session in Redis is corrupted (missing started_at), the handler auto-repairs it but the
// repaired session's duration starts from NOW — so the caller gets a valid (non-error) response.
// The duration will be near 0 for a just-repaired session, which is documented behaviour.
func TestHandleGetCreditRoll_CorruptedSession_Repairs(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const overlayID = "overlay-corrupt"

	overlay := &models.Overlay{ID: overlayID}
	configRepo := &mockConfigRepo{}
	overlayRepo := &mockOverlayRepo{overlay: overlay}

	h, mr := setupTestHandlerWithRedis(t, configRepo, overlayRepo)

	// Write a corrupted session: has session_id and state but no started_at
	key := "session:active:" + overlayID
	mr.HSet(key, "session_id", "corrupt-session-id")
	mr.HSet(key, "state", "ACTIVE")
	// deliberately omit "started_at"

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/public/"+overlayID+"/credit-roll", nil)
	c.Params = gin.Params{{Key: "id", Value: overlayID}}

	h.HandleGetCreditRoll(c)

	// The handler should auto-repair and return 200, not 500
	if w.Code != http.StatusOK {
		t.Fatalf("HandleGetCreditRoll() with corrupted session: got status %d, want %d\nbody: %s",
			w.Code, http.StatusOK, w.Body.String())
	}

	var resp models.CreditRollResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("HandleGetCreditRoll() response not valid JSON: %v", err)
	}

	// The session_id must have changed (repaired)
	if resp.SessionID == "corrupt-session-id" {
		t.Errorf("HandleGetCreditRoll() session_id after repair = %q, want a new UUID (repair did not fire)", resp.SessionID)
	}
}

// TestHandleGetCreditRoll_ZombieHash_TreatedAsNoSession is the regression test for the
// "Duration: 0 minutes" bug.
//
// Root cause: incrementCreditRollDisplay fires a background HIncrBy on "session:active:{id}"
// after the session is deleted by EndSession.  Redis HIncrBy creates the key if absent, leaving
// a hash that contains only "credit_roll_displayed_count".  The next call to getActiveSession
// sees a non-empty hash (len > 0), attempts to read "started_at", finds it missing, and returns
// "started_at missing from session".  getOrRepairSession matches "started_at" in that error and
// triggers the repair path, which creates a new session with startedAt=time.Now().
// calculateSessionDuration(time.Now()) == 0, so the response carries session_duration_seconds=0.
//
// Fix: getActiveSession must treat a hash with no "session_id" field as "no active session",
// not as a corrupted session — so the no-session fallback path (DB lookup) is taken instead
// of the repair path.
func TestHandleGetCreditRoll_ZombieHash_TreatedAsNoSession(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const overlayID = "overlay-zombie"

	overlay := &models.Overlay{ID: overlayID}
	configRepo := &mockConfigRepo{} // GetMostRecentCompletedSession returns error → no DB fallback
	overlayRepo := &mockOverlayRepo{overlay: overlay}

	h, mr := setupTestHandlerWithRedis(t, configRepo, overlayRepo)

	// Simulate the zombie hash: only credit_roll_displayed_count present (no session_id, no started_at).
	// This is what incrementCreditRollDisplay creates when it fires after EndSession deletes the key.
	key := "session:active:" + overlayID
	mr.HSet(key, "credit_roll_displayed_count", "3")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/public/"+overlayID+"/credit-roll", nil)
	c.Params = gin.Params{{Key: "id", Value: overlayID}}

	h.HandleGetCreditRoll(c)

	// With no real session and no DB fallback the handler must return 500 (not 200 with duration=0).
	// Before the fix, this returned 200 with session_duration_seconds=0 because the zombie hash
	// triggered the repair path, creating a new session with startedAt=time.Now().
	if w.Code == http.StatusOK {
		var resp models.CreditRollResponse
		_ = json.NewDecoder(w.Body).Decode(&resp)
		t.Errorf("HandleGetCreditRoll() with zombie hash returned 200 with session_duration_seconds=%d; "+
			"expected 500 — zombie hash must be treated as no session, not repaired",
			resp.SessionDurationSeconds)
	}
	if w.Code != http.StatusInternalServerError {
		t.Errorf("HandleGetCreditRoll() with zombie hash: got status %d, want %d", w.Code, http.StatusInternalServerError)
	}
}
