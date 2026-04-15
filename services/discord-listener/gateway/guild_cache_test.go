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

package gateway_test

import (
	"context"
	"testing"

	"github.com/caesar/all-chat/services/discord-listener/gateway"
)

// mockGuildCache is an in-memory implementation of gateway.GuildCache for testing.
type mockGuildCache struct {
	channels map[string]string
	roles    map[string]string
}

func newMockGuildCache() *mockGuildCache {
	return &mockGuildCache{
		channels: make(map[string]string),
		roles:    make(map[string]string),
	}
}

func (m *mockGuildCache) SetChannelName(_ context.Context, channelID, name string) error {
	m.channels[channelID] = name
	return nil
}

func (m *mockGuildCache) GetChannelName(_ context.Context, channelID string) (string, bool, error) {
	name, ok := m.channels[channelID]
	return name, ok, nil
}

func (m *mockGuildCache) DeleteChannelName(_ context.Context, channelID string) error {
	delete(m.channels, channelID)
	return nil
}

func (m *mockGuildCache) SetRoleName(_ context.Context, roleID, name string) error {
	m.roles[roleID] = name
	return nil
}

func (m *mockGuildCache) GetRoleName(_ context.Context, roleID string) (string, bool, error) {
	name, ok := m.roles[roleID]
	return name, ok, nil
}

func (m *mockGuildCache) DeleteRoleName(_ context.Context, roleID string) error {
	delete(m.roles, roleID)
	return nil
}

// newTestGatewayClientWithCache constructs a GatewayClient wired with the provided GuildCache.
func newTestGatewayClientWithCache(cache gateway.GuildCache) *gateway.GatewayClient {
	store := &mockSessionStore{}
	reg := &mockChannelRegistry{overlayID: "overlay-1", found: true}
	pub := &capturePublisher{}
	return gateway.NewGatewayClient(
		"token",
		"wss://gateway.discord.gg",
		store,
		nil, // logger
		reg,
		pub,
		cache,
	)
}

// --- Tests ---

func TestHandleGuildCreate_PopulatesChannelCache(t *testing.T) {
	cache := newMockGuildCache()
	client := newTestGatewayClientWithCache(cache)
	ctx := context.Background()

	data := gateway.GuildCreateData{
		ID:   "guild1",
		Name: "Test Guild",
		Channels: []gateway.DiscordChannel{
			{ID: "chan1", Name: "general", Type: 0},
			{ID: "chan2", Name: "random", Type: 0},
		},
	}

	if err := client.HandleGuildCreate(ctx, data); err != nil {
		t.Fatalf("HandleGuildCreate returned error: %v", err)
	}

	name, ok, _ := cache.GetChannelName(ctx, "chan1")
	if !ok || name != "general" {
		t.Errorf("expected channel chan1 = general, got %q (found=%v)", name, ok)
	}
	name, ok, _ = cache.GetChannelName(ctx, "chan2")
	if !ok || name != "random" {
		t.Errorf("expected channel chan2 = random, got %q (found=%v)", name, ok)
	}
}

func TestHandleGuildCreate_PopulatesRoleCache(t *testing.T) {
	cache := newMockGuildCache()
	client := newTestGatewayClientWithCache(cache)
	ctx := context.Background()

	data := gateway.GuildCreateData{
		ID:   "guild1",
		Name: "Test Guild",
		Roles: []gateway.DiscordRole{
			{ID: "role1", Name: "Admin"},
			{ID: "role2", Name: "Moderator"},
		},
	}

	if err := client.HandleGuildCreate(ctx, data); err != nil {
		t.Fatalf("HandleGuildCreate returned error: %v", err)
	}

	name, ok, _ := cache.GetRoleName(ctx, "role1")
	if !ok || name != "Admin" {
		t.Errorf("expected role role1 = Admin, got %q (found=%v)", name, ok)
	}
	name, ok, _ = cache.GetRoleName(ctx, "role2")
	if !ok || name != "Moderator" {
		t.Errorf("expected role role2 = Moderator, got %q (found=%v)", name, ok)
	}
}

func TestHandleChannelUpdate_UpdatesCache(t *testing.T) {
	cache := newMockGuildCache()
	ctx := context.Background()
	_ = cache.SetChannelName(ctx, "chan1", "old-name")

	client := newTestGatewayClientWithCache(cache)

	data := gateway.ChannelUpdateData{
		ID:      "chan1",
		Name:    "new-name",
		GuildID: "guild1",
	}

	if err := client.HandleChannelUpdate(ctx, data); err != nil {
		t.Fatalf("HandleChannelUpdate returned error: %v", err)
	}

	name, ok, _ := cache.GetChannelName(ctx, "chan1")
	if !ok || name != "new-name" {
		t.Errorf("expected channel chan1 = new-name, got %q (found=%v)", name, ok)
	}
}

func TestHandleChannelDelete_RemovesCache(t *testing.T) {
	cache := newMockGuildCache()
	ctx := context.Background()
	_ = cache.SetChannelName(ctx, "chan1", "general")

	client := newTestGatewayClientWithCache(cache)

	data := gateway.ChannelUpdateData{
		ID:      "chan1",
		GuildID: "guild1",
	}

	if err := client.HandleChannelDelete(ctx, data); err != nil {
		t.Fatalf("HandleChannelDelete returned error: %v", err)
	}

	_, ok, _ := cache.GetChannelName(ctx, "chan1")
	if ok {
		t.Error("expected channel chan1 to be deleted, but it was still found")
	}
}

func TestHandleGuildRoleUpdate_UpdatesCache(t *testing.T) {
	cache := newMockGuildCache()
	ctx := context.Background()
	_ = cache.SetRoleName(ctx, "role1", "old-role")

	client := newTestGatewayClientWithCache(cache)

	data := gateway.GuildRoleUpdateData{
		GuildID: "guild1",
		Role:    gateway.DiscordRole{ID: "role1", Name: "Mods"},
	}

	if err := client.HandleGuildRoleUpdate(ctx, data); err != nil {
		t.Fatalf("HandleGuildRoleUpdate returned error: %v", err)
	}

	name, ok, _ := cache.GetRoleName(ctx, "role1")
	if !ok || name != "Mods" {
		t.Errorf("expected role role1 = Mods, got %q (found=%v)", name, ok)
	}
}

func TestHandleGuildRoleDelete_RemovesCache(t *testing.T) {
	cache := newMockGuildCache()
	ctx := context.Background()
	_ = cache.SetRoleName(ctx, "role1", "Admin")

	client := newTestGatewayClientWithCache(cache)

	data := gateway.GuildRoleDeleteData{
		GuildID: "guild1",
		RoleID:  "role1",
	}

	if err := client.HandleGuildRoleDelete(ctx, data); err != nil {
		t.Fatalf("HandleGuildRoleDelete returned error: %v", err)
	}

	_, ok, _ := cache.GetRoleName(ctx, "role1")
	if ok {
		t.Error("expected role role1 to be deleted, but it was still found")
	}
}
