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

package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/caesar/all-chat/services/overlay-manager/models"
	"github.com/caesar/all-chat/shared/alerts"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// AlertProfileStore persists alert profiles (alert_profiles, migration 107).
type AlertProfileStore interface {
	ListAlertProfiles(ctx context.Context, overlayID string) ([]alerts.Profile, error)
	UpsertAlertProfiles(ctx context.Context, overlayID string, profiles []alerts.Profile) error
}

// AlertsHandler serves an alerts overlay's profiles and test button (ADR-0069).
type AlertsHandler struct {
	store    AlertProfileStore
	overlays OverlayRepository
	rdb      redis.UniversalClient
	now      func() time.Time
}

// NewAlertsHandler creates the alerts handler.
func NewAlertsHandler(store AlertProfileStore, overlays OverlayRepository, rdb redis.UniversalClient) *AlertsHandler {
	return &AlertsHandler{store: store, overlays: overlays, rdb: rdb, now: time.Now}
}

// ownedOverlay writes the 404 and returns nil unless the caller owns :id.
func (h *AlertsHandler) ownedOverlay(c *gin.Context) *models.Overlay {
	overlay, err := h.overlays.GetByIDAndUserID(c.Request.Context(), c.Param("id"), c.GetString("user_id"))
	if err != nil || overlay == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "overlay not found"})
		return nil
	}
	return overlay
}

// mergedProfiles is every known event type's profile: the stored row where
// there is one, the built-in default otherwise.
func (h *AlertsHandler) mergedProfiles(ctx context.Context, overlayID string) ([]alerts.Profile, error) {
	stored, err := h.store.ListAlertProfiles(ctx, overlayID)
	if err != nil {
		return nil, err
	}
	byType := make(map[string]alerts.Profile, len(stored))
	for _, p := range stored {
		byType[p.EventType] = p
	}
	out := make([]alerts.Profile, 0, len(alerts.EventTypes()))
	for _, et := range alerts.EventTypes() {
		p, ok := byType[et]
		if !ok {
			p, _ = alerts.DefaultProfile(et)
		}
		out = append(out, p)
	}
	return out, nil
}

// HandleGetProfiles handles GET /:id/alerts/profiles.
func (h *AlertsHandler) HandleGetProfiles(c *gin.Context) {
	overlay := h.ownedOverlay(c)
	if overlay == nil {
		return
	}
	profiles, err := h.mergedProfiles(c.Request.Context(), overlay.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load alert profiles"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"profiles": profiles})
}

// HandlePutProfiles handles PUT /:id/alerts/profiles: a bulk upsert, all or
// nothing. Gated by RequirePremium("alerts_editor") at the route.
func (h *AlertsHandler) HandlePutProfiles(c *gin.Context) {
	overlay := h.ownedOverlay(c)
	if overlay == nil {
		return
	}
	if overlay.Kind() != models.OverlayTypeAlerts {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("alert profiles belong to alerts overlays, but this overlay is of type '%s'", overlay.Kind())})
		return
	}
	var req struct {
		Profiles []alerts.Profile `json:"profiles" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	seen := make(map[string]bool, len(req.Profiles))
	for _, p := range req.Profiles {
		if err := p.Validate(); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if seen[p.EventType] {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("event_type %q is listed twice", p.EventType)})
			return
		}
		seen[p.EventType] = true
	}
	if err := h.store.UpsertAlertProfiles(c.Request.Context(), overlay.ID, req.Profiles); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save alert profiles"})
		return
	}
	profiles, err := h.mergedProfiles(c.Request.Context(), overlay.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load alert profiles"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"profiles": profiles})
}

// HandleTestAlert handles POST /:id/alerts/test: publishes a synthetic alert
// straight to overlay:{id}:alerts, so the gateway resolves and forwards it
// exactly like a real one.
func (h *AlertsHandler) HandleTestAlert(c *gin.Context) {
	overlay := h.ownedOverlay(c)
	if overlay == nil {
		return
	}
	if overlay.Kind() == models.OverlayTypeChat {
		c.JSON(http.StatusBadRequest, gin.H{"error": "test alerts need an alerts, goal or list overlay"})
		return
	}
	var req struct {
		EventType string `json:"event_type" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	alert, err := alerts.TestAlert(overlay.ID, req.EventType, h.now().UTC())
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	payload, err := json.Marshal(alert)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build test alert"})
		return
	}
	if err := h.rdb.Publish(c.Request.Context(), "overlay:"+overlay.ID+":alerts", payload).Err(); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to publish test alert"})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"alert_id": alert.AlertID})
}
