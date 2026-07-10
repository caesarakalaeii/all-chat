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

package tool

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/caesar/all-chat/services/support-bot/access"
	"github.com/caesar/all-chat/services/support-bot/redact"
)

type fakeTool struct {
	name    string
	admin   bool
	invoked *bool
	out     string
	effect  *ToolEffect
	err     error
}

func (f *fakeTool) Def() ToolDef {
	return ToolDef{Name: f.name, Description: "fake", Parameters: json.RawMessage(`{"type":"object"}`)}
}
func (f *fakeTool) AllowedFor(m access.Mode) bool {
	if f.admin {
		return m == access.ModeAdmin
	}
	return true
}
func (f *fakeTool) Invoke(_ context.Context, _ *ToolCtx, _ json.RawMessage) (ToolOutput, error) {
	if f.invoked != nil {
		*f.invoked = true
	}
	if f.err != nil {
		return ToolOutput{}, f.err
	}
	return ToolOutput{Content: f.out, Effect: f.effect}, nil
}

func ctxFor(mode access.Mode) *ToolCtx {
	return &ToolCtx{Mode: mode, Redactor: redact.NewRedactor()}
}

func TestDispatchUnknownToolFailsClosed(t *testing.T) {
	r := NewRegistry()
	res := r.Dispatch(context.Background(), ctxFor(access.ModeAdmin), "nope", nil)
	if !res.IsError || !strings.Contains(res.Content, "unknown tool") {
		t.Fatalf("expected unknown-tool error, got %+v", res)
	}
}

func TestDispatchAdminToolDeniedInSupport(t *testing.T) {
	invoked := false
	r := NewRegistry()
	r.Register(&fakeTool{name: "github_write", admin: true, invoked: &invoked, out: "did it"})
	res := r.Dispatch(context.Background(), ctxFor(access.ModeSupport), "github_write", nil)
	if !res.IsError || !strings.Contains(res.Content, "not permitted") {
		t.Fatalf("expected denial, got %+v", res)
	}
	if invoked {
		t.Fatal("denied tool must NOT be invoked")
	}
}

func TestDispatchAdminToolAllowedInAdmin(t *testing.T) {
	invoked := false
	r := NewRegistry()
	r.Register(&fakeTool{name: "github_write", admin: true, invoked: &invoked, out: "opened PR",
		effect: &ToolEffect{Tool: "github", Kind: "pr", URL: "https://x/pr/1"}})
	res := r.Dispatch(context.Background(), ctxFor(access.ModeAdmin), "github_write", nil)
	if res.IsError {
		t.Fatalf("unexpected error: %+v", res)
	}
	if !invoked {
		t.Fatal("allowed tool should be invoked")
	}
	if res.Effect == nil || res.Effect.Kind != "pr" {
		t.Fatalf("effect not propagated: %+v", res.Effect)
	}
	if !strings.Contains(res.Content, "<tool_output") {
		t.Fatalf("output not boundary-wrapped: %q", res.Content)
	}
}

func TestDispatchSupportRedactsOutput(t *testing.T) {
	r := NewRegistry()
	r.Register(&fakeTool{name: "kubectl", out: "pod ip is 10.1.2.3 token=ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"})
	res := r.Dispatch(context.Background(), ctxFor(access.ModeSupport), "kubectl", nil)
	if strings.Contains(res.Content, "10.1.2.3") {
		t.Fatalf("internal IP leaked in support mode: %q", res.Content)
	}
	if strings.Contains(res.Content, "ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789") {
		t.Fatalf("token leaked in support mode: %q", res.Content)
	}
}

func TestDispatchAdminDoesNotRedact(t *testing.T) {
	r := NewRegistry()
	r.Register(&fakeTool{name: "kubectl", out: "pod ip is 10.1.2.3"})
	res := r.Dispatch(context.Background(), ctxFor(access.ModeAdmin), "kubectl", nil)
	if !strings.Contains(res.Content, "10.1.2.3") {
		t.Fatalf("admin output should not be redacted: %q", res.Content)
	}
}

func TestDispatchErrorIsRedactedAndWrapped(t *testing.T) {
	r := NewRegistry()
	r.Register(&fakeTool{name: "kubectl", err: errors.New("failed talking to 10.9.9.9")})
	res := r.Dispatch(context.Background(), ctxFor(access.ModeSupport), "kubectl", nil)
	if !res.IsError {
		t.Fatal("expected error result")
	}
	if strings.Contains(res.Content, "10.9.9.9") {
		t.Fatalf("error not redacted: %q", res.Content)
	}
	if !strings.Contains(res.Content, "<tool_error") {
		t.Fatalf("error not boundary-wrapped: %q", res.Content)
	}
}

func TestDefsFilterByMode(t *testing.T) {
	r := NewRegistry()
	r.Register(&fakeTool{name: "read_file"})
	r.Register(&fakeTool{name: "github_write", admin: true})

	support := r.Defs(access.ModeSupport)
	if len(support) != 1 || support[0].Name != "read_file" {
		t.Fatalf("support should see only read_file, got %+v", support)
	}
	admin := r.Defs(access.ModeAdmin)
	if len(admin) != 2 {
		t.Fatalf("admin should see both tools, got %d", len(admin))
	}
}
