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

// Package agent runs the non-streaming agentic tool loop: call the LLM, execute any
// requested tools (fail-closed by access mode), feed the results back, and repeat
// until the model stops, the iteration budget is hit, or progress stalls. Its
// guardrails — fail-closed permissioning, repeat detection, and a no-progress abort —
// run on top of the OpenAI-compatible wire format.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/caesar/all-chat/services/support-bot/llm"
	"github.com/caesar/all-chat/services/support-bot/sanitize"
	"github.com/caesar/all-chat/services/support-bot/tool"
	"go.uber.org/zap"
)

// StopReason explains why the loop returned.
type StopReason string

const (
	StopEndTurn       StopReason = "end_turn"
	StopMaxIterations StopReason = "max_iterations"
	StopNoProgress    StopReason = "no_progress"
	StopCancelled     StopReason = "cancelled"
	StopError         StopReason = "error"
)

// Result is the outcome of a loop run. Text is the best-effort final answer even on
// non-EndTurn stops; Effects lists side-effecting tool actions in execution order.
type Result struct {
	Text       string
	Effects    []tool.ToolEffect
	Stop       StopReason
	Iterations int
}

// Config tunes the loop.
type Config struct {
	Model             string
	MaxTokens         int
	MaxIterations     int
	PerCallTimeout    time.Duration
	LoopWindow        int
	RepeatThreshold   int
	MaxParallelTools  int
	NoProgressAbort   int
	MaxInLoopMessages int
	Log               *zap.Logger
}

func (c Config) withDefaults() Config {
	if c.MaxIterations <= 0 {
		c.MaxIterations = 16
	}
	if c.PerCallTimeout <= 0 {
		c.PerCallTimeout = 120 * time.Second
	}
	if c.LoopWindow <= 0 {
		c.LoopWindow = 30
	}
	if c.RepeatThreshold <= 0 {
		c.RepeatThreshold = 5
	}
	if c.MaxParallelTools <= 0 {
		c.MaxParallelTools = 6
	}
	if c.NoProgressAbort <= 0 {
		c.NoProgressAbort = 10
	}
	if c.MaxInLoopMessages <= 0 {
		c.MaxInLoopMessages = 300
	}
	return c
}

// Run executes the loop. tctx.Mode is the (already-decided) access mode; the registry
// only advertises and only runs tools allowed in that mode. messages must already
// contain the system prompt and the initial user turn.
func Run(ctx context.Context, cfg Config, client llm.ChatClient, reg *tool.Registry, tctx *tool.ToolCtx, messages []llm.Message) (Result, error) {
	cfg = cfg.withDefaults()
	tools := toLLMTools(reg.Defs(tctx.Mode))
	det := newLoopDetector(cfg.LoopWindow, cfg.RepeatThreshold)

	var (
		lastText string
		effects  []tool.ToolEffect
		noText   int
	)

	for iter := 0; iter < cfg.MaxIterations; iter++ {
		if err := ctx.Err(); err != nil {
			return Result{Text: lastText, Effects: effects, Stop: StopCancelled, Iterations: iter}, err
		}

		req := llm.ChatRequest{Model: cfg.Model, Messages: messages, MaxTokens: cfg.MaxTokens}
		if len(tools) > 0 {
			req.Tools = tools
		}

		callCtx, cancel := context.WithTimeout(ctx, cfg.PerCallTimeout)
		resp, err := client.Chat(callCtx, req)
		cancel()
		if err != nil {
			return Result{Text: lastText, Effects: effects, Stop: StopError, Iterations: iter}, err
		}
		if len(resp.Choices) == 0 {
			return Result{Text: lastText, Effects: effects, Stop: StopError, Iterations: iter},
				fmt.Errorf("llm returned no choices")
		}

		choice := resp.Choices[0]
		msg := choice.Message
		messages = append(messages, msg) // append assistant turn verbatim (incl. tool_calls)

		text := msg.Text()
		if strings.TrimSpace(text) != "" {
			lastText = text
		}

		// End of turn: no tool calls requested.
		if len(msg.ToolCalls) == 0 {
			if strings.TrimSpace(text) == "" &&
				(choice.FinishReason == llm.FinishLength || choice.FinishReason == llm.FinishMaxTokens) {
				return Result{Text: lastText, Effects: effects, Stop: StopError, Iterations: iter + 1},
					fmt.Errorf("llm response truncated (max_tokens) with no content")
			}
			return Result{Text: lastText, Effects: effects, Stop: StopEndTurn, Iterations: iter + 1}, nil
		}

		// Track no-progress (tool calls but no assistant text).
		if strings.TrimSpace(text) == "" {
			noText++
		} else {
			noText = 0
		}

		// Execute each requested call, always emitting exactly one tool result per
		// tool_call_id so the next request's tool pairing is valid.
		for i, tc := range msg.ToolCalls {
			if i >= cfg.MaxParallelTools {
				messages = append(messages, llm.ToolResultMessage(tc.ID,
					wrapErr(tc.Function.Name, fmt.Sprintf("skipped: this turn asked for more than the %d tool calls allowed at once", cfg.MaxParallelTools))))
				continue
			}
			name := tc.Function.Name
			rawArgs := tc.Function.Arguments
			if det.wouldBlock(name, rawArgs) {
				messages = append(messages, llm.ToolResultMessage(tc.ID,
					wrapErr(name, fmt.Sprintf("skipped: %q was already run %d times with the same arguments — change the approach or answer with what you have", name, cfg.RepeatThreshold))))
				continue
			}
			det.record(name, rawArgs)

			args := json.RawMessage(strings.TrimSpace(rawArgs))
			if len(args) == 0 {
				args = json.RawMessage("{}")
			}
			dr := reg.Dispatch(ctx, tctx, name, args)
			messages = append(messages, llm.ToolResultMessage(tc.ID, dr.Content))
			if dr.Effect != nil {
				effects = append(effects, *dr.Effect)
			}
		}

		messages = truncateInLoop(messages, cfg.MaxInLoopMessages)

		if noText >= cfg.NoProgressAbort {
			if cfg.Log != nil {
				cfg.Log.Warn("agent loop aborting: no textual progress",
					zap.Int("iterations", iter+1), zap.Int("no_text_iters", noText))
			}
			return Result{Text: lastText, Effects: effects, Stop: StopNoProgress, Iterations: iter + 1}, nil
		}
	}

	if cfg.Log != nil {
		cfg.Log.Warn("agent loop hit max iterations", zap.Int("max", cfg.MaxIterations))
	}
	return Result{Text: lastText, Effects: effects, Stop: StopMaxIterations, Iterations: cfg.MaxIterations}, nil
}

// wrapErr mirrors the registry's error-result wrapping for loop-internal errors
// (parallel cap, repeat block) that never reach a Tool.
func wrapErr(toolName, msg string) string {
	return sanitize.WrapToolError(toolName, msg)
}

func toLLMTools(defs []tool.ToolDef) []llm.Tool {
	out := make([]llm.Tool, 0, len(defs))
	for _, d := range defs {
		params := d.Parameters
		if len(params) == 0 {
			params = json.RawMessage(`{"type":"object","properties":{}}`)
		}
		out = append(out, llm.Tool{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        d.Name,
				Description: d.Description,
				Parameters:  params,
			},
		})
	}
	return out
}

// truncateInLoop keeps the transcript under cap by dropping the oldest middle messages
// while preserving the first message (system) and the recent tail, then removes any
// orphaned leading tool messages so the OpenAI tool-pairing invariant holds.
func truncateInLoop(messages []llm.Message, cap int) []llm.Message {
	if cap <= 0 || len(messages) <= cap {
		return messages
	}
	head := messages[0]
	tail := messages[len(messages)-(cap-1):]
	// A tool message must be preceded by an assistant message with tool_calls; after
	// slicing, drop any leading tool-role messages that would be orphaned.
	for len(tail) > 0 && tail[0].Role == llm.RoleTool {
		tail = tail[1:]
	}
	out := make([]llm.Message, 0, len(tail)+1)
	out = append(out, head)
	out = append(out, tail...)
	return out
}
