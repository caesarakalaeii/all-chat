package gateway_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/caesar/all-chat/services/discord-listener/gateway"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
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

// TestGatewayClient_ReconnectAfterClose verifies that calling Connect() a second time
// after Close() succeeds. This covers the leadership-loss reconnect path: the lostCallback
// calls gwClient.Close(), which signals the current Connect() to exit; the reconnect loop
// then calls Connect() again. Without the done-channel reset at the start of Connect(),
// the second call would exit immediately (done is already closed).
func TestGatewayClient_ReconnectAfterClose(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	connectCount := 0
	connectCh := make(chan struct{}, 2)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		connectCount++
		connectCh <- struct{}{}

		// Send HELLO so Connect() gets past the handshake
		hello := gateway.GatewayPayload{Op: gateway.OpHello}
		helloData, _ := json.Marshal(gateway.HelloData{HeartbeatInterval: 45000})
		hello.D = json.RawMessage(helloData)
		_ = conn.WriteJSON(hello)

		// Wait for the client to send IDENTIFY/RESUME, then keep the connection open
		conn.ReadMessage() //nolint:errcheck

		// Block until client disconnects (simulates a stable connection)
		conn.ReadMessage() //nolint:errcheck
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	store := &MockRedis{store: make(map[string]string)}
	log, _ := zap.NewDevelopment()
	client := gateway.NewGatewayClient("tok", wsURL, store, log, nil, nil, nil)

	// First Connect — runs in background, blocks until the server closes
	ctx1, cancel1 := context.WithCancel(context.Background())
	done1 := make(chan error, 1)
	go func() {
		done1 <- client.Connect(ctx1)
	}()

	// Wait for first connection to be established
	select {
	case <-connectCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for first connection")
	}

	// Simulate lostCallback: Close() then cancel the first context
	client.Close()
	cancel1()
	<-done1 // wait for first Connect() to return

	// Second Connect — must succeed (done channel must have been reset)
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	done2 := make(chan error, 1)
	go func() {
		done2 <- client.Connect(ctx2)
	}()

	select {
	case <-connectCh:
		// Second connection established — test passes
	case <-time.After(2 * time.Second):
		t.Fatal("second Connect() did not establish connection — done channel was not reset")
	}

	cancel2()
	<-done2
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
