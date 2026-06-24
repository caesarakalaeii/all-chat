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
	"github.com/caesar/all-chat/shared/auth"
	"github.com/gin-gonic/gin"
)

// AuthCookieForward copies the access/refresh httpOnly cookies into custom
// request headers (X-Access-Token / X-Refresh-Token) before the gateway proxy
// forwards to auth-service. The proxy's L17 hop-header strip removes Cookie
// (and Referer/Origin) from forwarded requests, so auth-service cannot read
// the raw cookie. These custom headers are NOT in the L17 strip list, so they
// pass through. Used on /auth/refresh, /auth/logout, /auth/stop-impersonation
// (Task 11 wires it). (audit H3)
func AuthCookieForward() gin.HandlerFunc {
	return func(c *gin.Context) {
		if tok, err := c.Cookie(auth.CookieAccessToken); err == nil && tok != "" {
			c.Request.Header.Set("X-Access-Token", tok)
		}
		if tok, err := c.Cookie(auth.CookieRefreshToken); err == nil && tok != "" {
			c.Request.Header.Set("X-Refresh-Token", tok)
		}
		c.Next()
	}
}
