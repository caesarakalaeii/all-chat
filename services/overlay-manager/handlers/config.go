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
	"net/http"

	"github.com/caesar/all-chat/services/overlay-manager/clients"
	"github.com/caesar/all-chat/services/overlay-manager/models"
	"github.com/gin-gonic/gin"
)

// OverlayConfigRepository defines persistence operations needed by the handler.
// Having an interface keeps handlers easy to test.
type OverlayConfigRepository interface {
	GetByOverlayID(ctx context.Context, overlayID string) (*models.OverlayConfig, error)
	Update(ctx context.Context, config *models.OverlayConfig) error
}

// SevenTVResolver normalizes user-supplied 7TV input to a canonical emote-set ID.
type SevenTVResolver interface {
	Resolve(ctx context.Context, input string) (clients.ResolvedSet, error)
}

// BubbleColorsGate answers whether differently-coloured chat bubbles are open for
// a user (ADR-0008, gate key `bubble_colors`). Mirrors moderation-service's
// moderationGate: when the gate row is not premium everyone is in, otherwise only
// premium users are.
//
// It cannot be shared/middleware.RequirePremium: that aborts the whole request,
// and this route saves the entire overlay config (theme, filters, fonts, …). Only
// a handful of visual_settings keys are gated, so the check has to be per-field.
type BubbleColorsGate interface {
	BubbleColorsEnabled(ctx context.Context, userID string) (bool, error)
}

// bubbleColorSettings are the visual_settings keys the `bubble_colors` gate owns.
// Names match the frontend's VisualSettings fields (lib/types/visual-settings.ts);
// the emitted CSS lives in lib/utils/visual-settings-to-css.ts.
var bubbleColorSettings = []string{
	"bubblePalette",
	"twitchBubbleBg",
	"youtubeBubbleBg",
	"kickBubbleBg",
	"tiktokBubbleBg",
	"discordBubbleBg",
}

// ConfigHandler manages overlay configuration routes.
type ConfigHandler struct {
	repo            OverlayConfigRepository
	overlays        OverlayRepository
	sources         SourceRepository
	seventvResolver SevenTVResolver
	// bubbleColors may be nil, which means open — the same fail-open default
	// moderation-service's OpenGate provides, so a service wired without a gate
	// cache (or a test) behaves as the gate's seeded state does.
	bubbleColors BubbleColorsGate
}

// NewConfigHandler returns a ConfigHandler.
func NewConfigHandler(repo OverlayConfigRepository, overlays OverlayRepository, sources SourceRepository, seventvResolver SevenTVResolver, bubbleColors BubbleColorsGate) *ConfigHandler {
	return &ConfigHandler{repo: repo, overlays: overlays, sources: sources, seventvResolver: seventvResolver, bubbleColors: bubbleColors}
}

// bubbleColorsLocked reports whether the requester may NOT configure bubble
// colours. A gate-lookup error locks the controls: showing an editable control
// whose value the save path will drop is the failure mode this whole feature was
// built to avoid.
func (h *ConfigHandler) bubbleColorsLocked(ctx context.Context, userID string) bool {
	if h.bubbleColors == nil {
		return false
	}
	enabled, err := h.bubbleColors.BubbleColorsEnabled(ctx, userID)
	if err != nil {
		return true
	}
	return !enabled
}

// HandleGetConfig returns the configuration for an overlay owned by the requester.
func (h *ConfigHandler) HandleGetConfig(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	overlayID := c.Param("id")
	if overlayID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "overlay id is required"})
		return
	}

	if _, err := h.overlays.GetByIDAndUserID(c.Request.Context(), overlayID, userID.(string)); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "overlay not found"})
		return
	}

	config, err := h.repo.GetByOverlayID(c.Request.Context(), overlayID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "overlay config not found"})
		return
	}

	// Embedded pointer flattens into the same JSON object, so the response shape
	// is unchanged apart from one added field. Resolved per request and never
	// persisted; the unauthenticated HandleGetPublicConfig does not carry it.
	c.JSON(http.StatusOK, struct {
		*models.OverlayConfig
		BubbleColorsLocked bool `json:"bubble_colors_locked"`
	}{
		OverlayConfig:      config,
		BubbleColorsLocked: h.bubbleColorsLocked(c.Request.Context(), userID.(string)),
	})
}

// HandleUpdateConfig updates the overlay configuration for the owner.
func (h *ConfigHandler) HandleUpdateConfig(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	overlayID := c.Param("id")
	if overlayID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "overlay id is required"})
		return
	}

	if _, err := h.overlays.GetByIDAndUserID(c.Request.Context(), overlayID, userID.(string)); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "overlay not found"})
		return
	}

	config, err := h.repo.GetByOverlayID(c.Request.Context(), overlayID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "overlay config not found"})
		return
	}

	var req struct {
		DisplaySettings   map[string]any `json:"display_settings"`
		FilterSettings    map[string]any `json:"filter_settings"`
		Enable7TV         *bool          `json:"enable_7tv"`
		EnableBTTV        *bool          `json:"enable_bttv"`
		EnableFFZ         *bool          `json:"enable_ffz"`
		CustomCSS         *string        `json:"custom_css"`
		VisualSettings    map[string]any `json:"visual_settings"`
		SevenTVEmoteSetID *string        `json:"seventv_emote_set_id"`
		ThemeID           *string        `json:"theme_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.DisplaySettings != nil {
		config.DisplaySettings = req.DisplaySettings
	}
	if req.FilterSettings != nil {
		config.FilterSettings = req.FilterSettings
	}
	if req.Enable7TV != nil {
		config.Enable7TV = *req.Enable7TV
	}
	if req.EnableBTTV != nil {
		config.EnableBTTV = *req.EnableBTTV
	}
	if req.EnableFFZ != nil {
		config.EnableFFZ = *req.EnableFFZ
	}
	if req.CustomCSS != nil {
		config.CustomCSS = *req.CustomCSS
	}
	if req.VisualSettings != nil {
		// Gated keys are carried over from the stored config rather than rejected:
		// this route saves everything, so failing the request would block unrelated
		// edits, and overwriting with the incoming values would let a locked user
		// set them. Carrying over also means an unrelated save never silently wipes
		// colours configured while the gate was open.
		if h.bubbleColorsLocked(c.Request.Context(), userID.(string)) {
			for _, key := range bubbleColorSettings {
				delete(req.VisualSettings, key)
				if stored, ok := config.VisualSettings[key]; ok {
					req.VisualSettings[key] = stored
				}
			}
		}
		config.VisualSettings = req.VisualSettings
	}
	if req.ThemeID != nil {
		config.ThemeID = *req.ThemeID
	}
	if req.SevenTVEmoteSetID != nil {
		if h.seventvResolver == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "seventv resolver not configured"})
			return
		}
		resolved, err := h.seventvResolver.Resolve(c.Request.Context(), *req.SevenTVEmoteSetID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid 7TV reference: " + err.Error()})
			return
		}
		config.SevenTVEmoteSetID = resolved.EmoteSetID
	}

	if err := h.repo.Update(c.Request.Context(), config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update overlay config"})
		return
	}

	c.JSON(http.StatusOK, config)
}

// HandleResolveSevenTV validates a 7TV input string and returns the canonical
// emote-set descriptor without persisting anything. Lets the frontend show
// "Resolved: <set name> (N emotes)" feedback before the user saves.
func (h *ConfigHandler) HandleResolveSevenTV(c *gin.Context) {
	if h.seventvResolver == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "seventv resolver not configured"})
		return
	}

	var req struct {
		Input string `json:"input"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resolved, err := h.seventvResolver.Resolve(c.Request.Context(), req.Input)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resolved)
}

// HandleGetPublicConfig exposes safe configuration fields without authentication.
func (h *ConfigHandler) HandleGetPublicConfig(c *gin.Context) {
	overlayID := c.Param("id")
	if overlayID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "overlay id is required"})
		return
	}

	if _, err := h.overlays.GetByID(c.Request.Context(), overlayID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "overlay not found"})
		return
	}

	config, err := h.repo.GetByOverlayID(c.Request.Context(), overlayID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "overlay config not found"})
		return
	}

	// Get sources with their active status
	sources, err := h.sources.ListByOverlayID(c.Request.Context(), overlayID)
	if err != nil {
		// Log error but don't fail the request
		sources = []*models.ChatSource{}
	}

	// Build simplified source status for public consumption
	sourceStatus := make([]map[string]interface{}, 0, len(sources))
	for _, source := range sources {
		sourceStatus = append(sourceStatus, map[string]interface{}{
			"platform":     source.Platform,
			"channel_id":   source.ChannelID,
			"channel_name": source.ChannelName,
			"is_active":    source.IsActive,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"display_settings":     config.DisplaySettings,
		"filter_settings":      config.FilterSettings,
		"custom_css":           config.CustomCSS,
		"visual_settings":      config.VisualSettings,
		"seventv_emote_set_id": config.SevenTVEmoteSetID,
		"theme_id":             config.ThemeID,
		"sources":              sourceStatus,
	})
}
