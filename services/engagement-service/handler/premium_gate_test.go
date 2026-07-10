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

package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/caesar/all-chat/shared/featuregates"
	"github.com/caesar/all-chat/shared/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// TestEngagementPremiumGate pins the contract main.go relies on: STARTING a round is gated
// on featuregates.GateEngagement. A non-premium owner is 403'd while the gate is premium, a
// premium owner passes, and once the gate graduates to free every authenticated user passes.
// (The shared RequirePremium middleware is unit-tested on its own; this pins the gate KEY and
// the wiring engagement uses so a wrong key or a dropped gate is caught.)
func TestEngagementPremiumGate(t *testing.T) {
	gin.SetMode(gin.TestMode)

	run := func(gatePremium, userPremium, authenticated bool) int {
		gate := featuregates.NewFeatureGateCacheWithGates(map[string]bool{featuregates.GateEngagement: gatePremium})
		querier := func(ctx context.Context, userID string) (bool, error) { return userPremium, nil }
		r := gin.New()
		r.POST("/overlays/:id/polls",
			func(c *gin.Context) {
				if authenticated {
					c.Set("user_id", "owner-1")
				}
				c.Next()
			},
			middleware.RequirePremiumWithQuerier(gate, featuregates.GateEngagement, querier, nil),
			func(c *gin.Context) { c.JSON(http.StatusCreated, gin.H{"ok": true}) },
		)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/overlays/abc/polls", nil))
		return w.Code
	}

	assert.Equal(t, http.StatusForbidden, run(true, false, true), "premium gate: a non-premium owner is blocked from starting a round")
	assert.Equal(t, http.StatusCreated, run(true, true, true), "premium gate: a premium owner may start a round")
	assert.Equal(t, http.StatusCreated, run(false, false, true), "free gate: any authenticated owner may start a round")
	assert.Equal(t, http.StatusUnauthorized, run(true, false, false), "no user_id: unauthenticated request is rejected before the gate")
}
