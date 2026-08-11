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

package clients

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// guildFixture describes a fake guild for the member-authority reads: who owns it, which
// roles exist (id → permissions/position), and which roles each member holds.
type guildFixture struct {
	ownerID string
	roles   string // raw JSON array
	members map[string]string
	calls   map[string]int
}

// newMemberAuthorityServer serves GET /guilds/{g}, /guilds/{g}/members/{u} and
// /guilds/{g}/roles from a fixture, counting calls per path prefix.
func newMemberAuthorityServer(t *testing.T, f *guildFixture) *httptest.Server {
	t.Helper()
	if f.calls == nil {
		f.calls = map[string]int{}
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		switch {
		case p == "/users/@me":
			f.calls["identity"]++
			_, _ = w.Write([]byte(`{"id":"bot-1"}`))
		case strings.HasSuffix(p, "/roles"):
			f.calls["roles"]++
			_, _ = w.Write([]byte(f.roles))
		case strings.Contains(p, "/members/"):
			f.calls["member"]++
			uid := p[strings.LastIndex(p, "/")+1:]
			roles, ok := f.members[uid]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"message":"Unknown User","code":10013}`))
				return
			}
			_, _ = w.Write([]byte(`{"roles":` + roles + `}`))
		case strings.HasPrefix(p, "/guilds/"):
			f.calls["guild"]++
			_, _ = w.Write([]byte(`{"id":"g","owner_id":"` + f.ownerID + `"}`))
		default:
			t.Errorf("unexpected path %s", p)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
}

// standardGuild: @everyone (id == guild id "g") at position 0 with no permissions; "mods"
// at 5 with MANAGE_MESSAGES|MODERATE_MEMBERS; "admins" at 9 with ADMINISTRATOR; "unheld"
// at 20 to prove a role nobody in the fixture holds is never folded in.
func standardGuild() *guildFixture {
	return &guildFixture{
		ownerID: "owner-1",
		roles: `[{"id":"g","permissions":"0","position":0},
		         {"id":"mods","permissions":"1099511635968","position":5},
		         {"id":"admins","permissions":"8","position":9},
		         {"id":"unheld","permissions":"4","position":20}]`,
		members: map[string]string{
			"owner-1": `[]`,
			"mod-1":   `["mods"]`,
			"admin-1": `["admins"]`,
			"both-1":  `["mods","admins"]`,
			"plain-1": `[]`,
		},
	}
}

func TestDiscordMemberAuthority(t *testing.T) {
	f := standardGuild()
	srv := newMemberAuthorityServer(t, f)
	defer srv.Close()
	c := &DiscordClient{httpClient: srv.Client(), botToken: "bot-tok", baseURL: srv.URL}

	t.Run("a moderator's bits and highest position", func(t *testing.T) {
		m, err := c.MemberAuthority(context.Background(), "g", "mod-1")
		require.NoError(t, err)
		assert.True(t, m.InGuild)
		assert.False(t, m.IsGuildOwner)
		// MANAGE_MESSAGES (8192) | MODERATE_MEMBERS (1<<40) = 1099511635968
		assert.Equal(t, uint64(8192|(1<<40)), m.Permissions)
		assert.Equal(t, 5, m.HighestRolePos)
	})

	t.Run("multiple roles OR their permissions and take the highest position", func(t *testing.T) {
		m, err := c.MemberAuthority(context.Background(), "g", "both-1")
		require.NoError(t, err)
		assert.Equal(t, uint64(8192|(1<<40)|8), m.Permissions)
		assert.Equal(t, 9, m.HighestRolePos, "highest of the held roles, not the sum or the last seen")
	})

	t.Run("the guild owner is flagged", func(t *testing.T) {
		m, err := c.MemberAuthority(context.Background(), "g", "owner-1")
		require.NoError(t, err)
		assert.True(t, m.IsGuildOwner)
		assert.True(t, m.InGuild)
	})

	t.Run("an @everyone-only member sits at position 0", func(t *testing.T) {
		m, err := c.MemberAuthority(context.Background(), "g", "plain-1")
		require.NoError(t, err)
		assert.True(t, m.InGuild)
		assert.Equal(t, uint64(0), m.Permissions)
		assert.Equal(t, 0, m.HighestRolePos)
	})

	t.Run("a non-member is not in the guild and holds nothing", func(t *testing.T) {
		m, err := c.MemberAuthority(context.Background(), "g", "stranger")
		require.NoError(t, err, "404 is an answer, not a failure — it means not a member")
		assert.False(t, m.InGuild)
		assert.Equal(t, uint64(0), m.Permissions)
	})
}

// TestDiscordMemberAuthority_EveryoneIsAlwaysIncluded: @everyone is implicit — Discord does
// not list it in a member's roles — so a permission granted to @everyone must still be read.
func TestDiscordMemberAuthority_EveryoneIsAlwaysIncluded(t *testing.T) {
	f := &guildFixture{
		ownerID: "owner-1",
		roles:   `[{"id":"g","permissions":"8192","position":0}]`,
		members: map[string]string{"plain-1": `[]`},
	}
	srv := newMemberAuthorityServer(t, f)
	defer srv.Close()
	c := &DiscordClient{httpClient: srv.Client(), botToken: "bot-tok", baseURL: srv.URL}

	m, err := c.MemberAuthority(context.Background(), "g", "plain-1")
	require.NoError(t, err)
	assert.Equal(t, uint64(8192), m.Permissions, "@everyone's id is the guild id and is always held")
}

// TestDiscordMemberAuthority_NoGuildOrRoleReadWhenNotAMember: a 404 short-circuits, so a
// stranger costs one call rather than three.
func TestDiscordMemberAuthority_NoGuildOrRoleReadWhenNotAMember(t *testing.T) {
	f := standardGuild()
	srv := newMemberAuthorityServer(t, f)
	defer srv.Close()
	c := &DiscordClient{httpClient: srv.Client(), botToken: "bot-tok", baseURL: srv.URL}

	_, err := c.MemberAuthority(context.Background(), "g", "stranger")
	require.NoError(t, err)
	assert.Equal(t, 1, f.calls["member"])
	assert.Zero(t, f.calls["roles"], "no point reading roles for someone who holds none")
	assert.Zero(t, f.calls["guild"], "no point asking who owns the guild either")
}

// TestDiscordMemberAuthority_FailsClosed: any error other than a 404 must surface. Discord
// is the platform with no external backstop, so a swallowed error would become an
// authorization decision made on no information.
func TestDiscordMemberAuthority_FailsClosed(t *testing.T) {
	tests := []struct {
		name   string
		failOn string
		status int
		wantIs error
	}{
		{"unauthorized bot token on the member read", "member", http.StatusUnauthorized, ErrDiscordUnauthorized},
		{"forbidden on the member read", "member", http.StatusForbidden, ErrDiscordForbidden},
		{"server error on the member read", "member", http.StatusInternalServerError, nil},
		{"server error on the roles read", "roles", http.StatusInternalServerError, nil},
		{"server error on the guild read", "guild", http.StatusBadGateway, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				p := r.URL.Path
				kind := "guild"
				switch {
				case strings.HasSuffix(p, "/roles"):
					kind = "roles"
				case strings.Contains(p, "/members/"):
					kind = "member"
				}
				if kind == tc.failOn {
					w.WriteHeader(tc.status)
					return
				}
				switch kind {
				case "roles":
					_, _ = w.Write([]byte(`[{"id":"g","permissions":"0","position":0}]`))
				case "member":
					_, _ = w.Write([]byte(`{"roles":[]}`))
				default:
					_, _ = w.Write([]byte(`{"owner_id":"owner-1"}`))
				}
			}))
			defer srv.Close()
			c := &DiscordClient{httpClient: srv.Client(), botToken: "bot-tok", baseURL: srv.URL}

			m, err := c.MemberAuthority(context.Background(), "g", "mod-1")
			require.Error(t, err, "an unreadable standing must not be reported as a standing")
			if tc.wantIs != nil {
				assert.ErrorIs(t, err, tc.wantIs)
			}
			assert.False(t, m.InGuild, "the zero value denies everything")
		})
	}
}

// TestDiscordMemberAuthority_MalformedPositionIsAnError: a role position we cannot parse
// would silently rank a member at 0 and defeat the hierarchy check.
func TestDiscordMemberAuthority_MalformedPermissionsIsAnError(t *testing.T) {
	f := &guildFixture{
		ownerID: "owner-1",
		roles:   `[{"id":"g","permissions":"not-a-number","position":0}]`,
		members: map[string]string{"mod-1": `[]`},
	}
	srv := newMemberAuthorityServer(t, f)
	defer srv.Close()
	c := &DiscordClient{httpClient: srv.Client(), botToken: "bot-tok", baseURL: srv.URL}

	_, err := c.MemberAuthority(context.Background(), "g", "mod-1")
	require.Error(t, err)
}

// TestDiscordGuildBotPermissions_SharesTheMemberMachinery: the bot's own permissions are the
// same computation over the same reads, so the two must not drift. The bot cannot own a
// guild, so its path skips the ownership read.
func TestDiscordGuildBotPermissions_SkipsTheGuildOwnerRead(t *testing.T) {
	f := standardGuild()
	f.members["bot-1"] = `["mods"]`
	srv := newMemberAuthorityServer(t, f)
	defer srv.Close()
	c := &DiscordClient{httpClient: srv.Client(), botToken: "bot-tok", baseURL: srv.URL}

	perms, err := c.GuildBotPermissions(context.Background(), "g")
	require.NoError(t, err)
	assert.Equal(t, uint64(8192|(1<<40)), perms)
	assert.Zero(t, f.calls["guild"], "a bot can never be the guild owner, so that read is waste")
}

// TestDiscordGuildResolver_MemberAuthorityCacheTTLIsASecurityBound: ADR-0048 makes this TTL
// a security property — with GUILD_MEMBERS off Discord cannot push us a revocation, so the
// TTL is exactly how long a removed moderator keeps acting.
func TestDiscordGuildResolver_MemberAuthorityCacheTTLIsASecurityBound(t *testing.T) {
	assert.LessOrEqual(t, discordMemberAuthorityCacheTTL.Seconds(), 60.0,
		"ADR-0048 fixes the delegated-moderator permission cache at 60s or less")
}

// TestDiscordGuildResolver_MemberAuthorityCached: the check runs on every delegated action,
// so it must not cost three Discord calls each time — but it must also expire.
func TestDiscordGuildResolver_MemberAuthorityCached(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	f := standardGuild()
	srv := newMemberAuthorityServer(t, f)
	defer srv.Close()
	c := &DiscordClient{httpClient: srv.Client(), botToken: "bot-tok", baseURL: srv.URL}
	r := NewDiscordGuildResolver(c, rdb)

	for i := 0; i < 3; i++ {
		m, merr := r.MemberAuthority(context.Background(), "g", "mod-1")
		require.NoError(t, merr)
		assert.Equal(t, 5, m.HighestRolePos)
		assert.True(t, m.InGuild)
	}
	assert.Equal(t, 1, f.calls["member"], "the standing is read once then served from cache")

	// Expiry restores the live read — the whole point of the bound.
	mr.FastForward(discordMemberAuthorityCacheTTL + time.Second)
	_, err = r.MemberAuthority(context.Background(), "g", "mod-1")
	require.NoError(t, err)
	assert.Equal(t, 2, f.calls["member"], "once the TTL lapses the standing is re-read from Discord")
}

// TestDiscordGuildResolver_MemberAuthorityErrorNotCached: caching a failure would extend a
// transient Discord outage into a full TTL of denials.
func TestDiscordGuildResolver_MemberAuthorityErrorNotCached(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	c := &DiscordClient{httpClient: srv.Client(), botToken: "bot-tok", baseURL: srv.URL}
	r := NewDiscordGuildResolver(c, rdb)

	_, err1 := r.MemberAuthority(context.Background(), "g", "mod-1")
	_, err2 := r.MemberAuthority(context.Background(), "g", "mod-1")
	require.Error(t, err1)
	require.Error(t, err2)
	assert.Equal(t, 2, calls, "an error is not cached; the next call retries")
}

// TestDiscordGuildResolver_NonMemberIsCached: "not a member" is a real answer and the most
// likely one for a hostile prober, so it must be cached like any other — but only for the
// same bounded TTL.
func TestDiscordGuildResolver_NonMemberIsCached(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	f := standardGuild()
	srv := newMemberAuthorityServer(t, f)
	defer srv.Close()
	c := &DiscordClient{httpClient: srv.Client(), botToken: "bot-tok", baseURL: srv.URL}
	r := NewDiscordGuildResolver(c, rdb)

	for i := 0; i < 3; i++ {
		m, merr := r.MemberAuthority(context.Background(), "g", "stranger")
		require.NoError(t, merr)
		assert.False(t, m.InGuild)
	}
	assert.Equal(t, 1, f.calls["member"])
}
