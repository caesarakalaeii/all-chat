package handlers

import (
	"net/http"
	"strings"

	"github.com/caesar/all-chat/services/overlay-manager/clients"
	"github.com/caesar/all-chat/services/overlay-manager/models"
	"github.com/gin-gonic/gin"
)

type MockMessageHandler struct {
	overlayRepo OverlayRepository
	sourceRepo  SourceRepository
	mpClient    *clients.MessageProcessorClient
}

func NewMockMessageHandler(overlayRepo OverlayRepository, sourceRepo SourceRepository, mpClient *clients.MessageProcessorClient) *MockMessageHandler {
	return &MockMessageHandler{
		overlayRepo: overlayRepo,
		sourceRepo:  sourceRepo,
		mpClient:    mpClient,
	}
}

type mockMessageRequest struct {
	Platform    string                 `json:"platform"`
	ChannelID   string                 `json:"channel_id"`
	ChannelName string                 `json:"channel_name"`
	Text        string                 `json:"text" binding:"required"`
	Username    string                 `json:"username"`
	DisplayName string                 `json:"display_name"`
	AvatarURL   string                 `json:"avatar_url"`
	Color       string                 `json:"color"`
	Badges      []clients.MockBadge    `json:"badges"`
	Event       *clients.MockEventInfo `json:"event"`
	Metadata    map[string]interface{} `json:"metadata"`
}

func (h *MockMessageHandler) HandleSendMockMessage(c *gin.Context) {
	if h.mpClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "mock messaging unavailable"})
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	overlayID := c.Param("id")

	overlay, err := h.overlayRepo.GetByIDAndUserID(c.Request.Context(), overlayID, userID.(string))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "overlay not found"})
		return
	}

	var req mockMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	targetPlatform, channelID, channelName := h.resolveSource(c, overlayID, req.Platform, req.ChannelID, req.ChannelName)

	if req.Metadata == nil {
		req.Metadata = map[string]interface{}{}
	}

	username := strings.TrimSpace(req.Username)
	if username == "" {
		username = "mockuser"
	}

	displayName := strings.TrimSpace(req.DisplayName)
	if displayName == "" {
		displayName = username
	}

	payload := &clients.MockMessagePayload{
		OverlayID:   overlay.ID,
		Platform:    targetPlatform,
		ChannelID:   channelID,
		ChannelName: channelName,
		Username:    username,
		DisplayName: displayName,
		AvatarURL:   req.AvatarURL,
		Color:       req.Color,
		Badges:      req.Badges,
		Event:       req.Event,
		Text:        req.Text,
		Metadata:    req.Metadata,
	}

	if payload.Metadata == nil {
		payload.Metadata = map[string]interface{}{}
	}
	payload.Metadata["mock_source"] = "overlay-manager"

	if err := h.mpClient.SendMockMessage(c.Request.Context(), payload); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to dispatch mock message"})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{"status": "queued"})
}

func (h *MockMessageHandler) resolveSource(c *gin.Context, overlayID, requestedPlatform, requestedChannelID, requestedChannelName string) (string, string, string) {
	platform := strings.ToLower(strings.TrimSpace(requestedPlatform))
	channelID := strings.TrimSpace(requestedChannelID)
	channelName := strings.TrimSpace(requestedChannelName)

	sources, err := h.sourceRepo.ListByOverlayID(c.Request.Context(), overlayID)
	if err != nil {
		return fallbackTarget(platform, channelID, channelName)
	}

	var preferred *models.ChatSource

	// Try to find an explicit match first
	for _, source := range sources {
		if channelID != "" && source.ChannelID == channelID {
			preferred = source
			break
		}
		if platform != "" && strings.EqualFold(source.Platform, platform) {
			preferred = source
			break
		}
	}

	if preferred == nil && len(sources) > 0 {
		preferred = sources[0]
	}

	if preferred != nil {
		if platform == "" {
			platform = preferred.Platform
		}
		if channelID == "" {
			channelID = preferred.ChannelID
		}
		if channelName == "" {
			channelName = preferred.ChannelName
		}
	}

	return fallbackTarget(platform, channelID, channelName)
}

func fallbackTarget(platform, channelID, channelName string) (string, string, string) {
	if platform == "" {
		platform = "twitch"
	}
	if channelID == "" {
		// For Twitch, use "global" to enable Twitch global emotes (PogChamp, Kappa, etc.)
		// For other platforms, use a generic mock channel
		if platform == "twitch" {
			channelID = "global"
		} else {
			channelID = "mock-channel"
		}
	}
	if channelName == "" {
		channelName = channelID
	}
	return platform, channelID, channelName
}
