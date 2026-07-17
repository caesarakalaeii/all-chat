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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// onboardingTestRouter wires HandleUpdateOnboarding behind a stub middleware
// that injects the given context values (standing in for JWTAuthWithRevocation).
// The guard paths under test (401/403/400) never reach the repository, so a
// zero repo is fine; the success path is covered by
// repository.TestUserRepository_SetOnboardingCompleted.
func onboardingTestRouter(ctxValues map[string]string) *gin.Engine {
	router := setupTestRouter()
	h := &AuthHandler{logger: zap.NewNop()}
	router.PATCH("/me/onboarding", func(c *gin.Context) {
		for k, v := range ctxValues {
			c.Set(k, v)
		}
		h.HandleUpdateOnboarding(c)
	})
	return router
}

func TestHandleUpdateOnboarding_Unauthenticated(t *testing.T) {
	router := onboardingTestRouter(nil) // no user_id in context

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/me/onboarding", strings.NewReader(`{"completed":true}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestHandleUpdateOnboarding_ImpersonationForbidden(t *testing.T) {
	router := onboardingTestRouter(map[string]string{
		"user_id":         "11111111-1111-1111-1111-111111111111",
		"impersonated_by": "22222222-2222-2222-2222-222222222222",
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/me/onboarding", strings.NewReader(`{"completed":true}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestHandleUpdateOnboarding_InvalidBody(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"empty body", ``},
		{"missing completed", `{}`},
		{"wrong type", `{"completed":"yes"}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			router := onboardingTestRouter(map[string]string{
				"user_id": "11111111-1111-1111-1111-111111111111",
			})

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPatch, "/me/onboarding", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
			}
		})
	}
}
