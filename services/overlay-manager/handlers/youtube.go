package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/caesar/all-chat/services/overlay-manager/youtube"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// YouTubeHandler handles YouTube-related endpoints
type YouTubeHandler struct {
	resolver *youtube.Resolver
	logger   *zap.Logger
}

// NewYouTubeHandler creates a new YouTube handler
func NewYouTubeHandler(resolver *youtube.Resolver, logger *zap.Logger) *YouTubeHandler {
	return &YouTubeHandler{
		resolver: resolver,
		logger:   logger,
	}
}

// ResolveChannelRequest is the request body for channel resolution
type ResolveChannelRequest struct {
	Input string `json:"input" binding:"required"` // URL, handle, or channel ID
}

// ResolveChannelResponse is the response for channel resolution
type ResolveChannelResponse struct {
	ChannelID   string `json:"channel_id"`
	Title       string `json:"title,omitempty"`
	CustomURL   string `json:"custom_url,omitempty"`
	Thumbnail   string `json:"thumbnail,omitempty"`
	InputType   string `json:"input_type"` // "channel_id", "video_url", "handle", "channel_url"
}

// ResolveChannel resolves a YouTube URL/handle to a channel ID
// POST /api/v1/youtube/resolve
func (h *YouTubeHandler) ResolveChannel(c *gin.Context) {
	var req ResolveChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "input is required"})
		return
	}

	// Resolve to channel ID
	channelID, err := h.resolver.ResolveToChannelID(c.Request.Context(), req.Input)
	if err != nil {
		// Check if quota exhausted
		if errors.Is(err, youtube.ErrQuotaExhausted) {
			h.logger.Warn("YouTube API quota exhausted",
				zap.String("input", req.Input),
				zap.String("user_ip", c.ClientIP()),
			)
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "YouTube API quota exhausted",
				"message": "Daily YouTube API quota limit reached. Please try again after midnight PST.",
				"retry_after": "3600",
			})
			return
		}

		// Other errors
		h.logger.Warn("Failed to resolve YouTube input",
			zap.String("input", req.Input),
			zap.Error(err),
		)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Unable to resolve YouTube input: %v", err),
		})
		return
	}

	// Determine input type for user feedback
	inputType := detectInputType(req.Input, channelID)

	// Get channel info (optional, for better UX)
	channelInfo, err := h.resolver.GetChannelInfo(c.Request.Context(), channelID)
	if err != nil {
		// If we can't get info, just return the channel ID
		h.logger.Debug("Failed to get channel info",
			zap.String("channel_id", channelID),
			zap.Error(err),
		)
		c.JSON(http.StatusOK, ResolveChannelResponse{
			ChannelID: channelID,
			InputType: inputType,
		})
		return
	}

	c.JSON(http.StatusOK, ResolveChannelResponse{
		ChannelID: channelID,
		Title:     channelInfo.Title,
		CustomURL: channelInfo.CustomURL,
		Thumbnail: channelInfo.Thumbnail,
		InputType: inputType,
	})
}

// detectInputType determines what type of input was provided
func detectInputType(input, resolvedChannelID string) string {
	if input == resolvedChannelID {
		return "channel_id"
	}
	if strings.Contains(input, "youtube.com/watch") || strings.Contains(input, "youtu.be/") {
		return "video_url"
	}
	if strings.Contains(input, "youtube.com/channel/") {
		return "channel_url"
	}
	if strings.HasPrefix(input, "@") || strings.Contains(input, "youtube.com/@") {
		return "handle"
	}
	return "unknown"
}
