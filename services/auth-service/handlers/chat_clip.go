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
	"io"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const twitchClipsURL = "https://api.twitch.tv/helix/clips"

// InternalClipRequest is the body of POST /internal/twitch/clip.
type InternalClipRequest struct {
	UserID string `json:"user_id" binding:"required"`
}

// HandleInternalClip clips the streamer's live Twitch stream (ADR-0079). Called
// service-to-service by engagement-service when a chat command with create_clip fires.
// It uses the same users-row Twitch token as the streamer send path, so it works for
// Twitch-login streamers who granted clips:edit through the advanced-controls opt-in.
// The response carries the clip URL, or error_kind (stream_offline, missing_scope,
// reauth_required, send_failed) with a non-200 status.
func (h *ChatSendHandler) HandleInternalClip(c *gin.Context) {
	var req InternalClipRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
		return
	}
	ctx := c.Request.Context()
	user, err := h.userRepo.GetByID(ctx, req.UserID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	if user.IsBanned {
		c.JSON(http.StatusForbidden, gin.H{"error": "account banned"})
		return
	}
	if user.AuthProvider != "twitch" || user.TwitchID == nil || *user.TwitchID == "" {
		c.JSON(http.StatusConflict, gin.H{"error": "twitch_not_connected", "error_kind": "twitch_not_connected"})
		return
	}
	if err := h.refreshStreamerTokenIfNeeded(ctx, user); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "token expired", "error_kind": string(sendErrReauth)})
		return
	}

	clipURL, err := createTwitchClip(ctx, h.httpClient, twitchClipsURL, h.clientID, user.AccessToken, *user.TwitchID)
	if err != nil {
		kind := sendResultErrorKind(err)
		h.log.Info("chat command clip failed", zap.String("user_id", user.ID), zap.String("error_kind", string(kind)), zap.Error(err))
		c.JSON(http.StatusBadGateway, gin.H{"error": "clip failed", "error_kind": string(kind)})
		return
	}
	c.JSON(http.StatusOK, gin.H{"url": clipURL})
}

// createTwitchClip calls Helix Create Clip for broadcasterID and returns the clip's
// public URL. Twitch answers 202 while it finishes the clip in the background; the
// URL is valid immediately. 404 means the channel is not live.
func createTwitchClip(ctx context.Context, client *http.Client, endpoint, clientID, accessToken, broadcasterID string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint+"?broadcaster_id="+url.QueryEscape(broadcasterID), nil)
	if err != nil {
		return "", fmt.Errorf("build clip request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Client-Id", clientID)

	resp, err := client.Do(req)
	if err != nil {
		return "", &streamerSendError{kind: sendErrUpstream, msg: fmt.Sprintf("twitch clip request: %v", err)}
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))

	switch resp.StatusCode {
	case http.StatusAccepted, http.StatusOK:
	case http.StatusNotFound:
		return "", &streamerSendError{kind: sendErrOffline, msg: "twitch clip: channel is not live"}
	default:
		return "", classifyPlatformStatus("twitch", resp.StatusCode, string(body))
	}

	var out struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &out); err != nil || len(out.Data) == 0 || out.Data[0].ID == "" {
		return "", &streamerSendError{kind: sendErrUpstream, msg: "twitch clip: no clip id in response"}
	}
	return "https://clips.twitch.tv/" + url.PathEscape(out.Data[0].ID), nil
}
