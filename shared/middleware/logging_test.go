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
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// Backends sit behind the api-gateway, which passes Envoy's X-Forwarded-For on,
// and gin trusts every proxy by default, so the client IP gin's stock access
// log prints is the visitor's real address.
func TestGinAccessLog_AnonymisesIPAndRedactsQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var out bytes.Buffer
	prev := gin.DefaultWriter
	gin.DefaultWriter = &out
	t.Cleanup(func() { gin.DefaultWriter = prev })

	router := gin.New()
	router.Use(GinAccessLog())
	router.GET("/overlays", func(c *gin.Context) { c.Status(http.StatusOK) })
	router.GET("/health/live", func(c *gin.Context) { c.Status(http.StatusOK) })

	for _, path := range []string{"/overlays?code=oauth-secret&page=2", "/health/live"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.RemoteAddr = "10.42.0.7:51234"
		req.Header.Set("X-Forwarded-For", "203.0.113.42, 10.42.0.3")
		router.ServeHTTP(httptest.NewRecorder(), req)
	}

	logged := out.String()
	assert.NotContains(t, logged, "203.0.113.42")
	assert.Contains(t, logged, "203.0.113.0")
	assert.NotContains(t, logged, "oauth-secret")
	assert.Contains(t, logged, "page=2")
	assert.Contains(t, logged, "/overlays")
	assert.NotContains(t, logged, "/health/live", "probes stay out of the access log")
}
