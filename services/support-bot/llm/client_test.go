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
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"
)

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
