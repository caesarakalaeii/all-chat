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

package llm

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"
)

// modelListServer serves a gateway-shaped /v1/models response for discovery tests.
func modelListServer(t *testing.T, ids []string, status int) *httptest.Server {
	t.Helper()
	payload := `{"object":"list","data":[` + strings.Join(ids, ",") + `]}`
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(payload))
	}))
}

func TestDiscoverModel(t *testing.T) {
	entry := func(id string) string {
		return fmt.Sprintf(`{"id":%q,"object":"model","created":0}`, id)
	}
	ctx := context.Background()

	t.Run("pin set and listed wins", func(t *testing.T) {
		srv := modelListServer(t, []string{entry("m/one"), entry("m/pinned")}, http.StatusOK)
		defer srv.Close()
		got, err := DiscoverModel(ctx, Config{BaseURL: srv.URL, Model: "m/pinned"}, zap.NewNop())
		if err != nil {
			t.Fatal(err)
		}
		if got != "m/pinned" {
			t.Fatalf("pin must win: got %q", got)
		}
	})

	t.Run("pinned but not listed falls back to discovery", func(t *testing.T) {
		srv := modelListServer(t, []string{entry("m/fresh")}, http.StatusOK)
		defer srv.Close()
		got, err := DiscoverModel(ctx, Config{BaseURL: srv.URL, Model: "m/stale"}, zap.NewNop())
		if err != nil {
			t.Fatal(err)
		}
		if got != "m/fresh" {
			t.Fatalf("expected discovered fallback, got %q", got)
		}
	})

	t.Run("multiple unpinned candidates pick lexicographically smallest", func(t *testing.T) {
		ids := []string{entry("m/beta"), entry("m/alpha"), entry("m/gamma")}
		// Shuffle so the pick is proven deterministic, not list order.
		for i := range ids {
			j := (i*7 + 3) % len(ids)
			ids[i], ids[j] = ids[j], ids[i]
		}
		srv := modelListServer(t, ids, http.StatusOK)
		defer srv.Close()
		got, err := DiscoverModel(ctx, Config{BaseURL: srv.URL}, zap.NewNop())
		if err != nil {
			t.Fatal(err)
		}
		if got != "m/alpha" {
			t.Fatalf("expected smallest id m/alpha, got %q", got)
		}
	})

	t.Run("fetch failure with pin keeps the pin", func(t *testing.T) {
		srv := modelListServer(t, nil, http.StatusInternalServerError)
		defer srv.Close()
		got, err := DiscoverModel(ctx, Config{BaseURL: srv.URL, Model: "m/pinned"}, zap.NewNop())
		if err != nil {
			t.Fatal(err)
		}
		if got != "m/pinned" {
			t.Fatalf("fetch failure must keep the pin, got %q", got)
		}
	})

	t.Run("fetch failure without pin errors naming the URL", func(t *testing.T) {
		srv := modelListServer(t, nil, http.StatusInternalServerError)
		defer srv.Close()
		_, err := DiscoverModel(ctx, Config{BaseURL: srv.URL}, zap.NewNop())
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "/v1/models") {
			t.Fatalf("error must name the tried URL, got %v", err)
		}
	})
}
func testClient(t *testing.T, url string) *Client {
	t.Helper()
	c, err := New(Config{
		BaseURL:        url,
		Model:          "local-model",
		MaxRetries:     3,
		RetryBaseDelay: time.Millisecond,
		RequestTimeout: 2 * time.Second,
	}, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestChatToolCallRoundTrip(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/v1/chat/completions") {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":"1","model":"local-model",
			"choices":[{"index":0,"finish_reason":"tool_calls","message":{
				"role":"assistant","content":null,
				"tool_calls":[{"id":"call_1","type":"function","function":{"name":"read_file","arguments":"{\"path\":\"a.go\"}"}}]
			}}],
			"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}
		}`))
	}))
	defer srv.Close()

	resp, err := testClient(t, srv.URL).Chat(context.Background(), ChatRequest{Messages: []Message{TextMessage(RoleUser, "hi")}})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Choices) != 1 || resp.Choices[0].FinishReason != FinishToolCalls {
		t.Fatalf("unexpected choices: %+v", resp.Choices)
	}
	tc := resp.Choices[0].Message.ToolCalls
	if len(tc) != 1 || tc[0].Function.Name != "read_file" || tc[0].Function.Arguments != `{"path":"a.go"}` {
		t.Fatalf("tool call not parsed: %+v", tc)
	}
	if resp.Usage.PromptTokens != 10 {
		t.Fatalf("usage not parsed: %+v", resp.Usage)
	}
}

func TestChatRetriesThenSucceeds(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"transient"}`))
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"ok"}}]}`))
	}))
	defer srv.Close()

	resp, err := testClient(t, srv.URL).Chat(context.Background(), ChatRequest{Messages: []Message{TextMessage(RoleUser, "hi")}})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Choices[0].Message.Text() != "ok" {
		t.Fatalf("unexpected content: %q", resp.Choices[0].Message.Text())
	}
	if atomic.LoadInt32(&calls) != 2 {
		t.Fatalf("expected 2 calls (1 retry), got %d", calls)
	}
}

func TestChatMasksCredentialsInError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest) // 400: not retried
		_, _ = w.Write([]byte(`{"error":"bad key: Bearer sk-ant-supersecrettokenvalue1234567890"}`))
	}))
	defer srv.Close()

	_, err := testClient(t, srv.URL).Chat(context.Background(), ChatRequest{Messages: []Message{TextMessage(RoleUser, "hi")}})
	if err == nil {
		t.Fatal("expected an error")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.Kind != KindInvalidRequest {
		t.Fatalf("400 should map to KindInvalidRequest, got %v", apiErr.Kind)
	}
	if strings.Contains(apiErr.Error(), "supersecrettokenvalue") {
		t.Fatalf("credential leaked into error: %s", apiErr.Error())
	}
	if !strings.Contains(apiErr.Error(), "[REDACTED]") {
		t.Fatalf("expected [REDACTED] marker: %s", apiErr.Error())
	}
}

func TestChatConcurrentNoRaceOrCrosstalk(t *testing.T) {
	// One shared client hit by many concurrent Chat calls (the real Discord wiring):
	// each error must carry ONLY its own status, with no shared per-request state. Run
	// under `go test -race` to catch the data race this guards against.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"bad request body A123"}`))
	}))
	defer srv.Close()

	c := testClient(t, srv.URL)
	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := c.Chat(context.Background(), ChatRequest{Messages: []Message{TextMessage(RoleUser, "hi")}})
			var apiErr *APIError
			if !errors.As(err, &apiErr) || apiErr.Status != http.StatusBadRequest {
				t.Errorf("expected 400 APIError, got %v", err)
			}
		}()
	}
	wg.Wait()
}

func TestNewRejectsBadAPIKey(t *testing.T) {
	if _, err := New(Config{BaseURL: "http://x", APIKey: "bad\nkey"}, zap.NewNop()); err == nil {
		t.Fatal("API key with newline should be rejected")
	}
}

// The gateway 429s hobby-project traffic deliberately (production load gate).
// It must come back immediately as a KindBusy APIError, not be retried: the
// condition lasts minutes-to-hours and retrying would hammer the gate.
func TestChatDoesNotRetry429(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"message":"hobby traffic paused"}}`))
	}))
	defer srv.Close()

	_, err := testClient(t, srv.URL).Chat(context.Background(), ChatRequest{Messages: []Message{TextMessage(RoleUser, "hi")}})
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Kind != KindBusy || apiErr.Status != http.StatusTooManyRequests {
		t.Fatalf("expected KindBusy 429 APIError, got %v", err)
	}
	if n := atomic.LoadInt32(&calls); n != 1 {
		t.Fatalf("429 must not be retried, got %d calls", n)
	}
}
