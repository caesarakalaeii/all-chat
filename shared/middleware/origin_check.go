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
	"strings"

	"github.com/gin-gonic/gin"
)

// OriginAllowed reports whether origin is permitted by the allowlist. It is the
// single shared origin-matcher used by OriginCheck (CSRF defense), the CORS
// middleware, and the WebSocket origin checkers (audit M4 — previously three
// divergent implementations: OriginCheck did exact-only matching while CORS
// supported "/*" suffix wildcards and WS supported "*" suffix wildcards, so an
// extension origin like moz-extension://<uuid> passed CORS but was 403'd by
// OriginCheck on every state-changing request).
//
// Matching rules (applied per entry, first match wins):
//   - "*"          → allow all origins
//   - exact match  → origin == entry
//   - "/*" suffix  → prefix match on entry minus "/*" (CORS format,
//     e.g. "moz-extension:///*" → prefix "moz-extension://")
//   - "*" suffix  → prefix match on entry minus "*"  (WS format,
//     e.g. "moz-extension://*"  → prefix "moz-extension://")
func OriginAllowed(allowed []string, origin string) bool {
	for _, a := range allowed {
		if a == "*" || a == origin {
			return true
		}
		// Wildcard suffix patterns. Check "/*" first (more specific) so that
		// "moz-extension:///*" strips to "moz-extension://" rather than
		// "moz-extension:///".
		if strings.HasSuffix(a, "/*") {
			if strings.HasPrefix(origin, strings.TrimSuffix(a, "/*")) {
				return true
			}
		} else if strings.HasSuffix(a, "*") {
			if strings.HasPrefix(origin, strings.TrimSuffix(a, "*")) {
				return true
			}
		}
	}
	return false
}

// OriginCheck is a stateless CSRF defense paired with SameSite=Lax cookies.
// On state-changing methods (POST/PUT/DELETE/PATCH), if the request carries
// an Origin (or Referer fallback), it must be in the allowed list. Absent
// Origin/Referer is allowed (non-browser API clients authenticate via
// Authorization header). Safe methods (GET/HEAD/OPTIONS) are not checked.
// See docs/pi/specs/2026-06-23-h3-cookie-auth-design.md.
func OriginCheck(allowed []string) gin.HandlerFunc {
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
		if !OriginAllowed(allowed, origin) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "origin not allowed"})
			return
		}
		c.Next()
	}
}
