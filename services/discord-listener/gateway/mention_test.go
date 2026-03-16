package gateway_test

import (
	"context"
	"testing"

	"github.com/caesar/all-chat/services/discord-listener/gateway"
)

// mockGuildCacheForMentions is a configurable in-memory GuildCache for mention resolution tests.
type mockGuildCacheForMentions struct {
	channels map[string]string
	roles    map[string]string
}

func newMockGuildCacheForMentions() *mockGuildCacheForMentions {
	return &mockGuildCacheForMentions{
		channels: make(map[string]string),
		roles:    make(map[string]string),
	}
}

func (m *mockGuildCacheForMentions) SetChannelName(_ context.Context, channelID, name string) error {
	m.channels[channelID] = name
	return nil
}

func (m *mockGuildCacheForMentions) GetChannelName(_ context.Context, channelID string) (string, bool, error) {
	name, ok := m.channels[channelID]
	return name, ok, nil
}

func (m *mockGuildCacheForMentions) DeleteChannelName(_ context.Context, channelID string) error {
	delete(m.channels, channelID)
	return nil
}

func (m *mockGuildCacheForMentions) SetRoleName(_ context.Context, roleID, name string) error {
	m.roles[roleID] = name
	return nil
}

func (m *mockGuildCacheForMentions) GetRoleName(_ context.Context, roleID string) (string, bool, error) {
	name, ok := m.roles[roleID]
	return name, ok, nil
}

func (m *mockGuildCacheForMentions) DeleteRoleName(_ context.Context, roleID string) error {
	delete(m.roles, roleID)
	return nil
}

// --- ResolveMentions unit tests ---

func TestResolveMentions_UserMention(t *testing.T) {
	ctx := context.Background()
	cache := newMockGuildCacheForMentions()
	mentions := []gateway.DiscordUser{
		{ID: "123", Username: "alice", GlobalName: "Alice"},
	}

	result := gateway.ResolveMentions(ctx, "<@123>", mentions, cache, nil)
	if result != "@Alice" {
		t.Errorf("expected @Alice, got %q", result)
	}
}

func TestResolveMentions_GuildMemberVariant(t *testing.T) {
	ctx := context.Background()
	cache := newMockGuildCacheForMentions()
	mentions := []gateway.DiscordUser{
		{ID: "123", Username: "alice", GlobalName: "Alice"},
	}

	result := gateway.ResolveMentions(ctx, "<@!123>", mentions, cache, nil)
	if result != "@Alice" {
		t.Errorf("expected @Alice for guild member variant, got %q", result)
	}
}

func TestResolveMentions_ChannelMention(t *testing.T) {
	ctx := context.Background()
	cache := newMockGuildCacheForMentions()
	_ = cache.SetChannelName(ctx, "456", "general")

	result := gateway.ResolveMentions(ctx, "<#456>", nil, cache, nil)
	if result != "#general" {
		t.Errorf("expected #general, got %q", result)
	}
}

func TestResolveMentions_RoleMention(t *testing.T) {
	ctx := context.Background()
	cache := newMockGuildCacheForMentions()
	_ = cache.SetRoleName(ctx, "789", "Mods")

	result := gateway.ResolveMentions(ctx, "<@&789>", nil, cache, nil)
	if result != "@Mods" {
		t.Errorf("expected @Mods, got %q", result)
	}
}

func TestResolveMentions_UnresolvableUser(t *testing.T) {
	ctx := context.Background()
	cache := newMockGuildCacheForMentions()

	result := gateway.ResolveMentions(ctx, "<@999>", nil, cache, nil)
	if result != "@unknown" {
		t.Errorf("expected @unknown for unresolvable user, got %q", result)
	}
}

func TestResolveMentions_UnresolvableChannel(t *testing.T) {
	ctx := context.Background()
	cache := newMockGuildCacheForMentions()

	result := gateway.ResolveMentions(ctx, "<#999>", nil, cache, nil)
	if result != "#channel" {
		t.Errorf("expected #channel for unresolvable channel, got %q", result)
	}
}

func TestResolveMentions_UnresolvableRole(t *testing.T) {
	ctx := context.Background()
	cache := newMockGuildCacheForMentions()

	result := gateway.ResolveMentions(ctx, "<@&999>", nil, cache, nil)
	if result != "@unknown" {
		t.Errorf("expected @unknown for unresolvable role, got %q", result)
	}
}

func TestResolveMentions_MultipleInSameMessage(t *testing.T) {
	ctx := context.Background()
	cache := newMockGuildCacheForMentions()
	_ = cache.SetChannelName(ctx, "456", "general")

	mentions := []gateway.DiscordUser{
		{ID: "123", Username: "alice", GlobalName: "Alice"},
	}

	result := gateway.ResolveMentions(ctx, "hello <@123> see <#456>", mentions, cache, nil)
	if result != "hello @Alice see #general" {
		t.Errorf("expected 'hello @Alice see #general', got %q", result)
	}
}

// TestHandleMessageCreate_MentionResolved is an integration test verifying that
// HandleMessageCreate resolves mentions in content before publishing.
func TestHandleMessageCreate_MentionResolved(t *testing.T) {
	cache := newMockGuildCacheForMentions()
	ctx := context.Background()
	_ = cache.SetChannelName(ctx, "456", "announcements")

	pub := &capturePayloadPublisherForMentions{}
	reg := &mockChannelRegistry{overlayID: "overlay-1", found: true}
	store := &mockSessionStore{}

	client := gateway.NewGatewayClient(
		"token",
		"wss://gateway.discord.gg",
		store,
		nil,
		reg,
		pub,
		cache,
	)

	msg := gateway.MessageCreateData{
		ID:        "msg-10",
		ChannelID: "channel-1",
		Content:   "check <#456> for updates",
		Timestamp: "2026-01-01T00:00:00Z",
		Author: gateway.DiscordUser{
			ID:       "user-1",
			Username: "tester",
			Bot:      false,
		},
		Mentions: nil,
	}

	err := client.HandleMessageCreate(ctx, msg)
	if err != nil {
		t.Fatalf("HandleMessageCreate returned error: %v", err)
	}

	if pub.lastText != "check #announcements for updates" {
		t.Errorf("expected resolved text 'check #announcements for updates', got %q", pub.lastText)
	}
}

// capturePayloadPublisherForMentions captures the published message payload for text inspection.
type capturePayloadPublisherForMentions struct {
	lastText string
}

func (p *capturePayloadPublisherForMentions) Publish(_ context.Context, msg interface{}) error {
	if m, ok := msg.(map[string]interface{}); ok {
		if text, ok := m["text"].(string); ok {
			p.lastText = text
		}
	}
	return nil
}
