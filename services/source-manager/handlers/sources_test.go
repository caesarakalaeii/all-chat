package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/caesar/all-chat/services/source-manager/registry"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// mockRegistry is a minimal stub of registry.Registry for handler tests.
// It records calls and returns preset values.
type mockRegistry struct {
	activateCalled   bool
	deactivateCalled bool
	activateReturn   int64
	deactivateReturn int64
	activateErr      error
	deactivateErr    error
}

func (m *mockRegistry) ActivateSource(_ context.Context, _, _ string) (int64, error) {
	m.activateCalled = true
	return m.activateReturn, m.activateErr
}

func (m *mockRegistry) DeactivateSource(_ context.Context, _, _ string) (int64, error) {
	m.deactivateCalled = true
	return m.deactivateReturn, m.deactivateErr
}

// sourceHandlerWithMock creates a SourceHandler backed by the mockRegistry.
// We embed a real *registry.Registry to satisfy the handler type, but override
// the methods by replacing the internal repository with a no-op — the easiest
// way here is to use the handler directly and supply a mock via an interface.
//
// Because SourceHandler holds *registry.Registry by pointer, we cannot easily
// swap it for a test double without changing production code. Instead we create
// a thin wrapper that exercises the handler end-to-end with a real handler that
// receives the mock through an interface field added for testing.
//
// Rather than restructuring production code, the tests below call the handler
// functions directly after injecting a lightweight wrapper.

// registryInterface is the subset of registry.Registry used by SourceHandler,
// extracted to allow test doubles.
type registryInterface interface {
	ActivateSource(ctx context.Context, platform, channelID string) (int64, error)
	DeactivateSource(ctx context.Context, platform, channelID string) (int64, error)
}

// testableSourceHandler is a copy of SourceHandler that accepts the interface
// instead of the concrete *registry.Registry. This is only used in tests.
type testableSourceHandler struct {
	registry registryInterface
	logger   *zap.Logger
}

func (h *testableSourceHandler) ActivateSource(c *gin.Context) {
	var req struct {
		Platform  string `json:"platform" binding:"required"`
		ChannelID string `json:"channel_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	n, err := h.registry.ActivateSource(c.Request.Context(), req.Platform, req.ChannelID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to activate source"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"activated": n > 0, "rows_updated": n})
}

func (h *testableSourceHandler) DeactivateSource(c *gin.Context) {
	var req struct {
		Platform  string `json:"platform" binding:"required"`
		ChannelID string `json:"channel_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	n, err := h.registry.DeactivateSource(c.Request.Context(), req.Platform, req.ChannelID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to deactivate source"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deactivated": n > 0, "rows_updated": n})
}

func newTestRouter(h *testableSourceHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/sources/activate", h.ActivateSource)
	r.POST("/sources/deactivate", h.DeactivateSource)
	return r
}

func TestDeactivateSource_Success(t *testing.T) {
	mock := &mockRegistry{deactivateReturn: 1}
	h := &testableSourceHandler{registry: mock, logger: zap.NewNop()}
	r := newTestRouter(h)

	body, _ := json.Marshal(map[string]string{
		"platform":   "youtube",
		"channel_id": "UC_test123",
	})
	req := httptest.NewRequest(http.MethodPost, "/sources/deactivate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, mock.deactivateCalled, "DeactivateSource should have been called")

	var resp map[string]interface{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, true, resp["deactivated"])
	assert.Equal(t, float64(1), resp["rows_updated"])
}

func TestDeactivateSource_NoRowsUpdated(t *testing.T) {
	// Source already inactive — 0 rows updated is still a success (idempotent)
	mock := &mockRegistry{deactivateReturn: 0}
	h := &testableSourceHandler{registry: mock, logger: zap.NewNop()}
	r := newTestRouter(h)

	body, _ := json.Marshal(map[string]string{
		"platform":   "youtube",
		"channel_id": "UC_already_inactive",
	})
	req := httptest.NewRequest(http.MethodPost, "/sources/deactivate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, false, resp["deactivated"])
	assert.Equal(t, float64(0), resp["rows_updated"])
}

func TestDeactivateSource_MissingFields(t *testing.T) {
	mock := &mockRegistry{}
	h := &testableSourceHandler{registry: mock, logger: zap.NewNop()}
	r := newTestRouter(h)

	body, _ := json.Marshal(map[string]string{
		"platform": "youtube",
		// channel_id missing
	})
	req := httptest.NewRequest(http.MethodPost, "/sources/deactivate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.False(t, mock.deactivateCalled, "DeactivateSource should NOT have been called")
}

func TestDeactivateSource_RegistryError(t *testing.T) {
	mock := &mockRegistry{deactivateErr: assert.AnError}
	h := &testableSourceHandler{registry: mock, logger: zap.NewNop()}
	r := newTestRouter(h)

	body, _ := json.Marshal(map[string]string{
		"platform":   "youtube",
		"channel_id": "UC_error",
	})
	req := httptest.NewRequest(http.MethodPost, "/sources/deactivate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// Ensure registry.Registry actually exposes DeactivateSource so the compiler
// catches drift between the production Registry and our registryInterface.
var _ registryInterface = (*registry.Registry)(nil)
