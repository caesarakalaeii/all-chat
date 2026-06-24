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

package auth

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Cookie names shared by auth-service (issuer) and the api-gateway (reader).
const (
	CookieAccessToken  = "access_token"
	CookieRefreshToken = "refresh_token"
)

// SetAuthCookies issues httpOnly; Secure; SameSite=Lax cookies for the access
// and refresh JWTs. The access cookie is Path=/ (every same-origin request);
// the refresh cookie is Path=/api/v1/auth/ (only auth routes receive it).
// See docs/pi/specs/2026-06-23-h3-cookie-auth-design.md.
func SetAuthCookies(c *gin.Context, accessToken, refreshToken string, accessTTL, refreshTTL time.Duration) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     CookieAccessToken,
		Value:    accessToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(accessTTL.Seconds()),
	})
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     CookieRefreshToken,
		Value:    refreshToken,
		Path:     "/api/v1/auth/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(refreshTTL.Seconds()),
	})
}

// ClearAuthCookies expires both auth cookies (Max-Age=-1) so the browser
// deletes them. The refresh cookie Path must match the one used at issue time.
func ClearAuthCookies(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name: CookieAccessToken, Value: "", Path: "/", HttpOnly: true, Secure: true,
		SameSite: http.SameSiteLaxMode, MaxAge: -1,
	})
	http.SetCookie(c.Writer, &http.Cookie{
		Name: CookieRefreshToken, Value: "", Path: "/api/v1/auth/", HttpOnly: true, Secure: true,
		SameSite: http.SameSiteLaxMode, MaxAge: -1,
	})
}
