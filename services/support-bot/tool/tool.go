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

// Package tool defines the Tool interface, the per-request ToolCtx, and the registry
// that is the single choke point for tool execution. It is deliberately FAIL-CLOSED:
// every tool must state which access modes it is allowed in (via an embedded mixin),
// the registry only advertises mode-allowed tools to the model, and Dispatch re-checks
// the mode so a hallucinated or injected tool name can never execute.
package tool

import (
	"context"
	"encoding/json"

	"github.com/caesar/all-chat/services/support-bot/access"
	"github.com/caesar/all-chat/services/support-bot/redact"
	"go.uber.org/zap"
)

// ToolDef is the model-facing description of a tool (advertised as an OpenAI function).
type ToolDef struct {
	Name        string
	Description string
	Parameters  json.RawMessage // JSON Schema for the arguments object
}

// ToolEffect is a durable, side-effecting action a tool performed. It is surfaced to
// the caller so Discord can report it deterministically even if the model omits it.
type ToolEffect struct {
	Tool    string // tool that produced it, e.g. "github"
	Kind    string // "issue" | "pr" | "comment" | "review"
	URL     string // html_url for embedding
	Ref     string // e.g. "all-chat#523"
	Summary string // one-line human description
}

// ToolOutput is what a Tool.Invoke returns on success.
type ToolOutput struct {
	Content string      // raw tool output (pre-redaction, pre-wrapping)
	Effect  *ToolEffect // non-nil only for side-effecting tools
}

// ToolCtx carries the trusted, immutable-per-request context into tool execution.
// Every field is set by handler code before dispatch; nothing here is reachable from
// tool arguments or model output. In particular Mode is the access decision and is
// never influenced by message content.
type ToolCtx struct {
	Mode       access.Mode
	DiscordUID string // audit only
	ChannelID  string // audit only
	RepoPaths  []string
	Namespace  string
	Redactor   *redact.Redactor
	Log        *zap.Logger
}

// Tool is the interface every tool implements; AllowedFor is the fail-closed per-mode gate.
type Tool interface {
	Def() ToolDef
	AllowedFor(mode access.Mode) bool
	Invoke(ctx context.Context, tctx *ToolCtx, args json.RawMessage) (ToolOutput, error)
}

// BothModes marks a read-only tool that is safe in support and admin. Embed it.
type BothModes struct{}

// AllowedFor allows both modes.
func (BothModes) AllowedFor(access.Mode) bool { return true }

// AdminOnly marks a tool that only allow-listed maintainers may use. Embed it.
type AdminOnly struct{}

// AllowedFor allows only admin.
func (AdminOnly) AllowedFor(m access.Mode) bool { return m == access.ModeAdmin }

// Disabled marks a tool that is registered but never callable. Embed it.
type Disabled struct{}

// AllowedFor denies every mode.
func (Disabled) AllowedFor(access.Mode) bool { return false }
