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
	"strings"
	"sync"
	"time"

	"github.com/caesar/all-chat/services/auth-service/models"
	"github.com/caesar/all-chat/services/auth-service/repository"
	"github.com/caesar/all-chat/shared/quota"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Internal bot send (ADR-0081). engagement-service calls POST /internal/chat/bot-send
// (service JWT, same group as /internal/chat/announce) to post a chat-command reply
// or a timer message AS a bot sender. The sender is picked per platform, first match
// wins: the streamer's own account when chat_bot_settings.sender is 'self'; their
// custom bot (sender "custom_bot") when they are premium and have a chat_bot_accounts
// row for the platform; otherwise the All-Chat Bot (sender "allchat_bot"). Twitch and
// YouTube need a global row for it, Kick needs none, because the message goes through
// the streamer's own user token with type=bot and the bot identity is the one
// configured on our Kick app.
//
// A Twitch bot send additionally requires the streamer's channel:bot grant. Without
// that grant, or when the needed bot row is missing, the send falls back to self —
// chat must not go silent because a connected bot is broken; it goes out under the
// streamer's name and the result says which sender was used. Platforms the streamer
// has no sendable identity on are skipped, exactly like announce.
//
// No echo-collapse dedup keys are registered for bot senders: the echoed message
// arrives under a DIFFERENT author than the streamer, so the send-to-all pill logic
// (which collapses same-author echoes) does not apply. The streamer's own "self"
// sends keep that behaviour untouched.

// BotSendPurpose selects how the send behaves where a choice exists.
type BotSendPurpose string

const (
	BotSendPurposeCommand BotSendPurpose = "command"
	BotSendPurposeTimer   BotSendPurpose = "timer"
)

// BotSendResult is one platform's outcome in a bot-send response.
type BotSendResult struct {
	Platform string `json:"platform"`
	Success  bool   `json:"success"`
	Sender   string `json:"sender"` // "allchat_bot" | "custom_bot" | "self"
	Error    string `json:"error,omitempty"`
	// ErrorKind mirrors the announce result's machine-readable classification.
	ErrorKind string `json:"error_kind,omitempty"`
}

// botSendSender values reported to engagement-service.
const (
	botSenderAllChatBot = "allchat_bot"
	botSenderCustomBot  = "custom_bot"
	botSenderSelf       = "self"
)

// InternalBotSendRequest is the body of POST /internal/chat/bot-send.
// platforms is the ordered list the caller wants; engagement deduplicates and
// orders upstream, this endpoint simply intersects with sendable identity.
type InternalBotSendRequest struct {
	UserID    string         `json:"user_id" binding:"required"`
	Message   string         `json:"message" binding:"required"`
	Platforms []string       `json:"platforms" binding:"required"`
	Purpose   BotSendPurpose `json:"purpose"`
}

// HandleInternalBotSend posts a message to each requested platform as the resolved
// bot or streamer sender per the contract above. Partial success is normal.
func (h *ChatSendHandler) HandleInternalBotSend(c *gin.Context) {
	var req InternalBotSendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id, message and platforms are required"})
		return
	}
	if len(req.Message) == 0 || len(req.Message) > maxMessageLength {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("message must be between 1 and %d characters", maxMessageLength)})
		return
	}
	if req.Purpose == "" {
		req.Purpose = BotSendPurposeCommand
	}
	if req.Purpose != BotSendPurposeCommand && req.Purpose != BotSendPurposeTimer {
		c.JSON(http.StatusBadRequest, gin.H{"error": "purpose must be command or timer"})
		return
	}

	ctx := c.Request.Context()
	user, err := h.userRepo.GetByID(ctx, req.UserID)
	if err != nil {
		h.log.Warn("bot-send: failed to load user", zap.Error(err), zap.String("user_id", req.UserID))
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	if user.IsBanned {
		c.JSON(http.StatusForbidden, gin.H{"error": "account banned"})
		return
	}
	if err := h.refreshStreamerTokenIfNeeded(ctx, user); err != nil {
		h.log.Warn("bot-send: token refresh failed", zap.Error(err), zap.String("user_id", req.UserID))
		c.JSON(http.StatusUnauthorized, gin.H{"error": "token expired"})
		return
	}

	// The per-user sender choice lives in chat_bot_settings, written by
	// engagement-service. No row = All-Chat Bot.
	senderSetting := "bot"
	if h.botRepo != nil {
		if setting, err := h.botRepo.GetChatBotSenderSetting(ctx, user.ID); err != nil {
			h.log.Warn("bot-send: failed to read sender setting; defaulting to All-Chat Bot",
				zap.String("user_id", user.ID), zap.Error(err))
		} else {
			senderSetting = setting
		}
	}
	useStreamerSender := senderSetting == "self"

	// Intersect requested platforms with where the streamer actually has an identity
	// (the fallback target and the Kick bot-send carrier both need one). Kick uses
	// the send-to-all test, resolveKickSender, since a linked Kick is not on the users row.
	requested := map[string]bool{}
	for _, p := range req.Platforms {
		requested[strings.ToLower(p)] = true
	}
	targets := make([]string, 0, 3)
	for _, p := range []string{"twitch", "youtube", "kick"} {
		if !requested[p] {
			continue
		}
		if p == "kick" {
			if _, _, err := h.resolveKickSender(ctx, user); err != nil {
				continue
			}
		} else if !h.streamerHasPlatformCtx(ctx, user, p) {
			continue
		}
		targets = append(targets, p)
	}
	if len(targets) == 0 {
		c.JSON(http.StatusOK, gin.H{"success": false, "results": []BotSendResult{}})
		return
	}

	if ok, retryAfter := h.reserveStreamerRate(ctx, user.ID, len(targets)); !ok {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "rate_limited", "retry_after_seconds": retryAfter})
		return
	}

	// The streamer's channel:bot grant decides whether ANY Twitch bot identity can
	// speak in their chat; read it once. Kick needs none of the bot scopes on the
	// streamer and YouTube sends are keyed on the bot's own token.
	twitchBotAllowed := true
	if h.botRepoKnownTwitchScope() {
		granted, err := h.userRepo.GetPlatformGrantedScopes(ctx, user.ID, "twitch")
		if err != nil {
			// Fail CLOSED: without a grant answer we must not send under a bot
			// identity that Twitch would reject anyway — the fallback to self is
			// the correct degradation.
			twitchBotAllowed = false
			h.log.Warn("bot-send: could not read Twitch granted scopes; falling back to self",
				zap.String("user_id", user.ID), zap.Error(err))
		} else {
			twitchBotAllowed = containsScope(granted, "channel:bot")
		}
	}

	results := make([]BotSendResult, 0, len(targets))
	anySuccess := false
	for _, platform := range targets {
		result := h.botSendOnePlatform(ctx, user, platform, req.Message, req.Purpose, useStreamerSender, twitchBotAllowed)
		if result.Success {
			anySuccess = true
		}
		results = append(results, result)
	}

	c.JSON(http.StatusOK, gin.H{"success": anySuccess, "results": results})
}

// botRepoKnownTwitchScope reports whether the bot store is wired at all; without it
// (unit-test fixtures, degraded deployments) Twitch bot sends cannot check the
// streamer's grant, so they fall back to self like any other uncertainty. With a
// store wired, the Twitch grant decision happens per platform in the caller.
func (h *ChatSendHandler) botRepoKnownTwitchScope() bool { return h.botRepo != nil }

// ytBotChatResolver is a test seam: when nil (production) sendBotYouTubeMessage
// resolves the live chat id with the logic below; tests substitute it to assert
// the timer path stays Redis-only without standing up a database.
type ytBotChatResolver func(ctx context.Context, channelID string, purpose BotSendPurpose) (string, error)

// botSenderDecision is the resolution output for one platform.
type botSenderDecision struct {
	sender string // "allchat_bot" | "custom_bot" | "self" (already final)
	acct   *repository.ChatBotAccount
}

// botSendOnePlatform resolves the sender for one platform and sends, returning the
// per-platform result. No fallback happens mid-send: once a bot sender is chosen
// and the request goes out, a platform failure is reported as that platform's
// failure (engagement shows it in the command editor), not retried as self.
func (h *ChatSendHandler) botSendOnePlatform(
	ctx context.Context, user *models.User, platform, message string,
	purpose BotSendPurpose, useStreamerSender, twitchBotAllowed bool,
) BotSendResult {
	decision, fallback := h.botSenderFor(ctx, user, platform, twitchBotAllowed)
	if fallback {
		// The contract's fallback: the streamer's grant or bot row is missing, so
		// speak as self instead of failing the send entirely.
		h.log.Info("bot-send: bot sender unavailable; falling back to self",
			zap.String("user_id", user.ID), zap.String("platform", platform))
		return h.botSendAsSelf(ctx, user, platform, message)
	}

	var sendErr error
	switch platform {
	case "twitch":
		sendErr = h.sendBotTwitchMessage(ctx, decision.acct, user, message)
	case "kick":
		if decision.sender == botSenderCustomBot {
			sendErr = h.sendCustomBotKickMessage(ctx, decision.acct, user, message)
		} else {
			sendErr = h.sendAllChatBotKickMessage(ctx, user, message)
		}
	case "youtube":
		sendErr = h.sendBotYouTubeMessage(ctx, decision.acct, user, message, purpose)
	default:
		sendErr = fmt.Errorf("unsupported platform %q", platform)
	}
	if sendErr != nil {
		kind := sendResultErrorKind(sendErr)
		h.log.Warn("bot-send platform failed",
			zap.String("platform", platform), zap.String("sender", decision.sender),
			zap.String("error_kind", string(kind)), zap.Error(sendErr))
		return BotSendResult{Platform: platform, Success: false, Sender: decision.sender,
			Error: string(kind), ErrorKind: string(kind)}
	}
	return BotSendResult{Platform: platform, Success: true, Sender: decision.sender}
}

// botSendAsSelf sends under the streamer's identity, reusing the tested
// streamer-send path wholesale. Used when the user's sender setting is "self" and
// whenever a bot fallback fires.
func (h *ChatSendHandler) botSendAsSelf(ctx context.Context, user *models.User, platform, message string) BotSendResult {
	var sendErr error
	switch platform {
	case "twitch":
		sendErr = h.sendStreamerTwitchMessage(ctx, user, message)
	case "kick":
		sendErr = h.sendStreamerKickMessage(ctx, user, message)
	case "youtube":
		sendErr = h.sendStreamerYouTubeMessage(ctx, user, message)
	default:
		sendErr = fmt.Errorf("unsupported platform %q", platform)
	}
	if sendErr != nil {
		kind := sendResultErrorKind(sendErr)
		return BotSendResult{Platform: platform, Success: false, Sender: botSenderSelf,
			Error: string(kind), ErrorKind: string(kind)}
	}
	return BotSendResult{Platform: platform, Success: true, Sender: botSenderSelf}
}

// botSenderFor resolves which bot identity sends on one platform, and whether the
// caller must fall back to self. Resolution mirrors the contract:
//
//   - kick: always All-Chat Bot (no row needed);
//   - otherwise: custom row for a premium owner, then the global row.
//
// A Twitch fallback additionally fires when the streamer lacks channel:bot (checked
// before this call); YouTube falls back to self when its global row is missing, the
// same missing-row case the contract spells out for Twitch.
func (h *ChatSendHandler) botSenderFor(ctx context.Context, user *models.User, platform string, twitchBotAllowed bool) (*botSenderDecision, bool) {
	if platform == "kick" {
		return &botSenderDecision{sender: botSenderAllChatBot}, false
	}

	// A streamer without the channel:bot grant can never carry a bot send on
	// Twitch — not via their own custom bot either (Twitch rejects the send with
	// a 403, and the fallback to self is the correct degradation). Checked before
	// any row lookup so the grant gates every Twitch bot identity uniformly.
	if platform == "twitch" && !twitchBotAllowed {
		return nil, true
	}

	if user.IsPremium && h.botRepo != nil {
		if acct, err := h.botRepo.GetChatBotAccount(ctx, user.ID, platform); err == nil {
			return &botSenderDecision{sender: botSenderCustomBot, acct: acct}, false
		}
	}

	if h.botRepo != nil {
		if acct, err := h.botRepo.GetChatBotAccount(ctx, "", platform); err == nil {
			return &botSenderDecision{sender: botSenderAllChatBot, acct: acct}, false
		}
	}
	return nil, true
}

// --- Per-platform bot senders ---

// twitchAppTokenCache holds the shared Twitch app access token (client-credentials
// grant). The chat/messages endpoint accepts it for bot senders; it is cached until
// shortly before expiry so a burst of command replies never re-mints tokens.
type twitchAppTokenCache struct {
	mu     sync.Mutex
	client *http.Client

	clientID     string
	clientSecret string
	accessToken  string
	expiresAt    time.Time
}

// twitchTokenURL is Twitch's OAuth2 token endpoint (client-credentials grant).
const twitchTokenURL = "https://id.twitch.tv/oauth2/token"

// twitchAppToken returns a cached valid app access token, fetching a new one when
// the cached token expires within 2 minutes (a chat send in flight must never
// present a token that dies mid-request).
func (t *twitchAppTokenCache) token(ctx context.Context) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.accessToken != "" && time.Now().Before(t.expiresAt.Add(-2*time.Minute)) {
		return t.accessToken, nil
	}
	if t.clientID == "" || t.clientSecret == "" {
		return "", fmt.Errorf("twitch client credentials not configured")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		twitchTokenURL+"?client_id="+t.clientID+"&client_secret="+t.clientSecret+"&grant_type=client_credentials", nil)
	if err != nil {
		return "", fmt.Errorf("failed to create token request: %w", err)
	}
	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to request app access token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", fmt.Errorf("app access token request failed: status=%d body=%s", resp.StatusCode, string(body))
	}
	var parsed struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", fmt.Errorf("failed to parse app access token response: %w", err)
	}
	if parsed.AccessToken == "" {
		return "", fmt.Errorf("app access token response carried no token")
	}
	t.accessToken = parsed.AccessToken
	t.expiresAt = time.Now().Add(time.Duration(parsed.ExpiresIn) * time.Second)
	if t.expiresAt.IsZero() {
		return "", fmt.Errorf("app access token response carried no expiry")
	}
	return t.accessToken, nil
}

// sendBotTwitchMessage sends to Twitch chat as a bot account. Helix chat/messages
// here is authenticated with the APP access token (client credentials of our app)
// and carries sender_id = the bot's user id, per the platform mechanic the bot
// consent flow opted into with user:bot + user:write:chat.
func (h *ChatSendHandler) sendBotTwitchMessage(ctx context.Context, acct *repository.ChatBotAccount, user *models.User, message string) error {
	if user.TwitchID == nil || *user.TwitchID == "" {
		return fmt.Errorf("streamer has no Twitch account linked")
	}
	if acct == nil {
		return fmt.Errorf("no Twitch bot account configured")
	}
	if h.twitchAppTokens == nil {
		return fmt.Errorf("twitch app access token cache not configured")
	}
	appToken, err := h.twitchAppTokens.token(ctx)
	if err != nil {
		return fmt.Errorf("failed to resolve twitch app access token: %w", err)
	}

	reqBody := map[string]string{
		"broadcaster_id": *user.TwitchID, // the streamer's chat we speak into
		"sender_id":      acct.PlatformUserID,
		"message":        message,
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.twitch.tv/helix/chat/messages", strings.NewReader(string(bodyBytes)))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+appToken)
	req.Header.Set("Client-Id", h.clientID)
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		// A bot identity that lost its ability to speak surfaces the platform's
		// classification; engagement renders the error_kind per command.
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return classifyPlatformStatus("twitch", resp.StatusCode, string(body))
	}
	return nil
}

// sendAllChatBotKickMessage sends to Kick chat as the All-Chat Bot: the message
// goes out under the STREAMER's own user token but with type=bot, so Kick
// attributes it to the bot built into our Kick app rather than to the streamer.
// No chat_bot_accounts row is consulted — there is none by design.
func (h *ChatSendHandler) sendAllChatBotKickMessage(ctx context.Context, user *models.User, message string) error {
	accessToken, _, err := h.resolveKickSender(ctx, user)
	if err != nil {
		return err
	}

	reqBody := map[string]interface{}{
		"type":    "bot",
		"content": message,
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %w", err)
	}
	return h.postKickChat(ctx, accessToken, bodyBytes)
}

// sendCustomBotKickMessage sends to Kick chat as the streamer's CUSTOM bot: the
// bot account's own user token (type=user) speaks into the streamer's channel.
func (h *ChatSendHandler) sendCustomBotKickMessage(ctx context.Context, acct *repository.ChatBotAccount, user *models.User, message string) error {
	if acct == nil || acct.AccessToken == "" {
		return fmt.Errorf("no Kick bot token available")
	}
	_, broadcasterID, err := h.resolveKickSender(ctx, user)
	if err != nil {
		return err
	}

	reqBody := map[string]interface{}{
		"type":                "user",
		"content":             message,
		"broadcaster_user_id": broadcasterID,
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %w", err)
	}
	return h.postKickChat(ctx, acct.AccessToken, bodyBytes)
}

// postKickChat performs the POST /public/v1/chat call shared by the two Kick
// bot flavours; only the body and the presenting token differ.
func (h *ChatSendHandler) postKickChat(ctx context.Context, accessToken string, body []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.kick.com/public/v1/chat", strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return classifyPlatformStatus("kick", resp.StatusCode, string(body))
	}
	return nil
}

// sendBotYouTubeMessage inserts into the streamer's live chat as a bot account,
// using the bot's OWN OAuth token. Quota accounting matches the streamer path
// (a send costs 5 units regardless of whose token presents it). For
// purpose=timer the liveChatId comes ONLY from the Redis stream state
// (youtube:stream:state:{channel}) — the search/videos API fallback costs 100+
// quota units and would burn real budget on offline channels every timer tick;
// an offline channel is skipped with stream_offline instead.
func (h *ChatSendHandler) sendBotYouTubeMessage(ctx context.Context, acct *repository.ChatBotAccount, user *models.User, message string, purpose BotSendPurpose) error {
	if acct == nil {
		return fmt.Errorf("no YouTube bot account configured")
	}
	// The active channel comes from the DB exactly as in the streamer send path;
	// Tests without a pool pin it on the handler; a real streamer without a
	// connected YouTube channel (empty channel result) is handled upstream by
	// the caller's HasIdentity check, so an empty channel here (nil DB) just
	// means "no state" and the Redis lookup below misses.
	var channelID string
	if h.db != nil {
		var err error
		channelID, err = h.getActiveYouTubeChannelID(ctx, user.ID)
		if err != nil {
			return fmt.Errorf("failed to get YouTube channel ID: %w", err)
		}
	} else if h.ytBotChatChannel != "" {
		// Test wiring: the channel id the real Redis state is keyed on.
		channelID = h.ytBotChatChannel
	}

	accessToken := acct.AccessToken
	if !acct.TokenExpiresAt.IsZero() && time.Until(*acct.TokenExpiresAt) < 5*time.Minute {
		// Reuse the Google refresh code path the streamer flow uses: the bot row's
		// refresh token is rotated back into chat_bot_accounts so the next send
		// does not race another replica's refresh into an invalidated token.
		newToken, rerr := h.youtubeProvider.RefreshToken(ctx, acct.RefreshToken)
		if rerr != nil || newToken == nil || newToken.AccessToken == "" {
			h.log.Warn("bot-send: youtube bot token refresh failed",
				zap.String("platform", "youtube"), zap.Error(rerr))
			return &streamerSendError{kind: sendErrReauth, msg: "youtube bot token refresh failed"}
		}
		refreshToken := newToken.RefreshToken
		if refreshToken == "" {
			refreshToken = acct.RefreshToken
		}
		if storeErr := h.storeBotRefreshedToken(ctx, user.ID, "youtube", acct.Login,
			newToken.AccessToken, refreshToken, newToken.Expiry); storeErr != nil {
			h.log.Warn("bot-send: failed to persist refreshed youtube bot token", zap.Error(storeErr))
		}
		accessToken = newToken.AccessToken
	}

	var liveChatID string
	var err error
	if h.ytBotChat != nil {
		// Test seam: a substituted resolver owns the resolution, but production's
		// contract still applies — an empty liveChatId means the stream is not
		// live, which stays stream_offline regardless of who resolved it.
		liveChatID, err = h.ytBotChat(ctx, channelID, purpose)
		if err != nil {
			return err
		}
		if liveChatID == "" {
			return &streamerSendError{kind: sendErrOffline, msg: "streamer is not currently live on YouTube"}
		}
	} else if purpose == BotSendPurposeTimer {
		// Timer sends resolve the liveChatId ONLY from the Redis stream state —
		// the search/videos API fallback costs 100+ quota units and would burn
		// real budget on offline channels every tick.
		liveChatID, err = h.getYouTubeLiveChatIDFromRedis(ctx, channelID)
		if err != nil {
			return fmt.Errorf("failed to read stream state: %w", err)
		}
		if liveChatID == "" {
			return &streamerSendError{kind: sendErrOffline, msg: "streamer is not currently live on YouTube"}
		}
	} else {
		liveChatID, err = h.getYouTubeLiveChatID(ctx, channelID)
		if err != nil {
			return fmt.Errorf("failed to get live chat ID: %w", err)
		}
	}

	ok := h.reserveYouTubeSendQuota(ctx, quota.QuotaCostYouTubeSend)
	if !ok {
		return &streamerSendError{kind: sendErrQuota, msg: "youtube send blocked: daily quota exhausted"}
	}

	reqBody := map[string]interface{}{
		"snippet": map[string]interface{}{
			"liveChatId": liveChatID,
			"type":       "textMessageEvent",
			"textMessageDetails": map[string]string{
				"messageText": message,
			},
		},
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		h.settleYouTubeSendQuota(ctx, quota.QuotaCostYouTubeSend, false)
		return fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://www.googleapis.com/youtube/v3/liveChat/messages?part=snippet",
		strings.NewReader(string(bodyBytes)))
	if err != nil {
		h.settleYouTubeSendQuota(ctx, quota.QuotaCostYouTubeSend, false)
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		h.settleYouTubeSendQuota(ctx, quota.QuotaCostYouTubeSend, false)
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		h.settleYouTubeSendQuota(ctx, quota.QuotaCostYouTubeSend, false)
		return classifyPlatformStatus("youtube", resp.StatusCode, string(body))
	}
	h.settleYouTubeSendQuota(ctx, quota.QuotaCostYouTubeSend, true)
	return nil
}

// storeBotRefreshedToken writes a freshly rotated bot token back into
// chat_bot_accounts. Best-effort from the caller's view: a write failure costs
// one extra refresh next send, not the send itself.
func (h *ChatSendHandler) storeBotRefreshedToken(
	ctx context.Context, ownerUserID, platform, login string,
	accessToken, refreshToken string, expiresAt time.Time,
) error {
	if h.botRepo == nil || accessToken == "" {
		return nil
	}
	return h.botRepo.StoreRefreshedBotToken(ctx, ownerUserID, platform, login, accessToken, refreshToken, expiresAt)
}
