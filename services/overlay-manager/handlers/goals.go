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

// Goal and list overlay configuration + their public read paths (issue #955,
// ADR-0072). The owner routes mirror the alert-profile routes; the public
// routes serve the unauthenticated OBS renderer its initial state, which the
// alert-processor's goal / list_update frames then keep current.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/caesar/all-chat/services/overlay-manager/models"
	"github.com/caesar/all-chat/shared/alerts"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// GoalListStore persists goal_configs and list_configs rows. A missing row
// reads as (nil, nil).
type GoalListStore interface {
	GetGoalConfig(ctx context.Context, overlayID string) (*alerts.GoalConfig, error)
	UpsertGoalConfig(ctx context.Context, overlayID string, cfg alerts.GoalConfig) error
	GetListConfig(ctx context.Context, overlayID string) (*alerts.ListConfig, error)
	UpsertListConfig(ctx context.Context, overlayID string, cfg alerts.ListConfig) error
}

// GoalListHandler serves the goal and list routes.
type GoalListHandler struct {
	store    GoalListStore
	overlays OverlayRepository
	rdb      redis.UniversalClient
}

// NewGoalListHandler creates a GoalListHandler.
func NewGoalListHandler(store GoalListStore, overlays OverlayRepository, rdb redis.UniversalClient) *GoalListHandler {
	return &GoalListHandler{store: store, overlays: overlays, rdb: rdb}
}

// Unconfigured overlays render these until the owner saves a config.
var (
	defaultGoal = alerts.GoalConfig{Metric: "followers", Platform: "twitch", Target: 100}
	defaultList = alerts.ListConfig{Category: "bits", Limit: 10}
)

// overlayOfKind resolves the overlay (owner-checked when owned) and writes the
// 404/400 itself when it is missing or of another kind.
func (h *GoalListHandler) overlayOfKind(c *gin.Context, kind string, owned bool) *models.Overlay {
	var overlay *models.Overlay
	var err error
	if owned {
		overlay, err = h.overlays.GetByIDAndUserID(c.Request.Context(), c.Param("id"), c.GetString("user_id"))
	} else {
		overlay, err = h.overlays.GetByID(c.Request.Context(), c.Param("id"))
	}
	if err != nil || overlay == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "overlay not found"})
		return nil
	}
	if overlay.Kind() != kind {
		if !owned {
			// The public routes do not reveal what other kinds exist.
			c.JSON(http.StatusNotFound, gin.H{"error": "overlay not found"})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("this route belongs to %s overlays, but this overlay is of type '%s'", kind, overlay.Kind())})
		}
		return nil
	}
	return overlay
}

// goalState is the goal as the editor and the renderer read it.
func (h *GoalListHandler) goalState(ctx context.Context, overlayID string) (gin.H, alerts.GoalConfig, error) {
	stored, err := h.store.GetGoalConfig(ctx, overlayID)
	if err != nil {
		return nil, alerts.GoalConfig{}, err
	}
	cfg := defaultGoal
	if stored != nil {
		cfg = *stored
	}
	current := float64(cfg.StartingValue)
	value, err := h.rdb.Get(ctx, alerts.GoalKey(overlayID, cfg.Metric)).Float64()
	switch {
	case err == nil && alerts.IsPolledGoalMetric(cfg.Metric):
		current = value // the platform's authoritative count
	case err == nil:
		current += value // counted events on top of the starting value
	case !errors.Is(err, redis.Nil):
		return nil, cfg, err
	}
	status, err := h.rdb.Get(ctx, alerts.GoalStatusKey(overlayID)).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, cfg, err
	}
	return gin.H{"goal": cfg, "configured": stored != nil, "current": current, "status": status}, cfg, nil
}

func (h *GoalListHandler) writeGoal(c *gin.Context, overlay *models.Overlay) {
	state, _, err := h.goalState(c.Request.Context(), overlay.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load goal"})
		return
	}
	c.JSON(http.StatusOK, state)
}

// HandleGetGoal handles GET /overlays/:id/goal (owner).
func (h *GoalListHandler) HandleGetGoal(c *gin.Context) {
	if overlay := h.overlayOfKind(c, models.OverlayTypeGoal, true); overlay != nil {
		h.writeGoal(c, overlay)
	}
}

// HandleGetPublicGoal handles GET /overlays/public/:id/goal (OBS renderer).
func (h *GoalListHandler) HandleGetPublicGoal(c *gin.Context) {
	if overlay := h.overlayOfKind(c, models.OverlayTypeGoal, false); overlay != nil {
		h.writeGoal(c, overlay)
	}
}

// HandlePutGoal handles PUT /overlays/:id/goal (owner). reset_progress
// discards counted events so a new goal starts from starting_value. The saved
// goal is pushed to connected overlays at once.
func (h *GoalListHandler) HandlePutGoal(c *gin.Context) {
	overlay := h.overlayOfKind(c, models.OverlayTypeGoal, true)
	if overlay == nil {
		return
	}
	var req struct {
		alerts.GoalConfig
		ResetProgress bool `json:"reset_progress"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := req.GoalConfig.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ctx := c.Request.Context()
	if err := h.store.UpsertGoalConfig(ctx, overlay.ID, req.GoalConfig); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save goal"})
		return
	}
	if req.ResetProgress && !alerts.IsPolledGoalMetric(req.Metric) {
		if err := h.rdb.Del(ctx, alerts.GoalKey(overlay.ID, req.Metric)).Err(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to reset goal progress"})
			return
		}
	}
	state, cfg, err := h.goalState(ctx, overlay.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load goal"})
		return
	}
	// Best-effort: the renderer also re-reads the goal when it connects.
	h.publish(ctx, overlay.ID, alerts.GoalFrame{
		Type: "goal", OverlayID: overlay.ID, Goal: cfg.Metric,
		Current: state["current"].(float64), Target: cfg.Target, Title: cfg.Title,
	})
	c.JSON(http.StatusOK, state)
}

func (h *GoalListHandler) publish(ctx context.Context, overlayID string, frame any) {
	if payload, err := json.Marshal(frame); err == nil {
		_ = h.rdb.Publish(ctx, "overlay:"+overlayID+":alerts", payload).Err()
	}
}

func (h *GoalListHandler) listConfig(ctx context.Context, overlayID string) (alerts.ListConfig, bool, error) {
	stored, err := h.store.GetListConfig(ctx, overlayID)
	if err != nil || stored == nil {
		return defaultList, false, err
	}
	return *stored, true, nil
}

// HandleGetList handles GET /overlays/:id/list (owner).
func (h *GoalListHandler) HandleGetList(c *gin.Context) {
	overlay := h.overlayOfKind(c, models.OverlayTypeList, true)
	if overlay == nil {
		return
	}
	cfg, configured, err := h.listConfig(c.Request.Context(), overlay.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load list"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"list": cfg, "configured": configured})
}

// HandlePutList handles PUT /overlays/:id/list (owner) and pushes the newly
// selected board to connected overlays.
func (h *GoalListHandler) HandlePutList(c *gin.Context) {
	overlay := h.overlayOfKind(c, models.OverlayTypeList, true)
	if overlay == nil {
		return
	}
	var cfg alerts.ListConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := cfg.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ctx := c.Request.Context()
	if err := h.store.UpsertListConfig(ctx, overlay.ID, cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save list"})
		return
	}
	if entries, err := alerts.TopEntries(ctx, h.rdb, overlay.ID, cfg.Category, alerts.MaxListLimit); err == nil {
		h.publish(ctx, overlay.ID, alerts.ListUpdateFrame{
			Type: "list_update", OverlayID: overlay.ID, Category: cfg.Category, Entries: entries, Limit: cfg.Limit, Title: cfg.Title,
		})
	}
	c.JSON(http.StatusOK, gin.H{"list": cfg, "configured": true})
}

// HandleGetListTop handles GET /overlays/:id/list/top?category=&limit=
// (public: the OBS renderer's initial board). Defaults come from the list
// config; limit is clamped to 1..MaxListLimit.
func (h *GoalListHandler) HandleGetListTop(c *gin.Context) {
	overlay := h.overlayOfKind(c, models.OverlayTypeList, false)
	if overlay == nil {
		return
	}
	ctx := c.Request.Context()
	cfg, _, err := h.listConfig(ctx, overlay.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load list"})
		return
	}
	if category := c.Query("category"); category != "" {
		if !alerts.IsListCategory(category) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unknown category"})
			return
		}
		cfg.Category = category
	}
	if n, err := strconv.Atoi(c.Query("limit")); err == nil {
		cfg.Limit = min(max(n, 1), alerts.MaxListLimit)
	}
	entries, err := alerts.TopEntries(ctx, h.rdb, overlay.ID, cfg.Category, cfg.Limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read leaderboard"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"title": cfg.Title, "category": cfg.Category, "limit": cfg.Limit, "entries": entries})
}
