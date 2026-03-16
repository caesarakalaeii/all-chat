package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/caesar/all-chat/services/auth-service/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// getOrCreateViewerWithIdentity looks up or creates a durable viewer UUID
// for the platform user associated with the given session.
func (h *ViewerAuthHandler) getOrCreateViewerWithIdentity(c *gin.Context, session *models.ViewerSession) (uuid.UUID, error) {
	return h.identityRepo.GetOrCreateViewerByPlatform(c.Request.Context(), session.Platform, session.PlatformUserID)
}

// HandleTwitchExchange handles POST /viewer/twitch/exchange.
// Body: {"code": "...", "state": "..."}
// Response: {"token": "...", "expires_in": N, "viewer_info": {...}}
func (h *ViewerAuthHandler) HandleTwitchExchange(c *gin.Context) {
	var req struct {
		Code  string `json:"code"  binding:"required"`
		State string `json:"state" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing code or state"})
		return
	}

	// Verify state (same Redis key format as HandleTwitchCallback)
	stateKey := fmt.Sprintf("viewer_oauth_state:twitch:%s", req.State)
	stateJSON, err := h.redis.Get(c.Request.Context(), stateKey).Result()
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired state"})
		return
	}
	h.redis.Del(c.Request.Context(), stateKey)

	var stateData map[string]string
	if err := json.Unmarshal([]byte(stateJSON), &stateData); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid state data"})
		return
	}

	// Exchange code for token
	token, err := h.twitchProvider.ExchangeCode(c.Request.Context(), req.Code)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to exchange code"})
		return
	}

	// Get user info
	twitchUser, err := h.twitchProvider.GetUserInfoTwitch(c.Request.Context(), token.AccessToken)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user info"})
		return
	}

	// Encrypt tokens
	encryptedAccessToken, _ := h.cipher.Encrypt(token.AccessToken)
	var encryptedRefreshToken *string
	if token.RefreshToken != "" {
		encrypted, _ := h.cipher.Encrypt(token.RefreshToken)
		encryptedRefreshToken = &encrypted
	}

	// Get or create viewer session
	session, err := h.getOrCreateViewerSession(c.Request.Context(), "twitch", twitchUser, encryptedAccessToken, encryptedRefreshToken, token.Expiry)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create session"})
		return
	}

	// Get or create durable viewer identity
	viewerID, err := h.identityRepo.GetOrCreateViewerByPlatform(c.Request.Context(), session.Platform, session.PlatformUserID)
	if err != nil {
		h.logger.Error("Failed to get/create viewer identity", zap.Error(err))
		// Non-fatal: continue with zero UUID
		viewerID = uuid.Nil
	}

	linkedStreamer := h.findLinkedStreamer(c.Request.Context(), session.Platform, session.PlatformUserID)

	jwtToken, err := h.generateViewerJWT(session, viewerID, linkedStreamer)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, models.ViewerAuthResponse{
		Token:     jwtToken,
		ExpiresIn: int(h.jwtExpiry.Seconds()),
		ViewerInfo: models.ViewerInfo{
			ID:          session.ID,
			Platform:    session.Platform,
			Username:    session.Username,
			DisplayName: session.DisplayName,
			AvatarURL:   session.AvatarURL,
		},
	})
}

// HandleYouTubeExchange handles POST /viewer/youtube/exchange.
// Body: {"code": "...", "state": "..."}
// Response: {"token": "...", "expires_in": N, "viewer_info": {...}}
func (h *ViewerAuthHandler) HandleYouTubeExchange(c *gin.Context) {
	var req struct {
		Code  string `json:"code"  binding:"required"`
		State string `json:"state" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing code or state"})
		return
	}

	// Verify state (same Redis key format as HandleYouTubeCallback: oauth_state:{state})
	stateKey := fmt.Sprintf("oauth_state:%s", req.State)
	stateJSON, err := h.redis.Get(c.Request.Context(), stateKey).Result()
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired state"})
		return
	}
	h.redis.Del(c.Request.Context(), stateKey)

	var stateData map[string]string
	if err := json.Unmarshal([]byte(stateJSON), &stateData); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid state data"})
		return
	}

	// Exchange code for token
	token, err := h.youtubeProvider.ExchangeCode(c.Request.Context(), req.Code)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to exchange code"})
		return
	}

	// Get user info
	userInfo, err := h.youtubeProvider.GetUserInfo(c.Request.Context(), token.AccessToken)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user info"})
		return
	}

	// Encrypt tokens
	encryptedAccess, _ := h.cipher.Encrypt(token.AccessToken)
	var encryptedRefresh *string
	if token.RefreshToken != "" {
		encrypted, _ := h.cipher.Encrypt(token.RefreshToken)
		encryptedRefresh = &encrypted
	}

	// Get or create viewer session inline (YouTube uses different struct than getOrCreateViewerSession)
	session, err := h.viewerRepo.GetByPlatformUserID(c.Request.Context(), "youtube", userInfo.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get session"})
		return
	}

	if session == nil {
		session = &models.ViewerSession{
			Platform:       "youtube",
			PlatformUserID: userInfo.ID,
			Username:       userInfo.Name,
			DisplayName:    userInfo.Name,
			AccessToken:    encryptedAccess,
			RefreshToken:   encryptedRefresh,
			TokenExpiresAt: token.Expiry,
		}
		if userInfo.Picture != "" {
			session.AvatarURL = &userInfo.Picture
		}
		if err := h.viewerRepo.Create(c.Request.Context(), session); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create session"})
			return
		}
	} else {
		session.Username = userInfo.Name
		session.DisplayName = userInfo.Name
		session.AccessToken = encryptedAccess
		session.RefreshToken = encryptedRefresh
		session.TokenExpiresAt = token.Expiry
		if userInfo.Picture != "" {
			session.AvatarURL = &userInfo.Picture
		}
		if err := h.viewerRepo.Update(c.Request.Context(), session); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update session"})
			return
		}
	}

	// Get or create durable viewer identity
	viewerID, err := h.identityRepo.GetOrCreateViewerByPlatform(c.Request.Context(), session.Platform, session.PlatformUserID)
	if err != nil {
		h.logger.Error("Failed to get/create viewer identity", zap.Error(err))
		viewerID = uuid.Nil
	}

	linkedStreamer := h.findLinkedStreamer(c.Request.Context(), session.Platform, session.PlatformUserID)

	jwtToken, err := h.generateViewerJWT(session, viewerID, linkedStreamer)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, models.ViewerAuthResponse{
		Token:     jwtToken,
		ExpiresIn: int(h.jwtExpiry.Seconds()),
		ViewerInfo: models.ViewerInfo{
			ID:          session.ID,
			Platform:    session.Platform,
			Username:    session.Username,
			DisplayName: session.DisplayName,
			AvatarURL:   session.AvatarURL,
		},
	})
}

// HandleKickExchange handles POST /viewer/kick/exchange.
// Body: {"code": "...", "state": "..."}
// Response: {"token": "...", "expires_in": N, "viewer_info": {...}}
// Note: code_verifier is retrieved from stored state data in Redis, not from request body.
func (h *ViewerAuthHandler) HandleKickExchange(c *gin.Context) {
	var req struct {
		Code  string `json:"code"  binding:"required"`
		State string `json:"state" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing code or state"})
		return
	}

	// Verify state (same Redis key format as HandleKickCallback: oauth_state:{state})
	stateKey := fmt.Sprintf("oauth_state:%s", req.State)
	stateJSON, err := h.redis.Get(c.Request.Context(), stateKey).Result()
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired state"})
		return
	}
	h.redis.Del(c.Request.Context(), stateKey)

	var stateData map[string]string
	if err := json.Unmarshal([]byte(stateJSON), &stateData); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid state data"})
		return
	}

	// Retrieve PKCE code_verifier from state (stored server-side by HandleKickLogin)
	codeVerifier, ok := stateData["code_verifier"]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing code verifier"})
		return
	}

	// Exchange code with PKCE
	token, err := h.kickProvider.ExchangeCodeWithPKCE(c.Request.Context(), req.Code, codeVerifier)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to exchange code"})
		return
	}

	// Get user info
	userInfo, err := h.kickProvider.GetUserInfoKick(c.Request.Context(), token.AccessToken)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user info"})
		return
	}

	platformUserID := fmt.Sprintf("%d", userInfo.UserID)
	session, err := h.viewerRepo.GetByPlatformUserID(c.Request.Context(), "kick", platformUserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get session"})
		return
	}

	// Encrypt tokens
	encryptedAccess, _ := h.cipher.Encrypt(token.AccessToken)
	var encryptedRefresh *string
	if token.RefreshToken != "" {
		encrypted, _ := h.cipher.Encrypt(token.RefreshToken)
		encryptedRefresh = &encrypted
	}

	if session == nil {
		session = &models.ViewerSession{
			Platform:       "kick",
			PlatformUserID: platformUserID,
			Username:       userInfo.Name,
			DisplayName:    userInfo.Name,
			AccessToken:    encryptedAccess,
			RefreshToken:   encryptedRefresh,
			TokenExpiresAt: token.Expiry,
		}
		if userInfo.ProfilePicture != "" {
			session.AvatarURL = &userInfo.ProfilePicture
		}
		if err := h.viewerRepo.Create(c.Request.Context(), session); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create session"})
			return
		}
	} else {
		session.Username = userInfo.Name
		session.DisplayName = userInfo.Name
		session.AccessToken = encryptedAccess
		session.RefreshToken = encryptedRefresh
		session.TokenExpiresAt = token.Expiry
		if userInfo.ProfilePicture != "" {
			session.AvatarURL = &userInfo.ProfilePicture
		}
		if err := h.viewerRepo.Update(c.Request.Context(), session); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update session"})
			return
		}
	}

	// Get or create durable viewer identity
	viewerID, err := h.identityRepo.GetOrCreateViewerByPlatform(c.Request.Context(), session.Platform, session.PlatformUserID)
	if err != nil {
		h.logger.Error("Failed to get/create viewer identity", zap.Error(err))
		viewerID = uuid.Nil
	}

	linkedStreamer := h.findLinkedStreamer(c.Request.Context(), session.Platform, session.PlatformUserID)

	jwtToken, err := h.generateViewerJWT(session, viewerID, linkedStreamer)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, models.ViewerAuthResponse{
		Token:     jwtToken,
		ExpiresIn: int(h.jwtExpiry.Seconds()),
		ViewerInfo: models.ViewerInfo{
			ID:          session.ID,
			Platform:    session.Platform,
			Username:    session.Username,
			DisplayName: session.DisplayName,
			AvatarURL:   session.AvatarURL,
		},
	})
}

