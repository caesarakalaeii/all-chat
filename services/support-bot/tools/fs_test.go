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

func TestGrepSkipsSymlinks(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("SECRET_TOKEN_xyz"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "real.txt"), []byte("hello SECRET world"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "secret.txt"), filepath.Join(root, "link.txt")); err != nil {
		t.Skipf("symlinks unsupported on this platform: %v", err)
	}

	tctx := &tool.ToolCtx{Mode: access.ModeAdmin, RepoPaths: []string{root}}
	out, err := GrepTool{}.Invoke(context.Background(), tctx, json.RawMessage(`{"pattern":"SECRET"}`))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.Content, "SECRET_TOKEN_xyz") {
		t.Fatalf("grep followed a symlink out of the jail: %q", out.Content)
	}
	if !strings.Contains(out.Content, "real.txt") {
		t.Fatalf("grep should still match the real in-jail file: %q", out.Content)
	}
}

func TestReadFileJailed(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("content-here"), 0o644); err != nil {
		t.Fatal(err)
	}
	tctx := &tool.ToolCtx{Mode: access.ModeAdmin, RepoPaths: []string{root}}
	out, err := ReadFileTool{}.Invoke(context.Background(), tctx, json.RawMessage(`{"path":"a.txt"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.Content, "content-here") {
		t.Fatalf("read_file did not return content: %q", out.Content)
	}
	// Escape attempt.
	if _, err := (ReadFileTool{}).Invoke(context.Background(), tctx, json.RawMessage(`{"path":"../../etc/passwd"}`)); err == nil {
		t.Fatal("read_file should reject a path escaping the jail")
	}
}
