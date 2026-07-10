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

// Package llm is a minimal OpenAI-compatible chat-completions client with function/
// tool calling, targeting a locally hosted model (vLLM/Ollama/LM Studio/llama.cpp).
// It replaces the former Claude-Code-CLI subprocess. The wire types mirror the
// /v1/chat/completions schema exactly; function-call arguments are a JSON-encoded
// string in both directions, and tool results are sent back as role:"tool" messages.
package llm

import "encoding/json"

// Chat roles.
const (
	RoleSystem    = "system"
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleTool      = "tool"
)

// Finish reasons returned by the server.
const (
	FinishStop      = "stop"
	FinishToolCalls = "tool_calls"
	FinishFunc      = "function_call"
	FinishLength    = "length"
	FinishMaxTokens = "max_tokens"
)

// Message is used both outbound (history) and inbound (choices[].message). Content is
// a pointer so an assistant tool-call-only turn can send explicit null and a normal
// turn can send a string; inbound content may be null.
type Message struct {
	Role       string     `json:"role"`
	Content    *string    `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Name       string     `json:"name,omitempty"`
}

// ToolCall is an assistant -> tool request.
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
	Index    int          `json:"index,omitempty"`
}

// FunctionCall carries the requested function name and its arguments. Arguments is a
// JSON-ENCODED STRING (not a nested object), matching the OpenAI wire format.
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// Tool is a function tool advertised to the model.
type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

// ToolFunction describes the callable function.
type ToolFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters"`
}

// ChatRequest is the /v1/chat/completions request body.
type ChatRequest struct {
	Model       string          `json:"model"`
	Messages    []Message       `json:"messages"`
	Tools       []Tool          `json:"tools,omitempty"`
	ToolChoice  json.RawMessage `json:"tool_choice,omitempty"`
	Temperature *float64        `json:"temperature,omitempty"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
}

// ChatResponse is the /v1/chat/completions response body.
type ChatResponse struct {
	ID      string   `json:"id"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

// Choice is a single completion choice.
type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

// Usage is best-effort token accounting (some servers omit it on non-streaming).
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// TextMessage builds a role message carrying plain text content.
func TextMessage(role, content string) Message {
	c := content
	return Message{Role: role, Content: &c}
}

// ToolResultMessage builds the role:"tool" reply for a given tool call id.
func ToolResultMessage(toolCallID, content string) Message {
	c := content
	return Message{Role: RoleTool, ToolCallID: toolCallID, Content: &c}
}

// Text returns the message content or "" if nil.
func (m Message) Text() string {
	if m.Content == nil {
		return ""
	}
	return *m.Content
}
