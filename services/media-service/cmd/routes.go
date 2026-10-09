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
	"reflect"

	"github.com/caesar/all-chat/services/media-service/handlers"
	sharedauth "github.com/caesar/all-chat/shared/auth"
	"github.com/caesar/all-chat/shared/middleware"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// registerMediaRoutes wires the /api/v1/media route group. All routes are
// JWT-authenticated; user_id comes from the token, never from the request.
// Anonymous reads of media go straight to MinIO (media.allch.at), never
// through this service.
//
// Kept out of main() so the security wiring is testable: routes_test.go drives
// this function over HTTP and fails if either guard is dropped. The handler
// tests inject user_id in a parallel router, so without this seam an unguarded
// router builds, vets and tests green — the defect the review council found
// by deleting the auth guard.
func registerMediaRoutes(router *gin.Engine, keyChain *sharedauth.KeyChain, redisClient redis.UniversalClient, mediaHandler *handlers.MediaHandler) {
	api := router.Group("/api/v1")
	api.Use(middleware.JWTAuthWithRevocation(keyChain, usableRedisClient(redisClient)))
	media := api.Group("/media", mediaHandler.RequireStore)
	media.POST("/presign", mediaHandler.Presign)
	media.GET("", mediaHandler.List)
	media.DELETE("/*object_key", mediaHandler.Delete)
}

// usableRedisClient rejects the classic Go typed-nil trap at the seam where a
// caller hands the revocation client over: a nil *redis.Client stored in a
// redis.UniversalClient does NOT compare == nil, so shared/middleware's "is a
// client wired?" guard passes and its first rdb.Exists call nil-derefs — a
// panic per request that gin.Recovery turns into 500s on every media route
// instead of the documented fail-open skip. main() no longer builds that shape
// (it holds the interface, assigned only after a successful Ping), but this
// seam is the only place a future caller's shape can be caught, and
// routes_test.go pins both shapes: the untyped nil and the typed-nil box.
func usableRedisClient(client redis.UniversalClient) redis.UniversalClient {
	if client == nil {
		return nil
	}
	// Only a nil-able kind can be a typed nil under there. redis.UniversalClient
	// implementations are all pointers; anything else is a live value.
	if v := reflect.ValueOf(client); v.Kind() == reflect.Ptr && v.IsNil() {
		return nil
	}
	return client
}
