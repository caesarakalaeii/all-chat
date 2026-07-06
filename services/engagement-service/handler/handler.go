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

// Package handler holds the engagement service's HTTP handlers: owner-only
// poll/prediction/config management and viewer-facing web voting/wagering/balance
// (issue #523).
package handler

import (
	"errors"
	"net/http"

	"github.com/caesar/all-chat/services/engagement-service/announcer"
	"github.com/caesar/all-chat/services/engagement-service/publisher"
	"github.com/caesar/all-chat/services/engagement-service/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Handler wires the repository, publisher, and chat announcer into the HTTP layer.
type Handler struct {
	repo      *repository.Repository
	pub       *publisher.Publisher
	announcer *announcer.Announcer
	log       *zap.Logger
}

// New creates a Handler. announce may be a disabled Announcer (its methods no-op).
func New(repo *repository.Repository, pub *publisher.Publisher, announce *announcer.Announcer, log *zap.Logger) *Handler {
	return &Handler{repo: repo, pub: pub, announcer: announce, log: log}
}

// requireUser aborts unless a streamer (user) JWT authenticated the request. The
// shared middleware accepts either token type, so owner routes must reject a
// viewer token explicitly.
func requireUser(c *gin.Context) (string, bool) {
	uid := c.GetString("user_id")
	if uid == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "streamer authentication required"})
		return "", false
	}
	return uid, true
}

// requireViewer aborts unless a viewer JWT (with a durable viewer id) authenticated
// the request.
func requireViewer(c *gin.Context) (uuid.UUID, bool) {
	vidStr := c.GetString("viewer_id")
	if vidStr == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "viewer authentication required"})
		return uuid.Nil, false
	}
	vid, err := uuid.Parse(vidStr)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid viewer identity"})
		return uuid.Nil, false
	}
	return vid, true
}

// overlayIDParam parses and validates the :id path parameter.
func overlayIDParam(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid overlay id"})
		return uuid.Nil, false
	}
	return id, true
}

// requireOwnedOverlay parses :id and verifies the authenticated streamer owns it.
func (h *Handler) requireOwnedOverlay(c *gin.Context, userID string) (uuid.UUID, bool) {
	overlayID, ok := overlayIDParam(c)
	if !ok {
		return uuid.Nil, false
	}
	owns, err := h.repo.VerifyOverlayOwnership(c.Request.Context(), overlayID.String(), userID)
	if err != nil {
		h.log.Error("verify overlay ownership", zap.Error(err))
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "ownership check failed"})
		return uuid.Nil, false
	}
	if !owns {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "not the overlay owner"})
		return uuid.Nil, false
	}
	return overlayID, true
}

// uuidParam parses a named path parameter as a UUID.
func uuidParam(c *gin.Context, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "invalid " + name})
		return uuid.Nil, false
	}
	return id, true
}

// overlayIDQuery parses the required overlay_id query parameter (viewer routes).
func overlayIDQuery(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Query("overlay_id"))
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "overlay_id query parameter required"})
		return uuid.Nil, false
	}
	return id, true
}

// statusForRepoErr maps repository sentinels to HTTP codes.
func statusForRepoErr(err error) int {
	switch {
	case errors.Is(err, repository.ErrConflict):
		return http.StatusConflict
	case errors.Is(err, repository.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, repository.ErrNativeNoPoints):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}
