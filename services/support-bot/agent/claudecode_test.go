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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/caesar/all-chat/services/support-bot/access"
	"github.com/caesar/all-chat/services/support-bot/llm"
	"github.com/caesar/all-chat/services/support-bot/tool"
	"go.uber.org/zap"
)

// TestMain doubles as a fake `claude` CLI: Run executes this test binary with
// FAKE_CLAUDE set, and fakeClaude plays the CLI's side of the contract.
func TestMain(m *testing.M) {
	if mode := os.Getenv("FAKE_CLAUDE"); mode != "" {
		os.Exit(fakeClaude(mode))
	}
	os.Exit(m.Run())
}

// fakeClaude reads the MCP config from its args, lists and calls the bot's tools
// over HTTP like Claude Code would, and prints a `--output-format json` result.
func fakeClaude(mode string) int {
	args := os.Args[1:]
	flag := func(name string) string {
		for i, a := range args {
			if a == name && i+1 < len(args) {
				return args[i+1]
			}
		}
		return ""
	}
	prompt, _ := io.ReadAll(os.Stdin)
	result := func(isError bool, subtype, text string) int {
		out, _ := json.Marshal(map[string]any{
			"type": "result", "subtype": subtype, "is_error": isError, "result": text, "num_turns": 2,
		})
		fmt.Println(string(out))
		return 0
	}

	switch mode {
	case "usage-limit":
		return result(true, "success", "Claude AI usage limit reached|1763300000")
	case "max-turns", "max-turns-synth-fails":
		if flag("--mcp-config") == "" {
			// No --mcp-config: this is the no-tools write-up step.
			if mode == "max-turns-synth-fails" {
				return result(true, "success", "API Error: 500 Internal server error")
			}
			if got := flag("--max-turns"); got != "1" {
				fmt.Fprintf(os.Stderr, "write-up step --max-turns = %q, want 1\n", got)
				return 2
			}
			return result(false, "success", "synthesized: "+string(prompt))
		}
		// The main run: spend a tool call, as the run that prompted this fix did,
		// then report the turn limit with the empty result the real CLI prints.
		call, err := mcpClient(flag("--mcp-config"))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 2
		}
		call("initialize", map[string]any{"protocolVersion": "2025-06-18"})
		call("tools/call", map[string]any{"name": "echo", "arguments": map[string]any{"text": "no glow setting"}})
		return result(true, "error_max_turns", "")
	case "env":
		// Report what the CLI was given: secrets the bot holds must not leak in.
		return result(false, "success", fmt.Sprintf("token=%s github=%q home=%s",
			os.Getenv("CLAUDE_CODE_OAUTH_TOKEN"), os.Getenv("GITHUB_TOKEN"), os.Getenv("HOME")))
	}

	call, err := mcpClient(flag("--mcp-config"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}

	call("initialize", map[string]any{"protocolVersion": "2025-06-18"})
	listed := call("tools/list", map[string]any{})
	var names []string
	for _, t := range listed["result"].(map[string]any)["tools"].([]any) {
		names = append(names, t.(map[string]any)["name"].(string))
	}
	called := call("tools/call", map[string]any{"name": "echo", "arguments": map[string]any{"text": "pong"}})
	content := called["result"].(map[string]any)["content"].([]any)[0].(map[string]any)["text"].(string)
	return result(false, "success", fmt.Sprintf("tools=%s echo=%s system=%t prompt=%s",
		strings.Join(names, ","), content, flag("--system-prompt") == "be helpful", strings.TrimSpace(string(prompt))))
}

// mcpClient dials the MCP endpoint named in a --mcp-config value, and returns a
// function that makes one JSON-RPC call against it.
func mcpClient(mcpConfig string) (func(method string, params any) map[string]any, error) {
	var cfg struct {
		MCPServers map[string]struct {
			URL     string            `json:"url"`
			Headers map[string]string `json:"headers"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal([]byte(mcpConfig), &cfg); err != nil {
		return nil, fmt.Errorf("bad --mcp-config: %w", err)
	}
	srv := cfg.MCPServers["allchat"]
	return func(method string, params any) map[string]any {
		body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
		req, _ := http.NewRequest(http.MethodPost, srv.URL, bytes.NewReader(body))
		for k, v := range srv.Headers {
			req.Header.Set(k, v)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return map[string]any{"transport_error": err.Error()}
		}
		defer resp.Body.Close()
		var out map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&out)
		return out
	}, nil
}

// ccEchoTool is available in both modes and reports a side effect.
type ccEchoTool struct{ tool.BothModes }

func (ccEchoTool) Def() tool.ToolDef {
	return tool.ToolDef{Name: "echo", Description: "echo text",
		Parameters: json.RawMessage(`{"type":"object","properties":{"text":{"type":"string"}}}`)}
}
func (ccEchoTool) Invoke(_ context.Context, _ *tool.ToolCtx, args json.RawMessage) (tool.ToolOutput, error) {
	var p struct{ Text string }
	_ = json.Unmarshal(args, &p)
	return tool.ToolOutput{Content: p.Text, Effect: &tool.ToolEffect{Tool: "echo", Kind: "test", Summary: "echoed"},
		Attachment: &tool.Attachment{Name: "echo.png", Path: "/x/echo.png"}}, nil
}

// ccAdminTool must never be listed or callable in a SUPPORT session.
type ccAdminTool struct{ tool.AdminOnly }

func (ccAdminTool) Def() tool.ToolDef {
	return tool.ToolDef{Name: "push", Parameters: json.RawMessage(`{"type":"object"}`)}
}
func (ccAdminTool) Invoke(context.Context, *tool.ToolCtx, json.RawMessage) (tool.ToolOutput, error) {
	return tool.ToolOutput{Content: "pushed"}, nil
}

func newTestClaudeCode(t *testing.T, fakeMode string) *ClaudeCode {
	t.Helper()
	reg := tool.NewRegistry()
	reg.Register(ccEchoTool{})
	reg.Register(ccAdminTool{})
	cc, err := NewClaudeCode(ClaudeCodeConfig{
		Bin:      os.Args[0],
		Token:    "oauth-test-token",
		Model:    "opus",
		MaxTurns: 4,
		Home:     t.TempDir(),
		Env:      []string{"FAKE_CLAUDE=" + fakeMode},
		Log:      zap.NewNop(),
	}, reg)
	if err != nil {
		t.Fatalf("NewClaudeCode: %v", err)
	}
	t.Cleanup(func() { _ = cc.Close() })
	return cc
}

func supportTranscript() []llm.Message {
	return []llm.Message{llm.TextMessage(llm.RoleSystem, "be helpful"), llm.TextMessage(llm.RoleUser, "ping?")}
}

func TestClaudeCodeRun_ServesModeScopedToolsAndCollectsEffects(t *testing.T) {
	cc := newTestClaudeCode(t, "tools")
	res, err := cc.Run(context.Background(), &tool.ToolCtx{Mode: access.ModeSupport}, supportTranscript())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Stop != StopEndTurn {
		t.Errorf("stop = %v", res.Stop)
	}
	if !strings.Contains(res.Text, "tools=echo ") {
		t.Errorf("SUPPORT session must list only both-mode tools: %q", res.Text)
	}
	if !strings.Contains(res.Text, "echo=") || !strings.Contains(res.Text, "pong") {
		t.Errorf("tool call did not round-trip: %q", res.Text)
	}
	if !strings.Contains(res.Text, "system=true") || !strings.Contains(res.Text, "prompt=ping?") {
		t.Errorf("system prompt or user turn not passed: %q", res.Text)
	}
	if len(res.Effects) != 1 || res.Effects[0].Summary != "echoed" {
		t.Errorf("effects = %+v", res.Effects)
	}
	if len(res.Attachments) != 1 || res.Attachments[0].Name != "echo.png" {
		t.Errorf("attachments = %+v", res.Attachments)
	}

	admin, err := cc.Run(context.Background(), &tool.ToolCtx{Mode: access.ModeAdmin}, supportTranscript())
	if err != nil {
		t.Fatalf("admin Run: %v", err)
	}
	if !strings.Contains(admin.Text, "tools=echo,push ") {
		t.Errorf("ADMIN session must list admin tools too: %q", admin.Text)
	}
}

func TestClaudeCodeRun_PassesOnlyTheOAuthToken(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "ghp_must_not_leak")
	cc := newTestClaudeCode(t, "env")
	res, err := cc.Run(context.Background(), &tool.ToolCtx{Mode: access.ModeSupport}, supportTranscript())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(res.Text, "token=oauth-test-token") {
		t.Errorf("CLAUDE_CODE_OAUTH_TOKEN not passed: %q", res.Text)
	}
	if !strings.Contains(res.Text, `github=""`) {
		t.Errorf("the bot's own secrets leaked into the CLI env: %q", res.Text)
	}
}

func TestClaudeCodeRun_UsageLimitIsBusy(t *testing.T) {
	cc := newTestClaudeCode(t, "usage-limit")
	_, err := cc.Run(context.Background(), &tool.ToolCtx{Mode: access.ModeSupport}, supportTranscript())
	var apiErr *llm.APIError
	if !errors.As(err, &apiErr) || apiErr.Kind != llm.KindBusy {
		t.Fatalf("a usage limit must park the request as busy, got %v", err)
	}
}

func TestClaudeCodeRun_MaxTurnsIsPartial(t *testing.T) {
	cc := newTestClaudeCode(t, "max-turns")
	res, err := cc.Run(context.Background(), &tool.ToolCtx{Mode: access.ModeSupport}, supportTranscript())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Stop != StopMaxIterations {
		t.Errorf("stop = %v, want max iterations", res.Stop)
	}
}

func TestClaudeCodeRun_MaxTurnsSynthesizesAnswer(t *testing.T) {
	cc := newTestClaudeCode(t, "max-turns")
	res, err := cc.Run(context.Background(), &tool.ToolCtx{Mode: access.ModeSupport}, supportTranscript())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Stop != StopMaxIterations {
		t.Errorf("stop = %v, want max iterations: the answer is still partial", res.Stop)
	}
	if !strings.HasPrefix(res.Text, "synthesized:") {
		t.Errorf("text = %q, want the write-up step's answer", res.Text)
	}
	if !strings.Contains(res.Text, "Findings so far") {
		t.Errorf("text = %q, want the recorded findings in the write-up prompt", res.Text)
	}
}

func TestClaudeCodeRun_MaxTurnsKeepsEmptyTextWhenSynthesisFails(t *testing.T) {
	cc := newTestClaudeCode(t, "max-turns-synth-fails")
	res, err := cc.Run(context.Background(), &tool.ToolCtx{Mode: access.ModeSupport}, supportTranscript())
	if err != nil {
		t.Fatalf("a failed write-up step must not fail the run: %v", err)
	}
	if res.Stop != StopMaxIterations {
		t.Errorf("stop = %v, want max iterations", res.Stop)
	}
	if res.Text != "" {
		t.Errorf("text = %q, want empty", res.Text)
	}
}

func TestCCSessionToolLog_DropsOldestAndClipsResults(t *testing.T) {
	sess := &ccSession{}
	sess.recordToolCall("echo", `{"n":0}`, strings.Repeat("a", maxToolLogResult+500))
	for i := 1; i < 200; i++ {
		sess.recordToolCall("echo", fmt.Sprintf(`{"n":%d}`, i), strings.Repeat("b", 1000))
	}
	log := sess.toolLogText()
	if len(log) > maxToolLog {
		t.Errorf("tool log is %d bytes, want at most %d", len(log), maxToolLog)
	}
	if strings.Contains(log, `{"n":0}`) {
		t.Error("the oldest entry must be dropped first")
	}
	if !strings.Contains(log, `{"n":199}`) {
		t.Error("the newest entry must be kept")
	}
}

func TestCCSessionToolLog_ClipsALongResult(t *testing.T) {
	sess := &ccSession{}
	sess.recordToolCall("echo", "{}", strings.Repeat("a", maxToolLogResult+500))
	if got := len(sess.toolLogText()); got > maxToolLogResult+300 {
		t.Errorf("a single entry is %d bytes, want the result clipped to about %d", got, maxToolLogResult)
	}
}

func TestClaudeCodeMCP_RejectsUnknownSession(t *testing.T) {
	cc := newTestClaudeCode(t, "tools")
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`
	for _, auth := range []string{"", "Bearer not-a-session"} {
		req, _ := http.NewRequest(http.MethodPost, cc.url, strings.NewReader(body))
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("POST: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("auth %q: status %d, want 401", auth, resp.StatusCode)
		}
	}
}

// TestClaudeCodeRun_RealCLI drives the real `claude` binary against the bot's MCP
// endpoint: the end-to-end proof that Claude Code calls the bot's tools. It spends a
// little of the plan, so it runs only on request, with a `claude setup-token` token:
//
//	SUPPORT_BOT_CLAUDE_E2E=1 CLAUDE_CODE_OAUTH_TOKEN=... go test ./agent -run RealCLI
func TestClaudeCodeRun_RealCLI(t *testing.T) {
	token := os.Getenv("CLAUDE_CODE_OAUTH_TOKEN")
	if os.Getenv("SUPPORT_BOT_CLAUDE_E2E") != "1" || token == "" {
		t.Skip("set SUPPORT_BOT_CLAUDE_E2E=1 and CLAUDE_CODE_OAUTH_TOKEN to run against the real claude CLI")
	}
	reg := tool.NewRegistry()
	reg.Register(ccEchoTool{})
	cc, err := NewClaudeCode(ClaudeCodeConfig{
		Bin: "claude", Token: token, Model: "sonnet", MaxTurns: 4,
		Home: t.TempDir(), Log: zap.NewNop(),
	}, reg)
	if err != nil {
		t.Fatalf("NewClaudeCode: %v", err)
	}
	defer cc.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	res, err := cc.Run(ctx, &tool.ToolCtx{Mode: access.ModeSupport}, []llm.Message{
		llm.TextMessage(llm.RoleSystem, "You are a test harness. Use the tools you are given."),
		llm.TextMessage(llm.RoleUser, "Call the echo tool with text 'zebra-42', then reply with exactly what it returned."),
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(res.Effects) != 1 || !strings.Contains(res.Text, "zebra-42") {
		t.Errorf("real CLI did not call the bot's tool: effects=%+v text=%q", res.Effects, res.Text)
	}
}

// The strings are what the real CLI prints in its JSON `result` on failure.
func TestClassifyCLIError(t *testing.T) {
	cases := map[string]llm.Kind{
		"Failed to authenticate. API Error: 401 OAuth access token is invalid.": llm.KindAuth,
		"Claude AI usage limit reached|1763300000":                              llm.KindBusy,
		"API Error: 529 Overloaded":                                             llm.KindBusy,
		"API Error: 500 Internal server error":                                  llm.KindOther,
	}
	for text, want := range cases {
		var apiErr *llm.APIError
		if err := classifyCLIError(text); !errors.As(err, &apiErr) || apiErr.Kind != want {
			t.Errorf("%q: got %v, want kind %v", text, err, want)
		}
	}
}
