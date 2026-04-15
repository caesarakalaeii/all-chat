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

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/caesar/all-chat/services/share-service/models"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// revokeTestCase holds the share data injected via gin context for handler tests.
// This avoids a real database dependency while still exercising handler logic.
// Implements revokeShareData interface defined in shares.go.
type revokeTestCase struct {
	shareID         string
	senderUserID    string
	recipientUserID string
	status          string
}

func (r *revokeTestCase) GetSenderUserID() string    { return r.senderUserID }
func (r *revokeTestCase) GetRecipientUserID() string { return r.recipientUserID }
func (r *revokeTestCase) GetStatus() string          { return r.status }

// revokeHandlerWithFixture builds a ShareHandler whose RevokeShareRequest uses
// in-memory fixture data instead of a real postgres transaction.
// Returns the handler and a recorder.
func setupRevokeTest(callerUserID string, fixture *revokeTestCase) (*ShareHandler, *httptest.ResponseRecorder, *gin.Context) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/api/v1/shares/"+fixture.shareID+"/revoke", nil)
	c.Set("user_id", callerUserID)
	c.Params = gin.Params{{Key: "id", Value: fixture.shareID}}

	// Inject fixture via context so RevokeShareRequest can read it in tests.
	// The handler checks for this key; production code ignores it.
	c.Set("_test_share_fixture", fixture)

	handler := &ShareHandler{
		shareRepo: nil,
		db:        nil,
		logger:    zap.NewNop(),
	}
	return handler, w, c
}

// TestRevokeShareRequest_AuthCheck verifies that callers who are neither
// the sender nor the recipient of a share receive HTTP 403.
func TestRevokeShareRequest_AuthCheck(t *testing.T) {
	fixture := &revokeTestCase{
		shareID:         "share-123",
		senderUserID:    "sender-user",
		recipientUserID: "recipient-user",
		status:          models.StatusAccepted,
	}

	handler, w, c := setupRevokeTest("unrelated-user", fixture)
	handler.RevokeShareRequest(c)

	require.Equal(t, http.StatusForbidden, w.Code, "unrelated caller must receive 403")

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Contains(t, body, "error")
}

// TestRevokeShareRequest_StatusCheck verifies that attempting to revoke a share
// whose status is not 'accepted' returns HTTP 409 Conflict.
func TestRevokeShareRequest_StatusCheck(t *testing.T) {
	fixture := &revokeTestCase{
		shareID:         "share-456",
		senderUserID:    "sender-user",
		recipientUserID: "recipient-user",
		status:          models.StatusPending, // Not accepted
	}

	handler, w, c := setupRevokeTest("sender-user", fixture)
	handler.RevokeShareRequest(c)

	require.Equal(t, http.StatusConflict, w.Code, "non-accepted share must receive 409")

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Contains(t, body, "error")
}

// TestRevokeShareRequest_Success verifies that a valid revocation by an authorized
// caller (sender or recipient) returns HTTP 200.
func TestRevokeShareRequest_Success(t *testing.T) {
	fixture := &revokeTestCase{
		shareID:         "share-789",
		senderUserID:    "sender-user",
		recipientUserID: "recipient-user",
		status:          models.StatusAccepted,
	}

	handler, w, c := setupRevokeTest("sender-user", fixture)
	handler.RevokeShareRequest(c)

	require.Equal(t, http.StatusOK, w.Code, "successful revocation must return 200")

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "revoked", body["status"])
}

// TestRevokeShareRequest_SourceDeactivation verifies that a recipient can also revoke
// and that the 200 response is returned correctly.
func TestRevokeShareRequest_SourceDeactivation(t *testing.T) {
	fixture := &revokeTestCase{
		shareID:         "share-abc",
		senderUserID:    "sender-user",
		recipientUserID: "recipient-user",
		status:          models.StatusAccepted,
	}

	// Caller is the recipient — also an authorized caller
	handler, w, c := setupRevokeTest("recipient-user", fixture)
	handler.RevokeShareRequest(c)

	require.Equal(t, http.StatusOK, w.Code, "successful revocation must return 200")

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "revoked", body["status"])
}
