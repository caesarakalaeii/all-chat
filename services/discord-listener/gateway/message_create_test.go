package gateway_test

import (
	"context"
	"errors"
	"testing"

	"github.com/caesar/all-chat/services/discord-listener/gateway"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockSessionStore satisfies gateway.SessionStore for tests.
type mockSessionStore struct{}

func (m *mockSessionStore) Set(_ context.Context, _, _ string) error { return nil }
func (m *mockSessionStore) Get(_ context.Context, _ string) (string, error) {
	return "", errors.New("not found")
}

// mockChannelRegistry satisfies gateway.ChannelRegistry for tests.
type mockChannelRegistry struct {
	overlayID string
	found     bool
	err       error
}

func (m *mockChannelRegistry) GetOverlayForChannel(_ context.Context, _ string) (string, bool, error) {
	return m.overlayID, m.found, m.err
}

func (m *mockChannelRegistry) Subscribe(_ context.Context, _ chan<- string) error {
	return nil
}

// capturePublisher records Publish calls for assertion.
type capturePublisher struct {
	calls int
}

func (p *capturePublisher) Publish(_ context.Context, _ interface{}) error {
	p.calls++
	return nil
}

// TestHandleMessageCreate_BotFiltered verifies that bot messages are silently dropped.
func TestHandleMessageCreate_BotFiltered(t *testing.T) {
	pub := &capturePublisher{}
	reg := &mockChannelRegistry{overlayID: "overlay-1", found: true}
	store := &mockSessionStore{}

	client := gateway.NewGatewayClient(
		"token",
		"wss://gateway.discord.gg",
		store,
		nil, // logger — will panic if called; use in test context
		reg,
		pub,
		nil, // guildCache — not needed for create tests without mentions
	)

	msg := gateway.MessageCreateData{
		ID:        "msg-1",
		ChannelID: "channel-1",
		Content:   "hello",
		Author: gateway.DiscordUser{
			ID:       "user-1",
			Username: "bot-user",
			Bot:      true,
		},
	}

	err := client.HandleMessageCreate(context.Background(), msg)
	require.NoError(t, err)
	assert.Equal(t, 0, pub.calls, "expected no publish for bot message")
}

// TestHandleMessageCreate_UnknownChannel verifies that messages from unconfigured channels are dropped.
func TestHandleMessageCreate_UnknownChannel(t *testing.T) {
	pub := &capturePublisher{}
	reg := &mockChannelRegistry{found: false}
	store := &mockSessionStore{}

	client := gateway.NewGatewayClient(
		"token",
		"wss://gateway.discord.gg",
		store,
		nil,
		reg,
		pub,
		nil, // guildCache — not needed for create tests without mentions
	)

	msg := gateway.MessageCreateData{
		ID:        "msg-2",
		ChannelID: "unknown-channel",
		Content:   "hello",
		Author: gateway.DiscordUser{
			ID:       "user-2",
			Username: "some-user",
			Bot:      false,
		},
	}

	err := client.HandleMessageCreate(context.Background(), msg)
	require.NoError(t, err)
	assert.Equal(t, 0, pub.calls, "expected no publish for unknown channel")
}

// TestHandleMessageCreate_EmptyContent verifies that empty content on first message returns an error.
func TestHandleMessageCreate_EmptyContent(t *testing.T) {
	pub := &capturePublisher{}
	reg := &mockChannelRegistry{overlayID: "overlay-1", found: true}
	store := &mockSessionStore{}

	client := gateway.NewGatewayClient(
		"token",
		"wss://gateway.discord.gg",
		store,
		nil,
		reg,
		pub,
		nil, // guildCache — not needed for create tests without mentions
	)

	msg := gateway.MessageCreateData{
		ID:        "msg-3",
		ChannelID: "channel-1",
		Content:   "",
		Author: gateway.DiscordUser{
			ID:       "user-3",
			Username: "some-user",
			Bot:      false,
		},
	}

	err := client.HandleMessageCreate(context.Background(), msg)
	assert.Error(t, err, "expected error when first message has empty content")
	assert.Equal(t, 0, pub.calls, "expected no publish on empty content halt")
}

// TestHandleMessageCreate_HappyPath verifies that a valid message is published exactly once.
func TestHandleMessageCreate_HappyPath(t *testing.T) {
	pub := &capturePublisher{}
	reg := &mockChannelRegistry{overlayID: "overlay-1", found: true}
	store := &mockSessionStore{}

	client := gateway.NewGatewayClient(
		"token",
		"wss://gateway.discord.gg",
		store,
		nil,
		reg,
		pub,
		nil, // guildCache — not needed for create tests without mentions
	)

	msg := gateway.MessageCreateData{
		ID:        "msg-4",
		ChannelID: "channel-1",
		Content:   "hello world",
		Author: gateway.DiscordUser{
			ID:       "user-4",
			Username: "real-user",
			Bot:      false,
		},
	}

	err := client.HandleMessageCreate(context.Background(), msg)
	require.NoError(t, err)
	assert.Equal(t, 1, pub.calls, "expected exactly one publish for valid message")
}
