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
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/caesar/all-chat/services/support-bot/tool"
)

// screenshotSuffix is what Playwright appends to every visual baseline on CI.
const screenshotSuffix = "-chromium-linux.png"

// ScreenshotDir is where the frontend's visual-regression baselines live in the repo.
const ScreenshotDir = "frontend/tests/e2e/__screenshots__/visual-regression.spec.ts-snapshots"

// ImageTool attaches a rendered screen of the All-Chat UI to the answer. The images
// are the visual-regression baselines CI renders with demo data and keeps current,
// so the tool can only pick a name from that directory, never a path.
type ImageTool struct {
	tool.BothModes
	dir string
}

// NewImageTool serves the baselines in dir.
func NewImageTool(dir string) *ImageTool { return &ImageTool{dir: dir} }

// screens lists the available screen names, sorted.
func (t *ImageTool) screens() []string {
	entries, _ := os.ReadDir(t.dir)
	var names []string
	for _, e := range entries {
		if name, ok := strings.CutSuffix(e.Name(), screenshotSuffix); ok && !e.IsDir() {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func (t *ImageTool) Def() tool.ToolDef {
	enum, _ := json.Marshal(t.screens())
	return tool.ToolDef{
		Name: "send_image",
		Description: "Attach a screenshot of an All-Chat page to your answer, to show a user where something is. " +
			"The screens are rendered with demo data (not the user's account). Name = page, then width: 1280 is " +
			"desktop, 375 is a phone; a 'light-' prefix is the light theme. Pick the width and theme the user is on " +
			"(default desktop, dark). Attach at most two per answer and say in your text what to look at.",
		Parameters: json.RawMessage(`{"type":"object","properties":{"screen":{"type":"string","enum":` +
			string(enum) + `}},"required":["screen"]}`),
	}
}

func (t *ImageTool) Invoke(_ context.Context, _ *tool.ToolCtx, args json.RawMessage) (tool.ToolOutput, error) {
	var p struct {
		Screen string `json:"screen"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return tool.ToolOutput{}, fmt.Errorf("invalid arguments: %w", err)
	}
	for _, s := range t.screens() {
		if s == p.Screen {
			return tool.ToolOutput{
				Content:    "attached screenshot " + s + "; it is sent with your answer, so refer to it in your text",
				Attachment: &tool.Attachment{Name: s + ".png", Path: filepath.Join(t.dir, s+screenshotSuffix)},
			}, nil
		}
	}
	return tool.ToolOutput{}, fmt.Errorf("unknown screen %q; pick one of: %s", p.Screen, strings.Join(t.screens(), ", "))
}
