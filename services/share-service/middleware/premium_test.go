package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// mockGateChecker implements GateChecker for testing
type mockGateChecker struct {
	isPremiumResult bool
}

func (m *mockGateChecker) IsPremium(_ string) bool {
	return m.isPremiumResult
}

// newTestRouter creates a gin router that injects a user_id into the context
// before calling the premium middleware, simulating JWTAuth middleware.
func newTestRouter(userID string, handler gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/test", func(c *gin.Context) {
		if userID != "" {
			c.Set("user_id", userID)
		}
		c.Next()
	}, handler, func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	return router
}

// TestRequirePremiumGateFree tests that when the gate says feature is free,
// all authenticated users are allowed through without a DB check.
// D-15: gate is_premium=false => all authenticated users pass
func TestRequirePremiumGateFree(t *testing.T) {
	gate := &mockGateChecker{isPremiumResult: false}

	// When gate is free, DB is never queried. Pass nil db — if DB were called,
	// the middleware would panic, exposing the bug. Tests confirm DB not called.
	handler := RequirePremium(nil, gate, "sharing", nil)
	router := newTestRouter("user-123", handler)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestRequirePremiumGatePremiumDeniesNonPremiumUser tests that when gate requires premium,
// a non-premium user gets 403.
// D-16: gate is_premium=true + user is_premium=false => 403
func TestRequirePremiumGatePremiumDeniesNonPremiumUser(t *testing.T) {
	gate := &mockGateChecker{isPremiumResult: true}

	handler := RequirePremiumWithQuerier(gate, "sharing", func(_ context.Context, _ string) (bool, error) {
		return false, nil // user is not premium
	}, nil)
	router := newTestRouter("user-456", handler)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Body.String(), "Premium feature required")
}

// TestRequirePremiumGatePremiumAllowsPremiumUser tests that when gate requires premium,
// a premium user is allowed through.
// D-16: gate is_premium=true + user is_premium=true => 200
func TestRequirePremiumGatePremiumAllowsPremiumUser(t *testing.T) {
	gate := &mockGateChecker{isPremiumResult: true}

	handler := RequirePremiumWithQuerier(gate, "sharing", func(_ context.Context, _ string) (bool, error) {
		return true, nil // user IS premium
	}, nil)
	router := newTestRouter("user-789", handler)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestRequirePremiumGatePremiumUserNotFoundReturns500 tests that when DB query returns
// an error (user not found or connection error), the middleware returns 500.
func TestRequirePremiumGatePremiumUserNotFoundReturns500(t *testing.T) {
	gate := &mockGateChecker{isPremiumResult: true}

	handler := RequirePremiumWithQuerier(gate, "sharing", func(_ context.Context, _ string) (bool, error) {
		return false, assert.AnError // simulate DB error
	}, nil)
	router := newTestRouter("user-999", handler)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestRequirePremiumNoUserIDReturns401 tests that when no user_id is in context,
// the middleware returns 401 regardless of gate state.
func TestRequirePremiumNoUserIDReturns401(t *testing.T) {
	gate := &mockGateChecker{isPremiumResult: true} // gate says premium required

	handler := RequirePremiumWithQuerier(gate, "sharing", func(_ context.Context, _ string) (bool, error) {
		// Should never be called — authentication check is first
		t.Fatal("querier should not be called when user_id is missing")
		return true, nil
	}, nil)
	router := newTestRouter("", handler) // no user_id

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "authentication required")
}

// TestRequirePremiumGateFreeNoUserIDReturns401 tests that even when gate is free,
// unauthenticated requests return 401 (auth check always runs first).
// D-15 applies only to authenticated users.
func TestRequirePremiumGateFreeNoUserIDReturns401(t *testing.T) {
	gate := &mockGateChecker{isPremiumResult: false} // gate says free

	handler := RequirePremium(nil, gate, "sharing", nil)
	router := newTestRouter("", handler) // no user_id

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "authentication required")
}
