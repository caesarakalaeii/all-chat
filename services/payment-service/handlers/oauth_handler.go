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

// Package handlers contains the HTTP handlers for payment-service.
package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/caesar/all-chat/services/payment-service/entitlement"
	"github.com/caesar/all-chat/services/payment-service/patreon"
	"github.com/caesar/all-chat/services/payment-service/repository"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// stateTTL is how long a started Patreon connect flow remains valid.
const stateTTL = 30 * time.Minute

// Subject kinds bound to a connect flow.
const (
	subjectUser   = "user"
	subjectViewer = "viewer"
)

// OAuthHandler serves the "Connect Patreon" flow for both streamer users and
// viewers (ADR-0018 / ADR-0019). The subject is bound server-side to the one-time
// state, so a single callback completes either flow.
type OAuthHandler struct {
	oauth       *patreon.OAuth
	redis       *redis.Client
	tokenRepo   *repository.TokenRepository
	entitlement *entitlement.Service
	campaignID  string
	frontendURL string
	logger      *zap.Logger
}

// NewOAuthHandler builds an OAuthHandler.
func NewOAuthHandler(oauth *patreon.OAuth, rdb *redis.Client, tokenRepo *repository.TokenRepository, ent *entitlement.Service, campaignID, frontendURL string, logger *zap.Logger) *OAuthHandler {
	return &OAuthHandler{oauth: oauth, redis: rdb, tokenRepo: tokenRepo, entitlement: ent, campaignID: campaignID, frontendURL: frontendURL, logger: logger}
}

func stateKey(csrf string) string { return "oauth_state:patreon:" + csrf }

// connectState is the subject bound to a started Patreon connect flow. The callback
// reads it from Redis (authoritative), not from the URL, so the subject can't be
// tampered with.
type connectState struct {
	Kind string `json:"kind"` // subjectUser | subjectViewer
	ID   string `json:"id"`
}

// Connect starts the Patreon OAuth flow for the authenticated streamer user.
// Requires a user JWT. Returns the consent URL as JSON.
func (h *OAuthHandler) Connect(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	h.startConnect(c, connectState{Kind: subjectUser, ID: userID})
}

// ConnectViewer starts the Patreon OAuth flow for the authenticated viewer.
// Requires a viewer JWT. Returns the consent URL as JSON.
func (h *OAuthHandler) ConnectViewer(c *gin.Context) {
	viewerID := c.GetString("viewer_id")
	if viewerID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	h.startConnect(c, connectState{Kind: subjectViewer, ID: viewerID})
}

func (h *OAuthHandler) startConnect(c *gin.Context, st connectState) {
	csrf, err := randomToken()
	if err != nil {
		h.logger.Error("Failed to generate OAuth state", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to start connect flow"})
		return
	}

	payload, err := json.Marshal(st)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to start connect flow"})
		return
	}
	if err := h.redis.Set(c.Request.Context(), stateKey(csrf), payload, stateTTL).Err(); err != nil {
		h.logger.Error("Failed to persist OAuth state", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to start connect flow"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"auth_url": h.oauth.GetAuthURL(csrf)})
}

// Callback completes the Patreon OAuth flow for either subject. Public route: the
// browser arrives here from Patreon with no JWT, so the trust anchor is the
// Redis-stored state (consumed one-time with GETDEL). Redirects to the subject's
// frontend premium page.
func (h *OAuthHandler) Callback(c *gin.Context) {
	ctx := c.Request.Context()
	code := c.Query("code")
	csrf := c.Query("state")
	if code == "" || csrf == "" {
		h.redirect(c, subjectUser, "error")
		return
	}

	raw, err := h.redis.GetDel(ctx, stateKey(csrf)).Result()
	if err != nil || raw == "" {
		h.logger.Warn("Patreon callback with invalid/expired state", zap.Error(err))
		h.redirect(c, subjectUser, "error")
		return
	}
	var st connectState
	if err := json.Unmarshal([]byte(raw), &st); err != nil || st.ID == "" {
		h.logger.Warn("Patreon callback with malformed state", zap.Error(err))
		h.redirect(c, subjectUser, "error")
		return
	}

	token, err := h.oauth.ExchangeCode(ctx, code)
	if err != nil {
		h.logger.Error("Patreon code exchange failed", zap.String("subject", st.Kind), zap.Error(err))
		h.redirect(c, st.Kind, "error")
		return
	}

	snap, err := h.oauth.GetIdentityWithMembership(ctx, token.AccessToken, h.campaignID)
	if err != nil {
		h.logger.Error("Patreon identity fetch failed", zap.String("subject", st.Kind), zap.Error(err))
		h.redirect(c, st.Kind, "error")
		return
	}

	tok := repository.PatreonToken{
		PatreonUserID: snap.PatreonUserID,
		AccessToken:   token.AccessToken,
		RefreshToken:  token.RefreshToken,
		ExpiresAt:     token.Expiry,
		Scopes:        h.oauth.Scopes(),
	}
	var applyUserID, applyViewerID *string
	if st.Kind == subjectViewer {
		tok.ViewerID = &st.ID
		applyViewerID = &st.ID
	} else {
		tok.UserID = &st.ID
		applyUserID = &st.ID
	}

	if err := h.tokenRepo.Upsert(ctx, tok); err != nil {
		// A unique violation here means this Patreon account is already linked to a
		// different all-chat identity (one account ↔ one identity, ADR-0019).
		h.logger.Error("Failed to store Patreon token", zap.String("subject", st.Kind), zap.Error(err))
		h.redirect(c, st.Kind, "error")
		return
	}

	if _, _, err := h.entitlement.Apply(ctx, snap, applyUserID, applyViewerID, nil); err != nil {
		h.logger.Error("Failed to apply Patreon membership", zap.String("subject", st.Kind), zap.Error(err))
		h.redirect(c, st.Kind, "error")
		return
	}

	h.redirect(c, st.Kind, "connected")
}

// redirect sends the browser to the subject's premium page with a result code.
func (h *OAuthHandler) redirect(c *gin.Context, kind, result string) {
	path := "/settings/premium"
	if kind == subjectViewer {
		path = "/settings/viewer/premium"
	}
	c.Redirect(http.StatusFound, fmt.Sprintf("%s%s?patreon=%s", h.frontendURL, path, result))
}

// randomToken returns a 32-byte hex CSRF token.
func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
