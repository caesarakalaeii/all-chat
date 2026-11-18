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
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Accepting the terms is the user's own act. Each refused caller is rejected
// before persistence: h.userRepo is nil, so a regressed guard panics instead of
// recording an acceptance on the user's behalf. The success path is covered by
// repository.TestUserRepository_AcceptTos.
func TestHandleAcceptTos_RefusedCallers(t *testing.T) {
	cases := []struct {
		name string
		ctx  map[string]string
		want int
	}{
		{"unauthenticated", nil, http.StatusUnauthorized},
		{"impersonating admin", map[string]string{
			"user_id":         testUserID,
			"impersonated_by": "22222222-2222-2222-2222-222222222222",
		}, http.StatusForbidden},
		{"personal access token", patCtx(), http.StatusForbidden},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			router := gin.New()
			h := &AuthHandler{logger: zap.NewNop()}
			router.POST("/me/tos-acceptance", injectCtx(tc.ctx), h.HandleAcceptTos)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/me/tos-acceptance", nil))

			if w.Code != tc.want {
				t.Fatalf("status = %d, want %d; body=%s", w.Code, tc.want, w.Body.String())
			}
		})
	}
}
