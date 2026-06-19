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
	"testing"
	"time"

	"github.com/caesar/all-chat/services/auth-service/models"
	"github.com/caesar/all-chat/services/auth-service/oauth"
	"github.com/caesar/all-chat/services/auth-service/repository"
	"github.com/caesar/all-chat/shared/encryption"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

// Regression tests for the granted_scopes clobber bug: linking a second
// platform (YouTube/Kick) to a Twitch-login account must NOT overwrite the
// account's primary OAuth credentials or its granted_scopes record. Doing so
// erased the user:read:chat grant and silently demoted the streamer's channel
// from the EventSub listener back to the IRC listener.

func TestLinkMayReplacePrimaryCredentials(t *testing.T) {
	chatScopes := []string{"user:read:chat", "user:bot", "channel:bot"}
	googleScopes := []string{"https://www.googleapis.com/auth/youtube.readonly"}
	loginScopes := []string{"channel:read:redemptions", "bits:read"}

	tests := []struct {
		name           string
		authProvider   string
		platform       oauth.Platform
		existingScopes []string
		newScopes      []string
		want           bool
	}{
		{
			name:           "youtube link to twitch user never replaces credentials",
			authProvider:   "twitch",
			platform:       oauth.PlatformYouTube,
			existingScopes: chatScopes,
			newScopes:      googleScopes,
			want:           false,
		},
		{
			name:           "kick link to twitch user never replaces credentials",
			authProvider:   "twitch",
			platform:       oauth.PlatformKick,
			existingScopes: chatScopes,
			newScopes:      nil,
			want:           false,
		},
		{
			name:           "kick link to twitch user without prior scopes still declines",
			authProvider:   "twitch",
			platform:       oauth.PlatformKick,
			existingScopes: nil,
			newScopes:      nil,
			want:           false,
		},
		{
			name:           "twitch add-source reflow with chat scopes replaces credentials",
			authProvider:   "twitch",
			platform:       oauth.PlatformTwitch,
			existingScopes: nil,
			newScopes:      chatScopes,
			want:           true,
		},
		{
			name:           "twitch reflow without chat scopes must not downgrade an existing chat grant",
			authProvider:   "twitch",
			platform:       oauth.PlatformTwitch,
			existingScopes: chatScopes,
			newScopes:      loginScopes,
			want:           false,
		},
		{
			name:           "twitch reflow without chat scopes is fine when none were granted before",
			authProvider:   "twitch",
			platform:       oauth.PlatformTwitch,
			existingScopes: loginScopes,
			newScopes:      loginScopes,
			want:           true,
		},
		{
			name:           "kick link to kick user replaces credentials",
			authProvider:   "kick",
			platform:       oauth.PlatformKick,
			existingScopes: nil,
			newScopes:      nil,
			want:           true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := linkMayReplacePrimaryCredentials(tt.authProvider, tt.platform, tt.existingScopes, tt.newScopes)
			if got != tt.want {
				t.Errorf("linkMayReplacePrimaryCredentials(%q, %q, %v, %v) = %v, want %v",
					tt.authProvider, tt.platform, tt.existingScopes, tt.newScopes, got, tt.want)
			}
		})
	}
}

// --- integration tests against a real PostgreSQL (testcontainers) ---

const linkTestEncryptionKey = "0123456789abcdef0123456789abcdef"

// setupLinkTestDB starts a PostgreSQL container with the users schema needed
// by linkPlatformToUser (mirrors repository/user_repo_test.go).
func setupLinkTestDB(t *testing.T) (*pgxpool.Pool, func()) {
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
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
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("Failed to start container: %v", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("Failed to get container host: %v", err)
	}
	port, err := container.MappedPort(ctx, "5432")
	if err != nil {
		t.Fatalf("Failed to get container port: %v", err)
	}

	pool, err := pgxpool.New(ctx, "postgres://testuser:testpass@"+host+":"+port.Port()+"/testdb?sslmode=disable")
	if err != nil {
		t.Fatalf("Failed to create connection pool: %v", err)
	}

	schema := `
		CREATE TABLE IF NOT EXISTS twitch_oauth_tokens (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL,
			twitch_user_id VARCHAR(50) NOT NULL,
			twitch_login VARCHAR(100) NOT NULL,
			access_token TEXT NOT NULL,
			refresh_token TEXT NOT NULL,
			token_expires_at TIMESTAMP NOT NULL,
			granted_scopes TEXT[] NOT NULL DEFAULT '{}',
			encryption_version INT NOT NULL DEFAULT 1,
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
			UNIQUE(user_id, twitch_login)
		);
		CREATE TABLE IF NOT EXISTS users (
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
			is_banned BOOLEAN NOT NULL DEFAULT FALSE,
			banned_at TIMESTAMP,
			banned_reason TEXT,
			banned_by VARCHAR(255),
			access_token TEXT NOT NULL,
			refresh_token TEXT NOT NULL,
			token_expires_at TIMESTAMP NOT NULL,
			granted_scopes TEXT[] NOT NULL DEFAULT '{}',
			created_at TIMESTAMP DEFAULT NOW(),
			updated_at TIMESTAMP DEFAULT NOW()
		);
		CREATE TABLE IF NOT EXISTS kick_oauth_tokens (
			id SERIAL PRIMARY KEY,
			user_id UUID NOT NULL,
			channel_id VARCHAR(255) NOT NULL,
			kick_user_id VARCHAR(255),
			access_token TEXT NOT NULL,
			refresh_token TEXT NOT NULL,
			token_type VARCHAR(50) DEFAULT 'Bearer',
			expiry TIMESTAMP NOT NULL,
			granted_scopes TEXT[] NOT NULL DEFAULT '{}',
			encryption_version INT NOT NULL DEFAULT 1,
			created_at TIMESTAMP DEFAULT NOW(),
			updated_at TIMESTAMP DEFAULT NOW(),
			UNIQUE(user_id, channel_id)
		);
		CREATE TABLE IF NOT EXISTS youtube_oauth_tokens (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL,
			channel_id VARCHAR(255) NOT NULL,
			access_token TEXT NOT NULL,
			refresh_token TEXT NOT NULL,
			token_type VARCHAR(50) DEFAULT 'Bearer',
			expiry TIMESTAMP NOT NULL,
			granted_scopes TEXT[] NOT NULL DEFAULT '{}',
			encryption_version INT NOT NULL DEFAULT 1,
			created_at TIMESTAMP DEFAULT NOW(),
			updated_at TIMESTAMP DEFAULT NOW(),
			UNIQUE(user_id, channel_id)
		);
	`
	if _, err := pool.Exec(ctx, schema); err != nil {
		t.Fatalf("Failed to create schema: %v", err)
	}

	return pool, func() {
		pool.Close()
		container.Terminate(ctx)
	}
}

func newLinkTestHandler(t *testing.T, pool *pgxpool.Pool) (*PlatformAuthHandlerV2, *repository.UserRepository) {
	key, err := encryption.ParseKey(linkTestEncryptionKey)
	if err != nil {
		t.Fatalf("failed to parse test encryption key: %v", err)
	}
	cipher, err := encryption.NewAESEncryptor(key)
	if err != nil {
		t.Fatalf("failed to create cipher: %v", err)
	}
	repo := repository.NewUserRepository(pool, cipher)
	return &PlatformAuthHandlerV2{
		userRepo: repo,
		logger:   zap.NewNop(),
	}, repo
}

// fakePlatformUser is a minimal PlatformUserInfo for driving linkPlatformToUser.
type fakePlatformUser struct {
	id       string
	username string
	platform oauth.Platform
}

func (f *fakePlatformUser) GetID() string               { return f.id }
func (f *fakePlatformUser) GetUsername() string         { return f.username }
func (f *fakePlatformUser) GetDisplayName() string      { return f.username }
func (f *fakePlatformUser) GetProfileImageURL() string  { return "" }
func (f *fakePlatformUser) GetPlatform() oauth.Platform { return f.platform }

// tokenWithScopes builds an oauth2 token whose Extra("scope") carries the
// given scope list, the way Twitch's token endpoint returns it.
func tokenWithScopes(access string, scopes []string) *oauth2.Token {
	tok := &oauth2.Token{
		AccessToken:  access,
		RefreshToken: access + "-refresh",
		Expiry:       time.Now().Add(4 * time.Hour),
	}
	if scopes == nil {
		return tok
	}
	raw := make([]interface{}, len(scopes))
	for i, s := range scopes {
		raw[i] = s
	}
	return tok.WithExtra(map[string]interface{}{"scope": raw})
}

func createTwitchTestUser(t *testing.T, repo *repository.UserRepository, scopes []string) *models.User {
	t.Helper()
	twitchID := "tw-123456"
	user := &models.User{
		TwitchID:       &twitchID,
		AuthProvider:   "twitch",
		Username:       "streamer",
		DisplayName:    "Streamer",
		AccessToken:    "twitch-access-token",
		RefreshToken:   "twitch-refresh-token",
		TokenExpiresAt: time.Now().Add(3 * time.Hour),
		GrantedScopes:  scopes,
	}
	if err := repo.Create(context.Background(), user); err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}
	return user
}

func TestLinkPlatformToUser_CrossPlatformLinkPreservesTwitchGrant(t *testing.T) {
	pool, cleanup := setupLinkTestDB(t)
	defer cleanup()

	h, repo := newLinkTestHandler(t, pool)
	ctx := context.Background()

	chatScopes := []string{"user:read:chat", "user:bot", "channel:bot"}
	user := createTwitchTestUser(t, repo, chatScopes)

	// User connects YouTube as an additional source: the Google token must not
	// replace the Twitch credentials or the recorded chat-scope grant.
	googleToken := tokenWithScopes("google-access-token",
		[]string{"https://www.googleapis.com/auth/youtube.readonly"})
	_, err := h.linkPlatformToUser(ctx, user.ID, oauth.PlatformYouTube,
		&fakePlatformUser{id: "g-1", username: "streamer-yt", platform: oauth.PlatformYouTube}, googleToken)
	if err != nil {
		t.Fatalf("linkPlatformToUser(youtube) failed: %v", err)
	}

	scopes, err := repo.GetGrantedScopes(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetGrantedScopes failed: %v", err)
	}
	if !containsScope(scopes, "user:read:chat") {
		t.Errorf("granted_scopes lost user:read:chat after YouTube link: %v", scopes)
	}

	got, err := repo.GetByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if got.AccessToken != "twitch-access-token" {
		t.Errorf("primary access token was replaced by the linked platform's token: %q", got.AccessToken)
	}
	if got.RefreshToken != "twitch-refresh-token" {
		t.Errorf("primary refresh token was replaced by the linked platform's token: %q", got.RefreshToken)
	}

	// Same for Kick, whose token response carries no scope list at all — this
	// previously wiped granted_scopes to '{}'.
	kickToken := tokenWithScopes("kick-access-token", nil)
	_, err = h.linkPlatformToUser(ctx, user.ID, oauth.PlatformKick,
		&fakePlatformUser{id: "k-1", username: "streamer-kick", platform: oauth.PlatformKick}, kickToken)
	if err != nil {
		t.Fatalf("linkPlatformToUser(kick) failed: %v", err)
	}

	scopes, err = repo.GetGrantedScopes(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetGrantedScopes failed: %v", err)
	}
	if !containsScope(scopes, "user:read:chat") {
		t.Errorf("granted_scopes lost user:read:chat after Kick link: %v", scopes)
	}
}

func TestLinkPlatformToUser_TwitchReflowRecordsChatScopes(t *testing.T) {
	pool, cleanup := setupLinkTestDB(t)
	defer cleanup()

	h, repo := newLinkTestHandler(t, pool)
	ctx := context.Background()

	// Pre-migration user: consent predates the EventSub feature, no scopes recorded.
	user := createTwitchTestUser(t, repo, nil)

	chatScopes := []string{"channel:read:redemptions", "user:read:chat", "user:bot", "channel:bot"}
	twitchToken := tokenWithScopes("twitch-chat-token", chatScopes)
	_, err := h.linkPlatformToUser(ctx, user.ID, oauth.PlatformTwitch,
		&fakePlatformUser{id: "tw-123456", username: "streamer", platform: oauth.PlatformTwitch}, twitchToken)
	if err != nil {
		t.Fatalf("linkPlatformToUser(twitch reflow) failed: %v", err)
	}

	scopes, err := repo.GetGrantedScopes(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetGrantedScopes failed: %v", err)
	}
	if !containsScope(scopes, "user:read:chat") {
		t.Errorf("twitch add-source reflow did not record chat scopes: %v", scopes)
	}

	got, err := repo.GetByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if got.AccessToken != "twitch-chat-token" {
		t.Errorf("twitch reflow did not update the primary access token: %q", got.AccessToken)
	}
}

// --- linked Twitch credentials (ADR-0016) ---

func TestShouldStoreLinkedTwitchCredentials(t *testing.T) {
	tests := []struct {
		name         string
		authProvider string
		platform     oauth.Platform
		isAddSource  bool
		want         bool
	}{
		{"youtube account links twitch via add-source", "youtube", oauth.PlatformTwitch, true, true},
		{"kick account links twitch via add-source", "kick", oauth.PlatformTwitch, true, true},
		{"twitch account reflow stores on users row instead", "twitch", oauth.PlatformTwitch, true, false},
		{"youtube account links youtube", "youtube", oauth.PlatformYouTube, true, false},
		{"login flow never stores linked credentials", "youtube", oauth.PlatformTwitch, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldStoreLinkedTwitchCredentials(tt.authProvider, tt.platform, tt.isAddSource)
			if got != tt.want {
				t.Errorf("shouldStoreLinkedTwitchCredentials(%q, %q, %v) = %v, want %v",
					tt.authProvider, tt.platform, tt.isAddSource, got, tt.want)
			}
		})
	}
}

func TestStoreTwitchToken_PersistsLinkedCredentials(t *testing.T) {
	pool, cleanup := setupLinkTestDB(t)
	defer cleanup()

	_, repo := newLinkTestHandler(t, pool)
	ctx := context.Background()

	// A YouTube-login account (the Group B case).
	googleID := "g-987"
	ytUser := &models.User{
		GoogleID:       &googleID,
		AuthProvider:   "youtube",
		Username:       "youtube_987",
		DisplayName:    "Tumi",
		AccessToken:    "google-access",
		RefreshToken:   "google-refresh",
		TokenExpiresAt: time.Now().Add(time.Hour),
	}
	if err := repo.Create(ctx, ytUser); err != nil {
		t.Fatalf("failed to create youtube user: %v", err)
	}

	chatScopes := []string{"user:read:chat", "user:bot", "channel:bot"}
	twitchToken := tokenWithScopes("twitch-linked-token", chatScopes)

	if err := repo.StoreTwitchToken(ctx, ytUser.ID, "tw-555", "BLVTumi", twitchToken, chatScopes); err != nil {
		t.Fatalf("StoreTwitchToken failed: %v", err)
	}

	// The partition predicate matches case-insensitively on login with a valid
	// expiry and the chat scope — exactly what the other services will query.
	var matches bool
	err := pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM twitch_oauth_tokens t
			WHERE LOWER(t.twitch_login) = LOWER($1)
			  AND 'user:read:chat' = ANY(t.granted_scopes)
			  AND t.token_expires_at > NOW()
		)`, "blvtumi").Scan(&matches)
	if err != nil {
		t.Fatalf("predicate query failed: %v", err)
	}
	if !matches {
		t.Error("stored linked credentials do not satisfy the EventSub partition predicate")
	}

	// Tokens must be encrypted at rest.
	var storedAccess string
	if err := pool.QueryRow(ctx,
		`SELECT access_token FROM twitch_oauth_tokens WHERE user_id = $1`, ytUser.ID,
	).Scan(&storedAccess); err != nil {
		t.Fatalf("failed to read stored token: %v", err)
	}
	if storedAccess == "twitch-linked-token" {
		t.Error("access token stored in plaintext")
	}

	// Re-linking upserts rather than erroring.
	newToken := tokenWithScopes("twitch-linked-token-2", chatScopes)
	if err := repo.StoreTwitchToken(ctx, ytUser.ID, "tw-555", "BLVTumi", newToken, chatScopes); err != nil {
		t.Fatalf("StoreTwitchToken upsert failed: %v", err)
	}
	var rowCount int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM twitch_oauth_tokens WHERE user_id = $1`, ytUser.ID,
	).Scan(&rowCount); err != nil {
		t.Fatalf("failed to count rows: %v", err)
	}
	if rowCount != 1 {
		t.Errorf("expected 1 row after upsert, got %d", rowCount)
	}
}

// --- linked Kick + per-platform scope resolution (ADR-0017, migration 062) ---

func TestShouldStoreLinkedKickCredentials(t *testing.T) {
	tests := []struct {
		name         string
		authProvider string
		platform     oauth.Platform
		isAddSource  bool
		want         bool
	}{
		{"twitch account links kick via add-source", "twitch", oauth.PlatformKick, true, true},
		{"youtube account links kick via add-source", "youtube", oauth.PlatformKick, true, true},
		{"kick account reflow stores on users row instead", "kick", oauth.PlatformKick, true, false},
		{"twitch account links twitch", "twitch", oauth.PlatformTwitch, true, false},
		{"login flow never stores linked credentials", "twitch", oauth.PlatformKick, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldStoreLinkedKickCredentials(tt.authProvider, tt.platform, tt.isAddSource)
			if got != tt.want {
				t.Errorf("shouldStoreLinkedKickCredentials(%q, %q, %v) = %v, want %v",
					tt.authProvider, tt.platform, tt.isAddSource, got, tt.want)
			}
		})
	}
}

func TestStoreKickToken_PersistsLinkedCredentials(t *testing.T) {
	pool, cleanup := setupLinkTestDB(t)
	defer cleanup()

	_, repo := newLinkTestHandler(t, pool)
	ctx := context.Background()

	// A Twitch-login account that linked Kick for moderation.
	user := createTwitchTestUser(t, repo, []string{"user:read:chat"})

	kickToken := tokenWithScopes("kick-mod-token", []string{"user:read", "moderation:ban"})
	if err := repo.StoreKickToken(ctx, user.ID, "MyKickSlug", "9001", kickToken, []string{"user:read", "moderation:ban"}); err != nil {
		t.Fatalf("StoreKickToken failed: %v", err)
	}

	var (
		kickUserID   string
		scopes       []string
		storedAccess string
	)
	if err := pool.QueryRow(ctx,
		`SELECT kick_user_id, granted_scopes, access_token FROM kick_oauth_tokens WHERE user_id = $1 AND channel_id = $2`,
		user.ID, "MyKickSlug",
	).Scan(&kickUserID, &scopes, &storedAccess); err != nil {
		t.Fatalf("failed to read stored kick token: %v", err)
	}
	if kickUserID != "9001" {
		t.Errorf("kick_user_id = %q, want 9001", kickUserID)
	}
	if !containsScope(scopes, "moderation:ban") {
		t.Errorf("granted_scopes missing moderation:ban: %v", scopes)
	}
	if storedAccess == "kick-mod-token" {
		t.Error("access token stored in plaintext")
	}

	// Re-storing with a NARROWER scope set must merge (union), not drop moderation:ban,
	// and must upsert (one row).
	narrower := tokenWithScopes("kick-mod-token-2", []string{"user:read"})
	if err := repo.StoreKickToken(ctx, user.ID, "MyKickSlug", "9001", narrower, []string{"user:read"}); err != nil {
		t.Fatalf("StoreKickToken upsert failed: %v", err)
	}
	var (
		rowCount int
		scopes2  []string
	)
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*), (SELECT granted_scopes FROM kick_oauth_tokens WHERE user_id = $1 AND channel_id = $2)
		 FROM kick_oauth_tokens WHERE user_id = $1 AND channel_id = $2`,
		user.ID, "MyKickSlug",
	).Scan(&rowCount, &scopes2); err != nil {
		t.Fatalf("failed to count rows: %v", err)
	}
	if rowCount != 1 {
		t.Errorf("expected 1 row after upsert, got %d", rowCount)
	}
	if !containsScope(scopes2, "moderation:ban") {
		t.Errorf("upsert dropped moderation:ban (should union): %v", scopes2)
	}
}

func TestStoreYouTubeToken_MergesModerationScope(t *testing.T) {
	pool, cleanup := setupLinkTestDB(t)
	defer cleanup()

	_, repo := newLinkTestHandler(t, pool)
	ctx := context.Background()

	user := createTwitchTestUser(t, repo, []string{"user:read:chat"})
	const forceSSL = "https://www.googleapis.com/auth/youtube.force-ssl"
	const readonly = "https://www.googleapis.com/auth/youtube.readonly"

	// Moderation re-consent stores the force-ssl grant.
	modToken := tokenWithScopes("yt-mod-token", []string{readonly, forceSSL})
	if err := repo.StoreYouTubeToken(ctx, user.ID, "UCabc", modToken, []string{readonly, forceSSL}); err != nil {
		t.Fatalf("StoreYouTubeToken (mod) failed: %v", err)
	}

	// A later plain add-source (login scopes only) must NOT drop the force-ssl grant.
	plainToken := tokenWithScopes("yt-plain-token", []string{readonly})
	if err := repo.StoreYouTubeToken(ctx, user.ID, "UCabc", plainToken, []string{readonly}); err != nil {
		t.Fatalf("StoreYouTubeToken (plain) failed: %v", err)
	}

	var scopes []string
	if err := pool.QueryRow(ctx,
		`SELECT granted_scopes FROM youtube_oauth_tokens WHERE user_id = $1 AND channel_id = $2`,
		user.ID, "UCabc",
	).Scan(&scopes); err != nil {
		t.Fatalf("failed to read youtube scopes: %v", err)
	}
	if !containsScope(scopes, forceSSL) {
		t.Errorf("plain add-source clobbered the force-ssl moderation grant: %v", scopes)
	}
}

func TestGetPlatformGrantedScopes(t *testing.T) {
	pool, cleanup := setupLinkTestDB(t)
	defer cleanup()

	_, repo := newLinkTestHandler(t, pool)
	ctx := context.Background()

	// A Twitch-login streamer who linked both Kick and YouTube for moderation.
	user := createTwitchTestUser(t, repo, []string{"user:read:chat", "moderator:manage:chat_messages"})

	// Primary platform (login provider) reads the users row.
	twScopes, err := repo.GetPlatformGrantedScopes(ctx, user.ID, "twitch")
	if err != nil {
		t.Fatalf("GetPlatformGrantedScopes(twitch) failed: %v", err)
	}
	if !containsScope(twScopes, "moderator:manage:chat_messages") {
		t.Errorf("twitch (primary) scopes missing the users-row grant: %v", twScopes)
	}

	// A linked platform with no credential yet returns empty (not the users-row scopes).
	kickScopes, err := repo.GetPlatformGrantedScopes(ctx, user.ID, "kick")
	if err != nil {
		t.Fatalf("GetPlatformGrantedScopes(kick, empty) failed: %v", err)
	}
	if len(kickScopes) != 0 {
		t.Errorf("kick scopes should be empty before any Kick link, got %v", kickScopes)
	}

	// After linking Kick, it returns the per-link scopes — and NOT the cross-platform
	// Twitch login scopes (the bug: a Twitch scope must never leak into a Kick consent).
	kickToken := tokenWithScopes("k", []string{"user:read", "moderation:ban"})
	if err := repo.StoreKickToken(ctx, user.ID, "slug", "1", kickToken, []string{"user:read", "moderation:ban"}); err != nil {
		t.Fatalf("StoreKickToken failed: %v", err)
	}
	kickScopes, err = repo.GetPlatformGrantedScopes(ctx, user.ID, "kick")
	if err != nil {
		t.Fatalf("GetPlatformGrantedScopes(kick) failed: %v", err)
	}
	if !containsScope(kickScopes, "moderation:ban") {
		t.Errorf("kick scopes missing moderation:ban: %v", kickScopes)
	}
	if containsScope(kickScopes, "user:read:chat") || containsScope(kickScopes, "moderator:manage:chat_messages") {
		t.Errorf("cross-platform Twitch scopes leaked into kick scopes: %v", kickScopes)
	}
}
