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
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/caesar/all-chat/services/auth-service/repository"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// The Discord ACCOUNT LINK, as opposed to the bot invite in discord.go.
//
// All-Chat has always known which Discord *servers* a streamer connected and never who any user
// is on Discord, because the only Discord flow it ran was a `scope=bot` invite whose callback
// returns a guild_id and no identity. ADR-0048's Discord leg cannot work without the identity:
// Discord has no per-user moderation API, so the shared bot performs every write and All-Chat
// must itself read the acting human's guild permissions — which needs their snowflake.
//
// Both roles need one. A delegated moderator's own permissions are the check; the overlay owner's
// prove they control the guild being moderated in.

// discordFlowIdentity marks a state issued for the account-link flow.
const discordFlowIdentity = "identity"

// discordFlowState is what the shared callback must know about a flow it did not start. It lives
// in the state VALUE (server-side, in Redis) rather than in a query parameter: a client able to
// choose the branch could feed a bot invite's code into the link path, or the reverse.
type discordFlowState struct {
	// Kind is discordFlowIdentity for an account link. Empty means the bot invite.
	Kind string `json:"kind,omitempty"`
	// UserID is the authenticated user who started the flow.
	UserID string `json:"user_id"`
	// Return is an allowlisted key naming where to send the browser afterwards — a key, never a
	// URL, so this can never become an open redirect.
	Return string `json:"return,omitempty"`
}

// discordReturnPaths is the closed allowlist of post-link destinations. A streamer links from
// settings; a delegated moderator links from the channels-I-moderate area and has no reason to
// end up in someone else's settings.
var discordReturnPaths = map[string]string{
	"settings": "/settings",
	"moderate": "/moderate",
}

const defaultDiscordReturn = "settings"

// encodeDiscordFlowState renders a flow state for the state store.
func encodeDiscordFlowState(f discordFlowState) (string, error) {
	payload, err := json.Marshal(f)
	if err != nil {
		return "", fmt.Errorf("encode discord flow state: %w", err)
	}
	return string(payload), nil
}

// parseDiscordFlowState reads a stored state value.
//
// A bare user id (the pre-ADR-0048 format) is a bot invite. That fallback is not decoration: state
// entries live for ten minutes, so an invite already in flight when this deploy rolls must still
// complete rather than fail on an unrecognised state.
func parseDiscordFlowState(stored string) discordFlowState {
	if !strings.HasPrefix(strings.TrimSpace(stored), "{") {
		return discordFlowState{UserID: stored}
	}
	var f discordFlowState
	if err := json.Unmarshal([]byte(stored), &f); err != nil {
		// Unparseable JSON is not a bot invite either — return an empty state so the caller's
		// user-id check rejects it rather than acting for a user we cannot name.
		return discordFlowState{}
	}
	return f
}

// HandleIdentityConnect starts the Discord account link and returns the consent URL.
//
// Route: GET /discord/identity/connect?return=settings|moderate (JWT required)
func (h *DiscordHandler) HandleIdentityConnect(c *gin.Context) {
	userIDRaw, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID := fmt.Sprintf("%v", userIDRaw)

	returnKey := c.Query("return")
	if _, ok := discordReturnPaths[returnKey]; !ok {
		returnKey = defaultDiscordReturn
	}

	state, err := generateRandomString(32)
	if err != nil {
		h.log.Error("discord identity: failed to generate state", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}
	stored, err := encodeDiscordFlowState(discordFlowState{
		Kind:   discordFlowIdentity,
		UserID: userID,
		Return: returnKey,
	})
	if err != nil {
		h.log.Error("discord identity: failed to encode state", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}
	if err := h.stateStore.Set(c.Request.Context(), state, stored, 10*time.Minute); err != nil {
		h.log.Error("discord identity: failed to store OAuth state", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"auth_url": h.oauth.GetIdentityAuthURL(state)})
}

// completeIdentityLink finishes the account link. Called from the shared callback once the stored
// state has identified this as the link flow.
//
// Failures redirect with an error code rather than rendering JSON: the user is in a browser
// mid-OAuth, and the frontend owns the copy for each case.
func (h *DiscordHandler) completeIdentityLink(c *gin.Context, flow discordFlowState, code string) {
	ctx := c.Request.Context()
	if flow.UserID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid or expired state"})
		return
	}

	token, err := h.oauth.ExchangeCode(ctx, code)
	if err != nil {
		h.log.Error("discord identity: code exchange failed", zap.Error(err))
		h.redirectIdentity(c, flow.Return, "error=exchange_failed")
		return
	}

	identity, err := h.oauth.GetIdentity(ctx, token.AccessToken)
	if err != nil {
		h.log.Error("discord identity: identity read failed", zap.Error(err))
		h.redirectIdentity(c, flow.Return, "error=identity_unavailable")
		return
	}

	switch err := h.repo.UpsertIdentity(ctx, flow.UserID, identity.ID, identity.Username); {
	case errors.Is(err, repository.ErrDiscordIdentityClaimed):
		// Deliberately distinguishable: the remedy is a human one (unlink it on the other
		// account, or link the right Discord account here), and a generic failure would send the
		// user round the consent loop forever.
		h.log.Warn("discord identity: account already linked to another user",
			zap.String("user_id", flow.UserID))
		h.redirectIdentity(c, flow.Return, "error=already_linked")
		return
	case err != nil:
		h.log.Error("discord identity: failed to store link", zap.String("user_id", flow.UserID), zap.Error(err))
		h.redirectIdentity(c, flow.Return, "error=save_failed")
		return
	}

	// The identify grant has done its one job. It is not stored: All-Chat never acts as the user
	// on Discord — every write is the bot — so retaining the token would be a credential held for
	// no purpose.
	h.log.Info("discord identity: linked", zap.String("user_id", flow.UserID))
	h.redirectIdentity(c, flow.Return, "discord_account=linked")
}

// redirectIdentity sends the browser back to the allowlisted return path with a result marker.
func (h *DiscordHandler) redirectIdentity(c *gin.Context, returnKey, query string) {
	path, ok := discordReturnPaths[returnKey]
	if !ok {
		path = discordReturnPaths[defaultDiscordReturn]
	}
	c.Redirect(http.StatusFound, strings.TrimSuffix(h.frontendURL, "/")+path+"?"+query)
}

// HandleGetIdentity reports whether the caller has linked a Discord account.
//
// Route: GET /discord/identity (JWT required)
func (h *DiscordHandler) HandleGetIdentity(c *gin.Context) {
	userIDRaw, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID := fmt.Sprintf("%v", userIDRaw)

	identity, err := h.repo.GetIdentity(c.Request.Context(), userID)
	if errors.Is(err, repository.ErrNotFound) {
		c.JSON(http.StatusOK, gin.H{"linked": false})
		return
	}
	if err != nil {
		h.log.Error("discord identity: lookup failed", zap.String("user_id", userID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"linked":           true,
		"discord_user_id":  identity.DiscordUserID,
		"discord_username": identity.DiscordUsername,
		"linked_at":        identity.LinkedAt,
	})
}

// HandleDeleteIdentity unlinks the caller's Discord account. Idempotent.
//
// Allowed even while Discord moderation grants exist: the moderation path resolves the identity
// live and fails closed without one, so the effect is simply that Discord moderation stops
// working for this user — the right outcome for someone withdrawing the link.
//
// Route: DELETE /discord/identity (JWT required)
func (h *DiscordHandler) HandleDeleteIdentity(c *gin.Context) {
	userIDRaw, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID := fmt.Sprintf("%v", userIDRaw)

	if err := h.repo.DeleteIdentity(c.Request.Context(), userID); err != nil {
		h.log.Error("discord identity: unlink failed", zap.String("user_id", userID), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}
	h.log.Info("discord identity: unlinked", zap.String("user_id", userID))
	c.JSON(http.StatusOK, gin.H{"unlinked": true})
}
