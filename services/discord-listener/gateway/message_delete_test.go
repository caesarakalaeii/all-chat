package gateway_test

import (
	"context"
	"errors"
	"testing"

	"github.com/caesar/all-chat/services/discord-listener/gateway"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// capturePayloadPublisher records Publish calls and their payloads for detailed assertions.
type capturePayloadPublisher struct {
	calls    int
	payloads []interface{}
}

func (p *capturePayloadPublisher) Publish(_ context.Context, msg interface{}) error {
	p.calls++
	p.payloads = append(p.payloads, msg)
	return nil
}

// TestHandleMessageDelete_UnknownChannel verifies that MESSAGE_DELETE for a channel not in
// the registry results in no publish and no error.
func TestHandleMessageDelete_UnknownChannel(t *testing.T) {
	pub := &capturePayloadPublisher{}
	reg := &mockChannelRegistry{found: false}
	store := &mockSessionStore{}

	client := gateway.NewGatewayClient(
		"token",
		"wss://gateway.discord.gg",
		store,
		nil,
		reg,
		pub,
		nil, // guildCache — not needed for delete tests
	)

	msg := gateway.MessageDeleteData{
		ID:        "msg-del-1",
		ChannelID: "unknown-channel",
		GuildID:   "guild-1",
	}

	err := client.HandleMessageDelete(context.Background(), msg)
	require.NoError(t, err)
	assert.Equal(t, 0, pub.calls, "expected no publish for unknown channel")
}

// TestHandleMessageDelete_HappyPath verifies that MESSAGE_DELETE for a configured channel
// results in exactly one Publish with the correct deletion event fields.
func TestHandleMessageDelete_HappyPath(t *testing.T) {
	pub := &capturePayloadPublisher{}
	reg := &mockChannelRegistry{overlayID: "overlay-1", found: true}
	store := &mockSessionStore{}

	client := gateway.NewGatewayClient(
		"token",
		"wss://gateway.discord.gg",
		store,
		nil,
		reg,
		pub,
		nil, // guildCache — not needed for delete tests
	)

	msg := gateway.MessageDeleteData{
		ID:        "msg-del-2",
		ChannelID: "channel-1",
		GuildID:   "guild-1",
	}

	err := client.HandleMessageDelete(context.Background(), msg)
	require.NoError(t, err)
	require.Equal(t, 1, pub.calls, "expected exactly one publish for deletion")

	payload, ok := pub.payloads[0].(map[string]interface{})
	require.True(t, ok, "published payload must be map[string]interface{}")

	assert.Equal(t, "message_deletion", payload["event_type"], "event_type must be message_deletion")
	assert.Equal(t, "discord", payload["platform"], "platform must be discord")
	assert.Equal(t, "overlay-1", payload["overlay_id"], "overlay_id must match registry")

	eventData, ok := payload["event_data"].(map[string]interface{})
	require.True(t, ok, "event_data must be map[string]interface{}")
	assert.Equal(t, "single", eventData["deletion_type"], "deletion_type must be single")
	assert.Equal(t, "msg-del-2", eventData["target_msg_id"], "target_msg_id must be the deleted message snowflake")
}

// TestHandleMessageDelete_RegistryError verifies that a registry error results in no publish and no error.
func TestHandleMessageDelete_RegistryError(t *testing.T) {
	pub := &capturePayloadPublisher{}
	reg := &mockChannelRegistry{err: errors.New("redis connection lost")}
	store := &mockSessionStore{}

	client := gateway.NewGatewayClient(
		"token",
		"wss://gateway.discord.gg",
		store,
		nil,
		reg,
		pub,
		nil, // guildCache — not needed for delete tests
	)

	msg := gateway.MessageDeleteData{
		ID:        "msg-del-3",
		ChannelID: "channel-1",
		GuildID:   "guild-1",
	}

	err := client.HandleMessageDelete(context.Background(), msg)
	require.NoError(t, err, "registry errors must not propagate")
	assert.Equal(t, 0, pub.calls, "expected no publish on registry error")
}

// TestHandleMessageDeleteBulk_HappyPath verifies that MESSAGE_DELETE_BULK with 3 IDs for a
// configured channel results in exactly 3 Publish calls, each with a distinct target_msg_id.
func TestHandleMessageDeleteBulk_HappyPath(t *testing.T) {
	pub := &capturePayloadPublisher{}
	reg := &mockChannelRegistry{overlayID: "overlay-1", found: true}
	store := &mockSessionStore{}

	client := gateway.NewGatewayClient(
		"token",
		"wss://gateway.discord.gg",
		store,
		nil,
		reg,
		pub,
		nil, // guildCache — not needed for delete tests
	)

	bulk := gateway.MessageDeleteBulkData{
		IDs:       []string{"bulk-id-1", "bulk-id-2", "bulk-id-3"},
		ChannelID: "channel-1",
		GuildID:   "guild-1",
	}

	err := client.HandleMessageDeleteBulk(context.Background(), bulk)
	require.NoError(t, err)
	require.Equal(t, 3, pub.calls, "expected one publish per deleted message ID")

	seenIDs := make(map[string]bool)
	for _, rawPayload := range pub.payloads {
		payload, ok := rawPayload.(map[string]interface{})
		require.True(t, ok, "each payload must be map[string]interface{}")

		eventData, ok := payload["event_data"].(map[string]interface{})
		require.True(t, ok, "event_data must be map[string]interface{}")

		targetID, ok := eventData["target_msg_id"].(string)
		require.True(t, ok, "target_msg_id must be a string")
		seenIDs[targetID] = true
	}

	assert.True(t, seenIDs["bulk-id-1"], "bulk-id-1 must appear in published events")
	assert.True(t, seenIDs["bulk-id-2"], "bulk-id-2 must appear in published events")
	assert.True(t, seenIDs["bulk-id-3"], "bulk-id-3 must appear in published events")
}

// TestHandleMessageDeleteBulk_UnknownChannel verifies that MESSAGE_DELETE_BULK for an
// unconfigured channel results in 0 Publish calls.
func TestHandleMessageDeleteBulk_UnknownChannel(t *testing.T) {
	pub := &capturePayloadPublisher{}
	reg := &mockChannelRegistry{found: false}
	store := &mockSessionStore{}

	client := gateway.NewGatewayClient(
		"token",
		"wss://gateway.discord.gg",
		store,
		nil,
		reg,
		pub,
		nil, // guildCache — not needed for delete tests
	)

	bulk := gateway.MessageDeleteBulkData{
		IDs:       []string{"bulk-id-1", "bulk-id-2"},
		ChannelID: "unknown-channel",
		GuildID:   "guild-1",
	}

	err := client.HandleMessageDeleteBulk(context.Background(), bulk)
	require.NoError(t, err)
	assert.Equal(t, 0, pub.calls, "expected no publish for unknown channel in bulk delete")
}
