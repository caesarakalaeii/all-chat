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
	"strings"

	"github.com/anthropics/anthropic-sdk-go/v2"
	"github.com/anthropics/anthropic-sdk-go/v2/option"
)

// refusalText is what the user sees when Claude (and its server-side fallback)
// declines a request: the turn must still end with a visible answer.
const refusalText = "I can't help with that request. A maintainer can take a look if you open a GitHub issue."

// AnthropicConfig configures the Claude (Anthropic Messages API) client.
type AnthropicConfig struct {
	APIKey     string
	Model      string // used when a request leaves Model empty
	Effort     string // output_config.effort; empty = the model's default
	BaseURL    string // empty = https://api.anthropic.com
	MaxRetries int    // SDK retries on 408/409/429/5xx and connection errors
}

// AnthropicClient implements ChatClient over the Anthropic Messages API, so the
// agent loop and its OpenAI-shaped transcript stay provider-agnostic. Each assistant
// turn keeps its native content in Message.Native and is replayed verbatim: Claude's
// thinking blocks are bound to the conversation that produced them and must come
// back unchanged.
type AnthropicClient struct {
	client anthropic.Client
	model  string
	effort string
}

// NewAnthropic builds the Claude client.
func NewAnthropic(cfg AnthropicConfig) *AnthropicClient {
	opts := []option.RequestOption{
		option.WithAPIKey(cfg.APIKey),
		option.WithMaxRetries(cfg.MaxRetries),
	}
	if cfg.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(cfg.BaseURL))
	}
	return &AnthropicClient{client: anthropic.NewClient(opts...), model: cfg.Model, effort: cfg.Effort}
}

// Chat performs one Messages API round-trip.
func (c *AnthropicClient) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	model := req.Model
	if model == "" {
		model = c.model
	}
	system, messages := toAnthropicMessages(req.Messages)
	params := anthropic.BetaMessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: int64(req.MaxTokens),
		System:    system,
		Messages:  messages,
		Tools:     toAnthropicTools(req.Tools),
		// The tool loop resends a growing prefix every iteration; cache it.
		CacheControl: anthropic.NewBetaCacheControlEphemeralParam(),
		// A safety-classifier refusal is re-served by a fallback model inside the
		// same call instead of ending the user's request.
		Betas:     []anthropic.AnthropicBeta{anthropic.AnthropicBetaServerSideFallback2026_07_01},
		Fallbacks: anthropic.BetaFallbacksParamOfDefault(),
	}
	if c.effort != "" {
		params.OutputConfig = anthropic.BetaOutputConfigParam{Effort: anthropic.BetaOutputConfigEffort(c.effort)}
	}

	resp, err := c.client.Beta.Messages.New(ctx, params)
	if err != nil {
		return nil, mapAnthropicError(ctx, err)
	}
	return fromAnthropic(resp), nil
}

// toAnthropicMessages converts the OpenAI-shaped transcript: system messages become
// the top-level system prompt, and each run of role:"tool" messages becomes ONE user
// turn of tool_result blocks (the API requires all results of a turn together).
func toAnthropicMessages(in []Message) ([]anthropic.BetaTextBlockParam, []anthropic.BetaMessageParam) {
	var (
		system  []anthropic.BetaTextBlockParam
		out     []anthropic.BetaMessageParam
		results []anthropic.BetaContentBlockParamUnion
	)
	flush := func() {
		if len(results) > 0 {
			out = append(out, anthropic.NewBetaUserMessage(results...))
			results = nil
		}
	}
	for _, m := range in {
		if m.Role == RoleTool {
			results = append(results, anthropic.NewBetaToolResultBlock(m.ToolCallID, m.Text(), false))
			continue
		}
		flush()
		switch m.Role {
		case RoleSystem:
			system = append(system, anthropic.BetaTextBlockParam{Text: m.Text()})
		case RoleUser:
			out = append(out, anthropic.NewBetaUserMessage(anthropic.NewBetaTextBlock(m.Text())))
		case RoleAssistant:
			if native, ok := m.Native.(anthropic.BetaMessageParam); ok {
				out = append(out, native)
				continue
			}
			var blocks []anthropic.BetaContentBlockParamUnion
			if strings.TrimSpace(m.Text()) != "" {
				blocks = append(blocks, anthropic.NewBetaTextBlock(m.Text()))
			}
			for _, tc := range m.ToolCalls {
				args := strings.TrimSpace(tc.Function.Arguments)
				if args == "" {
					args = "{}"
				}
				blocks = append(blocks, anthropic.NewBetaToolUseBlock(tc.ID, json.RawMessage(args), tc.Function.Name))
			}
			if len(blocks) > 0 {
				out = append(out, anthropic.BetaMessageParam{OfAssistant: &anthropic.BetaAssistantMessageParam{Content: blocks}})
			}
		}
	}
	flush()
	return system, out
}

// toAnthropicTools maps function tools; the JSON schema's properties/required go in
// their typed fields and every other keyword (additionalProperties, ...) rides along.
func toAnthropicTools(in []Tool) []anthropic.BetaToolUnionParam {
	out := make([]anthropic.BetaToolUnionParam, 0, len(in))
	for _, t := range in {
		var schema map[string]any
		_ = json.Unmarshal(t.Function.Parameters, &schema)
		input := anthropic.BetaToolInputSchemaParam{Properties: schema["properties"], ExtraFields: map[string]any{}}
		if req, ok := schema["required"].([]any); ok {
			for _, r := range req {
				if s, ok := r.(string); ok {
					input.Required = append(input.Required, s)
				}
			}
		}
		for k, v := range schema {
			if k != "properties" && k != "required" && k != "type" {
				input.ExtraFields[k] = v
			}
		}
		tool := anthropic.BetaToolParam{Name: t.Function.Name, InputSchema: input}
		if t.Function.Description != "" {
			tool.Description = anthropic.String(t.Function.Description)
		}
		out = append(out, anthropic.BetaToolUnionParam{OfTool: &tool})
	}
	return out
}

// fromAnthropic maps a Messages response back to the single-choice OpenAI shape.
func fromAnthropic(resp *anthropic.BetaMessage) *ChatResponse {
	var (
		texts []string
		calls []ToolCall
	)
	for _, block := range resp.Content {
		switch b := block.AsAny().(type) {
		case anthropic.BetaTextBlock:
			if b.Text != "" {
				texts = append(texts, b.Text)
			}
		case anthropic.BetaToolUseBlock:
			calls = append(calls, ToolCall{
				ID:       b.ID,
				Type:     "function",
				Function: FunctionCall{Name: b.Name, Arguments: b.JSON.Input.Raw()},
			})
		}
	}
	text := strings.Join(texts, "\n")

	finish := FinishStop
	switch resp.StopReason {
	case anthropic.BetaStopReasonToolUse:
		finish = FinishToolCalls
	case anthropic.BetaStopReasonMaxTokens:
		finish = FinishLength
	case anthropic.BetaStopReasonRefusal:
		calls = nil
		if strings.TrimSpace(text) == "" {
			text = refusalText
		}
	}

	msg := Message{Role: RoleAssistant, Content: &text, ToolCalls: calls, Native: resp.ToParam()}
	return &ChatResponse{
		ID:      resp.ID,
		Model:   string(resp.Model),
		Choices: []Choice{{Message: msg, FinishReason: finish}},
		Usage: Usage{
			PromptTokens:     int(resp.Usage.InputTokens),
			CompletionTokens: int(resp.Usage.OutputTokens),
			TotalTokens:      int(resp.Usage.InputTokens + resp.Usage.OutputTokens),
		},
	}
}

// mapAnthropicError converts an SDK error into the loop's typed APIError, masking
// credentials. Cancellation passes through untouched so the loop reports it as such.
func mapAnthropicError(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	var apiErr *anthropic.Error
	if errors.As(err, &apiErr) {
		return mapStatus(apiErr.StatusCode, truncate(maskCredentials(apiErr.Error()), 500))
	}
	return &APIError{Kind: KindUnavailable, Message: maskCredentials(err.Error())}
}
