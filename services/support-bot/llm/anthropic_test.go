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
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// anthropicStub serves canned /v1/messages responses and records each request body.
type anthropicStub struct {
	srv      *httptest.Server
	bodies   []map[string]any
	headers  []http.Header
	status   int
	response string
}

func newAnthropicStub(t *testing.T, status int, response string) *anthropicStub {
	t.Helper()
	s := &anthropicStub{status: status, response: response}
	s.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Errorf("request body is not JSON: %v", err)
		}
		s.bodies = append(s.bodies, body)
		s.headers = append(s.headers, r.Header.Clone())
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(s.status)
		_, _ = io.WriteString(w, s.response)
	}))
	t.Cleanup(s.srv.Close)
	return s
}

func (s *anthropicStub) client() *AnthropicClient {
	return NewAnthropic(AnthropicConfig{
		APIKey:     "sk-ant-test",
		Model:      "claude-opus-5-5",
		Effort:     "medium",
		BaseURL:    s.srv.URL,
		MaxRetries: 0, // no SDK retries: one request per Chat call
	})
}

const toolUseResponse = `{
  "id": "msg_1", "type": "message", "role": "assistant", "model": "claude-opus-5-5",
  "stop_reason": "tool_use",
  "content": [
    {"type": "thinking", "thinking": "", "signature": "sig-abc"},
    {"type": "text", "text": "Checking the logs."},
    {"type": "tool_use", "id": "toolu_1", "name": "kubectl", "input": {"verb": "get", "resource": "pods"}}
  ],
  "usage": {"input_tokens": 10, "output_tokens": 5}
}`

func TestAnthropicChat_MapsToolUseResponse(t *testing.T) {
	stub := newAnthropicStub(t, 200, toolUseResponse)
	resp, err := stub.client().Chat(context.Background(), ChatRequest{
		MaxTokens: 16000,
		Messages: []Message{
			TextMessage(RoleSystem, "be helpful"),
			TextMessage(RoleUser, "are pods up?"),
		},
		Tools: []Tool{{Type: "function", Function: ToolFunction{
			Name:        "kubectl",
			Description: "read the cluster",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"verb":{"type":"string"}},"required":["verb"],"additionalProperties":false}`),
		}}},
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if len(resp.Choices) != 1 {
		t.Fatalf("want 1 choice, got %d", len(resp.Choices))
	}
	c := resp.Choices[0]
	if c.FinishReason != FinishToolCalls {
		t.Errorf("finish reason = %q, want %q", c.FinishReason, FinishToolCalls)
	}
	if c.Message.Text() != "Checking the logs." {
		t.Errorf("text = %q", c.Message.Text())
	}
	if len(c.Message.ToolCalls) != 1 {
		t.Fatalf("want 1 tool call, got %d", len(c.Message.ToolCalls))
	}
	tc := c.Message.ToolCalls[0]
	if tc.ID != "toolu_1" || tc.Function.Name != "kubectl" {
		t.Errorf("tool call = %+v", tc)
	}
	var args map[string]any
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil || args["verb"] != "get" {
		t.Errorf("arguments = %q (%v)", tc.Function.Arguments, err)
	}
	if c.Message.Native == nil {
		t.Error("assistant message must carry the native turn so thinking blocks replay verbatim")
	}
}

func TestAnthropicChat_TranslatesRequest(t *testing.T) {
	stub := newAnthropicStub(t, 200, toolUseResponse)
	cl := stub.client()
	first, err := cl.Chat(context.Background(), ChatRequest{
		MaxTokens: 16000,
		Messages:  []Message{TextMessage(RoleSystem, "be helpful"), TextMessage(RoleUser, "are pods up?")},
		Tools: []Tool{{Type: "function", Function: ToolFunction{
			Name:       "kubectl",
			Parameters: json.RawMessage(`{"type":"object","properties":{"verb":{"type":"string"}},"required":["verb"],"additionalProperties":false}`),
		}}},
	})
	if err != nil {
		t.Fatalf("first Chat: %v", err)
	}

	// Second turn: replay the assistant turn and answer its tool call, plus a second
	// tool result — consecutive tool messages must merge into ONE user turn.
	history := []Message{
		TextMessage(RoleSystem, "be helpful"),
		TextMessage(RoleUser, "are pods up?"),
		first.Choices[0].Message,
		ToolResultMessage("toolu_1", "3 pods running"),
		ToolResultMessage("toolu_2", "skipped"),
	}
	if _, err := cl.Chat(context.Background(), ChatRequest{MaxTokens: 16000, Messages: history}); err != nil {
		t.Fatalf("second Chat: %v", err)
	}

	body := stub.bodies[1]
	if body["model"] != "claude-opus-5-5" {
		t.Errorf("model = %v", body["model"])
	}
	if body["max_tokens"] != float64(16000) {
		t.Errorf("max_tokens = %v", body["max_tokens"])
	}
	if body["fallbacks"] != "default" {
		t.Errorf("fallbacks = %v, want \"default\"", body["fallbacks"])
	}
	if !strings.Contains(stub.headers[1].Get("anthropic-beta"), "server-side-fallback-2026-07-01") {
		t.Errorf("anthropic-beta = %q", stub.headers[1].Get("anthropic-beta"))
	}
	if oc, _ := body["output_config"].(map[string]any); oc["effort"] != "medium" {
		t.Errorf("output_config = %v", body["output_config"])
	}
	if _, ok := body["cache_control"]; !ok {
		t.Error("top-level cache_control missing: the tool loop resends a growing prefix")
	}
	sys, _ := body["system"].([]any)
	if len(sys) != 1 || sys[0].(map[string]any)["text"] != "be helpful" {
		t.Errorf("system = %v", body["system"])
	}

	msgs, _ := body["messages"].([]any)
	if len(msgs) != 3 {
		t.Fatalf("want 3 messages (user, assistant, merged tool results), got %d: %v", len(msgs), msgs)
	}
	assistant := msgs[1].(map[string]any)
	blocks := assistant["content"].([]any)
	if blocks[0].(map[string]any)["type"] != "thinking" || blocks[0].(map[string]any)["signature"] != "sig-abc" {
		t.Errorf("thinking block not replayed verbatim: %v", blocks[0])
	}
	results := msgs[2].(map[string]any)
	if results["role"] != "user" {
		t.Errorf("tool results role = %v", results["role"])
	}
	rb := results["content"].([]any)
	if len(rb) != 2 {
		t.Fatalf("want 2 tool_result blocks in one turn, got %d", len(rb))
	}
	if r0 := rb[0].(map[string]any); r0["type"] != "tool_result" || r0["tool_use_id"] != "toolu_1" {
		t.Errorf("tool result = %v", r0)
	}
}

func TestAnthropicChat_SendsToolSchema(t *testing.T) {
	stub := newAnthropicStub(t, 200, toolUseResponse)
	_, err := stub.client().Chat(context.Background(), ChatRequest{
		MaxTokens: 1000,
		Messages:  []Message{TextMessage(RoleUser, "hi")},
		Tools: []Tool{{Type: "function", Function: ToolFunction{
			Name:        "kubectl",
			Description: "read the cluster",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"verb":{"type":"string"}},"required":["verb"],"additionalProperties":false}`),
		}}},
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	tools, _ := stub.bodies[0]["tools"].([]any)
	if len(tools) != 1 {
		t.Fatalf("tools = %v", stub.bodies[0]["tools"])
	}
	tool := tools[0].(map[string]any)
	schema := tool["input_schema"].(map[string]any)
	if tool["name"] != "kubectl" || tool["description"] != "read the cluster" {
		t.Errorf("tool = %v", tool)
	}
	if schema["type"] != "object" || schema["additionalProperties"] != false {
		t.Errorf("input_schema = %v", schema)
	}
	if req, _ := schema["required"].([]any); len(req) != 1 || req[0] != "verb" {
		t.Errorf("required = %v", schema["required"])
	}
	if _, ok := schema["properties"].(map[string]any)["verb"]; !ok {
		t.Errorf("properties = %v", schema["properties"])
	}
}

func TestAnthropicChat_EndTurnAndRefusal(t *testing.T) {
	end := newAnthropicStub(t, 200, `{"id":"m","type":"message","role":"assistant","model":"claude-opus-5-5",
		"stop_reason":"end_turn","content":[{"type":"text","text":"All good."}],"usage":{"input_tokens":1,"output_tokens":1}}`)
	resp, err := end.client().Chat(context.Background(), ChatRequest{MaxTokens: 100, Messages: []Message{TextMessage(RoleUser, "hi")}})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if got := resp.Choices[0]; got.FinishReason != FinishStop || got.Message.Text() != "All good." || len(got.Message.ToolCalls) != 0 {
		t.Errorf("end_turn mapped to %+v", got)
	}

	refused := newAnthropicStub(t, 200, `{"id":"m","type":"message","role":"assistant","model":"claude-opus-5-5",
		"stop_reason":"refusal","content":[],"usage":{"input_tokens":1,"output_tokens":0}}`)
	resp, err = refused.client().Chat(context.Background(), ChatRequest{MaxTokens: 100, Messages: []Message{TextMessage(RoleUser, "hi")}})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if got := resp.Choices[0]; got.FinishReason != FinishStop || strings.TrimSpace(got.Message.Text()) == "" {
		t.Errorf("a refusal must end the turn with a visible answer, got %+v", got)
	}
}

func TestAnthropicChat_MapsErrors(t *testing.T) {
	cases := []struct {
		status int
		want   Kind
	}{
		{429, KindBusy},
		{401, KindAuth},
		{400, KindInvalidRequest},
		{529, KindRetryable},
		{500, KindRetryable},
	}
	for _, tc := range cases {
		stub := newAnthropicStub(t, tc.status, `{"type":"error","error":{"type":"x","message":"nope sk-ant-api03-SECRETSECRETSECRET"}}`)
		_, err := stub.client().Chat(context.Background(), ChatRequest{MaxTokens: 100, Messages: []Message{TextMessage(RoleUser, "hi")}})
		var apiErr *APIError
		if !errors.As(err, &apiErr) {
			t.Fatalf("status %d: want *APIError, got %v", tc.status, err)
		}
		if apiErr.Kind != tc.want || apiErr.Status != tc.status {
			t.Errorf("status %d: kind = %v status = %d, want %v", tc.status, apiErr.Kind, apiErr.Status, tc.want)
		}
		if strings.Contains(apiErr.Message, "SECRETSECRET") {
			t.Errorf("status %d: error message leaks a credential: %q", tc.status, apiErr.Message)
		}
	}
}
