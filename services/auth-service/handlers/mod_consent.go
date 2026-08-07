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
	"fmt"
	"net/http"
	"time"

	"github.com/caesar/all-chat/services/auth-service/oauth"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

// Delegated-moderator consent (ADR-0048).
//
// A moderator grants All-Chat their OWN moderation scopes so they can act on channels they
// moderate. Three properties make this a separate flow rather than a variant of the streamer's
// opt-in re-consent (ADR-0017):
//
//   - It must not reach addSourceToOverlay. The streamer's moderation re-consent is an
//     add-source state, and the shared callback calls addSourceToOverlay for every add-source
//     state; overlay-manager 404s that for a non-owner, so a moderator's consent would persist
//     their credential and THEN redirect to an error.
//   - It must not request the base login scopes. GetAuthURLWithScopes prepends them
//     unconditionally, which would ask a volunteer for channel-point, subscription and bits read
//     on their own channel — an ADR-0012 regression, and it reads badly on the consent screen.
//   - It must not touch their login credential or its granted_scopes. The credential goes to
//     mod_oauth_credentials, keyed on the moderator, which also keeps the scope-downgrade guard
//     entirely out of the picture.
//
// Consent is deferred to first use and is NOT bound to an overlay: Twitch's and Kick's
// moderation scopes are role-based rather than channel-scoped, so one consent per platform
// serves every streamer who delegated that platform.

// HandleModConsent starts a delegated moderator's own consent flow for one platform.
//
// The moderator must already be signed in — this grants a capability to an existing account and
// must never create one.
func (h *PlatformAuthHandlerV2) HandleModConsent(platform oauth.Platform) gin.HandlerFunc {
	return func(c *gin.Context) {
		provider, exists := h.providers[platform]
		if !exists {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Platform %s not supported", platform)})
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

		// Minimised to exactly the actions delegated to them. Note what is deliberately NOT
		// folded in, unlike the streamer flow: no chat-send scope (moderators get no send in
		// v1, and it is a distinct higher-trust capability), and no union with their existing
		// grant — the credential lands in its own table, so there is nothing to preserve and
		// no downgrade guard to satisfy.
		actions := splitActions(c.Query("actions"))
		twitchProvider, isTwitch := provider.(*oauth.TwitchOAuth)
		if !isTwitch {
			// Each platform leg is independently gated (ADR-0048). Kick needs a PKCE variant
			// of the minimal-scope builder; YouTube additionally needs the moderator's own
			// channel id resolved before a credential can be attributed.
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("delegated moderation for %s is not available yet", platform),
			})
			return
		}

		// Only the four moderation actions are delegatable. The shared mapper also accepts
		// "engagement", which maps to channel:read:polls / channel:read:predictions — scopes on
		// the requester's OWN channel, for a capability moderators do not get at all (polls and
		// predictions stay owner-only). Passing the raw query through would let a crafted URL
		// widen a volunteer's consent screen to scopes delegation never uses.
		delegatable := filterDelegatableActions(actions)
		if len(delegatable) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "no valid moderation actions; expected ?actions=delete,timeout,ban,unban",
			})
			return
		}

		modScopes := oauth.ModerationScopesForActions(delegatable)
		if len(modScopes) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "no valid moderation actions; expected ?actions=delete,timeout,ban,unban",
			})
			return
		}

		csrfToken, err := generateRandomString(32)
		if err != nil {
			h.logger.Error("Failed to generate CSRF token", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
			return
		}

		oauthState := oauth.NewModConsentState(csrfToken, userIDStr)
		if err := oauthState.Validate(); err != nil {
			h.logger.Error("Invalid mod-consent OAuth state", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
			return
		}
		stateStr, err := oauthState.Encode()
		if err != nil {
			h.logger.Error("Failed to encode mod-consent state", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
			return
		}

		authURL := twitchProvider.GetModConsentAuthURL(stateStr, modScopes)
		if authURL == "" {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to build consent URL"})
			return
		}

		stateKey := fmt.Sprintf("oauth_state:%s:%s", platform, csrfToken)
		if err := h.redis.Set(c.Request.Context(), stateKey, stateStr, 30*time.Minute).Err(); err != nil {
			h.logger.Error("Failed to store mod-consent state", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
			return
		}

		h.logger.Info("Generated delegated-moderator consent URL",
			zap.String("platform", string(platform)),
			zap.String("user_id", userIDStr),
			zap.Strings("mod_scopes", modScopes),
		)
		c.JSON(http.StatusOK, gin.H{"auth_url": authURL})
	}
}

// completeModConsent finishes a delegated moderator's consent: it stores their credential and
// returns them to the moderation area. It creates no account, links no platform, and adds no
// overlay source.
//
// It is called from the shared OAuth callback BEFORE any of the login / account-link /
// add-source machinery, and the caller returns immediately afterwards.
func (h *PlatformAuthHandlerV2) completeModConsent(
	c *gin.Context,
	platform oauth.Platform,
	state *oauth.OAuthState,
	platformUser oauth.PlatformUserInfo,
	token *oauth2.Token,
) {
	// Belt and braces: Validate() already requires this, but the credential is keyed on the
	// moderator and an empty user id must never reach the store.
	if state.UserID == "" {
		h.logger.Error("mod-consent callback without a user id", zap.String("platform", string(platform)))
		h.redirectModConsent(c, platform, state.CSRFToken, "", "invalid_state")
		return
	}

	granted := oauth.ExtractGrantedScopes(token)
	if err := h.userRepo.StoreModCredential(
		c.Request.Context(),
		state.UserID,
		string(platform),
		platformUser.GetID(),
		platformUser.GetUsername(),
		token,
		granted,
	); err != nil {
		h.logger.Error("Failed to store delegated-moderator credential",
			zap.String("platform", string(platform)),
			zap.String("user_id", state.UserID),
			zap.Error(err))
		h.redirectModConsent(c, platform, state.CSRFToken, "", "credential_store_failed")
		return
	}

	h.logger.Info("Stored delegated-moderator credential",
		zap.String("platform", string(platform)),
		zap.String("user_id", state.UserID),
		zap.String("platform_user_id", platformUser.GetID()),
		zap.Strings("granted_scopes", granted))

	h.redirectModConsent(c, platform, state.CSRFToken, string(platform), "")
}

// redirectModConsent returns the moderator to the moderation area.
//
// A plain redirect is correct here, unlike the add-source path's auth-code round trip: the
// moderator was already signed in when they started, and this flow deliberately does not
// re-issue a session — it grants a capability to an existing account rather than establishing
// one.
func (h *PlatformAuthHandlerV2) redirectModConsent(c *gin.Context, platform oauth.Platform, csrfToken, connected, errCode string) {
	target := fmt.Sprintf("%s/moderate", h.frontendURL)
	switch {
	case errCode != "":
		target = fmt.Sprintf("%s?error=%s&platform=%s", target, errCode, platform)
	case connected != "":
		target = fmt.Sprintf("%s?connected=%s", target, connected)
	}
	h.redirectWithTombstone(c, platform, csrfToken, target)
}

// delegatableActions are the only actions a streamer can delegate, and therefore the only ones a
// moderator's consent may request scopes for. Anything else — notably "engagement", which the
// shared scope mapper accepts — is dropped rather than silently widening the consent screen.
var delegatableActions = map[string]bool{
	"delete":  true,
	"timeout": true,
	"ban":     true,
	"unban":   true,
}

// filterDelegatableActions keeps only the delegatable actions, preserving order and dropping
// duplicates so the resulting consent screen is stable across repeat requests.
func filterDelegatableActions(actions []string) []string {
	seen := make(map[string]bool, len(actions))
	out := make([]string, 0, len(actions))
	for _, a := range actions {
		if delegatableActions[a] && !seen[a] {
			seen[a] = true
			out = append(out, a)
		}
	}
	return out
}

// ModCredentialStore is the subset of the user repository the mod-consent flow needs. Declared
// for the handler tests, which must be able to assert that a mod-consent callback stores a
// credential and creates no overlay source.
type ModCredentialStore interface {
	StoreModCredential(ctx context.Context, userID, platform, platformUserID, platformLogin string,
		token *oauth2.Token, grantedScopes []string) error
	DeleteModCredential(ctx context.Context, userID, platform string) error
}
