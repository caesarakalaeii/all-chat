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
	"testing"

	"github.com/caesar/all-chat/services/auth-service/oauth"
	"github.com/stretchr/testify/assert"
)

const (
	scopeChat = "user:read:chat"
	scopeDel  = "moderator:manage:chat_messages"
	scopeBan  = "moderator:manage:banned_users"
)

func TestWouldDowngradeScopes(t *testing.T) {
	tests := []struct {
		name     string
		existing []string
		incoming []string
		want     bool
	}{
		{"plain login over a chat grant is a downgrade", []string{scopeChat}, []string{"channel:read:subscriptions"}, true},
		{"plain login over a mod grant is a downgrade", []string{scopeBan}, []string{"channel:read:subscriptions"}, true},
		{"dropping a mod scope while keeping chat is still a downgrade", []string{scopeChat, scopeDel}, []string{scopeChat}, true},
		{"a superset is not a downgrade (the opt-in upgrade case)", []string{scopeChat}, []string{scopeChat, scopeBan}, false},
		{"identical scopes are not a downgrade", []string{scopeChat, scopeBan}, []string{scopeChat, scopeBan}, false},
		{"dropping a non-preservable scope is fine", []string{"bits:read"}, []string{}, false},
		{"no stored scopes never downgrades", nil, nil, false},
		{"adding mod scopes to a chat grant is an upgrade", []string{scopeChat}, []string{scopeChat, scopeDel, scopeBan}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, wouldDowngradeScopes(tt.existing, tt.incoming))
		})
	}
}

// The moderation re-consent requests existing ∪ action-scopes, so the issued token is
// always a superset and the guard above must permit the replacement. This ties the two
// halves together: union(existing, modScopes) must never be a downgrade of existing.
func TestModerationReconsentUnionIsNeverADowngrade(t *testing.T) {
	existing := []string{scopeChat, "channel:read:subscriptions"}
	modScopes := oauth.ModerationScopesForActions([]string{"delete", "ban"})
	merged := unionScopes(existing, modScopes)

	assert.False(t, wouldDowngradeScopes(existing, merged),
		"requesting existing ∪ mod-scopes must be an upgrade, not a downgrade")
	assert.Subset(t, merged, existing, "the union preserves every existing scope")
	assert.Subset(t, merged, modScopes, "the union adds the requested moderation scopes")
}

// The Kick re-consent likewise requests existing ∪ moderation:ban, and a plain Kick
// re-login (just user:read) must not clobber the stored moderation grant.
func TestKickModerationReconsentUnionIsNeverADowngrade(t *testing.T) {
	existing := []string{"user:read"}
	modScopes := oauth.KickModerationScopesForActions([]string{"timeout", "ban", "unban"})
	merged := unionScopes(existing, modScopes)

	assert.False(t, wouldDowngradeScopes(existing, merged),
		"requesting existing ∪ kick mod-scopes must be an upgrade, not a downgrade")
	assert.Subset(t, merged, []string{"moderation:ban"})

	// A subsequent plain Kick login (user:read only) over the mod grant is a downgrade
	// the guard must catch, so the stored moderation:ban grant is preserved.
	assert.True(t, wouldDowngradeScopes([]string{"user:read", "moderation:ban"}, []string{"user:read"}),
		"dropping the kick moderation grant on a plain re-login must be caught as a downgrade")
}

// Kick's delete scope is granted separately from its ban scope, so it needs its own entry
// in the downgrade guard: a plain re-login (or a consent for the ban scope alone) must not
// silently drop it, which would disable delete until the streamer noticed and re-consented.
func TestKickDeleteScopeIsPreserved(t *testing.T) {
	const deleteScope = "moderation:chat_message:manage"
	assert.Equal(t, []string{deleteScope}, oauth.KickModerationScopesForActions([]string{"delete"}))

	assert.True(t, wouldDowngradeScopes([]string{"user:read", deleteScope}, []string{"user:read"}),
		"a plain re-login must not drop the kick delete grant")
	assert.True(t,
		wouldDowngradeScopes(
			[]string{"user:read", deleteScope, "moderation:ban"},
			[]string{"user:read", "moderation:ban"},
		),
		"consenting for ban alone must not drop an existing delete grant")

	// And the union the re-consent actually requests is an upgrade for both scopes.
	existing := []string{"user:read", "moderation:ban"}
	merged := unionScopes(existing, oauth.KickModerationScopesForActions([]string{"delete", "timeout", "ban", "unban"}))
	assert.False(t, wouldDowngradeScopes(existing, merged))
	assert.Subset(t, merged, []string{deleteScope, "moderation:ban"})
}

func TestSplitActions(t *testing.T) {
	assert.Equal(t, []string{"delete", "ban"}, splitActions(" delete , ban "))
	assert.Equal(t, []string{"delete"}, splitActions("delete"))
	assert.Empty(t, splitActions(""))
	assert.Empty(t, splitActions("  ,  ,"))
}

// Google's granular consent screen lets a user approve the profile scope while
// declining youtube.readonly. The YouTube add-source flow needs youtube.readonly to
// resolve the channel (channels?mine=true); when it is absent the callback must
// surface an actionable error instead of letting GetPrimaryChannel 403 into an
// opaque 500. grantedYouTubeChannelAccess is that gate. (prod outage 2026-06-27)
func TestGrantedYouTubeChannelAccess(t *testing.T) {
	profile := "https://www.googleapis.com/auth/userinfo.profile"
	tests := []struct {
		name   string
		scopes []string
		want   bool
	}{
		{"approved youtube.readonly", []string{profile, oauth.YouTubeReadonlyScope}, true},
		{"declined youtube.readonly on consent screen", []string{"profile", profile}, false},
		{"only profile granted", []string{profile}, false},
		{"no scopes granted", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, grantedYouTubeChannelAccess(tt.scopes))
		})
	}
}
