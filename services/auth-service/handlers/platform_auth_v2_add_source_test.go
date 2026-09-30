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

// Handler-level regression coverage for the 2026-09-26 incident (overlay
// 36847b00), where the unit-testable seams all worked and the wiring above
// them was what failed:
//
//   - The Twitch add-source short-circuit reused a dead token because it
//     trusted the stale users.granted_scopes row; the streamer's three
//     "re-auth" attempts never showed a consent screen. The decision tree
//     (refresh-if-expired → validate-live-scopes → reuse or fall through)
//     is exercised here against stubbed Twitch endpoints.
//   - The Kick callback stored the users-endpoint display name as
//     channel_id ("Scuffed_Onigiri") while the kick-listener looks channels
//     up by slug ("scuffed-onigiri"), 404ing forever. The slug-resolution
//     branch is exercised end to end with a stubbed Kick API.

package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/caesar/all-chat/services/auth-service/oauth"
	"github.com/caesar/all-chat/services/auth-service/repository"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/zap/zaptest"
)

// addSourceStub is one HTTP stub serving every upstream call the two flows
// make: Twitch's /oauth2/validate + /oauth2/token, Kick's /oauth/token +
// /public/v1/users + /public/v1/channels, and the overlay-manager's internal
// add-source endpoint (whose requests are recorded for assertions).
type addSourceStub struct {
	srv *httptest.Server

	mu           sync.Mutex
	overlayCalls []map[string]interface{}

	// Behavior knobs, set per scenario.
	validateHandler func(callerToken string) (int, string)
	kickChannels    func() (int, string)
}

func newAddSourceStub(t *testing.T) *addSourceStub {
	t.Helper()
	s := &addSourceStub{}
	s.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/oauth2/validate":
			authz := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "OAuth"))
			status, body := s.validateHandler(authz)
			w.WriteHeader(status)
			_, _ = w.Write([]byte(body))
		case r.URL.Path == "/oauth2/token" || r.URL.Path == "/oauth/token":
			// Twitch (/oauth2/token) and Kick (/oauth/token) exchange and refresh
			// here in these tests; the Kick PKCE authorization_code exchange and
			// Twitch's oauth2-library refresh both parse this body.
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"access_token":"live-token","refresh_token":"fresh-refresh","expires_in":3600,"scope":"user:read:chat bits:read","token_type":"bearer"}`))
		case r.URL.Path == "/public/v1/users":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":[{"user_id":130554250,"name":"Scuffed_Onigiri","profile_picture":"p"}],"message":"OK"}`))
		case r.URL.Path == "/public/v1/channels":
			status, body := s.kickChannels()
			w.WriteHeader(status)
			_, _ = w.Write([]byte(body))
		case strings.HasPrefix(r.URL.Path, "/internal/overlays/"):
			s.mu.Lock()
			defer s.mu.Unlock()
			body, _ := io.ReadAll(r.Body)
			var parsed map[string]interface{}
			_ = json.Unmarshal(body, &parsed)
			s.overlayCalls = append(s.overlayCalls, parsed)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"added"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(s.srv.Close)
	return s
}

// stubTransport rewrites every request to the stub server, so providers can keep
// their production endpoint constants while the test controls responses.
type stubTransport struct{ base string }

func (tr stubTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	base, _ := url.Parse(tr.base)
	req.URL.Scheme = base.Scheme
	req.URL.Host = base.Host
	return http.DefaultTransport.RoundTrip(req)
}

func (s *addSourceStub) transport() http.RoundTripper { return stubTransport{base: s.srv.URL} }

func (s *addSourceStub) overlayCallCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.overlayCalls)
}

func (s *addSourceStub) lastOverlayCall() map[string]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.overlayCalls[len(s.overlayCalls)-1]
}

// setupAddSourceDB spins up a scratch PostgreSQL with the users table the
// repository writes against, and returns the pool.
func setupAddSourceDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "postgres:16-alpine",
			ExposedPorts: []string{"5432/tcp"},
			Env: map[string]string{
				"POSTGRES_USER":     "testuser",
				"POSTGRES_PASSWORD": "testpass",
				"POSTGRES_DB":       "testdb",
			},
			WaitingFor: wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		t.Skipf("cannot start postgres testcontainer (docker unavailable?): %v", err)
	}

	host, err := container.Host(ctx)
	require.NoError(t, err)
	port, err := container.MappedPort(ctx, "5432")
	require.NoError(t, err)

	pool, err := pgxpool.New(ctx, fmt.Sprintf(
		"postgres://testuser:testpass@%s:%s/testdb?sslmode=disable", host, port.Port()))
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
		CREATE TABLE users (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			twitch_id VARCHAR(50) UNIQUE,
			google_id VARCHAR(100) UNIQUE,
			kick_id VARCHAR(255) UNIQUE,
			auth_provider VARCHAR(20) NOT NULL DEFAULT 'twitch',
			username VARCHAR(50) UNIQUE NOT NULL,
			display_name VARCHAR(100) NOT NULL,
			profile_image_url TEXT,
			is_admin BOOLEAN NOT NULL DEFAULT FALSE,
			is_premium BOOLEAN NOT NULL DEFAULT FALSE,
			is_beta_tester BOOLEAN NOT NULL DEFAULT FALSE,
			is_ambassador BOOLEAN NOT NULL DEFAULT FALSE,
			premium_admin_override_expires_at TIMESTAMP,
			is_banned BOOLEAN NOT NULL DEFAULT FALSE,
			banned_at TIMESTAMP,
			banned_reason TEXT,
			banned_by UUID,
			access_token TEXT NOT NULL,
			refresh_token TEXT NOT NULL,
			token_expires_at TIMESTAMP NOT NULL,
			granted_scopes TEXT[] NOT NULL DEFAULT '{}',
			created_at TIMESTAMP DEFAULT NOW(),
			updated_at TIMESTAMP DEFAULT NOW(),
			onboarding_completed_at TIMESTAMP NULL,
			CONSTRAINT check_auth_ids CHECK (
				(auth_provider = 'twitch' AND twitch_id IS NOT NULL) OR
				(auth_provider = 'youtube' AND google_id IS NOT NULL) OR
				(auth_provider = 'kick' AND kick_id IS NOT NULL)
			)
		);
		CREATE TABLE banned_platform_ids (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			platform VARCHAR(50) NOT NULL,
			platform_id VARCHAR(100) NOT NULL,
			banned_by UUID,
			reason TEXT NOT NULL,
			banned_at TIMESTAMP NOT NULL DEFAULT NOW(),
			unbanned_at TIMESTAMP NULL,
			is_active BOOLEAN NOT NULL DEFAULT TRUE
		);
		CREATE TABLE kick_oauth_tokens (
			id SERIAL PRIMARY KEY,
			user_id UUID NOT NULL,
			channel_id VARCHAR(255) NOT NULL,
			access_token TEXT NOT NULL,
			refresh_token TEXT NOT NULL,
			token_type VARCHAR(50) DEFAULT 'Bearer',
			expiry TIMESTAMP NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			encryption_version SMALLINT NOT NULL DEFAULT 0,
			kick_user_id VARCHAR(255),
			granted_scopes TEXT[] NOT NULL DEFAULT '{}',
			UNIQUE(user_id, channel_id)
		);
	`)
	require.NoError(t, err)

	t.Cleanup(func() {
		pool.Close()
		_ = container.Terminate(ctx)
	})
	return pool
}

// seedUser inserts a Twitch-login user with the given stored token state and
// returns its id. Cipher is nil in these tests, so tokens round-trip verbatim.
func seedUser(t *testing.T, pool *pgxpool.Pool, accessToken string, expiry time.Time, scopes []string) string {
	t.Helper()
	var id string
	err := pool.QueryRow(context.Background(), `
		INSERT INTO users (twitch_id, auth_provider, username, display_name,
			access_token, refresh_token, token_expires_at, granted_scopes)
		VALUES ('1532957417', 'twitch', 'scuffedonigiri', 'ScuffedOnigiri',
			$1, 'stored-refresh', $2, $3)
		RETURNING id`, accessToken, expiry, scopes).Scan(&id)
	require.NoError(t, err)
	return id
}

// newTwitchHandler wires a PlatformAuthHandlerV2 whose Twitch provider talks
// only to the stub server. overlayManagerURL points at the same stub. The
// provider's single client drives GetUserInfo, ValidateToken AND the
// oauth2-library refresh (RefreshToken injects it into the token source's
// context), so one stub covers the whole flow.
func newTwitchHandler(t *testing.T, stub *addSourceStub, repo *repository.UserRepository, rdb *redis.Client) *PlatformAuthHandlerV2 {
	t.Helper()
	provider := oauth.NewTwitchOAuth("cid", "secret", "http://localhost:3000/api/v1/auth/twitch/callback").
		WithHTTPClient(&http.Client{Transport: stub.transport()})
	return &PlatformAuthHandlerV2{
		providers:         map[oauth.Platform]oauth.OAuthProvider{oauth.PlatformTwitch: provider},
		userRepo:          repo,
		redis:             rdb,
		userKeyChain:      testUserKeyChain("test-secret"),
		jwtExpiry:         time.Hour,
		overlayManagerURL: stub.srv.URL,
		logger:            zaptest.NewLogger(t),
		frontendURL:       "http://localhost:3000",
	}
}

func ginContext(t *testing.T, userID, overlayID string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/auth/twitch/"+overlayID, nil)
	c.Request.Header.Set("Authorization", "Bearer test-jwt")
	c.Set("user_id", userID)
	c.Params = gin.Params{{Key: "overlay_id", Value: overlayID}}
	return c, w
}

// Scenario: stored token is expired but refreshable, and the fresh token carries
// the chat scopes. The short-circuit must refresh FIRST (a validate against the
// stale token 401s, which used to poison this path into a fake "revoked"
// verdict), then reuse — with the stored scope record untouched.
func TestHandleAddSource_TwitchShortCircuit_RefreshesThenReuses(t *testing.T) {
	if testing.Short() {
		t.Skip("integration")
	}
	pool := setupAddSourceDB(t)
	repo := repository.NewUserRepository(pool, nil)
	// Expiry seed must be UTC: the column is a naive TIMESTAMP read by pgx as UTC,
	// so a local-time seed "an hour ago" in a zone ahead of UTC lands in the
	// FUTURE for the DB and the refresh branch never executes (the same trap as
	// the PAT expiry test fixed in this branch).
	userID := seedUser(t, pool, "old-token", time.Now().UTC().Add(-time.Hour),
		[]string{"user:read:chat", "user:bot", "channel:bot"})

	stub := newAddSourceStub(t)
	stub.validateHandler = func(callerToken string) (int, string) {
		// The refresh endpoint returns access_token "live-token", so after a
		// successful refresh the validate call must carry exactly that.
		if callerToken != "live-token" {
			return http.StatusUnauthorized, `{"status":401,"message":"invalid access token"}`
		}
		return http.StatusOK, `{"client_id":"cid","login":"scuffedonigiri","user_id":"1532957417",
			"scopes":["user:read:chat","user:bot","channel:bot"]}`
	}

	rdb := redis.NewClient(&redis.Options{Addr: miniredis.RunT(t).Addr()})
	h := newTwitchHandler(t, stub, repo, rdb)

	c, w := ginContext(t, userID, "36847b00-5329-44f2-9008-81daa1a34991")
	h.HandleAddSource(oauth.PlatformTwitch)(c)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"reused_existing_credentials":true`)
	assert.Equal(t, 1, stub.overlayCallCount())

	// The refresh persisted its token…
	got, err := repo.GetByID(context.Background(), userID)
	require.NoError(t, err)
	assert.Equal(t, "live-token", got.AccessToken)

	// …and the scope record was NOT wiped: clearing a working grant's scopes is
	// the regression that tore down healthy chat for every expired token.
	scopes, err := repo.GetGrantedScopes(context.Background(), userID)
	require.NoError(t, err)
	assert.Contains(t, scopes, "user:read:chat")
}

// Scenario: Twitch-side the grant is dead (validate 401). The short-circuit
// must NOT reuse the token (that was the incident: three silent "re-auths"),
// must clear the stale scope record so gating stops trusting it, and must
// hand the streamer a real consent URL.
func TestHandleAddSource_TwitchShortCircuit_RevokedGrantFallsThroughToConsent(t *testing.T) {
	if testing.Short() {
		t.Skip("integration")
	}
	pool := setupAddSourceDB(t)
	repo := repository.NewUserRepository(pool, nil)
	userID := seedUser(t, pool, "dead-token", time.Now().Add(time.Hour),
		[]string{"user:read:chat", "user:bot", "channel:bot"})

	stub := newAddSourceStub(t)
	stub.validateHandler = func(_ string) (int, string) {
		return http.StatusUnauthorized, `{"status":401,"message":"invalid access token"}`
	}

	rdb := redis.NewClient(&redis.Options{Addr: miniredis.RunT(t).Addr()})
	h := newTwitchHandler(t, stub, repo, rdb)

	c, w := ginContext(t, userID, "36847b00-5329-44f2-9008-81daa1a34991")
	h.HandleAddSource(oauth.PlatformTwitch)(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	authURL, ok := body["auth_url"].(string)
	require.True(t, ok, "a revoked grant must produce a consent URL, got: %s", w.Body.String())
	assert.Contains(t, authURL, "force_verify=true",
		"the consent URL must re-prompt, not silently reissue the same narrow grant")

	assert.Equal(t, 0, stub.overlayCallCount(), "a dead token must never reach overlay-manager again")

	// The stale record is cleared so EventSub gating and every other scope-gated
	// decision stops believing the dead grant until a fresh consent lands.
	scopes, err := repo.GetGrantedScopes(context.Background(), userID)
	require.NoError(t, err)
	assert.NotContains(t, scopes, "user:read:chat")
}

// Scenario: Twitch returns a valid token whose live scopes no longer include
// user:read:chat (grant was narrowed Twitch-side while the DB still claims it).
// Same fall-through contract as a hard 401 — no reuse, stale record aligned to
// what the token actually grants.
func TestHandleAddSource_TwitchShortCircuit_NarrowedLiveGrantFallsThrough(t *testing.T) {
	if testing.Short() {
		t.Skip("integration")
	}
	pool := setupAddSourceDB(t)
	repo := repository.NewUserRepository(pool, nil)
	userID := seedUser(t, pool, "live-token", time.Now().Add(time.Hour),
		[]string{"user:read:chat", "user:bot", "channel:bot"})

	stub := newAddSourceStub(t)
	stub.validateHandler = func(_ string) (int, string) {
		return http.StatusOK, `{"client_id":"cid","login":"scuffedonigiri","user_id":"1532957417",
			"scopes":["bits:read","moderator:read:followers"]}`
	}

	rdb := redis.NewClient(&redis.Options{Addr: miniredis.RunT(t).Addr()})
	h := newTwitchHandler(t, stub, repo, rdb)

	c, w := ginContext(t, userID, "36847b00-5329-44f2-9008-81daa1a34991")
	h.HandleAddSource(oauth.PlatformTwitch)(c)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	_, hasConsentURL := body["auth_url"]
	assert.True(t, hasConsentURL, "a live-but-narrowed token must fall through to consent, got: %s", w.Body.String())
	assert.Equal(t, 0, stub.overlayCallCount())

	scopes, err := repo.GetGrantedScopes(context.Background(), userID)
	require.NoError(t, err)
	assert.NotContains(t, scopes, "user:read:chat")
}

// Scenario: Twitch's validate endpoint is down (5xx). The short-circuit must
// NOT reuse the token — "never reuse unverified credentials" — and must fall
// through to the consent flow. Unlike the revoked/narrowed verdicts, the
// stored scope record is NOT touched: nothing proved it stale.
func TestHandleAddSource_TwitchShortCircuit_TransientValidateFailureFallsThrough(t *testing.T) {
	if testing.Short() {
		t.Skip("integration")
	}
	pool := setupAddSourceDB(t)
	repo := repository.NewUserRepository(pool, nil)
	userID := seedUser(t, pool, "maybe-live-token", time.Now().Add(time.Hour),
		[]string{"user:read:chat", "user:bot", "channel:bot"})

	stub := newAddSourceStub(t)
	stub.validateHandler = func(_ string) (int, string) {
		return http.StatusInternalServerError, `{"error":"server exploded"}`
	}

	rdb := redis.NewClient(&redis.Options{Addr: miniredis.RunT(t).Addr()})
	h := newTwitchHandler(t, stub, repo, rdb)

	c, w := ginContext(t, userID, "36847b00-5329-44f2-9008-81daa1a34991")
	h.HandleAddSource(oauth.PlatformTwitch)(c)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	_, hasConsentURL := body["auth_url"]
	assert.True(t, hasConsentURL, "an unverifiable token must fall through to consent, got: %s", w.Body.String())
	assert.Equal(t, 0, stub.overlayCallCount(), "an unverifiable token must never reach overlay-manager")

	// The stored record is not cleared on this branch: Twitch being down proves
	// nothing about the grant, and wiping the record would gate EventSub off a
	// grant that may still be live.
	scopes, err := repo.GetGrantedScopes(context.Background(), userID)
	require.NoError(t, err)
	assert.Contains(t, scopes, "user:read:chat",
		"a transient validate failure must not clear the stored scope record")
}

// Scenario (Kick): the OAuth callback must store the streamer's REAL channel
// slug from GET /public/v1/channels, not the users-endpoint display name. The
// names differ for any streamer whose display name is not their slug — and
// handing the display name to the kick-listener 404s on every sync
// (the incident's "Kick nothing" half).
func TestHandleCallback_KickAddSource_StoresChannelSlug(t *testing.T) {
	if testing.Short() {
		t.Skip("integration")
	}
	pool := setupAddSourceDB(t)
	repo := repository.NewUserRepository(pool, nil)
	userID := seedUser(t, pool, "twitch-login-token", time.Now().Add(time.Hour),
		[]string{"user:read:chat"})
	overlayID := "36847b00-5329-44f2-9008-81daa1a34991"

	stub := newAddSourceStub(t)
	stub.kickChannels = func() (int, string) {
		return http.StatusOK, `{"data":[{"broadcaster_user_id":130554250,"slug":"scuffed-onigiri"}],"message":"OK"}`
	}

	kickCallbackScenario(t, pool, repo, stub, userID, overlayID, "scuffed-onigiri")
}

// Same flow with the channel endpoint down: the flow must not die (the user
// consented successfully); it falls back to the display name with a Warn, and
// the streamer can re-add once Kick recovers.
func TestHandleCallback_KickAddSource_FallsBackToDisplayNameWhenChannelLookupFails(t *testing.T) {
	if testing.Short() {
		t.Skip("integration")
	}
	pool := setupAddSourceDB(t)
	repo := repository.NewUserRepository(pool, nil)
	userID := seedUser(t, pool, "twitch-login-token", time.Now().Add(time.Hour), []string{"user:read:chat"})
	overlayID := "36847b00-5329-44f2-9008-81daa1a34991"

	stub := newAddSourceStub(t)
	stub.kickChannels = func() (int, string) {
		return http.StatusInternalServerError, `{"message":"boom"}`
	}

	kickCallbackScenario(t, pool, repo, stub, userID, overlayID, "Scuffed_Onigiri")
}

// kickCallbackScenario drives the full OAuth callback for a Kick add-source
// with a verified shared oauth state, then asserts which channel_id the
// overlay-manager stub received.
func kickCallbackScenario(t *testing.T, _ *pgxpool.Pool, repo *repository.UserRepository, stub *addSourceStub, userID, overlayID, wantChannelID string) {
	t.Helper()

	csrf := "csrf-test-123"
	rdb := redis.NewClient(&redis.Options{Addr: miniredis.RunT(t).Addr()})
	state := oauth.NewAddSourceState(csrf, overlayID, userID)
	require.NoError(t, state.Validate())
	stateStr, err := state.Encode()
	require.NoError(t, err)
	require.NoError(t, rdb.Set(context.Background(), "oauth_state:kick:"+csrf, stateStr, 30*time.Minute).Err())
	require.NoError(t, rdb.Set(context.Background(), "oauth_verifier:kick:"+csrf, "v", 30*time.Minute).Err())

	provider := oauth.NewKickOAuth("cid", "secret", "http://localhost:3000/api/v1/auth/kick/callback").
		WithHTTPClient(&http.Client{Transport: stub.transport()})

	h := &PlatformAuthHandlerV2{
		providers:         map[oauth.Platform]oauth.OAuthProvider{oauth.PlatformKick: provider},
		userRepo:          repo,
		redis:             rdb,
		userKeyChain:      testUserKeyChain("test-secret"),
		jwtExpiry:         time.Hour,
		overlayManagerURL: stub.srv.URL,
		logger:            zaptest.NewLogger(t),
		frontendURL:       "http://localhost:3000",
	}

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	q := url.Values{}
	q.Set("code", "code-1")
	q.Set("state", stateStr)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/auth/kick/callback?"+q.Encode(), nil)
	c.Request.Header.Set("X-Forwarded-Host", "localhost:3000")

	h.HandleCallback(oauth.PlatformKick)(c)

	assert.Equal(t, http.StatusFound, w.Code, "successful callback redirects, got %d: %s", w.Code, w.Body.String())

	require.Equal(t, 1, stub.overlayCallCount(), "source must be added exactly once")
	added := stub.lastOverlayCall()
	assert.Equal(t, wantChannelID, added["channel_id"],
		"overlay-manager must receive the identifier kick-listener can resolve (the slug), not the display name")
}
