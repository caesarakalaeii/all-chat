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
	localmiddleware "github.com/caesar/all-chat/services/api-gateway/middleware"
	"github.com/gin-gonic/gin"
)

// registerViewerRoutes forwards auth-service's viewer routes on the JWT-protected
// /api/v1 group; auth-service re-validates the viewer token.
func registerViewerRoutes(api *gin.RouterGroup, forward gin.HandlerFunc) {
	api.GET("/auth/viewer/me", forward)
	// Erase the viewer's own data (ADR-0088). No AuthCookieForward: the viewer token is
	// the bearer, and a streamer cookie forwarded as X-Access-Token is another login.
	api.DELETE("/auth/viewer/me", forward)
	// AuthCookieForward so the handler can blacklist the viewer token on logout
	// (audit #18); harmless when the viewer authenticated via the Authorization
	// bearer header (the common case — viewer tokens live in localStorage).
	api.POST("/auth/viewer/logout", localmiddleware.AuthCookieForward(), forward)
	api.POST("/auth/viewer/chat/send", forward)
	api.GET("/auth/viewer/cosmetics", forward)
	api.PATCH("/auth/viewer/cosmetics", forward)
	api.GET("/auth/viewer/linked-platforms", forward)
	api.DELETE("/auth/viewer/linked-platforms/:platform", forward)
	api.GET("/auth/viewer/data-export", forward) // DSGVO Art. 15/20 for viewers
}
