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

package oauth

import (
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const ytForceSSL = "https://www.googleapis.com/auth/youtube.force-ssl"

func TestYouTubeModerationScopesForActions(t *testing.T) {
	assert.Equal(t, []string{ytForceSSL}, YouTubeModerationScopesForActions([]string{"ban"}))
	assert.Empty(t, YouTubeModerationScopesForActions([]string{"delete"}), "unsupported actions are ignored")
	assert.Empty(t, YouTubeModerationScopesForActions(nil))
	// Deduped even if ban appears with other (ignored) actions.
	assert.Equal(t, []string{ytForceSSL}, YouTubeModerationScopesForActions([]string{"ban", "timeout"}))
}

func TestYouTubeGetAuthURLWithScopes(t *testing.T) {
	o := NewYouTubeOAuth("cid", "secret", "http://localhost/cb")
	extra := []string{"https://www.googleapis.com/auth/youtube.readonly", ytForceSSL}

	raw := o.GetAuthURLWithScopes("state123", extra)
	u, err := url.Parse(raw)
	require.NoError(t, err)
	q := u.Query()

	assert.Equal(t, "state123", q.Get("state"))
	assert.Equal(t, "consent", q.Get("prompt"), "prompt=consent re-prompts for the new scope and reissues a refresh token")
	assert.Equal(t, "offline", q.Get("access_type"))

	scopes := strings.Fields(q.Get("scope"))
	assert.Contains(t, scopes, ytForceSSL, "the moderation scope is requested")
	assert.Contains(t, scopes, "https://www.googleapis.com/auth/userinfo.profile", "base login scopes are still requested")

	count := 0
	for _, s := range scopes {
		if s == "https://www.googleapis.com/auth/youtube.readonly" {
			count++
		}
	}
	assert.Equal(t, 1, count, "a scope present in both base and extra must appear once")
}
