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

package oauth_test

import (
	"context"
	"net/url"
	"strconv"
	"testing"

	"github.com/caesar/all-chat/services/auth-service/oauth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiscordOAuth_GetAuthURL(t *testing.T) {
	d := oauth.NewDiscordOAuth("client123", "secret456", "https://example.com/callback")
	authURL := d.GetAuthURL("csrf-state-token")

	parsed, err := url.Parse(authURL)
	require.NoError(t, err)
	q := parsed.Query()

	assert.Equal(t, "https", parsed.Scheme)
	assert.Equal(t, "discord.com", parsed.Host)
	assert.Equal(t, "/oauth2/authorize", parsed.Path)
	assert.Equal(t, "bot", q.Get("scope"))
	assert.Equal(t, "68608", q.Get("permissions"))
	assert.Equal(t, "client123", q.Get("client_id"))
	assert.Equal(t, "csrf-state-token", q.Get("state"))
	assert.Equal(t, "https://example.com/callback", q.Get("redirect_uri"))
	assert.Equal(t, "code", q.Get("response_type"))
}

func TestDiscordOAuth_GetModerationAuthURL(t *testing.T) {
	d := oauth.NewDiscordOAuth("client123", "secret456", "https://example.com/callback")
	authURL := d.GetModerationAuthURL("csrf-state-token")

	parsed, err := url.Parse(authURL)
	require.NoError(t, err)
	q := parsed.Query()

	assert.Equal(t, "bot", q.Get("scope"))
	// The moderation re-invite requests the base permissions PLUS MANAGE_MESSAGES (delete),
	// MODERATE_MEMBERS (timeout) and BAN_MEMBERS (ban/unban).
	perms, perr := strconv.ParseUint(q.Get("permissions"), 10, 64)
	require.NoError(t, perr)
	assert.Equal(t, oauth.ModerationBotPermissions, perms)
	assert.NotZero(t, perms&oauth.PermManageMessages, "must request MANAGE_MESSAGES")
	assert.NotZero(t, perms&oauth.PermModerateMembers, "must request MODERATE_MEMBERS")
	assert.NotZero(t, perms&oauth.PermBanMembers, "must request BAN_MEMBERS")
	assert.NotZero(t, perms&oauth.RequiredBotPermissions, "must keep the base listener permissions")
}

func TestDiscordOAuth_GetPlatform(t *testing.T) {
	d := oauth.NewDiscordOAuth("id", "secret", "https://example.com/cb")
	assert.Equal(t, oauth.PlatformDiscord, d.GetPlatform())
}

func TestDiscordOAuth_GetUserInfo_ReturnsError(t *testing.T) {
	d := oauth.NewDiscordOAuth("id", "secret", "https://example.com/cb")
	info, err := d.GetUserInfo(context.Background(), "some-token")
	assert.Nil(t, info)
	assert.Error(t, err, "GetUserInfo should return an error for Discord bot auth (no user identity)")
}

func TestCheckBotPermissions_AllGranted(t *testing.T) {
	// All required bits present
	missing := oauth.ComputeMissingPermissions(68608)
	assert.Empty(t, missing, "no missing permissions when all bits set")
}

func TestCheckBotPermissions_MissingViewChannel(t *testing.T) {
	// permissions = PermSendMessages | PermReadMessageHistory (missing PermViewChannel)
	missing := oauth.ComputeMissingPermissions(2048 | 65536)
	assert.Equal(t, []string{"View Channels"}, missing)
}

func TestCheckBotPermissions_MissingMultiple(t *testing.T) {
	// permissions = 0 (nothing granted)
	missing := oauth.ComputeMissingPermissions(0)
	assert.Len(t, missing, 3)
	assert.Contains(t, missing, "View Channels")
	assert.Contains(t, missing, "Send Messages")
	assert.Contains(t, missing, "Read Message History")
}
