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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/caesar/all-chat/services/support-bot/access"
	"github.com/caesar/all-chat/services/support-bot/tool"
)

func screenshotDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, f := range []string{"dashboard-populated-1280-chromium-linux.png", "settings-375-chromium-linux.png", "notes.txt"} {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("png"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestImageTool_DefListsScreens(t *testing.T) {
	def := NewImageTool(screenshotDir(t)).Def()
	if def.Name != "send_image" {
		t.Errorf("name = %q", def.Name)
	}
	var schema struct {
		Properties struct {
			Screen struct {
				Enum []string `json:"enum"`
			} `json:"screen"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(def.Parameters, &schema); err != nil {
		t.Fatalf("schema: %v", err)
	}
	if got := strings.Join(schema.Properties.Screen.Enum, ","); got != "dashboard-populated-1280,settings-375" {
		t.Errorf("screens = %q", got)
	}
}

func TestImageTool_AttachesTheScreen(t *testing.T) {
	dir := screenshotDir(t)
	out, err := NewImageTool(dir).Invoke(context.Background(), &tool.ToolCtx{Mode: access.ModeSupport},
		json.RawMessage(`{"screen":"settings-375"}`))
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if out.Attachment == nil || out.Attachment.Path != filepath.Join(dir, "settings-375-chromium-linux.png") || out.Attachment.Name != "settings-375.png" {
		t.Errorf("attachment = %+v", out.Attachment)
	}
	if out.Effect != nil {
		t.Error("an image is not a side effect: it must not ping the lead developer")
	}
}

func TestImageTool_RefusesUnknownOrEscapingNames(t *testing.T) {
	it := NewImageTool(screenshotDir(t))
	for _, screen := range []string{"nope", "../settings-375", "notes", ""} {
		args, _ := json.Marshal(map[string]string{"screen": screen})
		if _, err := it.Invoke(context.Background(), &tool.ToolCtx{}, args); err == nil {
			t.Errorf("screen %q was accepted", screen)
		}
	}
}
