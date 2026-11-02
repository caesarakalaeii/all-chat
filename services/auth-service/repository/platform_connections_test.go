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
	"errors"
	"sort"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

// connectionsTestRepo builds a repository over the full migration set with two
// accounts: a YouTube-login streamer who linked Twitch, Kick, Facebook and
// Instagram, and a bystander whose rows must survive every disconnect.
func connectionsTestRepo(t *testing.T) (repo *UserRepository, streamer, bystander string, cleanup func()) {
	t.Helper()
	if testing.Short() {
		t.Skip("needs PostgreSQL")
	}
	pool, cleanup := setupMigrationTestDB(t)
	runMigrations(t, pool, loadUpMigrations(t))
	repo = NewUserRepository(pool, reverseCipher{})
	ctx := context.Background()

	insertUser := func(provider, idColumn, username string) string {
		var id string
		if err := pool.QueryRow(ctx, `
			INSERT INTO users (auth_provider, `+idColumn+`, username, display_name, access_token, refresh_token, token_expires_at)
			VALUES ($1, $2, $2, $2, 'login-access', 'login-refresh', NOW() + INTERVAL '1 hour')
			RETURNING id`, provider, username).Scan(&id); err != nil {
			t.Fatalf("insert user %s: %v", username, err)
		}
		return id
	}
	streamer = insertUser("youtube", "google_id", "streamer")
	bystander = insertUser("twitch", "twitch_id", "bystander")

	tok := func(access string) *oauth2.Token {
		return &oauth2.Token{AccessToken: access, RefreshToken: access + "-refresh", Expiry: time.Now().Add(time.Hour)}
	}
	for _, uid := range []string{streamer, bystander} {
		must(t, repo.StoreTwitchToken(ctx, uid, "1001", "streamer_tw", tok("tw-"+uid), nil))
		must(t, repo.StoreKickToken(ctx, uid, "streamer-kick", "2002", tok("kick-"+uid), nil))
		must(t, repo.StoreYouTubeToken(ctx, uid, "UCstreamer", tok("yt-"+uid), nil))
		must(t, repo.StoreFacebookPageToken(ctx, uid, "page-1", "Streamer Page", "fb-"+uid, nil))
		must(t, repo.StoreInstagramToken(ctx, uid, "ig-1", "streamer.ig", "ig-"+uid, nil, nil))
		must(t, repo.StoreModCredential(ctx, uid, "twitch", "1001", "streamer_tw", tok("mod-"+uid), []string{"moderator:manage:chat_messages"}))
	}
	return repo, streamer, bystander, cleanup
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func countRows(t *testing.T, repo *UserRepository, table, userID string) int {
	t.Helper()
	var n int
	if err := repo.db.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM "+table+" WHERE user_id = $1", userID).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

func TestListPlatformConnections(t *testing.T) {
	repo, streamer, _, cleanup := connectionsTestRepo(t)
	defer cleanup()

	got, err := repo.ListPlatformConnections(context.Background(), streamer)
	if err != nil {
		t.Fatalf("ListPlatformConnections: %v", err)
	}
	byPlatform := map[string]PlatformConnection{}
	for _, c := range got {
		byPlatform[c.Platform] = c
	}

	want := map[string][]string{
		"twitch":    {"streamer_tw"},
		"kick":      {"streamer-kick"},
		"youtube":   {"UCstreamer"},
		"facebook":  {"Streamer Page"},
		"instagram": {"streamer.ig"},
	}
	for platform, accounts := range want {
		c, ok := byPlatform[platform]
		if !ok {
			t.Errorf("%s missing from connections", platform)
			continue
		}
		sort.Strings(c.Accounts)
		if len(c.Accounts) != len(accounts) || c.Accounts[0] != accounts[0] {
			t.Errorf("%s accounts = %v, want %v", platform, c.Accounts, accounts)
		}
	}
	if !byPlatform["youtube"].IsLogin {
		t.Error("youtube is the login provider and must be flagged is_login")
	}
	if byPlatform["twitch"].IsLogin {
		t.Error("twitch is a linked platform, not the login")
	}
}

// The login provider is listed even when no per-link row exists: its tokens live on
// the users row, and hiding it would hide a credential we hold.
func TestListPlatformConnections_LoginWithoutLinkRows(t *testing.T) {
	repo, _, bystander, cleanup := connectionsTestRepo(t)
	defer cleanup()
	ctx := context.Background()
	for _, table := range []string{"twitch_oauth_tokens", "mod_oauth_credentials"} {
		if _, err := repo.db.Exec(ctx, `DELETE FROM `+table+` WHERE user_id = $1`, bystander); err != nil {
			t.Fatal(err)
		}
	}

	got, err := repo.ListPlatformConnections(ctx, bystander)
	if err != nil {
		t.Fatalf("ListPlatformConnections: %v", err)
	}
	for _, c := range got {
		if c.Platform == "twitch" {
			if !c.IsLogin || len(c.Accounts) != 1 || c.Accounts[0] != "bystander" {
				t.Errorf("twitch login connection = %+v, want is_login with the username", c)
			}
			return
		}
	}
	t.Error("the twitch login provider is missing from connections")
}

func TestDisconnectPlatform_DeletesEveryCredentialOfThatPlatformOnly(t *testing.T) {
	repo, streamer, bystander, cleanup := connectionsTestRepo(t)
	defer cleanup()
	ctx := context.Background()

	removed, err := repo.DisconnectPlatform(ctx, streamer, "twitch")
	if err != nil {
		t.Fatalf("DisconnectPlatform: %v", err)
	}

	if n := countRows(t, repo, "twitch_oauth_tokens", streamer); n != 0 {
		t.Errorf("twitch_oauth_tokens rows left = %d, want 0", n)
	}
	var modRows int
	must(t, repo.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM mod_oauth_credentials WHERE user_id = $1 AND platform = 'twitch'`, streamer).Scan(&modRows))
	if modRows != 0 {
		t.Errorf("twitch moderator credential left behind (%d rows)", modRows)
	}
	for _, table := range []string{"kick_oauth_tokens", "youtube_oauth_tokens", "facebook_oauth_tokens", "instagram_oauth_tokens"} {
		if n := countRows(t, repo, table, streamer); n != 1 {
			t.Errorf("%s rows = %d, want 1 (another platform must be untouched)", table, n)
		}
	}
	if n := countRows(t, repo, "twitch_oauth_tokens", bystander); n != 1 {
		t.Errorf("bystander twitch rows = %d, want 1", n)
	}

	// The removed tokens come back decrypted, so they can be revoked at the provider.
	got := map[string]bool{}
	for _, r := range removed {
		got[r.AccessToken] = true
		got[r.RefreshToken] = true
	}
	for _, want := range []string{"tw-" + streamer, "tw-" + streamer + "-refresh", "mod-" + streamer, "mod-" + streamer + "-refresh"} {
		if !got[want] {
			t.Errorf("removed tokens %v do not include %q", removed, want)
		}
	}
}

func TestDisconnectPlatform_MetaPlatformsHaveNoRefreshToken(t *testing.T) {
	repo, streamer, _, cleanup := connectionsTestRepo(t)
	defer cleanup()
	ctx := context.Background()

	for _, platform := range []string{"facebook", "instagram"} {
		removed, err := repo.DisconnectPlatform(ctx, streamer, platform)
		if err != nil {
			t.Fatalf("DisconnectPlatform(%s): %v", platform, err)
		}
		if len(removed) != 1 || removed[0].AccessToken == "" || removed[0].RefreshToken != "" {
			t.Errorf("%s removed = %+v, want one page token and no refresh token", platform, removed)
		}
	}
	for _, table := range []string{"facebook_oauth_tokens", "instagram_oauth_tokens"} {
		if n := countRows(t, repo, table, streamer); n != 0 {
			t.Errorf("%s rows left = %d", table, n)
		}
	}
}

// A Facebook or Instagram source cannot work without its Page token (overlay-manager
// refuses to create one without it, and the listeners poll with it), so disconnecting
// removes those sources from the user's overlays. Sources of platforms that can run
// without the user's token (Twitch, YouTube, Kick) stay.
func TestDisconnectPlatform_RemovesSourcesThatNeedTheToken(t *testing.T) {
	repo, streamer, bystander, cleanup := connectionsTestRepo(t)
	defer cleanup()
	ctx := context.Background()

	addSources := func(userID string) string {
		var overlayID string
		must(t, repo.db.QueryRow(ctx,
			`INSERT INTO overlays (user_id, name) VALUES ($1, 'main') RETURNING id`, userID).Scan(&overlayID))
		for _, s := range [][2]string{{"facebook", "page-1"}, {"instagram", "ig-1"}, {"twitch", "streamer_tw"}, {"kick", "streamer-kick"}} {
			_, err := repo.db.Exec(ctx,
				`INSERT INTO overlay_chat_sources (overlay_id, platform, channel_id, channel_name) VALUES ($1, $2, $3, $3)`,
				overlayID, s[0], s[1])
			must(t, err)
		}
		return overlayID
	}
	streamerOverlay := addSources(streamer)
	bystanderOverlay := addSources(bystander)

	sourceCount := func(overlayID, platform string) int {
		var n int
		must(t, repo.db.QueryRow(ctx,
			`SELECT COUNT(*) FROM overlay_chat_sources WHERE overlay_id = $1 AND platform = $2`, overlayID, platform).Scan(&n))
		return n
	}

	for _, platform := range []string{"facebook", "instagram", "twitch", "kick"} {
		if _, err := repo.DisconnectPlatform(ctx, streamer, platform); err != nil {
			t.Fatalf("DisconnectPlatform(%s): %v", platform, err)
		}
	}

	for platform, want := range map[string]int{"facebook": 0, "instagram": 0, "twitch": 1, "kick": 1} {
		if got := sourceCount(streamerOverlay, platform); got != want {
			t.Errorf("streamer %s sources = %d, want %d", platform, got, want)
		}
		if got := sourceCount(bystanderOverlay, platform); got != 1 {
			t.Errorf("bystander %s sources = %d, want 1 (another account's overlay is never touched)", platform, got)
		}
	}
}

// The login provider's credentials back the account itself; they go only with the
// account (DELETE /me). Refusing here also leaves the per-link rows of the login
// platform in place, since the next sign-in would write them straight back.
func TestDisconnectPlatform_RefusesTheLoginProvider(t *testing.T) {
	repo, streamer, _, cleanup := connectionsTestRepo(t)
	defer cleanup()
	ctx := context.Background()

	_, err := repo.DisconnectPlatform(ctx, streamer, "youtube")
	if !errors.Is(err, ErrLoginPlatform) {
		t.Fatalf("err = %v, want ErrLoginPlatform", err)
	}
	if n := countRows(t, repo, "youtube_oauth_tokens", streamer); n != 1 {
		t.Errorf("youtube rows = %d, want 1 (a refused disconnect deletes nothing)", n)
	}
}

func TestDisconnectPlatform_UnknownPlatform(t *testing.T) {
	repo, streamer, _, cleanup := connectionsTestRepo(t)
	defer cleanup()

	if _, err := repo.DisconnectPlatform(context.Background(), streamer, "users; DROP TABLE users"); !errors.Is(err, ErrUnsupportedPlatform) {
		t.Fatalf("err = %v, want ErrUnsupportedPlatform", err)
	}
}
