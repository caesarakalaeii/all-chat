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

package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/caesar/all-chat/services/support-bot/access"
	"github.com/caesar/all-chat/services/support-bot/tool"
)

const maxKubeOutput = 16000

// KubectlTool runs a fixed set of READ-ONLY kubectl actions. There is no write action;
// writes are structurally impossible and the ServiceAccount RBAC is get/list/watch
// only. Both access modes may use it; support-mode log output is summarized, not raw.
type KubectlTool struct{ tool.BothModes }

type kubectlParams struct {
	Action    string `json:"action"`
	Resource  string `json:"resource,omitempty"`
	Name      string `json:"name,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Selector  string `json:"selector,omitempty"`
	Container string `json:"container,omitempty"`
	Since     string `json:"since,omitempty"`
	TailLines int    `json:"tail_lines,omitempty"`
}

func (KubectlTool) Def() tool.ToolDef {
	return tool.ToolDef{
		Name: "kubectl",
		Description: "Run a read-only kubectl query against the allchat namespace. " +
			"action is one of: get, describe, logs, events, top. Use it to inspect pod status, restarts, recent warning events, and logs. Writes are not possible.",
		Parameters: json.RawMessage(`{"type":"object","properties":{` +
			`"action":{"type":"string","enum":["get","describe","logs","events","top"]},` +
			`"resource":{"type":"string","description":"resource type for get/describe, e.g. pods, deployment"},` +
			`"name":{"type":"string","description":"resource or pod name"},` +
			`"namespace":{"type":"string","description":"optional namespace override (defaults to allchat)"},` +
			`"selector":{"type":"string","description":"optional label selector for get, e.g. app=api-gateway"},` +
			`"container":{"type":"string","description":"optional container name for logs"},` +
			`"since":{"type":"string","description":"optional log look-back, e.g. 15m"},` +
			`"tail_lines":{"type":"integer","description":"optional max log lines"}},` +
			`"required":["action"]}`),
	}
}

func (t KubectlTool) Invoke(ctx context.Context, tctx *tool.ToolCtx, args json.RawMessage) (tool.ToolOutput, error) {
	var p kubectlParams
	if err := json.Unmarshal(args, &p); err != nil {
		return tool.ToolOutput{}, fmt.Errorf("invalid arguments: %w", err)
	}

	ns := tctx.Namespace
	if p.Namespace != "" {
		if err := validateKubeName("namespace", p.Namespace); err != nil {
			return tool.ToolOutput{}, err
		}
		ns = p.Namespace
	}
	if ns == "" {
		ns = "allchat"
	}

	argv, err := t.buildArgs(p, ns)
	if err != nil {
		return tool.ToolOutput{}, err
	}

	cmd := exec.CommandContext(ctx, "kubectl", argv...)
	cmd.Env = strippedProxyEnv()
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return tool.ToolOutput{}, fmt.Errorf("kubectl %s failed: %s", p.Action, summarizeStderr(stderr.String()))
	}

	out := stdout.String()
	// In support mode the detail-heavy actions (logs, describe, events) are aggregated
	// into counts + normalized patterns rather than dumped raw — a `describe` includes
	// literal env values and a `Last State` message can carry a stack trace, and an
	// `events` MESSAGE column can carry internal detail; `get`/`top` are tabular status
	// and fall through to the registry's blanket redaction instead.
	if tctx.Mode == access.ModeSupport && tctx.Redactor != nil &&
		(p.Action == "logs" || p.Action == "describe" || p.Action == "events") {
		return tool.ToolOutput{Content: tctx.Redactor.SummarizeLogs(out, 8)}, nil
	}
	if len(out) > maxKubeOutput {
		out = out[:maxKubeOutput] + "\n... (output truncated)"
	}
	if strings.TrimSpace(out) == "" {
		out = "(no output)"
	}
	return tool.ToolOutput{Content: out}, nil
}

// buildArgs assembles a validated argument slice. Auth/scope flags (-n) go before any
// positional; there is never a "--" separator (no exec path).
func (t KubectlTool) buildArgs(p kubectlParams, ns string) ([]string, error) {
	if err := validateSelector(p.Selector); err != nil {
		return nil, err
	}
	if err := validateSince(p.Since); err != nil {
		return nil, err
	}
	if p.Resource != "" {
		if err := validateKubeName("resource", p.Resource); err != nil {
			return nil, err
		}
		if isBlockedResource(p.Resource) {
			return nil, fmt.Errorf("resource %q is not permitted", p.Resource)
		}
	}
	if p.Name != "" {
		if err := validateKubeName("name", p.Name); err != nil {
			return nil, err
		}
	}
	if p.Container != "" {
		if err := validateKubeName("container", p.Container); err != nil {
			return nil, err
		}
	}

	switch p.Action {
	case "get":
		if p.Resource == "" {
			return nil, fmt.Errorf("get requires a resource")
		}
		argv := []string{"get", "-n", ns, p.Resource}
		if p.Name != "" {
			argv = append(argv, p.Name)
		}
		if p.Selector != "" {
			argv = append(argv, "-l", p.Selector)
		}
		return argv, nil
	case "describe":
		if p.Resource == "" || p.Name == "" {
			return nil, fmt.Errorf("describe requires a resource and name")
		}
		return []string{"describe", "-n", ns, p.Resource, p.Name}, nil
	case "logs":
		if p.Name == "" {
			return nil, fmt.Errorf("logs requires a pod name")
		}
		argv := []string{"logs", "-n", ns, p.Name}
		if p.Container != "" {
			argv = append(argv, "-c", p.Container)
		}
		if p.Since != "" {
			argv = append(argv, "--since", p.Since)
		}
		tail := p.TailLines
		if tail <= 0 || tail > 2000 {
			tail = 500
		}
		argv = append(argv, "--tail", strconv.Itoa(tail))
		return argv, nil
	case "events":
		return []string{"get", "events", "-n", ns, "--sort-by=.lastTimestamp"}, nil
	case "top":
		return []string{"top", "pods", "-n", ns}, nil
	default:
		return nil, fmt.Errorf("action %q not permitted (read-only: get, describe, logs, events, top)", p.Action)
	}
}

// isBlockedResource rejects secret resources regardless of the read-only RBAC backstop.
func isBlockedResource(resource string) bool {
	lower := strings.ToLower(resource)
	// strip API group suffix (secrets.v1.core -> secrets)
	if i := strings.IndexByte(lower, '.'); i > 0 {
		lower = lower[:i]
	}
	return strings.HasPrefix(lower, "secret")
}

// strippedProxyEnv returns the process environment minus proxy variables so kubectl
// talks to the API server directly and never dumps proxy creds.
func strippedProxyEnv() []string {
	drop := map[string]bool{
		"HTTP_PROXY": true, "HTTPS_PROXY": true, "http_proxy": true, "https_proxy": true,
		"ALL_PROXY": true, "all_proxy": true,
	}
	src := os.Environ()
	out := make([]string, 0, len(src))
	for _, kv := range src {
		key := kv
		if i := strings.IndexByte(kv, '='); i >= 0 {
			key = kv[:i]
		}
		if drop[key] {
			continue
		}
		out = append(out, kv)
	}
	return out
}

// summarizeStderr reduces kubectl stderr to a short, safe message: first non-empty
// line, capped. (The registry additionally redacts it.)
func summarizeStderr(stderr string) string {
	for _, ln := range strings.Split(stderr, "\n") {
		ln = strings.TrimSpace(ln)
		if ln != "" {
			if len(ln) > 400 {
				ln = ln[:400]
			}
			return ln
		}
	}
	return "unknown error"
}
