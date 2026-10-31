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
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/caesar/all-chat/services/auth-service/models"
	"github.com/caesar/all-chat/services/auth-service/oauth"
	"github.com/caesar/all-chat/services/auth-service/repository"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

type recordingTransport struct {
	prefixes   []string
	responses  []string          // recorded URLs, in order
	bodies     map[string]string // URL -> request body
	headers    map[string]string // "URL|Header" -> value
	unexpected []string          // URLs with no matching prefix
	// tokenBody is served as the response body of an id.twitch.tv token
	// request. Empty means a plain {} (no token), so only the token-cache test
	// sets it.
	tokenBody string
}

type chatCall struct {
	url           string
	body          string
	authorization string
	clientID      string
}

func (t *recordingTransport) chatCalls() []chatCall {
	out := make([]chatCall, 0, len(t.responses))
	for _, url := range t.responses {
		out = append(out, chatCall{
			url:           url,
			body:          t.bodies[url],
			authorization: t.headers[url+"|Authorization"],
			clientID:      t.headers[url+"|Client-Id"],
		})
	}
	return out
}

func (t *recordingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	body := ""
	if r.Body != nil {
		b, err := io.ReadAll(r.Body)
		if err != nil {
			return nil, err
		}
		body = string(b)
	}
	for _, prefix := range t.prefixes {
		if strings.HasPrefix(r.URL.String(), prefix) {
			if t.bodies == nil {
				t.bodies = map[string]string{}
				t.headers = map[string]string{}
			}
			t.responses = append(t.responses, r.URL.String())
			t.bodies[r.URL.String()] = body
			t.headers[r.URL.String()+"|Authorization"] = r.Header.Get("Authorization")
			t.headers[r.URL.String()+"|Client-Id"] = r.Header.Get("Client-Id")
			respBody := "{}"
			if strings.Contains(r.URL.String(), "id.twitch.tv/oauth2/token") && t.tokenBody != "" {
				respBody = t.tokenBody
			}
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(respBody)), Header: http.Header{}}, nil
		}
	}
	t.unexpected = append(t.unexpected, r.URL.String())
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("{}")), Header: http.Header{}}, nil
}

// stubBotStore is the minimum chatBotAccountStore for resolution tests.
type stubBotStore struct {
	settings  string // sender setting returned for any user
	global    map[string]*repository.ChatBotAccount
	custom    map[string]*repository.ChatBotAccount // keyed owner|platform
	refreshed *storeRefreshCall
}

type storeRefreshCall struct {
	ownerUserID, platform, login, accessToken, refreshToken string
	expiresAt                                               time.Time
}

func (s *stubBotStore) GetChatBotSenderSetting(ctx context.Context, userID string) (string, error) {
	return s.settings, nil
}

func (s *stubBotStore) GetChatBotAccount(ctx context.Context, ownerUserID, platform string) (*repository.ChatBotAccount, error) {
	if ownerUserID == "" {
		if acct, ok := s.global[platform]; ok {
			return acct, nil
		}
	} else if acct, ok := s.custom[ownerUserID+"|"+platform]; ok {
		return acct, nil
	}
	return nil, repository.ErrChatBotNotFound
}

func (s *stubBotStore) StoreRefreshedBotToken(ctx context.Context, ownerUserID, platform, login, accessToken, refreshToken string, expiresAt time.Time) error {
	s.refreshed = &storeRefreshCall{ownerUserID, platform, login, accessToken, refreshToken, expiresAt}
	return nil
}

// newBotTestHandler wires a handler with mocks for the full
// HandleInternalBotSend flow (twitch/kick only — YouTube identity is checked
// against overlay tables, outside the fake's reach).
func newBotTestHandler(t *testing.T, store *stubBotStore, streamer *models.User) (*gin.Engine, *ChatSendHandler) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	userRepo := new(MockUserRepository)
	userRepo.On("GetByID", mock.Anything, streamer.ID).Return(streamer, nil)
	userRepo.On("GetPlatformGrantedScopes", mock.Anything, streamer.ID, "twitch").
		Maybe().Return([]string{"user:read:chat", oauth.TwitchChatScopes[1], "channel:bot"}, nil)

	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	transport := &recordingTransport{}

	handler := &ChatSendHandler{
		log:             zap.NewNop(),
		viewerRepo:      nil, // not touched by bot-send
		userRepo:        userRepo,
		httpClient:      &http.Client{Transport: transport},
		clientID:        "test-client-id",
		redisClient:     rc,
		youtubeProvider: new(MockOAuthProvider),
	}
	handler.WithChatBotStore(store, "test-client-secret")
	return gin.New(), handler
}

func botSendRequest(t *testing.T, h *ChatSendHandler, streamer *models.User, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := gin.New()
	r.POST("/internal/chat/bot-send", h.HandleInternalBotSend)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/internal/chat/bot-send", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func streamerFixture(userID string) *models.User {
	twitchID, kickID := "Tstreamer", "123456"
	return &models.User{
		ID:             userID,
		Username:       "streamer",
		TwitchID:       &twitchID,
		KickID:         &kickID,
		AuthProvider:   "twitch",
		AccessToken:    "streamer-token",
		TokenExpiresAt: time.Now().Add(1 * time.Hour),
	}
}

// botYtChannel is the streamer's YouTube channel id the send resolution keys on
// in the three YouTube tests below.
const botYtChannel = "UCstreaming"

// The botSenderFor table: which identity speaks per platform, one test per row.
// The fallback-to-self branch lives in the caller (HandleInternalBotSend) and is
// exercised in the end-to-end suite through botSendRequest.

func TestBotSenderFor_SelfSettingResolution(t *testing.T) {
	user := streamerFixture("u1")
	store := &stubBotStore{}

	// Kick is always the All-Chat Bot carrier.
	decision, fallback := (&ChatSendHandler{botRepo: store}).botSenderFor(context.Background(), user, "kick", true)
	assert.False(t, fallback)
	assert.Equal(t, botSenderAllChatBot, decision.sender)

	// A missing global row on Twitch falls back to self.
	_, fallback = (&ChatSendHandler{botRepo: store}).botSenderFor(context.Background(), user, "twitch", true)
	assert.True(t, fallback, "no global Twitch bot row must fall back to self")
}

// A premium owner's custom row outranks the global All-Chat Bot row.
func TestBotSenderFor_PremiumCustomRowWins(t *testing.T) {
	user := streamerFixture("u1")
	user.IsPremium = true
	store := &stubBotStore{
		custom: map[string]*repository.ChatBotAccount{
			"u1|twitch": {Platform: "twitch", PlatformUserID: "botuid", Login: "mybot"},
		},
	}
	decision, fallback := (&ChatSendHandler{botRepo: store}).botSenderFor(context.Background(), user, "twitch", true)
	assert.False(t, fallback)
	assert.Equal(t, botSenderCustomBot, decision.sender)
	require.NotNil(t, decision.acct)
	assert.Equal(t, "mybot", decision.acct.Login)
}

func TestBotSenderFor_NonPremiumIgnoresCustomRow(t *testing.T) {
	// A custom row must never speak for a non-premium owner: resolution falls
	// through to the All-Chat Bot (or self when that row is missing too).
	user := streamerFixture("u1")
	store := &stubBotStore{
		custom: map[string]*repository.ChatBotAccount{
			"u1|twitch": {Platform: "twitch", PlatformUserID: "botuid", Login: "mybot"},
		},
	}
	_, fallback := (&ChatSendHandler{botRepo: store}).botSenderFor(context.Background(), user, "twitch", true)
	assert.True(t, fallback, "non-premium owner must not reach their custom row")
}

func TestBotSenderFor_TwitchWithoutChannelBotGrantFallsBack(t *testing.T) {
	// even a perfectly connected global bot cannot speak without the
	// broadcaster's channel:bot grant.
	user := streamerFixture("u1")
	store := &stubBotStore{
		global: map[string]*repository.ChatBotAccount{
			"twitch": {Platform: "twitch", PlatformUserID: "botuid"},
		},
	}
	_, fallback := (&ChatSendHandler{botRepo: store}).botSenderFor(context.Background(), user, "twitch", false)
	assert.True(t, fallback, "missing channel:bot grant must fall back to self")
}

func TestBotSenderFor_GlobalRowPresent(t *testing.T) {
	user := streamerFixture("u1")
	store := &stubBotStore{
		global: map[string]*repository.ChatBotAccount{
			"twitch": {Platform: "twitch", PlatformUserID: "botuid"},
		},
	}
	decision, fallback := (&ChatSendHandler{botRepo: store}).botSenderFor(context.Background(), user, "twitch", true)
	assert.False(t, fallback)
	assert.Equal(t, botSenderAllChatBot, decision.sender)
}

// --- Per-platform request bodies ---

// The Twitch bot send must present the APP access token over its own
// client-credentials round trip (id.twitch.tv), never the streamer's or the bot
// account's user token — Helix chat/messages keys the bot identity on sender_id,
// not on the presenting token. The send body pairs the streamer's channel
// (broadcaster_id) with the bot row's account (sender_id).
func TestSendBotTwitchMessage_BodyAndHeaders(t *testing.T) {
	transport := &recordingTransport{
		prefixes:  []string{"https://api.twitch.tv/", "https://id.twitch.tv/"},
		tokenBody: `{"access_token":"app-token","expires_in":3600}`,
	}
	user := streamerFixture("u1")
	h := &ChatSendHandler{
		log:        zap.NewNop(),
		httpClient: &http.Client{Transport: transport},
		clientID:   "test-client-id",
		twitchAppTokens: &twitchAppTokenCache{
			client:       &http.Client{Transport: transport},
			clientID:     "test-client-id",
			clientSecret: "test-client-secret",
		},
	}
	acct := &repository.ChatBotAccount{Platform: "twitch", PlatformUserID: "botuid"}

	require.NoError(t, h.sendBotTwitchMessage(context.Background(), acct, user, "hi"))

	calls := transport.chatCalls()
	require.Len(t, calls, 2, "one client-credentials token fetch, one chat send")
	tokenCall, sendCall := calls[0], calls[1]
	assert.True(t, strings.HasPrefix(tokenCall.url, "https://id.twitch.tv/oauth2/token"),
		"the app token is fetched via client credentials: %s", tokenCall.url)
	assert.Contains(t, tokenCall.url, "grant_type=client_credentials")
	assert.Contains(t, tokenCall.url, "client_id=test-client-id")
	assert.Contains(t, tokenCall.url, "client_secret=test-client-secret")
	assert.Equal(t, `{"broadcaster_id":"Tstreamer","message":"hi","sender_id":"botuid"}`, sendCall.body)
	assert.Equal(t, "Bearer app-token", sendCall.authorization,
		"the send must present the app access token, not the streamer or bot user token")
	assert.Equal(t, "test-client-id", sendCall.clientID)
	assert.True(t, strings.HasPrefix(sendCall.url, "https://api.twitch.tv/helix/chat/messages"),
		"the send goes to Helix chat/messages: %s", sendCall.url)
}

// kickLoginStreamerFixture is a streamer who signed in with Kick, so the users
// row itself carries their Kick token and id.
func kickLoginStreamerFixture(userID string) *models.User {
	user := streamerFixture(userID)
	user.AuthProvider = "kick"
	user.TwitchID = nil
	return user
}

// linkedKickSource is a Kick credential linked to a Twitch-login account
// (kick_oauth_tokens, #1111): users.kick_id is empty and users.access_token is
// a Twitch token.
func linkedKickSource() *fakeKickTokenSource {
	return &fakeKickTokenSource{cred: &repository.LinkedKickCredential{
		ID: 7, KickUserID: "130554250", AccessToken: "kick-linked-token", RefreshToken: "r",
		Expiry: time.Now().Add(time.Hour),
	}}
}

func TestSendAllChatBotKickMessage_TypeBotWithStreamerToken(t *testing.T) {
	// Kick's All-Chat Bot: no DB row, type=bot, and the token that presents is
	// the STREAMER's (the bot identity comes from the Kick app the streamer's
	// token belongs to).
	transport := &recordingTransport{prefixes: []string{"https://api.kick.com/"}}
	user := kickLoginStreamerFixture("u1")
	h := &ChatSendHandler{log: zap.NewNop(), httpClient: &http.Client{Transport: transport}}

	require.NoError(t, h.sendAllChatBotKickMessage(context.Background(), user, "hello"))

	calls := transport.chatCalls()
	require.Len(t, calls, 1)
	assert.Equal(t, `{"content":"hello","type":"bot"}`, calls[0].body)
	assert.Equal(t, "Bearer streamer-token", calls[0].authorization, "the All-Chat Kick bot travels under the streamer's own user token")
}

func TestSendAllChatBotKickMessage_LinkedKickUsesLinkedCredential(t *testing.T) {
	// A Twitch-login streamer's users row holds a Twitch token, which Kick
	// rejects; the streamer's Kick token is the linked credential.
	transport := &recordingTransport{prefixes: []string{"https://api.kick.com/"}}
	user := streamerFixture("u1")
	user.KickID = nil
	h := &ChatSendHandler{log: zap.NewNop(), httpClient: &http.Client{Transport: transport}, kickTokenSource: linkedKickSource()}

	require.NoError(t, h.sendAllChatBotKickMessage(context.Background(), user, "hello"))

	calls := transport.chatCalls()
	require.Len(t, calls, 1)
	assert.Equal(t, `{"content":"hello","type":"bot"}`, calls[0].body)
	assert.Equal(t, "Bearer kick-linked-token", calls[0].authorization, "must never send the users-row (Twitch) token to Kick")
}

func TestSendCustomBotKickMessage_TypeUserWithBotToken(t *testing.T) {
	// A custom Kick bot speaks with its OWN user token (type=user), addressing
	// the streamer's numeric Kick id — the inverse of the All-Chat flavour above.
	transport := &recordingTransport{prefixes: []string{"https://api.kick.com/"}}
	user := kickLoginStreamerFixture("u1")
	h := &ChatSendHandler{log: zap.NewNop(), httpClient: &http.Client{Transport: transport}}
	acct := &repository.ChatBotAccount{Platform: "kick", PlatformUserID: "654321", AccessToken: "bot-token"}

	require.NoError(t, h.sendCustomBotKickMessage(context.Background(), acct, user, "hello"))

	calls := transport.chatCalls()
	require.Len(t, calls, 1)
	assert.Equal(t, `{"broadcaster_user_id":"123456","content":"hello","type":"user"}`, calls[0].body)
	assert.Equal(t, "Bearer bot-token", calls[0].authorization, "a custom Kick bot speaks with its OWN user token")
}

func TestSendCustomBotKickMessage_LinkedKickAddressesLinkedChannel(t *testing.T) {
	transport := &recordingTransport{prefixes: []string{"https://api.kick.com/"}}
	user := streamerFixture("u1")
	user.KickID = nil
	h := &ChatSendHandler{log: zap.NewNop(), httpClient: &http.Client{Transport: transport}, kickTokenSource: linkedKickSource()}
	acct := &repository.ChatBotAccount{Platform: "kick", PlatformUserID: "654321", AccessToken: "bot-token"}

	require.NoError(t, h.sendCustomBotKickMessage(context.Background(), acct, user, "hello"))

	calls := transport.chatCalls()
	require.Len(t, calls, 1)
	assert.Equal(t, `{"broadcaster_user_id":"130554250","content":"hello","type":"user"}`, calls[0].body)
	assert.Equal(t, "Bearer bot-token", calls[0].authorization)
}

// botSendResults posts a bot-send for platforms and returns the per-platform
// results plus the Kick calls that went out.
func botSendResults(t *testing.T, streamer *models.User, kickSource KickTokenSource, platforms string) ([]BotSendResult, []chatCall) {
	t.Helper()
	_, h := newBotTestHandler(t, &stubBotStore{settings: "bot"}, streamer)
	h.kickTokenSource = kickSource
	transport := h.httpClient.Transport.(*recordingTransport)
	transport.prefixes = []string{"https://api.kick.com/"}

	w := botSendRequest(t, h, streamer, `{"user_id":"`+streamer.ID+`","message":"hello","platforms":`+platforms+`}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var resp struct {
		Results []BotSendResult `json:"results"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	return resp.Results, transport.chatCalls()
}

func TestHandleInternalBotSend_LinkedKickStreamerGetsKickReply(t *testing.T) {
	// A command typed in Kick chat by a Twitch-login streamer's viewer must be
	// answered there: their Kick identity is the linked credential, not users.kick_id.
	streamer := streamerFixture("u1")
	streamer.KickID = nil

	results, calls := botSendResults(t, streamer, linkedKickSource(), `["kick"]`)

	require.Len(t, results, 1)
	assert.Equal(t, BotSendResult{Platform: "kick", Success: true, Sender: botSenderAllChatBot}, results[0])
	require.Len(t, calls, 1)
	assert.Equal(t, "Bearer kick-linked-token", calls[0].authorization)
}

func TestHandleInternalBotSend_NoKickIdentitySkipsKick(t *testing.T) {
	// A timer asks for twitch and kick; a streamer without Kick sending enabled
	// must not get a failed Kick result every tick.
	streamer := streamerFixture("u1")
	streamer.KickID = nil

	results, calls := botSendResults(t, streamer, &fakeKickTokenSource{}, `["kick"]`)

	assert.Empty(t, results)
	assert.Empty(t, calls)
}

func TestSendBotYouTubeMessage_TimerUsesOnlyRedisStreamState(t *testing.T) {
	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	// Cached streamstate: the same (key, shape) the youtube-listener's
	// StreamStateStore.SaveStreamState writes while live.
	require.NoError(t, mr.Set("youtube:stream:state:"+botYtChannel,
		`{"live_chat_id":"chatid","is_live":true}`))

	transport := &recordingTransport{prefixes: []string{"https://www.googleapis.com/"}}
	user := streamerFixture("u1")
	h := &ChatSendHandler{
		log:              zap.NewNop(),
		httpClient:       &http.Client{Transport: transport},
		redisClient:      rc,
		youtubeProvider:  new(MockOAuthProvider),
		ytBotChatChannel: botYtChannel,
	}
	acct := &repository.ChatBotAccount{
		Platform:    "youtube",
		AccessToken: "bot-token",
		// Long-lived expiry so no refresh fires under the fake.
		TokenExpiresAt: timePtr(time.Now().Add(1 * time.Hour)),
	}

	// The test seam stands in for the liveChatId resolution: production's timer
	// branch keys on the streamer's channel and consults the Redis stream state;
	// the search/videos API fallback of the streamer path never runs for bot
	// timer sends. The seam defers to h's REAL Redis client, so this exercises
	// what production runs. require.* inside it would bloat the test with a full
	// stack trace on failure, so a wrong channel/purpose surfaces via assert
	// t.Errorf below.
	h.ytBotChat = func(ctx context.Context, channelID string, purpose BotSendPurpose) (string, error) {
		assert.Equal(t, botYtChannel, channelID, "resolution keys on the streamer's channel")
		assert.Equal(t, BotSendPurposeTimer, purpose)
		if channelID != botYtChannel {
			return "", nil
		}
		return h.getYouTubeLiveChatIDFromRedis(ctx, channelID)
	}

	require.NoError(t, h.sendBotYouTubeMessage(context.Background(), acct, user, "timer msg", BotSendPurposeTimer))
	calls := transport.chatCalls()
	require.Len(t, calls, 1)
	assert.Equal(t, "Bearer bot-token", calls[0].authorization)
	var snippet struct {
		Snippet struct {
			LiveChatID string `json:"liveChatId"`
		} `json:"snippet"`
	}
	require.NoError(t, json.Unmarshal([]byte(calls[0].body), &snippet))
	assert.Equal(t, "chatid", snippet.Snippet.LiveChatID)
}

func TestSendBotYouTubeMessage_TimerOfflineStaysRedisOnly(t *testing.T) {
	// The same seam, but the stream is offline (empty Redis). The resolution
	// returns the empty liveChatId production maps to stream_offline, and the
	// transport records NOTHING: a timer send on an offline channel must not
	// burn quota on a search.list or videos.list fallback.
	user := streamerFixture("u1")
	transport := &recordingTransport{prefixes: []string{"https://www.googleapis.com/"}}
	h := &ChatSendHandler{
		log:              zap.NewNop(),
		httpClient:       &http.Client{Transport: transport},
		redisClient:      redis.NewClient(&redis.Options{Addr: miniredis.RunT(t).Addr()}),
		youtubeProvider:  new(MockOAuthProvider),
		ytBotChatChannel: botYtChannel,
	}
	acct := &repository.ChatBotAccount{
		Platform:       "youtube",
		AccessToken:    "bot-token",
		TokenExpiresAt: timePtr(time.Now().Add(1 * time.Hour)),
	}
	h.ytBotChat = func(ctx context.Context, channelID string, purpose BotSendPurpose) (string, error) {
		assert.Equal(t, botYtChannel, channelID)
		return h.getYouTubeLiveChatIDFromRedis(ctx, channelID)
	}

	err := h.sendBotYouTubeMessage(context.Background(), acct, user, "timer msg", BotSendPurposeTimer)
	require.Error(t, err)
	se, ok := err.(*streamerSendError)
	require.True(t, ok, "an offline channel is a typed stream_offline failure, not a raw upstream one: %+v", err)
	assert.Equal(t, sendErrOffline, se.kind)

	assert.Empty(t, transport.responses, "a timer send on an offline channel must make no network call at all")
}

func TestRefreshBotTokenStoreCall_YouTubeRefresh(t *testing.T) {
	// Near expiry (5-minute lead) the YouTube bot refresh path rotates the token
	// back into chat_bot_accounts (storeBotRefreshedToken) and presents the NEW
	// token on the send itself: parallel replicas must never race a single-use
	// Google refresh token into an invalidated grant.
	refresh := new(MockRefreshProvider)
	refresh.On("RefreshToken", mock.Anything, "old-refresh").Return(&oauth2.Token{
		AccessToken:  "new-token",
		RefreshToken: "new-refresh",
		Expiry:       time.Now().Add(1 * time.Hour),
	}, nil)

	transport := &recordingTransport{prefixes: []string{"https://www.googleapis.com/"}}
	user := streamerFixture("u1")
	store := &stubBotStore{}
	h := &ChatSendHandler{
		log:              zap.NewNop(),
		httpClient:       &http.Client{Transport: transport},
		redisClient:      redis.NewClient(&redis.Options{Addr: miniredis.RunT(t).Addr()}),
		youtubeProvider:  refresh,
		botRepo:          store,
		ytBotChatChannel: botYtChannel,
	}
	acct := &repository.ChatBotAccount{
		Platform:       "youtube",
		Login:          "mybot",
		AccessToken:    "stale-token",
		RefreshToken:   "old-refresh",
		TokenExpiresAt: timePtr(time.Now().Add(1 * time.Minute)), // inside the 5-minute refresh lead
	}
	// The seam defers to the real Redis path (no state → stream_offline in the
	// offline case), so only the token rotation above is exercised here. A
	// command purpose in production also passes the seam; the contract is the
	// same: resolve, then send with the fresh token.
	h.ytBotChat = func(ctx context.Context, channelID string, purpose BotSendPurpose) (string, error) {
		assert.Equal(t, botYtChannel, channelID)
		// No stream state under the fake — the send must still go through with
		// the freshly rotated token (the platform leg, not the resolution).
		return "LC_fake", nil
	}

	require.NoError(t, h.sendBotYouTubeMessage(context.Background(), acct, user, "timer msg", BotSendPurposeTimer))

	require.NotNil(t, store.refreshed)
	assert.Equal(t, "youtube", store.refreshed.platform)
	assert.Equal(t, "new-token", store.refreshed.accessToken)
	assert.Equal(t, "new-refresh", store.refreshed.refreshToken)
	calls := transport.chatCalls()
	require.Len(t, calls, 1)
	assert.Equal(t, "Bearer new-token", calls[0].authorization, "the send must present the freshly rotated token")
}

type MockRefreshProvider struct{ mock.Mock }

func (m *MockRefreshProvider) RefreshToken(ctx context.Context, refreshToken string) (*oauth2.Token, error) {
	args := m.Called(ctx, refreshToken)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*oauth2.Token), args.Error(1)
}
