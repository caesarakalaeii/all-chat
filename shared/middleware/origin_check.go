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
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
)

// OriginCheck is a stateless CSRF defense paired with SameSite=Lax cookies.
// On state-changing methods (POST/PUT/DELETE/PATCH), if the request carries
// an Origin (or Referer fallback), it must be in the allowed list. Absent
// Origin/Referer is allowed (non-browser API clients authenticate via
// Authorization header). Safe methods (GET/HEAD/OPTIONS) are not checked.
// See docs/pi/specs/2026-06-23-h3-cookie-auth-design.md.
func OriginCheck(allowed []string) gin.HandlerFunc {
	allowedSet := make(map[string]bool, len(allowed))
	for _, o := range allowed {
		allowedSet[o] = true
	}
	return func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch:
		default:
			c.Next()
			return
		}
		origin := c.GetHeader("Origin")
		if origin == "" {
			origin = c.GetHeader("Referer")
			// Referer is a full URL; strip to origin (scheme://host).
			if origin != "" {
				if u, err := url.Parse(origin); err == nil && u.Host != "" {
					origin = u.Scheme + "://" + u.Host
				}
			}
		}
		if origin == "" {
			// Non-browser client (no Origin/Referer) — allowed; relies on
			// Authorization header for auth.
			c.Next()
			return
		}
		if !allowedSet[origin] {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "origin not allowed"})
			return
		}
		c.Next()
	}
}
