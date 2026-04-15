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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/caesar/all-chat/services/share-service/featuregates"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// mockFeatureGateDB satisfies featureGateDB interface for testing
type mockFeatureGateDB struct {
	queryRows    []FeatureGateResponse
	queryErr     error
	execRowsAff  int64
	execErr      error
}

func (m *mockFeatureGateDB) QueryFeatureGates(_ context.Context) ([]FeatureGateResponse, error) {
	return m.queryRows, m.queryErr
}

func (m *mockFeatureGateDB) UpdateFeatureGate(_ context.Context, key string, isPremium bool) (int64, error) {
	return m.execRowsAff, m.execErr
}

// mockFeatureGateRedis satisfies featureGateRedis interface for testing
type mockFeatureGateRedis struct {
	publishErr     error
	publishedCalls []publishCall
}

type publishCall struct {
	channel string
	payload string
}

func (m *mockFeatureGateRedis) Publish(_ context.Context, channel string, payload interface{}) error {
	m.publishedCalls = append(m.publishedCalls, publishCall{
		channel: channel,
		payload: payload.(string),
	})
	return m.publishErr
}

// newTestFeatureGatesHandler creates an AdminFeatureGatesHandler with mock dependencies
func newTestFeatureGatesHandler(db featureGateDB, rc featureGateRedis) *AdminFeatureGatesHandler {
	return &AdminFeatureGatesHandler{
		db:     db,
		redis:  rc,
		logger: zap.NewNop(),
	}
}

// setupTestContext creates a gin test context with a response recorder
func setupTestContext(method, url string, body []byte) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	var req *http.Request
	if body != nil {
		req = httptest.NewRequest(method, url, bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, url, nil)
	}
	c.Request = req
	return c, w
}

// TestListGates tests the GET /api/v1/admin/feature-gates endpoint
func TestListGates_ReturnsAllGates(t *testing.T) {
	gates := []FeatureGateResponse{
		{FeatureKey: "sharing", IsPremium: true, Description: "Overlay share requests"},
		{FeatureKey: "custom-badges", IsPremium: false, Description: "Custom badge support"},
	}

	mockDB := &mockFeatureGateDB{queryRows: gates}
	mockRedis := &mockFeatureGateRedis{}
	h := newTestFeatureGatesHandler(mockDB, mockRedis)

	c, w := setupTestContext(http.MethodGet, "/api/v1/admin/feature-gates", nil)
	h.ListGates(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp []FeatureGateResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Len(t, resp, 2)
	assert.Equal(t, "sharing", resp[0].FeatureKey)
	assert.Equal(t, true, resp[0].IsPremium)
	assert.Equal(t, "custom-badges", resp[1].FeatureKey)
	assert.Equal(t, false, resp[1].IsPremium)
}

func TestListGates_ReturnsEmptyArrayWhenNoGates(t *testing.T) {
	mockDB := &mockFeatureGateDB{queryRows: []FeatureGateResponse{}}
	mockRedis := &mockFeatureGateRedis{}
	h := newTestFeatureGatesHandler(mockDB, mockRedis)

	c, w := setupTestContext(http.MethodGet, "/api/v1/admin/feature-gates", nil)
	h.ListGates(c)

	assert.Equal(t, http.StatusOK, w.Code)

	// Must be [] not null
	assert.Contains(t, w.Body.String(), "[]")

	var resp []FeatureGateResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotNil(t, resp)
	assert.Len(t, resp, 0)
}

func TestListGates_Returns500OnDBError(t *testing.T) {
	mockDB := &mockFeatureGateDB{queryErr: errors.New("connection lost")}
	mockRedis := &mockFeatureGateRedis{}
	h := newTestFeatureGatesHandler(mockDB, mockRedis)

	c, w := setupTestContext(http.MethodGet, "/api/v1/admin/feature-gates", nil)
	h.ListGates(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "failed to load feature gates")
}

// TestUpdateGate tests the PATCH /api/v1/admin/feature-gates/:key endpoint
func TestUpdateGate_SetIsPremiumFalse(t *testing.T) {
	// CRITICAL: Test that is_premium=false is accepted — validates *bool pointer approach
	mockDB := &mockFeatureGateDB{execRowsAff: 1}
	mockRedis := &mockFeatureGateRedis{}
	h := newTestFeatureGatesHandler(mockDB, mockRedis)

	body, _ := json.Marshal(map[string]interface{}{"is_premium": false})
	c, w := setupTestContext(http.MethodPatch, "/api/v1/admin/feature-gates/sharing", body)
	c.Params = gin.Params{{Key: "key", Value: "sharing"}}
	h.UpdateGate(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "sharing", resp["feature_key"])
	assert.Equal(t, false, resp["is_premium"])

	// Verify Redis invalidation was published
	require.Len(t, mockRedis.publishedCalls, 1)
	assert.Equal(t, featuregates.PubSubChannel, mockRedis.publishedCalls[0].channel)
	assert.Equal(t, "sharing", mockRedis.publishedCalls[0].payload)
}

func TestUpdateGate_SetIsPremiumTrue(t *testing.T) {
	mockDB := &mockFeatureGateDB{execRowsAff: 1}
	mockRedis := &mockFeatureGateRedis{}
	h := newTestFeatureGatesHandler(mockDB, mockRedis)

	body, _ := json.Marshal(map[string]interface{}{"is_premium": true})
	c, w := setupTestContext(http.MethodPatch, "/api/v1/admin/feature-gates/sharing", body)
	c.Params = gin.Params{{Key: "key", Value: "sharing"}}
	h.UpdateGate(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "sharing", resp["feature_key"])
	assert.Equal(t, true, resp["is_premium"])

	require.Len(t, mockRedis.publishedCalls, 1)
	assert.Equal(t, featuregates.PubSubChannel, mockRedis.publishedCalls[0].channel)
}

func TestUpdateGate_Returns404ForNonExistentKey(t *testing.T) {
	mockDB := &mockFeatureGateDB{execRowsAff: 0} // 0 rows affected = key not found
	mockRedis := &mockFeatureGateRedis{}
	h := newTestFeatureGatesHandler(mockDB, mockRedis)

	body, _ := json.Marshal(map[string]interface{}{"is_premium": false})
	c, w := setupTestContext(http.MethodPatch, "/api/v1/admin/feature-gates/unknown-feature", body)
	c.Params = gin.Params{{Key: "key", Value: "unknown-feature"}}
	h.UpdateGate(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "feature gate not found")

	// No Redis publish when gate not found
	assert.Len(t, mockRedis.publishedCalls, 0)
}

func TestUpdateGate_Returns400WhenBodyMissing(t *testing.T) {
	mockDB := &mockFeatureGateDB{}
	mockRedis := &mockFeatureGateRedis{}
	h := newTestFeatureGatesHandler(mockDB, mockRedis)

	// Empty body — no is_premium field
	c, w := setupTestContext(http.MethodPatch, "/api/v1/admin/feature-gates/sharing", []byte("{}"))
	c.Params = gin.Params{{Key: "key", Value: "sharing"}}
	h.UpdateGate(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "is_premium field required")
}

func TestUpdateGate_Returns400WhenBodyInvalid(t *testing.T) {
	mockDB := &mockFeatureGateDB{}
	mockRedis := &mockFeatureGateRedis{}
	h := newTestFeatureGatesHandler(mockDB, mockRedis)

	// Malformed JSON
	c, w := setupTestContext(http.MethodPatch, "/api/v1/admin/feature-gates/sharing", []byte("not-json"))
	c.Params = gin.Params{{Key: "key", Value: "sharing"}}
	h.UpdateGate(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateGate_PublishErrorDoesNotFailRequest(t *testing.T) {
	// Redis publish failing should not cause the request to fail — DB is already updated
	mockDB := &mockFeatureGateDB{execRowsAff: 1}
	mockRedis := &mockFeatureGateRedis{publishErr: errors.New("redis unreachable")}
	h := newTestFeatureGatesHandler(mockDB, mockRedis)

	body, _ := json.Marshal(map[string]interface{}{"is_premium": false})
	c, w := setupTestContext(http.MethodPatch, "/api/v1/admin/feature-gates/sharing", body)
	c.Params = gin.Params{{Key: "key", Value: "sharing"}}
	h.UpdateGate(c)

	// Must still return 200 despite Redis error
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "sharing")
}
