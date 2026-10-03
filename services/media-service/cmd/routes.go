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
	"github.com/caesar/all-chat/services/media-service/handlers"
	sharedauth "github.com/caesar/all-chat/shared/auth"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// registerMediaRoutes wires the /api/v1/media route group.
//
// RED STUB — do not ship. This is the unguarded wiring on purpose: the routes
// are registered WITHOUT JWTAuthWithRevocation and WITHOUT RequireStore, i.e.
// exactly the router the review council produced by deleting the auth guard.
// routes_test.go is watched failing against this shape before the guarded
// wiring lands in the next commit.
func registerMediaRoutes(router *gin.Engine, keyChain *sharedauth.KeyChain, redisClient redis.UniversalClient, mediaHandler *handlers.MediaHandler) {
	media := router.Group("/api/v1/media")
	media.POST("/presign", mediaHandler.Presign)
	media.GET("", mediaHandler.List)
	media.DELETE("/*object_key", mediaHandler.Delete)
}