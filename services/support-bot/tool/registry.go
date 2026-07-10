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
	"strings"

	"github.com/caesar/all-chat/services/support-bot/access"
	"github.com/caesar/all-chat/services/support-bot/sanitize"
	"go.uber.org/zap"
)

// DispatchResult is the outcome of a single tool invocation, ready to become a
// role:"tool" message. Content is already boundary-tag-wrapped and (in support mode)
// redacted.
type DispatchResult struct {
	Content string
	IsError bool
	Effect  *ToolEffect
}

// Registry holds the tools and is the single execution choke point.
type Registry struct {
	tools map[string]Tool
	order []string
}

// NewRegistry builds an empty registry.
func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

// Register adds a tool. A duplicate name overwrites the previous registration.
func (r *Registry) Register(t Tool) {
	name := t.Def().Name
	if _, exists := r.tools[name]; !exists {
		r.order = append(r.order, name)
	}
	r.tools[name] = t
}

// Defs returns the model-facing definitions for exactly the tools allowed in mode, in
// registration order. Tools not allowed in mode are never advertised to the model.
func (r *Registry) Defs(mode access.Mode) []ToolDef {
	defs := make([]ToolDef, 0, len(r.order))
	for _, name := range r.order {
		t := r.tools[name]
		if t.AllowedFor(mode) {
			defs = append(defs, t.Def())
		}
	}
	return defs
}

// canonical resolves a (possibly mis-cased) tool name to its registered name.
func (r *Registry) canonical(name string) (string, bool) {
	if _, ok := r.tools[name]; ok {
		return name, true
	}
	for _, k := range r.order {
		if strings.EqualFold(k, name) {
			return k, true
		}
	}
	return "", false
}

// Dispatch resolves, gates, and executes a tool call, returning a wrapped result.
// Order is load-bearing:
//  1. unknown name -> fail closed,
//  2. not allowed in this mode -> fail closed (never invoked),
//  3. invoke; any error becomes a redacted, boundary-wrapped error result,
//  4. success -> (support mode) redact -> boundary-wrap.
//
// It never returns a Go error: a tool failure or denial is data for the model, not a
// fatal condition for the loop.
func (r *Registry) Dispatch(ctx context.Context, tctx *ToolCtx, name string, args json.RawMessage) DispatchResult {
	cn, ok := r.canonical(name)
	if !ok {
		return DispatchResult{Content: sanitize.WrapToolError(name, "unknown tool"), IsError: true}
	}
	t := r.tools[cn]
	if !t.AllowedFor(tctx.Mode) {
		if tctx.Log != nil {
			tctx.Log.Warn("tool denied by access mode",
				zap.String("tool", cn),
				zap.String("mode", tctx.Mode.String()),
				zap.String("discord_uid", tctx.DiscordUID))
		}
		return DispatchResult{
			Content: sanitize.WrapToolError(cn, "tool not permitted in this access mode"),
			IsError: true,
		}
	}

	out, err := t.Invoke(ctx, tctx, args)
	if err != nil {
		msg := err.Error()
		if tctx.Redactor != nil {
			msg = tctx.Redactor.Redact(msg)
		}
		return DispatchResult{Content: sanitize.WrapToolError(cn, msg), IsError: true}
	}

	content := out.Content
	if tctx.Mode == access.ModeSupport && tctx.Redactor != nil {
		content = tctx.Redactor.Redact(content)
	}
	return DispatchResult{Content: sanitize.WrapToolOutput(cn, content), Effect: out.Effect}
}
