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

package refresher_test

import (
	"context"
	"testing"
	"time"

	authOAuth "github.com/caesar/all-chat/services/auth-service/oauth"
	"github.com/caesar/all-chat/services/token-refresh-service/repository"
	"golang.org/x/oauth2"
)

// modProviders returns providers that refresh successfully for both delegated platforms.
func modProviders() map[authOAuth.Platform]authOAuth.OAuthProvider {
	fresh := &oauth2.Token{AccessToken: "new-access", RefreshToken: "new-refresh", Expiry: time.Now().Add(4 * time.Hour)}
	return map[authOAuth.Platform]authOAuth.OAuthProvider{
		authOAuth.PlatformTwitch: &fakeProvider{platform: authOAuth.PlatformTwitch, token: fresh},
		authOAuth.PlatformKick:   &fakeProvider{platform: authOAuth.PlatformKick, token: fresh},
	}
}

// Delegated-moderator credentials (ADR-0048) must be refreshed like every other credential.
// Without this loop the credential simply expires: the grant keeps looking active in the UI
// while every action fails, and the 90-day dormancy rule can never fire because the moderator
// stopped being able to act long before that.

func modCredential(userID, platform string) *repository.ExpiringToken {
	return &repository.ExpiringToken{
		ID:           userID,
		Platform:     platform,
		ChannelID:    platform, // the row is keyed (user_id, platform)
		Username:     "volunteer_mod",
		TokenType:    "mod_credential",
		AccessToken:  "old-access",
		RefreshToken: "refresh-me",
		ExpiresAt:    time.Now().Add(2 * time.Minute),
	}
}

func TestProcessBatch_RefreshesModCredentials(t *testing.T) {
	repo := &fakeRepo{
		modCredentials: []*repository.ExpiringToken{modCredential("mod-1", "twitch")},
	}
	m := newTestManager(repo, modProviders())

	if err := m.ProcessBatch(context.Background()); err != nil {
		t.Fatalf("ProcessBatch: %v", err)
	}

	repo.mu.Lock()
	defer repo.mu.Unlock()
	if len(repo.updatedModCreds) != 1 {
		t.Fatalf("got %d moderator credential updates, want 1", len(repo.updatedModCreds))
	}
	got := repo.updatedModCreds[0]
	if got.userID != "mod-1" {
		t.Errorf("userID = %q, want mod-1", got.userID)
	}
	if got.platform != "twitch" {
		t.Errorf("platform = %q, want twitch — the row is keyed (user_id, platform), so a wrong value here silently updates nothing", got.platform)
	}
}

// The table spans platforms, unlike every other source in this loop, so each row's platform must
// come from the row rather than being assumed.
func TestProcessBatch_ModCredentialsKeepTheirOwnPlatform(t *testing.T) {
	repo := &fakeRepo{
		modCredentials: []*repository.ExpiringToken{
			modCredential("mod-1", "twitch"),
			modCredential("mod-2", "kick"),
		},
	}
	m := newTestManager(repo, modProviders())

	if err := m.ProcessBatch(context.Background()); err != nil {
		t.Fatalf("ProcessBatch: %v", err)
	}

	repo.mu.Lock()
	defer repo.mu.Unlock()
	seen := map[string]string{}
	for _, u := range repo.updatedModCreds {
		seen[u.userID] = u.platform
	}
	if seen["mod-1"] != "twitch" {
		t.Errorf("mod-1 updated with platform %q, want twitch", seen["mod-1"])
	}
	if seen["mod-2"] != "kick" {
		t.Errorf("mod-2 updated with platform %q, want kick", seen["mod-2"])
	}
}

// A moderator credential must never be written back through another source's updater — that
// would target the wrong table and, worse, could write a moderator's token into a row a
// listener selects from by channel.
func TestProcessBatch_ModCredentialDoesNotTouchOtherTables(t *testing.T) {
	repo := &fakeRepo{
		modCredentials: []*repository.ExpiringToken{modCredential("mod-1", "twitch")},
	}
	m := newTestManager(repo, modProviders())

	if err := m.ProcessBatch(context.Background()); err != nil {
		t.Fatalf("ProcessBatch: %v", err)
	}

	repo.mu.Lock()
	defer repo.mu.Unlock()
	if len(repo.updatedTwitchLinks) != 0 {
		t.Errorf("a moderator credential was written to twitch_oauth_tokens (%d calls) — that table is selected from by channel with no user scoping", len(repo.updatedTwitchLinks))
	}
}
