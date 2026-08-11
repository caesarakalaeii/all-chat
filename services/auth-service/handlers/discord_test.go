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
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/caesar/all-chat/services/auth-service/models"
	"github.com/caesar/all-chat/services/auth-service/oauth"
	"github.com/caesar/all-chat/services/auth-service/repository"
	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
)

// --- Mock implementations ---

type mockDiscordOAuth struct {
	authURL          string
	exchangeToken    *oauth2.Token
	exchangeErr      error
	missingPerms     []string
	checkPermsErr    error
	checkPermsCalled bool
	// Account-link flow (ADR-0048).
	identity     *oauth.DiscordIdentity
	identityErr  error
	identityAuth string
}

func (m *mockDiscordOAuth) GetAuthURL(state string) string {
	if m.authURL != "" {
		return m.authURL
	}
	return "https://discord.com/oauth2/authorize?client_id=test&scope=bot&permissions=68608&state=" + state
}

func (m *mockDiscordOAuth) GetModerationAuthURL(state string) string {
	return "https://discord.com/oauth2/authorize?client_id=test&scope=bot&permissions=1099511704580&state=" + state
}

func (m *mockDiscordOAuth) ExchangeCode(_ context.Context, _ string) (*oauth2.Token, error) {
	return m.exchangeToken, m.exchangeErr
}

func (m *mockDiscordOAuth) CheckBotPermissions(_ context.Context, _ string) ([]string, error) {
	m.checkPermsCalled = true
	return m.missingPerms, m.checkPermsErr
}

func (m *mockDiscordOAuth) GetGuildInfo(_ context.Context, guildID string) (*oauth.GuildInfo, error) {
	return &oauth.GuildInfo{Name: "Guild " + guildID, Icon: ""}, nil
}

func (m *mockDiscordOAuth) GetIdentityAuthURL(state string) string {
	if m.identityAuth != "" {
		return m.identityAuth
	}
	return "https://discord.com/oauth2/authorize?client_id=test&scope=identify&state=" + state
}

func (m *mockDiscordOAuth) GetIdentity(_ context.Context, _ string) (*oauth.DiscordIdentity, error) {
	if m.identityErr != nil {
		return nil, m.identityErr
	}
	if m.identity != nil {
		return m.identity, nil
	}
	return &oauth.DiscordIdentity{ID: "198569499228766208", Username: "volunteer"}, nil
}

type mockDiscordRepo struct {
	upsertCalled        bool
	upsertErr           error
	deleteCalled        bool
	deleteErr           error
	deleteSourcesCalled bool
	deleteSourcesErr    error
	listGuilds          []*models.DiscordGuild
	listErr             error
	getGuild            *models.DiscordGuild
	getErr              error
	// Account-link flow (ADR-0048).
	upsertIdentityCalled bool
	upsertIdentityUser   string
	upsertIdentityDiscID string
	upsertIdentityErr    error
	getIdentity          *models.DiscordIdentity
	getIdentityErr       error
	deleteIdentityCalled bool
	deleteIdentityErr    error
}

func (m *mockDiscordRepo) UpsertIdentity(_ context.Context, userID, discordUserID, _ string) error {
	m.upsertIdentityCalled = true
	m.upsertIdentityUser, m.upsertIdentityDiscID = userID, discordUserID
	return m.upsertIdentityErr
}

func (m *mockDiscordRepo) GetIdentity(_ context.Context, _ string) (*models.DiscordIdentity, error) {
	if m.getIdentityErr != nil {
		return nil, m.getIdentityErr
	}
	if m.getIdentity == nil {
		return nil, repository.ErrNotFound
	}
	return m.getIdentity, nil
}

func (m *mockDiscordRepo) DeleteIdentity(_ context.Context, _ string) error {
	m.deleteIdentityCalled = true
	return m.deleteIdentityErr
}

func (m *mockDiscordRepo) UpsertGuild(_ context.Context, _ *models.DiscordGuild) error {
	m.upsertCalled = true
	return m.upsertErr
}

func (m *mockDiscordRepo) DeleteGuild(_ context.Context, _, _ string) error {
	m.deleteCalled = true
	return m.deleteErr
}

func (m *mockDiscordRepo) ListGuildsByUser(_ context.Context, _ string) ([]*models.DiscordGuild, error) {
	return m.listGuilds, m.listErr
}

func (m *mockDiscordRepo) GetGuild(_ context.Context, _, _ string) (*models.DiscordGuild, error) {
	return m.getGuild, m.getErr
}

func (m *mockDiscordRepo) DeleteDiscordSourcesByGuildID(_ context.Context, _ string) error {
	m.deleteSourcesCalled = true
	return m.deleteSourcesErr
}

// --- Tests ---

// TestHandleDiscordConnect verifies that GET /discord/connect returns 200 with a bot_invite_url
// containing scope=bot when called with a valid user_id in the Gin context (simulating JWT middleware).
func TestHandleDiscordConnect(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockOAuth := &mockDiscordOAuth{}
	mockRepo := &mockDiscordRepo{}

	handler := newTestDiscordHandlerNoRedis(mockOAuth, mockRepo, "http://localhost:3000")

	router := gin.New()
	router.GET("/discord/connect", func(c *gin.Context) {
		c.Set("user_id", "user-123")
		handler.HandleConnect(c)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/discord/connect", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	inviteURL, ok := resp["bot_invite_url"]
	if !ok {
		t.Fatal("response missing bot_invite_url")
	}
	if inviteURL == "" {
		t.Fatal("bot_invite_url is empty")
	}
	// Must contain scope=bot
	if !containsString(inviteURL, "scope=bot") {
		t.Errorf("bot_invite_url %q does not contain scope=bot", inviteURL)
	}
}

// TestHandleDiscordConnect_ModerationReinvite verifies that ?moderation=true returns the
// elevated invite URL (the moderation permission bitfield), so the dashboard's re-invite
// CTA upgrades the bot's permissions in place (ADR-0017).
func TestHandleDiscordConnect_ModerationReinvite(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := newTestDiscordHandlerNoRedis(&mockDiscordOAuth{}, &mockDiscordRepo{}, "http://localhost:3000")

	router := gin.New()
	router.GET("/discord/connect", func(c *gin.Context) {
		c.Set("user_id", "user-123")
		handler.HandleConnect(c)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/discord/connect?moderation=true", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	// The moderation re-invite uses the elevated permission bitfield, not the base 68608.
	if !containsString(resp["bot_invite_url"], "permissions=1099511704580") {
		t.Errorf("moderation re-invite URL %q does not request the elevated permissions", resp["bot_invite_url"])
	}
}

// TestHandleDiscordConnect_MissingPermsNonFatal verifies that missing-permissions reports
// from CheckBotPermissions do NOT block the callback. Discord enforces permissions=68608 at
// invite time, so a successful code exchange is sufficient proof — the membership check is
// advisory only. Regression coverage for the non-fatal behavior introduced in f1befa7c.
func TestHandleDiscordConnect_MissingPermsNonFatal(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockOAuth := &mockDiscordOAuth{
		exchangeToken: &oauth2.Token{AccessToken: "tok", Expiry: time.Now().Add(time.Hour)},
		missingPerms:  []string{"View Channels"},
	}
	mockRepo := &mockDiscordRepo{}

	handler := newTestDiscordHandlerNoRedis(mockOAuth, mockRepo, "http://localhost:3000")
	handler.stateStore = &fakeStateStore{states: map[string]string{"validstate": "user-456"}}

	router := gin.New()
	router.GET("/discord/callback", handler.HandleCallback)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/discord/callback?state=validstate&code=authcode&guild_id=guild-789", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusFound && w.Code != http.StatusSeeOther && w.Code != http.StatusTemporaryRedirect {
		t.Errorf("expected redirect (3xx) — missing perms must be non-fatal, got %d: %s", w.Code, w.Body.String())
	}

	if !mockRepo.upsertCalled {
		t.Error("UpsertGuild must be called even when permissions check reports missing perms")
	}
}

// TestHandleGetGuilds verifies that GET /guilds returns a JSON array of the authenticated
// user's connected guilds.
func TestHandleGetGuilds(t *testing.T) {
	gin.SetMode(gin.TestMode)

	icon := "abc123"
	mockRepo := &mockDiscordRepo{
		listGuilds: []*models.DiscordGuild{
			{ID: "1", UserID: "user-1", GuildID: "guild-111", GuildName: "Server One", GuildIcon: &icon, ConnectedAt: time.Now()},
			{ID: "2", UserID: "user-1", GuildID: "guild-222", GuildName: "Server Two", GuildIcon: nil, ConnectedAt: time.Now()},
		},
	}

	handler := newTestDiscordHandlerNoRedis(&mockDiscordOAuth{}, mockRepo, "http://localhost:3000")

	router := gin.New()
	router.GET("/guilds", func(c *gin.Context) {
		c.Set("user_id", "user-1")
		handler.HandleGetGuilds(c)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/guilds", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var guilds []*models.DiscordGuild
	if err := json.Unmarshal(w.Body.Bytes(), &guilds); err != nil {
		t.Fatalf("failed to parse guilds response: %v", err)
	}
	if len(guilds) != 2 {
		t.Errorf("expected 2 guilds, got %d", len(guilds))
	}
}

// TestHandleGetGuildChannels verifies channel listing:
//   - Only type=0 (text) channels are returned (voice type=2 excluded)
//   - Channels are grouped by parent_id into categories
//   - Channels without a parent_id land in "Uncategorized"
func TestHandleGetGuildChannels(t *testing.T) {
	gin.SetMode(gin.TestMode)

	icon := "icon123"
	mockRepo := &mockDiscordRepo{
		getGuild: &models.DiscordGuild{
			ID:        "1",
			UserID:    "user-1",
			GuildID:   "guild-111",
			GuildName: "Test Server",
			GuildIcon: &icon,
		},
	}

	// Set up a fake Discord channels endpoint
	discordServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v10/guilds/guild-111/channels" {
			channels := []map[string]interface{}{
				{"id": "cat-1", "name": "TEXT CHANNELS", "type": 4, "position": 0},
				{"id": "ch-general", "name": "general", "type": 0, "position": 1, "parent_id": "cat-1"},
				{"id": "ch-voice", "name": "Voice Chat", "type": 2, "position": 2, "parent_id": "cat-1"},
				{"id": "ch-orphan", "name": "orphan-text", "type": 0, "position": 3},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(channels)
			return
		}
		http.NotFound(w, r)
	}))
	defer discordServer.Close()

	handler := newTestDiscordHandlerNoRedis(&mockDiscordOAuth{}, mockRepo, "http://localhost:3000")
	handler.discordAPIBase = discordServer.URL + "/api/v10"

	router := gin.New()
	router.GET("/guilds/:guild_id/channels", func(c *gin.Context) {
		c.Set("user_id", "user-1")
		handler.HandleGetGuildChannels(c)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/guilds/guild-111/channels", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Categories []struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			Channels []struct {
				ID       string `json:"id"`
				Name     string `json:"name"`
				Position int    `json:"position"`
			} `json:"channels"`
		} `json:"categories"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse channels response: %v", err)
	}

	// Voice channel must be excluded — only type=0 text channels appear
	allChannelIDs := collectChannelIDs(resp.Categories)
	if containsString(allChannelIDs, "ch-voice") {
		t.Error("voice channel (type=2) must not be included in channels response")
	}
	if !containsString(allChannelIDs, "ch-general") {
		t.Error("text channel ch-general missing from response")
	}
	if !containsString(allChannelIDs, "ch-orphan") {
		t.Error("orphan text channel ch-orphan missing from response")
	}

	// orphan-text must appear in an "Uncategorized" category
	var foundUncategorized bool
	for _, cat := range resp.Categories {
		if cat.Name == "Uncategorized" {
			foundUncategorized = true
			var hasOrphan bool
			for _, ch := range cat.Channels {
				if ch.ID == "ch-orphan" {
					hasOrphan = true
				}
			}
			if !hasOrphan {
				t.Error("ch-orphan must appear in Uncategorized category")
			}
		}
	}
	if !foundUncategorized {
		t.Error("expected an 'Uncategorized' category for channels without parent_id")
	}
}

// TestHandleDiscordDisconnect_APIFailure verifies that disconnect always cleans up local DB
// records even when the Discord Leave Guild REST call fails.
func TestHandleDiscordDisconnect_APIFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)

	icon := "icon123"
	mockRepo := &mockDiscordRepo{
		getGuild: &models.DiscordGuild{
			ID:        "1",
			UserID:    "user-1",
			GuildID:   "guild-111",
			GuildName: "Test Server",
			GuildIcon: &icon,
		},
	}

	// Discord Leave Guild endpoint always returns 500 (simulates API failure)
	discordServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer discordServer.Close()

	handler := newTestDiscordHandlerNoRedis(&mockDiscordOAuth{}, mockRepo, "http://localhost:3000")
	handler.discordAPIBase = discordServer.URL + "/api/v10"

	router := gin.New()
	router.DELETE("/guilds/:guild_id", func(c *gin.Context) {
		c.Set("user_id", "user-1")
		handler.HandleDisconnect(c)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/guilds/guild-111", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent && w.Code != http.StatusOK {
		t.Errorf("expected 204 or 200 on disconnect, got %d: %s", w.Code, w.Body.String())
	}

	if !mockRepo.deleteCalled {
		t.Error("DeleteGuild must be called even when Discord Leave Guild API fails")
	}
	if !mockRepo.deleteSourcesCalled {
		t.Error("DeleteDiscordSourcesByGuildID must be called even when Discord Leave Guild API fails")
	}
}

// TestHandleDiscordCallback_StoresGuild verifies the happy path: a valid state+code in the
// callback leads to UpsertGuild being called and a redirect to the frontend URL.
func TestHandleDiscordCallback_StoresGuild(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockOAuth := &mockDiscordOAuth{
		exchangeToken: &oauth2.Token{AccessToken: "tok-abc", Expiry: time.Now().Add(time.Hour)},
		missingPerms:  nil, // all permissions present
	}
	mockRepo := &mockDiscordRepo{}

	handler := newTestDiscordHandlerNoRedis(mockOAuth, mockRepo, "http://localhost:3000")
	handler.stateStore = &fakeStateStore{states: map[string]string{"goodstate": "user-999"}}

	router := gin.New()
	router.GET("/discord/callback", handler.HandleCallback)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/discord/callback?state=goodstate&code=authcode&guild_id=guild-abc&guild_name=MyServer", nil)
	router.ServeHTTP(w, req)

	// Expect a redirect to the frontend
	if w.Code != http.StatusFound && w.Code != http.StatusSeeOther && w.Code != http.StatusTemporaryRedirect {
		t.Errorf("expected redirect (3xx), got %d: %s", w.Code, w.Body.String())
	}

	if !mockRepo.upsertCalled {
		t.Error("UpsertGuild must be called on successful callback")
	}

	location := w.Header().Get("Location")
	if !containsString(location, "discord=connected") {
		t.Errorf("redirect Location %q must contain 'discord=connected'", location)
	}
}

// --- Test helper types (test-only, not exported) ---

// fakeStateStore wraps memStateStore to allow pre-populating states in tests.
// This is a test alias — tests use &fakeStateStore{states: map[string]string{...}}.
type fakeStateStore = memStateStore

// containsString checks if s contains substr.
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())
}

// collectChannelIDs flattens category channels into a comma-separated string for assertion.
func collectChannelIDs(categories []struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Channels []struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Position int    `json:"position"`
	} `json:"channels"`
}) string {
	var ids string
	for _, cat := range categories {
		for _, ch := range cat.Channels {
			ids += ch.ID + ","
		}
	}
	return ids
}
