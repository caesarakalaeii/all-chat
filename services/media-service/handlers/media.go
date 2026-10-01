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

// Package handlers serves the media-service HTTP API: presigned MinIO
// uploads plus the registry list/delete (issue #949).
package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/caesar/all-chat/services/media-service/models"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// MaxUploadBytes is the per-object upload limit (10 MiB).
const MaxUploadBytes = 10 << 20

// allowedContentTypes is the closed set the presign endpoint accepts
// (ADR-0064 alert media: sounds, images, GIFs, and short video clips).
var allowedContentTypes = map[string]bool{
	"image/png":  true,
	"image/jpeg": true,
	"image/gif":  true,
	"image/webp": true,
	"audio/mpeg": true,
	"audio/ogg":  true,
	"audio/wav":  true,
	"audio/webm": true,
	"video/webm": true,
}

// MediaRegistry is the media_objects index (migration 099). Handlers depend
// on the seam, not the Postgres repository, so tests run without a database.
type MediaRegistry interface {
	Create(ctx context.Context, obj *models.MediaObject) error
	CountByUser(ctx context.Context, userID string) (int, error)
	ListByUser(ctx context.Context, userID string) ([]models.MediaObject, error)
	DeleteByOwner(ctx context.Context, userID, objectKey string) (bool, error)
}

// ObjectStore is the MinIO seam: presign an upload URL and remove an object.
// The concrete implementation lives in the storage package.
type ObjectStore interface {
	Available() bool
	PresignPut(ctx context.Context, objectKey string, expiry time.Duration) (string, error)
	Remove(ctx context.Context, objectKey string) error
}

// MediaConfig carries the env-derived settings the handlers need.
type MediaConfig struct {
	// PublicBaseURL is the prefix public URLs are built from
	// (public_url = PublicBaseURL + "/" + object_key).
	PublicBaseURL string
	// PresignExpiry is how long a presigned PUT URL stays valid.
	PresignExpiry time.Duration
	// MaxObjectsPerUser is the per-user quota on registered media.
	MaxObjectsPerUser int
}

// MediaHandler serves the /media routes.
type MediaHandler struct {
	registry MediaRegistry
	store    ObjectStore
	cfg      MediaConfig
	log      *zap.Logger
}

// NewMediaHandler wires the handler. store may be a nil-backed unavailable
// store (storage.Disabled) — media routes then answer 503.
func NewMediaHandler(registry MediaRegistry, store ObjectStore, cfg MediaConfig, log *zap.Logger) *MediaHandler {
	return &MediaHandler{registry: registry, store: store, cfg: cfg, log: log}
}

// RequireStore gates media routes on the object store being configured.
func (h *MediaHandler) RequireStore(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented"})
	c.Abort()
}

// Presign validates an upload request and returns the presigned PUT URL.
func (h *MediaHandler) Presign(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented"})
}

// List returns the caller's registered media from the DB registry.
func (h *MediaHandler) List(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented"})
}

// Delete removes the registry row and the MinIO object, owner-checked.
func (h *MediaHandler) Delete(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "not implemented"})
}
