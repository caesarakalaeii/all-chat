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

package repository

import (
	"context"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

// reverseCipher is a trivially reversible stand-in for the real encryptor: enough to prove the
// token is transformed on the way in and recoverable, without pulling key management into the test.
type reverseCipher struct{}

func (reverseCipher) Encrypt(s string) (string, error) { return reverse(s), nil }
func (reverseCipher) Decrypt(s string) (string, error) { return reverse(s), nil }

func reverse(s string) string {
	r := []rune(s)
	for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}
	return string(r)
}

// modCredTestRepo builds a repository over a database with the full migration set applied, so
// mod_oauth_credentials matches migration 080 exactly (constraints included).
func modCredTestRepo(t *testing.T) (*UserRepository, string, func()) {
	t.Helper()
	pool, cleanup := setupMigrationTestDB(t)
	runMigrations(t, pool, loadUpMigrations(t))

	ctx := context.Background()
	var modID string
	err := pool.QueryRow(ctx, `
		INSERT INTO users (twitch_id, auth_provider, username, display_name,
		                   access_token, refresh_token, token_expires_at)
		VALUES ('717171', 'twitch', 'volunteer_mod', 'Volunteer Mod',
		        'access', 'refresh', NOW() + INTERVAL '4 hours')
		RETURNING id`).Scan(&modID)
	if err != nil {
		t.Fatalf("failed to insert moderator: %v", err)
	}

	return NewUserRepository(pool, reverseCipher{}), modID, cleanup
}

func TestStoreModCredential(t *testing.T) {
	repo, modID, cleanup := modCredTestRepo(t)
	defer cleanup()
	ctx := context.Background()

	token := &oauth2.Token{
		AccessToken:  "mod-access",
		RefreshToken: "mod-refresh",
		TokenType:    "bearer",
		Expiry:       time.Now().Add(4 * time.Hour),
	}

	err := repo.StoreModCredential(ctx, modID, "twitch", "717171", "volunteer_mod",
		token, []string{"moderator:manage:chat_messages"})
	if err != nil {
		t.Fatalf("StoreModCredential: %v", err)
	}

	var stored, platformUserID string
	var scopes []string
	var version int
	err = repo.db.QueryRow(ctx, `
		SELECT access_token, platform_user_id, granted_scopes, encryption_version
		FROM mod_oauth_credentials WHERE user_id = $1 AND platform = 'twitch'`, modID).
		Scan(&stored, &platformUserID, &scopes, &version)
	if err != nil {
		t.Fatalf("reading back the credential: %v", err)
	}

	if stored == "mod-access" {
		t.Error("the access token was stored in plaintext")
	}
	if reverse(stored) != "mod-access" {
		t.Errorf("stored token does not decrypt to the original: %q", stored)
	}
	if platformUserID != "717171" {
		t.Errorf("platform_user_id = %q, want 717171 (this is what we send as moderator_id)", platformUserID)
	}
	if version != 1 {
		t.Errorf("encryption_version = %d, want 1", version)
	}
	if len(scopes) != 1 || scopes[0] != "moderator:manage:chat_messages" {
		t.Errorf("granted_scopes = %v", scopes)
	}
}

// Exactly one row per (moderator, platform): one refresh owner, so nothing races
// token-refresh-service. Re-consenting replaces rather than accumulating.
func TestStoreModCredential_ReplacesRatherThanAccumulates(t *testing.T) {
	repo, modID, cleanup := modCredTestRepo(t)
	defer cleanup()
	ctx := context.Background()

	first := &oauth2.Token{AccessToken: "first", RefreshToken: "r1", Expiry: time.Now().Add(time.Hour)}
	if err := repo.StoreModCredential(ctx, modID, "twitch", "111", "acct-one", first, []string{"moderator:manage:chat_messages"}); err != nil {
		t.Fatalf("first store: %v", err)
	}

	// A different Twitch account for the same All-Chat user.
	second := &oauth2.Token{AccessToken: "second", RefreshToken: "r2", Expiry: time.Now().Add(time.Hour)}
	if err := repo.StoreModCredential(ctx, modID, "twitch", "222", "acct-two", second, []string{"moderator:manage:banned_users"}); err != nil {
		t.Fatalf("second store: %v", err)
	}

	var count int
	if err := repo.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM mod_oauth_credentials WHERE user_id = $1 AND platform = 'twitch'`, modID).
		Scan(&count); err != nil {
		t.Fatalf("counting rows: %v", err)
	}
	if count != 1 {
		t.Errorf("got %d credential rows, want exactly 1 — a second row means two refresh owners", count)
	}

	var platformUserID string
	var scopes []string
	if err := repo.db.QueryRow(ctx,
		`SELECT platform_user_id, granted_scopes FROM mod_oauth_credentials WHERE user_id = $1 AND platform = 'twitch'`, modID).
		Scan(&platformUserID, &scopes); err != nil {
		t.Fatalf("reading back: %v", err)
	}
	if platformUserID != "222" {
		t.Errorf("platform_user_id = %q, want the most recent consent (222)", platformUserID)
	}
	if len(scopes) != 1 || scopes[0] != "moderator:manage:banned_users" {
		t.Errorf("granted_scopes = %v, want the newest grant's scopes", scopes)
	}
}

// Distinct platforms coexist: a moderator can consent for Twitch and Kick independently.
func TestStoreModCredential_PerPlatformRows(t *testing.T) {
	repo, modID, cleanup := modCredTestRepo(t)
	defer cleanup()
	ctx := context.Background()

	for _, platform := range []string{"twitch", "kick"} {
		token := &oauth2.Token{AccessToken: "a-" + platform, Expiry: time.Now().Add(time.Hour)}
		if err := repo.StoreModCredential(ctx, modID, platform, "id-"+platform, platform+"-login", token, nil); err != nil {
			t.Fatalf("store %s: %v", platform, err)
		}
	}

	var count int
	if err := repo.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM mod_oauth_credentials WHERE user_id = $1`, modID).Scan(&count); err != nil {
		t.Fatalf("counting rows: %v", err)
	}
	if count != 2 {
		t.Errorf("got %d rows, want one per platform", count)
	}
}

func TestStoreModCredential_Rejections(t *testing.T) {
	repo, modID, cleanup := modCredTestRepo(t)
	defer cleanup()
	ctx := context.Background()

	valid := func() *oauth2.Token {
		return &oauth2.Token{AccessToken: "a", Expiry: time.Now().Add(time.Hour)}
	}

	cases := []struct {
		name       string
		user       string
		platform   string
		platformID string
		token      *oauth2.Token
		wantSubstr string
	}{
		{"no user id", "", "twitch", "1", valid(), "user_id"},
		{"no platform", modID, "", "1", valid(), "platform is required"},
		{"no platform user id", modID, "twitch", "", valid(), "platform_user_id"},
		{"nil token", modID, "twitch", "1", nil, "access token"},
		{"empty access token", modID, "twitch", "1", &oauth2.Token{}, "access token"},
		{
			"already-expired token", modID, "twitch", "1",
			&oauth2.Token{AccessToken: "a", Expiry: time.Now().Add(-time.Hour)},
			"expired",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := repo.StoreModCredential(ctx, tc.user, tc.platform, tc.platformID, "login", tc.token, nil)
			if err == nil {
				t.Fatalf("expected a rejection, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantSubstr) {
				t.Errorf("error %q does not mention %q", err, tc.wantSubstr)
			}
		})
	}
}

// Revoking a moderator's last grant must be able to leave no usable token behind.
func TestDeleteModCredential(t *testing.T) {
	repo, modID, cleanup := modCredTestRepo(t)
	defer cleanup()
	ctx := context.Background()

	token := &oauth2.Token{AccessToken: "a", Expiry: time.Now().Add(time.Hour)}
	if err := repo.StoreModCredential(ctx, modID, "twitch", "1", "login", token, nil); err != nil {
		t.Fatalf("store: %v", err)
	}
	if err := repo.DeleteModCredential(ctx, modID, "twitch"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	var count int
	if err := repo.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM mod_oauth_credentials WHERE user_id = $1`, modID).Scan(&count); err != nil {
		t.Fatalf("counting rows: %v", err)
	}
	if count != 0 {
		t.Errorf("credential survived deletion")
	}
}
