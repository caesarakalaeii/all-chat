package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// mockRevokeRepo records whether RevokeShare was called and with which share_id.
type mockRevokeRepo struct {
	revokeCalledWith string
	revokeErr        error
}

func (m *mockRevokeRepo) RevokeShare(shareID string) error {
	m.revokeCalledWith = shareID
	return m.revokeErr
}

// TestRevokeShareRequest_AuthCheck verifies that callers who are neither
// the sender nor the recipient of a share receive HTTP 403.
func TestRevokeShareRequest_AuthCheck(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("DELETE", "/api/v1/shares/share-123", nil)

	// Caller is a third party — not the sender or recipient
	c.Set("user_id", "unrelated-user")
	c.Params = gin.Params{{Key: "id", Value: "share-123"}}

	handler := &ShareHandler{
		shareRepo: nil,
		logger:    zap.NewNop(),
	}

	// RevokeShareRequest does not yet exist — this causes a compile error (RED gate).
	handler.RevokeShareRequest(c)

	require.Equal(t, http.StatusForbidden, w.Code, "unrelated caller must receive 403")

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Contains(t, body, "error")
}

// TestRevokeShareRequest_StatusCheck verifies that attempting to revoke a share
// whose status is not 'accepted' returns HTTP 409 Conflict.
func TestRevokeShareRequest_StatusCheck(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("DELETE", "/api/v1/shares/share-456", nil)

	// Caller is the sender — valid caller
	c.Set("user_id", "sender-user")
	c.Params = gin.Params{{Key: "id", Value: "share-456"}}

	handler := &ShareHandler{
		shareRepo: nil,
		logger:    zap.NewNop(),
	}

	// RevokeShareRequest does not yet exist — compile error (RED gate).
	handler.RevokeShareRequest(c)

	require.Equal(t, http.StatusConflict, w.Code, "non-accepted share must receive 409")

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Contains(t, body, "error")
}

// TestRevokeShareRequest_Success verifies that a valid revocation by an authorized
// caller (sender or recipient) returns HTTP 200 and calls RevokeShare on the repo.
func TestRevokeShareRequest_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mock := &mockRevokeRepo{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("DELETE", "/api/v1/shares/share-789", nil)

	// Caller is the sender — valid authorized caller
	c.Set("user_id", "sender-user")
	c.Params = gin.Params{{Key: "id", Value: "share-789"}}

	handler := &ShareHandler{
		shareRepo: nil,
		logger:    zap.NewNop(),
	}
	_ = mock

	// RevokeShareRequest does not yet exist — compile error (RED gate).
	handler.RevokeShareRequest(c)

	require.Equal(t, http.StatusOK, w.Code, "successful revocation must return 200")

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "revoked", body["status"])
}

// TestRevokeShareRequest_SourceDeactivation verifies that on successful revocation
// the repository method responsible for deactivating overlay_chat_sources is invoked
// with the correct share_id.
func TestRevokeShareRequest_SourceDeactivation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mock := &mockRevokeRepo{}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("DELETE", "/api/v1/shares/share-abc", nil)

	// Caller is the recipient — also an authorized caller
	c.Set("user_id", "recipient-user")
	c.Params = gin.Params{{Key: "id", Value: "share-abc"}}

	handler := &ShareHandler{
		shareRepo: nil,
		logger:    zap.NewNop(),
	}
	_ = mock

	// RevokeShareRequest does not yet exist — compile error (RED gate).
	handler.RevokeShareRequest(c)

	require.Equal(t, http.StatusOK, w.Code, "successful revocation must return 200")
	assert.Equal(t, "share-abc", mock.revokeCalledWith,
		"RevokeShare must be called with the share_id so overlay_chat_sources is deactivated")
}
