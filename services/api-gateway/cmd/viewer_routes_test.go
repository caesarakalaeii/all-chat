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

func TestRegisterViewerRoutes_ForwardsEveryViewerRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	var forwarded []string
	registerViewerRoutes(r.Group("/api/v1"), func(c *gin.Context) {
		forwarded = append(forwarded, c.Request.Method+" "+c.Request.URL.Path)
		c.Status(http.StatusNoContent)
	})

	routes := []struct{ method, path string }{
		{http.MethodGet, "/api/v1/auth/viewer/me"},
		{http.MethodDelete, "/api/v1/auth/viewer/me"}, // erase own viewer data (ADR-0088)
		{http.MethodPost, "/api/v1/auth/viewer/logout"},
		{http.MethodPost, "/api/v1/auth/viewer/chat/send"},
		{http.MethodGet, "/api/v1/auth/viewer/cosmetics"},
		{http.MethodPatch, "/api/v1/auth/viewer/cosmetics"},
		{http.MethodGet, "/api/v1/auth/viewer/linked-platforms"},
		{http.MethodDelete, "/api/v1/auth/viewer/linked-platforms/kick"},
	}
	for _, req := range routes {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(req.method, req.path, nil))
		assert.Equal(t, http.StatusNoContent, w.Code, req.method+" "+req.path)
	}
	assert.Len(t, forwarded, len(routes))
}

// Viewer tokens travel as a bearer. The delete must not pick up the streamer session
// cookie as X-Access-Token: auth-service would read it as the token to revoke.
func TestRegisterViewerRoutes_DeleteDoesNotForwardStreamerCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	var accessHeader string
	registerViewerRoutes(r.Group("/api/v1"), func(c *gin.Context) {
		accessHeader = c.GetHeader("X-Access-Token")
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/auth/viewer/me", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: "streamer-session"})
	r.ServeHTTP(httptest.NewRecorder(), req)
	assert.Empty(t, accessHeader)
}
