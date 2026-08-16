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
	sharedmiddleware "github.com/caesar/all-chat/shared/middleware"
	"github.com/gin-gonic/gin"
)

// CookieToBearer normalizes the access_token httpOnly cookie into the
// Authorization: Bearer header so the downstream shared.JWTAuth middleware
// (unchanged) validates it. If an Authorization header is already present
// (non-browser clients / old builds), it takes precedence (backward compat).
// This is the cookie-boundary normalization for H3.
//
// A personal access token (ADR-0051) is deliberately NOT promoted from the cookie.
// The cookie is the browser-session channel; a PAT is a header credential for
// non-browser clients, which never set cookies. Promoting one would mean an attacker
// who can plant a cookie in a victim's browser (a forced-login / session-fixation
// shaped bug) could make the victim's browser act as the ATTACKER's account, which is
// a confusing-deputy problem for no legitimate use case. PATs therefore only ever
// arrive in an explicit Authorization header.
func CookieToBearer() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetHeader("Authorization") == "" {
			if tok, err := c.Cookie(auth.CookieAccessToken); err == nil && tok != "" &&
				!sharedmiddleware.IsAPIToken(tok) {
				c.Request.Header.Set("Authorization", "Bearer "+tok)
			}
		}
		c.Next()
	}
}
