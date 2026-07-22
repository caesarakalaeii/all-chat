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
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/caesar/all-chat/services/support-bot/memory"
	"github.com/caesar/all-chat/services/support-bot/tool"
)

// MemoryTool exposes the bot's own learning store (recall/store/update). It is the
// internal scratch store, not external data, so it is available in both modes.
type MemoryTool struct {
	tool.BothModes
	repo *memory.Repository
}

// NewMemoryTool builds the memory tool.
func NewMemoryTool(repo *memory.Repository) *MemoryTool {
	return &MemoryTool{repo: repo}
}

type memoryParams struct {
	Action  string   `json:"action"`
	Tags    []string `json:"tags,omitempty"`
	Type    string   `json:"type,omitempty"`
	Content string   `json:"content,omitempty"`
	ID      int      `json:"id,omitempty"`
}

func (t *MemoryTool) Def() tool.ToolDef {
	return tool.ToolDef{
		Name: "memory",
		Description: "Recall or record durable notes about this codebase. action is one of: " +
			"recall (fetch notes matching tags), store (save a new note; type is error_pattern|correction|codebase_insight), " +
			"update (revise a note by id). Keep stored notes to one or two sentences.",
		Parameters: json.RawMessage(`{"type":"object","properties":{` +
			`"action":{"type":"string","enum":["recall","store","update"]},` +
			`"tags":{"type":"array","items":{"type":"string"},"description":"service names / error types / concepts"},` +
			`"type":{"type":"string","enum":["error_pattern","correction","codebase_insight"]},` +
			`"content":{"type":"string","description":"the note text (store, update)"},` +
			`"id":{"type":"integer","description":"note id (update)"}},` +
			`"required":["action"]}`),
	}
}

func (t *MemoryTool) Invoke(ctx context.Context, tctx *tool.ToolCtx, args json.RawMessage) (tool.ToolOutput, error) {
	var p memoryParams
	if err := json.Unmarshal(args, &p); err != nil {
		return tool.ToolOutput{}, fmt.Errorf("invalid arguments: %w", err)
	}
	switch p.Action {
	case "recall":
		if len(p.Tags) == 0 {
			return tool.ToolOutput{}, fmt.Errorf("recall requires tags")
		}
		mems, err := t.repo.Retrieve(ctx, p.Tags)
		if err != nil {
			return tool.ToolOutput{}, err
		}
		if len(mems) == 0 {
			return tool.ToolOutput{Content: "(no relevant memories)"}, nil
		}
		var b strings.Builder
		for _, m := range mems {
			fmt.Fprintf(&b, "- [%s] (id:%d) %s\n", m.Type, m.ID, m.Content)
		}
		return tool.ToolOutput{Content: strings.TrimRight(b.String(), "\n")}, nil
	case "store":
		if !memory.ValidType(memory.Type(p.Type)) {
			return tool.ToolOutput{}, fmt.Errorf("type must be error_pattern, correction, or codebase_insight")
		}
		if strings.TrimSpace(p.Content) == "" {
			return tool.ToolOutput{}, fmt.Errorf("store requires content")
		}
		if err := t.repo.Store(ctx, memory.Marker{Type: memory.Type(p.Type), Tags: p.Tags, Content: p.Content}); err != nil {
			return tool.ToolOutput{}, err
		}
		return tool.ToolOutput{Content: "stored"}, nil
	case "update":
		if p.ID <= 0 || strings.TrimSpace(p.Content) == "" {
			return tool.ToolOutput{}, fmt.Errorf("update requires id and content")
		}
		if err := t.repo.Update(ctx, p.ID, p.Content); err != nil {
			return tool.ToolOutput{}, err
		}
		return tool.ToolOutput{Content: "updated"}, nil
	default:
		return tool.ToolOutput{}, fmt.Errorf("action %q not supported", p.Action)
	}
}
