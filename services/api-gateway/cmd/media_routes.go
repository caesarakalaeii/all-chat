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

import "github.com/gin-gonic/gin"

// registerMediaRoutes forwards media-service's routes (presigned alert-media
// uploads, ADR-0068/0071) on the JWT-protected /api/v1 group; media-service
// re-validates the token and owns every policy check.
func registerMediaRoutes(api *gin.RouterGroup, forward gin.HandlerFunc) {
	api.POST("/media/presign", forward)
	api.GET("/media", forward)
	api.DELETE("/media/*object_key", forward)
}
