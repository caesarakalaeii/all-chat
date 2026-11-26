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
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/caesar/all-chat/services/support-bot/llm"
	"github.com/caesar/all-chat/services/support-bot/tool"
	"go.uber.org/zap"
)

// mcpServerName is how the CLI names the bot's tools: mcp__allchat__<tool>.
const mcpServerName = "allchat"

// ClaudeCodeConfig configures the Claude Code runner (ADR-0093).
type ClaudeCodeConfig struct {
	Bin      string   // the claude CLI
	Token    string   // CLAUDE_CODE_OAUTH_TOKEN from `claude setup-token`; empty = the CLI's own login
	Model    string   // --model, e.g. "opus"
	MaxTurns int      // --max-turns
	Home     string   // writable HOME for the CLI; its work/ subdir is the empty cwd
	Env      []string // extra environment for the CLI (e.g. HTTPS_PROXY)
	Log      *zap.Logger
}

// ClaudeCode answers by running the unmodified Claude Code CLI (`claude -p`) with the
// subscription OAuth token, instead of the in-process loop. The CLI gets no built-in
// tools: the bot's own registry is served to it over a localhost MCP endpoint, and each
// run gets a session bound to the requester's access mode, so the fail-closed per-mode
// gate, the redaction and the effect reporting stay exactly as in Run.
type ClaudeCode struct {
	cfg ClaudeCodeConfig
	reg *tool.Registry
	srv *http.Server
	url string

	mu       sync.Mutex
	sessions map[string]*ccSession
}

// ccSession is one CLI run's view of the tool registry.
type ccSession struct {
	tctx *tool.ToolCtx

	// ponytail: one lock serialises a run's tool calls (the loop detector and effects
	// are shared); per-call locking if parallel kubectl/grafana reads ever matter.
	mu          sync.Mutex
	det         *loopDetector
	effects     []tool.ToolEffect
	attachments []tool.Attachment

	// toolLog is what the run found, kept so that a run which spends its whole turn
	// budget on tools can still be written up into an answer. It is bounded: the
	// oldest entries go first, because the newest ones are the ones the model was
	// closing in on.
	toolLog      []toolLogEntry
	toolLogBytes int
}

// toolLogEntry is one tool call and what it returned, blocked and failed calls
// included: "that lookup found nothing" is itself a finding worth writing up.
type toolLogEntry struct {
	name   string
	args   string
	result string
}

func (e toolLogEntry) size() int { return len(e.name) + len(e.args) + len(e.result) }

const (
	// maxToolLogResult bounds one tool result, and maxToolLog the whole log, so the
	// write-up prompt stays a small fraction of the model's context.
	maxToolLogResult = 2 << 10
	maxToolLog       = 24 << 10
)

// recordToolCall appends one call to the session's bounded log, dropping the oldest
// entries once the log is full. call holds s.mu around it.
func (s *ccSession) recordToolCall(name, args, result string) {
	e := toolLogEntry{name: name, args: args, result: clipTo(result, maxToolLogResult)}
	s.toolLog = append(s.toolLog, e)
	s.toolLogBytes += e.size()
	for s.toolLogBytes > maxToolLog && len(s.toolLog) > 1 {
		s.toolLogBytes -= s.toolLog[0].size()
		s.toolLog = s.toolLog[1:]
	}
}

// toolLogText renders the recorded calls for the write-up prompt.
func (s *ccSession) toolLogText() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var b strings.Builder
	for _, e := range s.toolLog {
		fmt.Fprintf(&b, "## %s %s\n%s\n\n", e.name, e.args, e.result)
	}
	return strings.TrimSpace(b.String())
}

// clipTo cuts s to at most n bytes, marking that it was cut.
func clipTo(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "\n…(truncated)"
}

// NewClaudeCode starts the MCP endpoint on 127.0.0.1 and prepares the CLI's HOME.
func NewClaudeCode(cfg ClaudeCodeConfig, reg *tool.Registry) (*ClaudeCode, error) {
	if err := os.MkdirAll(filepath.Join(cfg.Home, "work"), 0o700); err != nil {
		return nil, fmt.Errorf("claude home: %w", err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("mcp listener: %w", err)
	}
	c := &ClaudeCode{cfg: cfg, reg: reg, sessions: map[string]*ccSession{}}
	c.url = "http://" + ln.Addr().String() + "/mcp"
	c.srv = &http.Server{Handler: http.HandlerFunc(c.serveMCP), ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = c.srv.Serve(ln) }()
	return c, nil
}

// Close stops the MCP endpoint.
func (c *ClaudeCode) Close() error { return c.srv.Close() }

// Run answers one question. messages is the transcript the bot builds for the loop:
// its system messages become --system-prompt and its user messages the prompt.
func (c *ClaudeCode) Run(ctx context.Context, tctx *tool.ToolCtx, messages []llm.Message) (Result, error) {
	var system, prompt []string
	for _, m := range messages {
		switch m.Role {
		case llm.RoleSystem:
			system = append(system, m.Text())
		case llm.RoleUser:
			prompt = append(prompt, m.Text())
		}
	}

	token, sess := c.open(tctx)
	defer c.close(token)

	mcpConfig, _ := json.Marshal(map[string]any{"mcpServers": map[string]any{mcpServerName: map[string]any{
		"type": "http", "url": c.url, "headers": map[string]string{"Authorization": "Bearer " + token},
	}}})
	args := []string{
		"-p", "--output-format", "json",
		"--model", c.cfg.Model,
		"--max-turns", strconv.Itoa(c.cfg.MaxTurns),
		"--system-prompt", strings.Join(system, "\n\n"),
		"--tools", "", // no built-in tools: no shell, no file access, no web
		"--mcp-config", string(mcpConfig),
		"--strict-mcp-config",
		"--allowedTools", "mcp__" + mcpServerName,
		"--no-session-persistence",
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, c.cfg.Bin, args...)
	cmd.WaitDelay = 5 * time.Second
	cmd.Dir = filepath.Join(c.cfg.Home, "work") // empty: no CLAUDE.md, no project settings
	cmd.Env = c.env()
	cmd.Stdin = strings.NewReader(strings.Join(prompt, "\n\n"))
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()

	sess.mu.Lock()
	res := Result{Effects: sess.effects, Attachments: sess.attachments, Messages: messages, Stop: StopError}
	sess.mu.Unlock()
	if ctx.Err() != nil {
		res.Stop = StopCancelled
		return res, ctx.Err()
	}

	var out struct {
		Subtype  string `json:"subtype"`
		IsError  bool   `json:"is_error"`
		Result   string `json:"result"`
		NumTurns int    `json:"num_turns"`
	}
	if err := json.Unmarshal(lastLine(stdout.Bytes()), &out); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if runErr != nil {
			detail = runErr.Error() + ": " + detail
		}
		return res, classifyCLIError(detail)
	}
	res.Text, res.Iterations = out.Result, out.NumTurns
	switch {
	case out.Subtype == "error_max_turns":
		res.Stop = StopMaxIterations
		if strings.TrimSpace(out.Result) == "" {
			text, err := c.synthesize(ctx, strings.Join(system, "\n\n"), strings.Join(prompt, "\n\n"), sess)
			if ctx.Err() != nil {
				res.Stop = StopCancelled
				return res, ctx.Err()
			}
			// A failed write-up step must not fail the run, and in particular must not
			// turn a usage limit there into a busy error for the whole request: the
			// main run already succeeded at everything it managed to do.
			if err != nil {
				if c.cfg.Log != nil {
					c.cfg.Log.Warn("write-up step after max turns failed", zap.Error(err))
				}
			} else {
				res.Text = text
			}
		}
		return res, nil
	case out.IsError:
		return res, classifyCLIError(out.Result)
	}
	res.Stop = StopEndTurn
	return res, nil
}

// synthesize runs the CLI once more with no tools at all, to turn what the run did
// find into an answer. The main run hit --max-turns and printed nothing, so without
// this the user gets only the "I stopped early" prefix and no findings.
//
// It deliberately passes no --mcp-config, --strict-mcp-config or --allowedTools: with
// the turn budget already gone, this step must write, not search.
func (c *ClaudeCode) synthesize(ctx context.Context, system, prompt string, sess *ccSession) (string, error) {
	findings := sess.toolLogText()
	if findings == "" {
		return "", fmt.Errorf("no findings to write up")
	}
	args := []string{
		"-p", "--output-format", "json",
		"--model", c.cfg.Model,
		"--max-turns", "1",
		"--system-prompt", system,
		"--tools", "",
		"--no-session-persistence",
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, c.cfg.Bin, args...)
	cmd.WaitDelay = 5 * time.Second
	cmd.Dir = filepath.Join(c.cfg.Home, "work")
	cmd.Env = c.env()
	cmd.Stdin = strings.NewReader(prompt + "\n\nFindings so far (tool budget exhausted):\n\n" + findings +
		"\n\nThe tool budget is gone, so you cannot look anything up. Answer the user's " +
		"question now from these findings alone, say plainly what is still unknown, and do not call tools.")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()
	if ctx.Err() != nil {
		return "", ctx.Err()
	}

	var out struct {
		IsError bool   `json:"is_error"`
		Result  string `json:"result"`
	}
	if err := json.Unmarshal(lastLine(stdout.Bytes()), &out); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if runErr != nil {
			detail = runErr.Error() + ": " + detail
		}
		return "", fmt.Errorf("write-up step: %s", clip(detail))
	}
	if out.IsError {
		return "", fmt.Errorf("write-up step: %s", clip(out.Result))
	}
	if strings.TrimSpace(out.Result) == "" {
		return "", fmt.Errorf("write-up step returned no text")
	}
	return out.Result, nil
}

// env is the CLI's whole environment: the bot's own secrets (GitHub, Discord,
// database, Grafana) are never passed through.
func (c *ClaudeCode) env() []string {
	env := []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + c.cfg.Home,
		"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1",
		"DISABLE_AUTOUPDATER=1",
	}
	if c.cfg.Token != "" {
		env = append(env, "CLAUDE_CODE_OAUTH_TOKEN="+c.cfg.Token)
	}
	return append(env, c.cfg.Env...)
}

var (
	busyCLIError = regexp.MustCompile(`(?i)usage limit|rate.?limit|hit your limit|overloaded|\b(429|529)\b`)
	authCLIError = regexp.MustCompile(`(?i)\b(401|403)\b|oauth|invalid api key|authenticat|log ?in`)
)

// classifyCLIError maps a CLI failure onto the loop's error kinds, so a usage or
// rate limit parks the request in the busy queue like a gateway 429 does.
func classifyCLIError(text string) error {
	msg := clip(text)
	switch {
	case busyCLIError.MatchString(text):
		return &llm.APIError{Kind: llm.KindBusy, Message: msg}
	case authCLIError.MatchString(text):
		return &llm.APIError{Kind: llm.KindAuth, Message: msg}
	default:
		return &llm.APIError{Kind: llm.KindOther, Message: msg}
	}
}

// clip bounds CLI output quoted in an error; the CLI never prints its token.
func clip(s string) string {
	if len(s) > 500 {
		s = s[:500] + "..."
	}
	return s
}

// lastLine returns the last non-empty line: --output-format json prints one object.
func lastLine(b []byte) []byte {
	lines := bytes.Split(bytes.TrimSpace(b), []byte("\n"))
	return lines[len(lines)-1]
}

func (c *ClaudeCode) open(tctx *tool.ToolCtx) (string, *ccSession) {
	raw := make([]byte, 32)
	_, _ = rand.Read(raw)
	token := hex.EncodeToString(raw)
	sess := &ccSession{tctx: tctx, det: newLoopDetector(30, 5)}
	c.mu.Lock()
	c.sessions[token] = sess
	c.mu.Unlock()
	return token, sess
}

func (c *ClaudeCode) close(token string) {
	c.mu.Lock()
	delete(c.sessions, token)
	c.mu.Unlock()
}

// serveMCP is a minimal MCP Streamable HTTP server (JSON responses, no SSE): just
// initialize, tools/list, tools/call and ping. The bearer token selects the session,
// and with it the access mode; an unknown token gets nothing.
func (c *ClaudeCode) serveMCP(w http.ResponseWriter, r *http.Request) {
	c.mu.Lock()
	sess := c.sessions[strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")]
	c.mu.Unlock()
	if sess == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID     json.RawMessage `json:"id"`
		Method string          `json:"method"`
		Params json.RawMessage `json:"params"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if len(req.ID) == 0 { // a notification: nothing to answer
		w.WriteHeader(http.StatusAccepted)
		return
	}

	result, rpcErr := c.handle(r.Context(), sess, req.Method, req.Params)
	resp := map[string]any{"jsonrpc": "2.0", "id": req.ID}
	if rpcErr != nil {
		resp["error"] = rpcErr
	} else {
		resp["result"] = result
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (c *ClaudeCode) handle(ctx context.Context, sess *ccSession, method string, params json.RawMessage) (any, map[string]any) {
	switch method {
	case "initialize":
		var p struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		_ = json.Unmarshal(params, &p)
		if p.ProtocolVersion == "" {
			p.ProtocolVersion = "2025-06-18"
		}
		return map[string]any{
			"protocolVersion": p.ProtocolVersion,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "allchat-support-bot", "version": "1"},
		}, nil
	case "ping":
		return map[string]any{}, nil
	case "tools/list":
		var tools []map[string]any
		for _, d := range c.reg.Defs(sess.tctx.Mode) {
			schema := d.Parameters
			if len(schema) == 0 {
				schema = json.RawMessage(`{"type":"object","properties":{}}`)
			}
			tools = append(tools, map[string]any{"name": d.Name, "description": d.Description, "inputSchema": schema})
		}
		return map[string]any{"tools": tools}, nil
	case "tools/call":
		var p struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, map[string]any{"code": -32602, "message": "invalid params"}
		}
		args := strings.TrimSpace(string(p.Arguments))
		if args == "" || args == "null" {
			args = "{}"
		}
		return c.call(ctx, sess, p.Name, args), nil
	default:
		return nil, map[string]any{"code": -32601, "message": "method not found: " + method}
	}
}

// call dispatches one tool call through the registry (the per-mode gate), with the
// same repeat guard as the in-process loop, and records any side effect.
func (c *ClaudeCode) call(ctx context.Context, sess *ccSession, name, args string) map[string]any {
	sess.mu.Lock()
	defer sess.mu.Unlock()
	text := func(s string, isErr bool) map[string]any {
		return map[string]any{"content": []map[string]any{{"type": "text", "text": s}}, "isError": isErr}
	}
	if sess.det.wouldBlock(name, args) {
		blocked := wrapErr(name, fmt.Sprintf("skipped: %q was already run %d times with the same arguments — change the approach or answer with what you have", name, sess.det.threshold))
		sess.recordToolCall(name, args, blocked)
		return text(blocked, true)
	}
	sess.det.record(name, args)
	dr := c.reg.Dispatch(ctx, sess.tctx, name, json.RawMessage(args))
	sess.recordToolCall(name, args, dr.Content)
	if dr.Effect != nil {
		sess.effects = append(sess.effects, *dr.Effect)
	}
	if dr.Attachment != nil {
		sess.attachments = append(sess.attachments, *dr.Attachment)
	}
	return text(dr.Content, dr.IsError)
}
