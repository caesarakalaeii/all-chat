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
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/caesar/all-chat/services/media-service/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// MaxUploadBytes is the per-object upload limit (10 MiB).
const MaxUploadBytes = 10 << 20

// maxFilenameLength bounds the stored filename and the object key built
// from it (MinIO caps object keys at 1024 bytes; user_id and the uuid
// segment take under 100 of those, the filename gets the rest of the
// headroom).
const maxFilenameLength = 255

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
	PublicBaseURL     string
	PresignExpiry     time.Duration
	MaxObjectsPerUser int
}

type MediaHandler struct {
	registry MediaRegistry
	store    ObjectStore
	cfg      MediaConfig
	log      *zap.Logger
}

// NewMediaHandler wires the handler. store may be storage.Disabled — media
// routes then answer 503 instead of erroring halfway through a request.
func NewMediaHandler(registry MediaRegistry, store ObjectStore, cfg MediaConfig, log *zap.Logger) *MediaHandler {
	return &MediaHandler{registry: registry, store: store, cfg: cfg, log: log}
}

// RequireStore gates media routes on the object store being configured.
// Without MINIO_ENDPOINT the service still starts (env-gating, like
// youtube-listener-innertube's optional subscribers) and every media route
// answers 503.
func (h *MediaHandler) RequireStore(c *gin.Context) {
	if !h.store.Available() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "media storage is not configured",
		})
		c.Abort()
		return
	}
	c.Next()
}

// presignRequest is the POST /media/presign body.
type presignRequest struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
}

// Presign validates an upload request, writes the registry row and returns
// the presigned PUT URL. The row is written BEFORE the client uploads:
// single-node MinIO has no event notification, so completion cannot be
// confirmed, and a row whose object never arrived is a harmless orphan the
// list endpoint tolerates.
func (h *MediaHandler) Presign(c *gin.Context) {
	userID := c.GetString("user_id") // set by JWTAuth middleware
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user id missing from token"})
		return
	}

	var req presignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	filename, err := sanitizeFilename(req.Filename)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !allowedContentTypes[req.ContentType] {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error": "unsupported content type: " + req.ContentType,
		})
		return
	}
	if req.Size <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "size must be positive"})
		return
	}
	if req.Size > MaxUploadBytes {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{
			"error": fmt.Sprintf("file exceeds the %d MiB upload limit", MaxUploadBytes>>20),
		})
		return
	}

	count, err := h.registry.CountByUser(c.Request.Context(), userID)
	if err != nil {
		h.log.Error("Failed to count media objects", zap.String("user_id", userID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check media quota"})
		return
	}
	if count >= h.cfg.MaxObjectsPerUser {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error": fmt.Sprintf("media quota exceeded: at most %d objects per user", h.cfg.MaxObjectsPerUser),
		})
		return
	}

	// Key scheme per ADR-0064: the uuid segment makes the public-read URL
	// unguessable, the user_id prefix scopes the owner check on delete.
	objectKey := fmt.Sprintf("%s/%s/%s", userID, uuid.NewString(), filename)

	uploadURL, err := h.store.PresignPut(c.Request.Context(), objectKey, h.cfg.PresignExpiry)
	if err != nil {
		// Upstream storage failure, not a validation one. No registry row:
		// the client has no URL to upload through.
		h.log.Error("Failed to presign upload", zap.String("object_key", objectKey), zap.Error(err))
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to presign upload"})
		return
	}

	obj := &models.MediaObject{
		UserID:      userID,
		ObjectKey:   objectKey,
		Filename:    filename,
		ContentType: req.ContentType,
		SizeBytes:   req.Size,
	}
	if err := h.registry.Create(c.Request.Context(), obj); err != nil {
		h.log.Error("Failed to register media object", zap.String("object_key", objectKey), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to register media object"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"object_key": objectKey,
		"upload_url": uploadURL,
		"public_url": h.publicURL(objectKey),
	})
}

// List returns the caller's registered media from the DB registry — never a
// bucket listing, so rows whose upload never happened (orphan rows) are
// surfaced as-is.
func (h *MediaHandler) List(c *gin.Context) {
	userID := c.GetString("user_id") // set by JWTAuth middleware
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user id missing from token"})
		return
	}

	media, err := h.registry.ListByUser(c.Request.Context(), userID)
	if err != nil {
		h.log.Error("Failed to list media objects", zap.String("user_id", userID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list media"})
		return
	}
	if media == nil {
		media = []models.MediaObject{}
	}

	c.JSON(http.StatusOK, gin.H{"media": media})
}

// Delete removes the registry row (owner-checked) and then the MinIO object.
// A MinIO failure is logged but NOT fatal: the registry row is the source of
// truth for "the user deleted this", and an orphaned object in a
// public-read bucket is unreachable junk, not a live leak.
func (h *MediaHandler) Delete(c *gin.Context) {
	userID := c.GetString("user_id") // set by JWTAuth middleware
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user id missing from token"})
		return
	}

	// The route is a wildcard (object keys contain slashes); gin hands the
	// param over with a leading slash. Only emptiness is rejected here:
	// keys this service issues may contain ".." (a plain filename like
	// "a..b.png" has no path separators), and the key is only ever used
	// as an opaque MinIO key and an exact-match SQL predicate, so there
	// is no traversal to guard against.
	objectKey := strings.TrimPrefix(c.Param("object_key"), "/")
	if objectKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "object key is required"})
		return
	}

	deleted, err := h.registry.DeleteByOwner(c.Request.Context(), userID, objectKey)
	if err != nil {
		h.log.Error("Failed to delete media object row", zap.String("user_id", userID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete media"})
		return
	}
	if !deleted {
		// 404 rather than 403: an object key that is not yours is
		// indistinguishable from one that does not exist.
		c.JSON(http.StatusNotFound, gin.H{"error": "media object not found"})
		return
	}

	if err := h.store.Remove(c.Request.Context(), objectKey); err != nil {
		h.log.Error("Failed to remove MinIO object (registry row deleted)",
			zap.String("object_key", objectKey), zap.Error(err))
	}

	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

// sanitizeFilename reduces the client-supplied filename to a bare base name
// so no path component can leak into the object key, and bounds its length.
func sanitizeFilename(raw string) (string, error) {
	filename := strings.TrimSpace(raw)
	if filename == "" {
		return "", fmt.Errorf("filename is required")
	}
	filename = path.Base(filename)
	if filename == "/" || filename == `\` {
		return "", fmt.Errorf("filename is required")
	}
	// "." and ".." are dot path segments, not names: every URL client
	// normalizes them away before sending, so a key ending in one could be
	// neither played through public_url nor deleted through the route.
	// Percent-escaping cannot rescue them either — "%2e%2e" normalizes the
	// same way — so reject the degenerate name instead of registering an
	// unreachable object. Names merely containing dots stay legal.
	if filename == "." || filename == ".." {
		return "", fmt.Errorf("filename must be a name, not a path segment")
	}
	if len(filename) > maxFilenameLength {
		return "", fmt.Errorf("filename exceeds %d characters", maxFilenameLength)
	}
	return filename, nil
}

// publicURL builds the play URL for an object key. Each key segment is
// percent-escaped: the key itself stays raw in MinIO and in the registry,
// but a raw "#" or "?" in a URL truncates it into a fragment or query
// string and a bare "%" is an invalid escape, so the advertised URL would
// miss the object it names even though the upload succeeded. S3 decodes the
// request path back to the raw key, so the escaped URL addresses the
// uploaded object.
func (h *MediaHandler) publicURL(objectKey string) string {
	segments := strings.Split(objectKey, "/")
	for i, segment := range segments {
		segments[i] = url.PathEscape(segment)
	}
	return strings.TrimSuffix(h.cfg.PublicBaseURL, "/") + "/" + strings.Join(segments, "/")
}
