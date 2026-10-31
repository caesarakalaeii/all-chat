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
	"fmt"
	"net/http"
	"time"

	"github.com/caesar/all-chat/services/auth-service/oauth"
	"github.com/caesar/all-chat/services/auth-service/repository"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

// Chat bot connect (ADR-0081).
//
// A bot chat account consents ONCE per platform; its OAuth row lands in
// chat_bot_accounts and the callback NEVER logs that account in. Two flavors:
//
//   - custom (`?scope=custom`, gated RequireEarlyAccess + RequirePremium on
//     gate custom_chat_bot): the streamer's own bot; returns to /commands.
//   - allchat (`?scope=allchat`, admin only): the global All-Chat Bot the
//     admin connects from /admin/chat-bots; Twitch and YouTube only — Kick's
//     All-Chat Bot is the bot built into our Kick app and needs no credential.
//
// The flow mirrors HandleModConsent: its own OAuth action (no add-source
// machinery, no login state, no touching the consenting account's session),
// reusing the existing per-platform callback and redirect_uri.

// WithChatBotStore wires the bot account repository into the V2 auth handler.
func (h *PlatformAuthHandlerV2) WithChatBotStore(store *repository.ChatBotAccountStore) *PlatformAuthHandlerV2 {
	h.botRepo = store
	return h
}

// botConnectScopes returns the scopes a bot consent must request per platform,
// and whether the platform has a bot leg at all (Kick's allchat flavor has none).
func botConnectScopes(platform oauth.Platform) ([]string, bool) {
	switch platform {
	case oauth.PlatformTwitch:
		// user:bot marks the account as an API bot; user:write:chat lets Helix
		// chat/messages speak under it. Bot consents request NO login scopes.
		return []string{"user:bot", "user:write:chat"}, true
	case oauth.PlatformKick:
		// chat:write sends; user:read is the identity read the callback needs
		// to know WHICH Kick account consented (same as mod-consent).
		return []string{"chat:write", "user:read"}, true
	case oauth.PlatformYouTube:
		// force-ssl covers liveChatMessages.insert; the identity scope lets the
		// callback attribute the credential.
		return []string{"https://www.googleapis.com/auth/youtube.force-ssl", oauth.YouTubeIdentityScope}, true
	default:
		return nil, false
	}
}

// botConnectAuthURL builds the per-platform consent URL with the given scopes,
// reusing each provider's minimal-scope consent builder (no login scopes).
func botConnectAuthURL(provider oauth.OAuthProvider, platform oauth.Platform, state string, scopes []string) (authURL string, codeVerifier string) {
	switch p := provider.(type) {
	case *oauth.TwitchOAuth:
		return p.GetModConsentAuthURL(state, scopes), ""
	case *oauth.KickOAuth:
		return p.GetModConsentAuthURLPKCE(state, scopes)
	case *oauth.YouTubeOAuth:
		return p.GetModConsentAuthURL(state, scopes), ""
	default:
		return "", ""
	}
}

// HandleBotConnect starts a chat-bot consent flow (ADR-0081).
// Registered per-platform under /auth/:platform/bot-connect with
// ?scope=custom|allchat; gating is done by the route-level middleware.
func (h *PlatformAuthHandlerV2) HandleBotConnect(platform oauth.Platform) gin.HandlerFunc {
	return func(c *gin.Context) {
		scopeFlavor := c.Query("scope")
		if scopeFlavor != oauth.PurposeBotConnectCustom && scopeFlavor != oauth.PurposeBotConnectAllChat {
			c.JSON(http.StatusBadRequest, gin.H{"error": "scope must be custom or allchat"})
			return
		}
		if scopeFlavor == oauth.PurposeBotConnectAllChat && platform == oauth.PlatformKick {
			// The Kick All-Chat Bot is a bot configured on our Kick app that
			// speaks through the streamer's own token; there is nothing to connect.
			c.JSON(http.StatusBadRequest, gin.H{"error": "the Kick All-Chat Bot needs no separate connection"})
			return
		}

		provider, exists := h.providers[platform]
		if !exists {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Platform %s not supported", platform)})
			return
		}

		scopes, legLanded := botConnectScopes(platform)
		if !legLanded {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("chat bot connect for %s is not available", platform)})
			return
		}

		userID, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized - please log in first"})
			return
		}
		userIDStr, ok := userID.(string)
		if !ok || userIDStr == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user context"})
			return
		}

		// ReturnPath is fixed by the flavor, server-side: custom goes back to the
		// commands editor, allchat to the admin page. The client cannot widen it.
		returnPath := oauth.ChatCommandsReturnPath
		if scopeFlavor == oauth.PurposeBotConnectAllChat {
			returnPath = oauth.ChatBotsAdminReturnPath
		}

		csrfToken, err := generateRandomString(32)
		if err != nil {
			h.logger.Error("Failed to generate CSRF token", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
			return
		}

		// Origin handling: see HandleLogin.
		origin := requestFrontendOrigin(c)
		provider = providerForOrigin(provider, platform, origin)
		oauthState := oauth.NewBotConnectState(csrfToken, userIDStr, scopeFlavor, returnPath)
		oauthState.Origin = origin
		if err := oauthState.Validate(); err != nil {
			h.logger.Error("Invalid bot-connect OAuth state", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
			return
		}
		stateStr, err := oauthState.Encode()
		if err != nil {
			h.logger.Error("Failed to encode bot-connect state", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
			return
		}

		authURL, codeVerifier := botConnectAuthURL(provider, platform, stateStr, scopes)
		if authURL == "" {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to build consent URL"})
			return
		}

		// Kick mandates PKCE; the verifier goes under the key the shared
		// callback's exchange reads (GetDel, single use).
		if codeVerifier != "" {
			verifierKey := fmt.Sprintf("oauth_verifier:%s:%s", platform, csrfToken)
			if err := h.redis.Set(c.Request.Context(), verifierKey, codeVerifier, 30*time.Minute).Err(); err != nil {
				h.logger.Error("Failed to store bot-connect PKCE verifier", zap.Error(err))
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
				return
			}
		}

		stateKey := fmt.Sprintf("oauth_state:%s:%s", platform, csrfToken)
		if err := h.redis.Set(c.Request.Context(), stateKey, stateStr, 30*time.Minute).Err(); err != nil {
			h.logger.Error("Failed to store bot-connect state", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
			return
		}

		h.logger.Info("Generated chat-bot consent URL",
			zap.String("platform", string(platform)),
			zap.String("user_id", userIDStr),
			zap.String("scope", scopeFlavor),
			zap.Strings("bot_scopes", scopes),
		)
		c.JSON(http.StatusOK, gin.H{"auth_url": authURL})
	}
}

// completeBotConnect finishes a chat-bot consent: it resolves the bot's platform
// identity and stores the credential row, then redirects back to the flavor's
// return path with ?bot=connected|error. It creates no account, links no
// platform, adds no overlay source, and never issues a session for the bot.
func (h *PlatformAuthHandlerV2) completeBotConnect(
	c *gin.Context,
	platform oauth.Platform,
	state *oauth.OAuthState,
	provider oauth.OAuthProvider,
	platformUser oauth.PlatformUserInfo,
	token *oauth2.Token,
) {
	if h.botRepo == nil {
		h.logger.Error("bot-connect callback without a bot store", zap.String("platform", string(platform)))
		h.redirectBotConnect(c, platform, state, "", "not_configured")
		return
	}

	platformUserID := platformUser.GetID()
	login := platformUser.GetUsername()
	displayName := platformUser.GetDisplayName()

	// The bot row's platform_user_id is what a Twitch send puts into sender_id.
	// Kick's users-endpoint id is numeric and correct as-is; for YouTube the USER
	// id is not the channel id, so resolve the channel — the insert keys on it
	// and the status shows it as the connected identity.
	if platform == oauth.PlatformYouTube {
		ytProvider, ok := provider.(*oauth.YouTubeOAuth)
		if !ok {
			h.logger.Error("YouTube provider assertion failed in bot connect")
			h.redirectBotConnect(c, platform, state, "", "youtube_no_channel")
			return
		}
		channelInfo, channelErr := ytProvider.GetPrimaryChannel(c.Request.Context(), token.AccessToken)
		if channelErr != nil || channelInfo.ChannelID == "" {
			h.logger.Error("Failed to resolve YouTube channel for bot connect",
				zap.String("platform", string(platform)), zap.Error(channelErr))
			h.redirectBotConnect(c, platform, state, "", "youtube_no_channel")
			return
		}
		platformUserID = channelInfo.ChannelID
		login = channelInfo.ChannelID
		displayName = channelInfo.Title
	}

	// botOwnerFor: the All-Chat Bot flavor stores an owner-less (NULL-owner) row —
	// NULL, not the connecting admin's id, is what bot-send resolves the global
	// bot by. A custom consent is attributed to the consenting streamer; Validate
	// guarantees the state carries a user id.
	botOwner := state.UserID
	if state.Purpose == oauth.PurposeBotConnectAllChat {
		botOwner = ""
	}

	granted := oauth.ExtractGrantedScopes(token)
	if err := h.botRepo.StoreChatBotToken(
		c.Request.Context(),
		botOwner,
		string(platform),
		platformUserID,
		login,
		displayName,
		token,
		granted,
	); err != nil {
		h.logger.Error("Failed to store chat-bot credential",
			zap.String("platform", string(platform)),
			zap.String("user_id", state.UserID),
			zap.Error(err))
		h.redirectBotConnect(c, platform, state, "", "credential_store_failed")
		return
	}

	h.logger.Info("Stored chat-bot credential",
		zap.String("platform", string(platform)),
		zap.String("user_id", state.UserID),
		zap.String("purpose", state.Purpose),
		zap.String("platform_user_id", platformUserID),
		zap.Strings("granted_scopes", granted))
	h.redirectBotConnect(c, platform, state, "connected", "")
}

// redirectBotConnect returns the browser to the flavor's return path, appending
// ?bot=connected|error as the frontend contract expects. A plain redirect, like
// redirectModConsent: a bot consent grants a capability to an existing identity,
// it never establishes one, so there is no auth-code round trip.
func (h *PlatformAuthHandlerV2) redirectBotConnect(c *gin.Context, platform oauth.Platform, state *oauth.OAuthState, connected, errCode string) {
	base := stateOrigin(state) + state.ReturnPath
	target := fmt.Sprintf("%s?bot=connected", base)
	switch {
	case errCode != "":
		target = fmt.Sprintf("%s?bot=error&error=%s&platform=%s", base, errCode, platform)
	case connected != "connected":
		target = base
	}
	h.redirectWithTombstone(c, platform, state.CSRFToken, target)
}

// HandleGetChatBots is GET /auth/chat-bots: the streamer's view of their connected
// bots. custom.* lists the caller's own rows (null when a platform has none);
// allchat.* lists the global rows for every user — those bot names are public
// (they post in every chat they serve). Only CONNECTING a global bot is admin-only.
func (h *PlatformAuthHandlerV2) HandleGetChatBots(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized - please log in first"})
		return
	}
	userIDStr, ok := userID.(string)
	if !ok || userIDStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user context"})
		return
	}

	user, err := h.userRepo.GetByID(c.Request.Context(), userIDStr)
	if err != nil {
		h.logger.Error("Failed to load user for chat-bots status", zap.Error(err), zap.String("user_id", userIDStr))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load user"})
		return
	}

	resp := gin.H{
		"allchat":            map[string]any{"twitch": botPlatformStatus(nil), "kick": gin.H{"connected": true}, "youtube": botPlatformStatus(nil)},
		"custom":             map[string]any{"twitch": nil, "kick": nil, "youtube": nil},
		"twitch_bot_granted": false,
		"premium":            user.IsPremium,
	}

	if h.botRepo != nil {
		// The streamer's own custom bots; every platform key is present, null when empty.
		custom, err := h.botRepo.ListChatBotAccounts(c.Request.Context(), userIDStr)
		if err != nil {
			h.logger.Warn("Failed to list custom chat bots", zap.String("user_id", userIDStr), zap.Error(err))
		} else {
			customMap := map[string]any{"twitch": nil, "kick": nil, "youtube": nil}
			for _, acct := range custom {
				customMap[acct.Platform] = gin.H{"login": acct.Login, "display_name": acct.DisplayName}
			}
			resp["custom"] = customMap
		}

		// The global rows. Their names are public (they post in every chat), so every
		// streamer sees which account will answer; only connecting them is admin-only.
		allchat, err := h.botRepo.ListChatBotAccounts(c.Request.Context(), "")
		if err != nil {
			h.logger.Warn("Failed to list allchat bot rows", zap.Error(err))
		} else {
			allchatMap := map[string]any{"twitch": botPlatformStatus(nil), "kick": gin.H{"connected": true}, "youtube": botPlatformStatus(nil)}
			for _, acct := range allchat {
				allchatMap[acct.Platform] = botPlatformStatus(acct)
			}
			resp["allchat"] = allchatMap
		}

		// The streamer's channel:bot grant decides whether the frontend shows the
		// "allow the All-Chat Bot" re-consent button.
		granted, err := h.userRepo.GetPlatformGrantedScopes(c.Request.Context(), userIDStr, "twitch")
		if err == nil {
			resp["twitch_bot_granted"] = containsScope(granted, "channel:bot")
		}
	}

	c.JSON(http.StatusOK, resp)
}

// botPlatformStatus renders one allchat bot row (nil = not connected) — Kick is
// always connected because its bot lives on our Kick app, not in this table.
func botPlatformStatus(acct *repository.ChatBotAccount) any {
	if acct == nil {
		return gin.H{"connected": false, "login": "", "display_name": ""}
	}
	return gin.H{"connected": true, "login": acct.Login, "display_name": acct.DisplayName}
}

// HandleDeleteCustomChatBot re-consents away a streamer's custom bot on one
// platform (DELETE /auth/chat-bots/custom/:platform). The chat_bot_accounts row
// goes; the sender setting is engagement-service's, untouched here.
func (h *PlatformAuthHandlerV2) HandleDeleteCustomChatBot(c *gin.Context) {
	userID, ok := h.currentUserIDOr401(c)
	if !ok {
		return
	}
	h.deleteChatBot(c, userID, c.Param("platform"), "custom")
}

// HandleDeleteAllChatBot disconnects the global All-Chat Bot row (admin anyway
// via route middleware). DELETE /auth/chat-bots/allchat/:platform.
func (h *PlatformAuthHandlerV2) HandleDeleteAllChatBot(c *gin.Context) {
	userID, ok := h.currentUserIDOr401(c)
	if !ok {
		return
	}
	h.deleteChatBot(c, userID, c.Param("platform"), "allchat")
}

func (h *PlatformAuthHandlerV2) currentUserIDOr401(c *gin.Context) (string, bool) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized - please log in first"})
		return "", false
	}
	userIDStr, ok := userID.(string)
	if !ok || userIDStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user context"})
		return "", false
	}
	return userIDStr, true
}

func (h *PlatformAuthHandlerV2) deleteChatBot(c *gin.Context, userID, platform, flavor string) {
	if platform != "twitch" && platform != "kick" && platform != "youtube" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported platform"})
		return
	}
	if h.botRepo == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "chat bots not configured"})
		return
	}
	// The allchat flavor is admin-gated at the route, but the OWNER match below
	// is what makes the custom delete safe: deleteChatBot only ever removes rows
	// owned by the caller for the custom flavor.
	owner := userID
	if flavor == "allchat" {
		owner = "" // global row; route middleware already required an admin.
	}
	if err := h.botRepo.DeleteChatBotAccount(c.Request.Context(), owner, platform); err != nil {
		h.logger.Error("Failed to delete chat bot", zap.String("flavor", flavor),
			zap.String("platform", platform), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to disconnect bot"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}
