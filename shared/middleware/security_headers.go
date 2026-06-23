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

import "github.com/gin-gonic/gin"

// SecurityHeaders adds standard security headers to all responses.
//
// HSTS (Strict-Transport-Security) is included for defense-in-depth when the
// gateway sits behind a TLS-terminating ingress/load-balancer. It is only
// effective when the client connects via HTTPS (audit L16). CSP is intentionally
// NOT set here because per-route CSP is handled at the frontend layer (Next.js
// headers() / nginx).
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		// HSTS: max-age 1 year, include subdomains, preload (audit L16).
		// Only honored by browsers over HTTPS; harmless over HTTP.
		c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
		c.Next()
	}
}
