package gateway_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/caesar/all-chat/services/discord-listener/gateway"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGatewayTypes_IntentBitmask(t *testing.T) {
	assert.Equal(t, 33281, gateway.RequiredIntents,
		"RequiredIntents must equal GUILDS(1) | GUILD_MESSAGES(512) | MESSAGE_CONTENT(32768)")
	assert.Equal(t, 1, gateway.IntentGuilds)
	assert.Equal(t, 512, gateway.IntentGuildMessages)
	assert.Equal(t, 32768, gateway.IntentMessageContent)
}

func TestGatewayTypes_OpCodes(t *testing.T) {
	assert.Equal(t, 2, gateway.OpIdentify)
	assert.Equal(t, 10, gateway.OpHello)
	assert.Equal(t, 1, gateway.OpHeartbeat)
	assert.Equal(t, 11, gateway.OpHeartbeatACK)
}

func TestGatewayClient_IdentifyPayload(t *testing.T) {
	payload := gateway.BuildIdentifyPayload("test-token")
	assert.Equal(t, gateway.OpIdentify, payload.Op)

	var d gateway.IdentifyData
	require.NoError(t, json.Unmarshal(payload.D, &d))
	assert.Equal(t, "test-token", d.Token)
	assert.Equal(t, gateway.RequiredIntents, d.Intents)
	assert.Equal(t, "linux", d.Properties.OS)
	assert.Equal(t, "allchat-discord-listener", d.Properties.Browser)
	assert.Equal(t, "allchat-discord-listener", d.Properties.Device)
}

// MockRedis for testing READY handler without live Redis
type MockRedis struct {
	store map[string]string
}

func newMockRedis() *MockRedis { return &MockRedis{store: make(map[string]string)} }
func (m *MockRedis) Set(_ context.Context, key, value string) error {
	m.store[key] = value
	return nil
}
func (m *MockRedis) Get(_ context.Context, key string) (string, error) {
	v, ok := m.store[key]
	if !ok {
		return "", fmt.Errorf("key not found")
	}
	return v, nil
}

func TestGatewayClient_ReadyHandler(t *testing.T) {
	mockRedis := newMockRedis()
	ready := gateway.ReadyEventData{
		SessionID:        "sess-abc123",
		ResumeGatewayURL: "wss://us-east1-b.gateway.discord.gg",
	}

	err := gateway.HandleReady(context.Background(), ready, mockRedis)
	require.NoError(t, err)
	assert.Equal(t, "sess-abc123", mockRedis.store[gateway.RedisKeySessionID])
	assert.Equal(t, "wss://us-east1-b.gateway.discord.gg", mockRedis.store[gateway.RedisKeyResumeURL])
}
