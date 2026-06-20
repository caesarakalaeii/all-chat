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

package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// betaTrue / betaFalse / betaErr are canned betaTesterQuerier values.
func betaResult(isBeta bool) betaTesterQuerier {
	return func(_ context.Context, _ string) (bool, error) { return isBeta, nil }
}

// betaNeverCalled fails the test if the querier is invoked — used to prove the
// gate short-circuits before any DB hit.
func betaNeverCalled(t *testing.T) betaTesterQuerier {
	return func(_ context.Context, _ string) (bool, error) {
		t.Fatal("beta-tester querier should not be called when gate is not early-access")
		return false, nil
	}
}

// TestRequireEarlyAccessGateOff: a non-early-access feature lets any authenticated
// user through without consulting the DB (defers to any premium gate on the route).
func TestRequireEarlyAccessGateOff(t *testing.T) {
	gate := &mockGateChecker{isEarlyAccessResult: false}
	handler := RequireEarlyAccessWithQuerier(gate, "beta-feature", betaNeverCalled(t), nil)
	router := newTestRouter("user-123", handler)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestRequireEarlyAccessUnauthenticated: no user_id in context => 401, even before
// the gate is consulted.
func TestRequireEarlyAccessUnauthenticated(t *testing.T) {
	gate := &mockGateChecker{isEarlyAccessResult: true}
	handler := RequireEarlyAccessWithQuerier(gate, "beta-feature", betaNeverCalled(t), nil)
	router := newTestRouter("", handler) // no user_id injected

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestRequireEarlyAccessDeniesNonBeta: early-access gate + non-beta user => 403.
func TestRequireEarlyAccessDeniesNonBeta(t *testing.T) {
	gate := &mockGateChecker{isEarlyAccessResult: true}
	handler := RequireEarlyAccessWithQuerier(gate, "beta-feature", betaResult(false), nil)
	router := newTestRouter("user-123", handler)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

// TestRequireEarlyAccessAllowsBeta: early-access gate + beta-tester user => 200.
func TestRequireEarlyAccessAllowsBeta(t *testing.T) {
	gate := &mockGateChecker{isEarlyAccessResult: true}
	handler := RequireEarlyAccessWithQuerier(gate, "beta-feature", betaResult(true), nil)
	router := newTestRouter("user-123", handler)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestRequireEarlyAccessQuerierError: a DB error while checking beta status => 500
// (fail closed, do not leak the feature).
func TestRequireEarlyAccessQuerierError(t *testing.T) {
	gate := &mockGateChecker{isEarlyAccessResult: true}
	querier := func(_ context.Context, _ string) (bool, error) {
		return false, errors.New("db down")
	}
	handler := RequireEarlyAccessWithQuerier(gate, "beta-feature", querier, nil)
	router := newTestRouter("user-123", handler)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
