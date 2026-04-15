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

// TestSubscribeQueuesWhenNotReady verifies that subscriptions are queued when connection is not ready
func TestSubscribeQueuesWhenNotReady(t *testing.T) {
	logger := zap.NewNop()
	handler := func(ctx context.Context, update *EmoteSetUpdate) error {
		return nil
	}

	client := NewClient(logger, handler)

	// Subscribe before connection is ready
	ctx := context.Background()
	err := client.Subscribe(ctx, "test-emote-set-1")
	if err != nil {
		t.Fatalf("Subscribe returned error: %v", err)
	}

	// Verify subscription was queued
	client.mu.RLock()
	if len(client.pendingSubscribe) != 1 {
		t.Errorf("Expected 1 pending subscription, got %d", len(client.pendingSubscribe))
	}
	if client.pendingSubscribe[0] != "test-emote-set-1" {
		t.Errorf("Expected pending subscription to be 'test-emote-set-1', got '%s'", client.pendingSubscribe[0])
	}
	client.mu.RUnlock()
}

// TestReadyStateSignaledAfterHello verifies that ready state is signaled after HELLO
func TestReadyStateSignaledAfterHello(t *testing.T) {
	logger := zap.NewNop()
	handler := func(ctx context.Context, update *EmoteSetUpdate) error {
		return nil
	}

	client := NewClient(logger, handler)

	// Verify initial state is not ready
	client.mu.RLock()
	if client.isReady {
		t.Error("Expected client to not be ready initially")
	}
	client.mu.RUnlock()

	// Create HELLO message
	helloData := HelloData{
		HeartbeatInterval: 30000,
		SessionID:         "test-session",
		SubscriptionLimit: 10,
	}
	data, _ := json.Marshal(helloData)

	// Handle HELLO message
	ctx := context.Background()
	go func() {
		// Set up a minimal connection
		upgrader := websocket.Upgrader{}
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				return
			}
			defer conn.Close()
			time.Sleep(500 * time.Millisecond) // Keep connection open
		}))
		defer server.Close()

		wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
		conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, nil)
		if err != nil {
			return
		}

		client.mu.Lock()
		client.conn = conn
		client.mu.Unlock()
	}()

	time.Sleep(100 * time.Millisecond) // Wait for connection

	err := client.handleHello(ctx, data)
	if err != nil {
		t.Fatalf("handleHello returned error: %v", err)
	}

	// Give handleHello time to complete (100ms delay + processing)
	time.Sleep(200 * time.Millisecond)

	// Verify ready state was set
	client.mu.RLock()
	if !client.isReady {
		t.Error("Expected client to be ready after HELLO")
	}
	client.mu.RUnlock()
}

// TestSubscribeWaitsForReady verifies that Subscribe waits for ready state
func TestSubscribeWaitsForReady(t *testing.T) {
	logger := zap.NewNop()
	handler := func(ctx context.Context, update *EmoteSetUpdate) error {
		return nil
	}

	client := NewClient(logger, handler)

	// Create a test WebSocket server
	upgrader := websocket.Upgrader{}
	subscriptionReceived := make(chan bool, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()

		// Send HELLO message
		hello := Message{Op: OpHello}
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

		// Wait for subscription message
		var msg Message
		if err := conn.ReadJSON(&msg); err != nil {
			return
		}

		if msg.Op == OpSubscribe {
			subscriptionReceived <- true
		}

		time.Sleep(500 * time.Millisecond) // Keep connection open
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	// Connect and start reading
	ctx := context.Background()
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}

	client.mu.Lock()
	client.conn = conn
	client.readLoopDone = make(chan struct{})
	client.mu.Unlock()

	go client.readMessages(ctx)

	// Wait for HELLO to be processed
	time.Sleep(250 * time.Millisecond)

	// Now subscribe - should go through immediately since connection is ready
	err = client.Subscribe(ctx, "test-emote-set")
	if err != nil {
		t.Fatalf("Subscribe returned error: %v", err)
	}

	// Verify subscription was received
	select {
	case <-subscriptionReceived:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("Subscription was not sent to server")
	}

	client.Close()
}
