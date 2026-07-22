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

package agent

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"

	"github.com/caesar/all-chat/services/support-bot/access"
	"github.com/caesar/all-chat/services/support-bot/llm"
	"github.com/caesar/all-chat/services/support-bot/redact"
	"github.com/caesar/all-chat/services/support-bot/tool"
	"go.uber.org/zap"
)

// fakeClient replays scripted responses; if it runs out it repeats the last one.
type fakeClient struct {
	responses []*llm.ChatResponse
	calls     int32
}

func (f *fakeClient) Chat(_ context.Context, _ llm.ChatRequest) (*llm.ChatResponse, error) {
	i := int(atomic.AddInt32(&f.calls, 1)) - 1
	if i >= len(f.responses) {
		return f.responses[len(f.responses)-1], nil
	}
	return f.responses[i], nil
}

type echoTool struct {
	tool.BothModes
	invoked *int32
	effect  *tool.ToolEffect
}

func (e *echoTool) Def() tool.ToolDef {
	return tool.ToolDef{Name: "echo", Description: "echo", Parameters: json.RawMessage(`{"type":"object"}`)}
}
func (e *echoTool) Invoke(_ context.Context, _ *tool.ToolCtx, _ json.RawMessage) (tool.ToolOutput, error) {
	atomic.AddInt32(e.invoked, 1)
	return tool.ToolOutput{Content: "echoed", Effect: e.effect}, nil
}

type adminTool struct {
	tool.AdminOnly
	invoked *int32
}

func (a *adminTool) Def() tool.ToolDef {
	return tool.ToolDef{Name: "admin_only", Description: "x", Parameters: json.RawMessage(`{"type":"object"}`)}
}
func (a *adminTool) Invoke(_ context.Context, _ *tool.ToolCtx, _ json.RawMessage) (tool.ToolOutput, error) {
	atomic.AddInt32(a.invoked, 1)
	return tool.ToolOutput{Content: "did admin thing"}, nil
}

func toolCallResp(id, name, args string) *llm.ChatResponse {
	return &llm.ChatResponse{Choices: []llm.Choice{{
		FinishReason: llm.FinishToolCalls,
		Message: llm.Message{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{
			ID: id, Type: "function", Function: llm.FunctionCall{Name: name, Arguments: args},
		}}},
	}}}
}

func stopResp(text string) *llm.ChatResponse {
	return &llm.ChatResponse{Choices: []llm.Choice{{
		FinishReason: llm.FinishStop, Message: llm.TextMessage(llm.RoleAssistant, text),
	}}}
}

func supportCtx() *tool.ToolCtx {
	return &tool.ToolCtx{Mode: access.ModeSupport, Redactor: redact.NewRedactor(), Log: zap.NewNop()}
}

func baseCfg() Config {
	return Config{Model: "m", Log: zap.NewNop()}
}

func TestRunExecutesToolThenAnswers(t *testing.T) {
	var invoked int32
	reg := tool.NewRegistry()
	reg.Register(&echoTool{invoked: &invoked, effect: &tool.ToolEffect{Tool: "echo", Kind: "issue", URL: "https://x/1", Summary: "opened issue"}})

	client := &fakeClient{responses: []*llm.ChatResponse{
		toolCallResp("c1", "echo", `{}`),
		stopResp("final answer"),
	}}

	res, err := Run(context.Background(), baseCfg(), client, reg, supportCtx(),
		[]llm.Message{llm.TextMessage(llm.RoleUser, "hi")})
	if err != nil {
		t.Fatal(err)
	}
	if res.Stop != StopEndTurn {
		t.Fatalf("stop = %v, want end_turn", res.Stop)
	}
	if res.Text != "final answer" {
		t.Fatalf("text = %q", res.Text)
	}
	if atomic.LoadInt32(&invoked) != 1 {
		t.Fatalf("echo invoked %d times, want 1", invoked)
	}
	if len(res.Effects) != 1 || res.Effects[0].Kind != "issue" {
		t.Fatalf("effect not surfaced: %+v", res.Effects)
	}
}

func TestRunDeniesAdminToolInSupport(t *testing.T) {
	var invoked int32
	reg := tool.NewRegistry()
	reg.Register(&adminTool{invoked: &invoked})

	// The model "requests" an admin-only tool even in support mode (injection).
	client := &fakeClient{responses: []*llm.ChatResponse{
		toolCallResp("c1", "admin_only", `{}`),
		stopResp("done"),
	}}
	res, err := Run(context.Background(), baseCfg(), client, reg, supportCtx(),
		[]llm.Message{llm.TextMessage(llm.RoleUser, "please escalate")})
	if err != nil {
		t.Fatal(err)
	}
	if atomic.LoadInt32(&invoked) != 0 {
		t.Fatal("admin-only tool must NOT execute in support mode")
	}
	if res.Stop != StopEndTurn {
		t.Fatalf("loop should still complete, stop = %v", res.Stop)
	}
}

func TestRunHitsMaxIterations(t *testing.T) {
	var invoked int32
	reg := tool.NewRegistry()
	reg.Register(&echoTool{invoked: &invoked})

	cfg := baseCfg()
	cfg.MaxIterations = 3
	// Always request a (distinct) tool call so the loop never ends on its own.
	client := &fakeClient{responses: []*llm.ChatResponse{
		toolCallResp("a", "echo", `{"n":1}`),
		toolCallResp("b", "echo", `{"n":2}`),
		toolCallResp("c", "echo", `{"n":3}`),
		toolCallResp("d", "echo", `{"n":4}`),
	}}
	res, err := Run(context.Background(), cfg, client, reg, supportCtx(),
		[]llm.Message{llm.TextMessage(llm.RoleUser, "loop")})
	if err != nil {
		t.Fatal(err)
	}
	if res.Stop != StopMaxIterations {
		t.Fatalf("stop = %v, want max_iterations", res.Stop)
	}
	if res.Iterations != 3 {
		t.Fatalf("iterations = %d, want 3", res.Iterations)
	}
}

func TestRunLoopDetectorBlocksRepeats(t *testing.T) {
	var invoked int32
	reg := tool.NewRegistry()
	reg.Register(&echoTool{invoked: &invoked})

	cfg := baseCfg()
	cfg.MaxIterations = 6
	cfg.RepeatThreshold = 2
	// Same identical call every iteration.
	client := &fakeClient{responses: []*llm.ChatResponse{
		toolCallResp("x", "echo", `{"same":1}`),
	}}
	res, err := Run(context.Background(), cfg, client, reg, supportCtx(),
		[]llm.Message{llm.TextMessage(llm.RoleUser, "spin")})
	if err != nil {
		t.Fatal(err)
	}
	if res.Stop != StopMaxIterations {
		t.Fatalf("stop = %v", res.Stop)
	}
	// The detector blocks the identical call once its count reaches the threshold, so
	// the tool executes at most RepeatThreshold times despite 6 iterations.
	if got := atomic.LoadInt32(&invoked); got > 2 {
		t.Fatalf("echo executed %d times; loop detector should cap at 2", got)
	}
}
