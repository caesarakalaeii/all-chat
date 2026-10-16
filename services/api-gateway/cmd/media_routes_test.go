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

package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// The media-service registry entry alone routes nothing: the gateway only
// forwards paths registered on a group. Without these, every upload 404'd at
// the gateway (production, 2026-10-16).
func TestRegisterMediaRoutes_ForwardsEveryMediaServiceRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	var forwarded []string
	registerMediaRoutes(r.Group("/api/v1"), func(c *gin.Context) {
		forwarded = append(forwarded, c.Request.Method+" "+c.Request.URL.Path)
		c.Status(http.StatusNoContent)
	})

	for _, req := range []struct{ method, path string }{
		{http.MethodPost, "/api/v1/media/presign"},
		{http.MethodGet, "/api/v1/media"},
		{http.MethodDelete, "/api/v1/media/u-1/0b6e/air%20horn.mp3"},
	} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(req.method, req.path, nil))
		assert.Equal(t, http.StatusNoContent, w.Code, req.method+" "+req.path)
	}
	assert.Len(t, forwarded, 3)
}
