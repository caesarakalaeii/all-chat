package seventv

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// TestClientHandlesHelloMessage verifies that the client properly handles the HELLO message
func TestClientHandlesHelloMessage(t *testing.T) {
	logger := zap.NewNop()

	handler := func(ctx context.Context, update *EmoteSetUpdate) error {
		return nil
	}

	client := NewClient(logger, handler)

	// Create a test WebSocket server
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()

		// Send HELLO message
		hello := Message{
			Op: OpHello,
		}
		helloData := HelloData{
			HeartbeatInterval: 30000,
			SessionID:         "test-session",
			SubscriptionLimit: 10,
		}
		data, _ := json.Marshal(helloData)
		hello.D = data

		if err := conn.WriteJSON(hello); err != nil {
			t.Fatal(err)
		}

		// Wait for heartbeat
		time.Sleep(100 * time.Millisecond)
	}))
	defer server.Close()

	// Replace with test server URL
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	// Manually dial the test server
	ctx := context.Background()
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}

	client.mu.Lock()
	client.conn = conn
	client.mu.Unlock()

	// Start reading messages in a goroutine
	go client.readMessages(ctx)

	// Give it time to process HELLO
	time.Sleep(100 * time.Millisecond)

	// Verify session ID was set
	if client.sessionID != "test-session" {
		t.Errorf("Expected session ID to be 'test-session', got '%s'", client.sessionID)
	}

	// Verify heartbeat interval was set
	expectedInterval := 30 * time.Second
	if client.heartbeatInterval != expectedInterval {
		t.Errorf("Expected heartbeat interval to be %v, got %v", expectedInterval, client.heartbeatInterval)
	}

	client.Close()
}

// TestClientHandlesDispatchEvent verifies that the client properly handles DISPATCH events
func TestClientHandlesDispatchEvent(t *testing.T) {
	logger := zap.NewNop()
	var receivedUpdate *EmoteSetUpdate

	handler := func(ctx context.Context, update *EmoteSetUpdate) error {
		receivedUpdate = update
		return nil
	}

	client := NewClient(logger, handler)

	// Create DISPATCH message
	update := EmoteSetUpdate{
		ID:   "test-emote-set",
		Name: "Test Emote Set",
		Pushed: []struct {
			Key   string `json:"key"`
			Value struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"value"`
		}{
			{
				Key: "Kappa",
				Value: struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				}{
					ID:   "emote-id-123",
					Name: "Kappa",
				},
			},
		},
	}

	updateJSON, _ := json.Marshal(update)
	dispatch := DispatchData{
		Type: "emote_set.update",
		Body: updateJSON,
	}
	dispatchJSON, _ := json.Marshal(dispatch)

	msg := Message{
		Op: OpDispatch,
		D:  dispatchJSON,
	}

	// Handle the message directly
	ctx := context.Background()
	if err := client.handleMessage(ctx, &msg); err != nil {
		t.Fatal(err)
	}

	// Verify the handler was called
	if receivedUpdate == nil {
		t.Fatal("Expected handler to be called, but it wasn't")
	}

	// Verify the update data
	if receivedUpdate.ID != "test-emote-set" {
		t.Errorf("Expected emote set ID to be 'test-emote-set', got '%s'", receivedUpdate.ID)
	}

	if len(receivedUpdate.Pushed) != 1 {
		t.Errorf("Expected 1 pushed emote, got %d", len(receivedUpdate.Pushed))
	}

	if receivedUpdate.Pushed[0].Key != "Kappa" {
		t.Errorf("Expected pushed emote key to be 'Kappa', got '%s'", receivedUpdate.Pushed[0].Key)
	}
}
