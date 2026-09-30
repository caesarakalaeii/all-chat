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
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/caesar/all-chat/services/support-bot/tool"
)

const (
	defaultReadBytes = 64 * 1024
	maxReadBytes     = 256 * 1024
	maxGlobMatches   = 200
	maxGrepResults   = 100
	maxGrepFileBytes = 1 << 20
	maxGrepFiles     = 20000
	maxLineLen       = 300
)

var skipDirs = map[string]bool{
	".git": true, "node_modules": true, "dist": true, "build": true,
	"vendor": true, ".next": true, "out": true, ".cache": true,
}

// ReadFileTool reads a file from the jailed repository checkouts.
type ReadFileTool struct{ tool.BothModes }

type readFileParams struct {
	Path     string `json:"path"`
	MaxBytes int    `json:"max_bytes,omitempty"`
}

func (ReadFileTool) Def() tool.ToolDef {
	return tool.ToolDef{
		Name:        "read_file",
		Description: "Read a file from the project source checkout. `path` is relative to a repository root (e.g. services/api-gateway/cmd/main.go).",
		Parameters: json.RawMessage(`{"type":"object","properties":{` +
			`"path":{"type":"string","description":"repo-relative file path"},` +
			`"max_bytes":{"type":"integer","description":"optional max bytes to read"}},` +
			`"required":["path"]}`),
	}
}

func (ReadFileTool) Invoke(ctx context.Context, tctx *tool.ToolCtx, args json.RawMessage) (tool.ToolOutput, error) {
	var p readFileParams
	if err := json.Unmarshal(args, &p); err != nil {
		return tool.ToolOutput{}, fmt.Errorf("invalid arguments: %w", err)
	}
	if strings.TrimSpace(p.Path) == "" {
		return tool.ToolOutput{}, fmt.Errorf("path is required")
	}
	abs, err := resolveInRoots(tctx.RepoPaths, p.Path)
	if err != nil {
		return tool.ToolOutput{}, err
	}
	limit := p.MaxBytes
	if limit <= 0 || limit > maxReadBytes {
		limit = defaultReadBytes
	}
	f, err := os.Open(abs)
	if err != nil {
		return tool.ToolOutput{}, fmt.Errorf("open failed")
	}
	defer f.Close()
	buf := make([]byte, limit)
	n, _ := f.Read(buf)
	rel := repoRel(tctx.RepoPaths, abs)
	return tool.ToolOutput{Content: fmt.Sprintf("// %s\n%s", rel, string(buf[:n]))}, nil
}

// GlobTool lists files matching a glob within the repositories.
type GlobTool struct{ tool.BothModes }

type globParams struct {
	Pattern string `json:"pattern"`
}

func (GlobTool) Def() tool.ToolDef {
	return tool.ToolDef{
		Name:        "glob",
		Description: "List repository files matching a glob pattern (e.g. services/*/cmd/main.go). Returns repo-relative paths.",
		Parameters: json.RawMessage(`{"type":"object","properties":{` +
			`"pattern":{"type":"string","description":"glob pattern relative to a repo root"}},` +
			`"required":["pattern"]}`),
	}
}

func (GlobTool) Invoke(ctx context.Context, tctx *tool.ToolCtx, args json.RawMessage) (tool.ToolOutput, error) {
	var p globParams
	if err := json.Unmarshal(args, &p); err != nil {
		return tool.ToolOutput{}, fmt.Errorf("invalid arguments: %w", err)
	}
	if strings.TrimSpace(p.Pattern) == "" {
		return tool.ToolOutput{}, fmt.Errorf("pattern is required")
	}
	if strings.Contains(p.Pattern, "..") {
		return tool.ToolOutput{}, fmt.Errorf("pattern must not contain '..'")
	}
	var matches []string
	for _, root := range tctx.RepoPaths {
		abs, err := filepath.Abs(root)
		if err != nil {
			continue
		}
		found, _ := filepath.Glob(filepath.Join(abs, p.Pattern))
		for _, m := range found {
			if rootFor(tctx.RepoPaths, m) == "" {
				continue
			}
			matches = append(matches, repoRel(tctx.RepoPaths, m))
			if len(matches) >= maxGlobMatches {
				break
			}
		}
		if len(matches) >= maxGlobMatches {
			break
		}
	}
	if len(matches) == 0 {
		return tool.ToolOutput{Content: "(no matches)"}, nil
	}
	return tool.ToolOutput{Content: strings.Join(matches, "\n")}, nil
}

// GrepTool searches repository file contents with a regular expression.
type GrepTool struct{ tool.BothModes }

type grepParams struct {
	Pattern    string `json:"pattern"`
	Path       string `json:"path,omitempty"`
	Glob       string `json:"glob,omitempty"`
	MaxResults int    `json:"max_results,omitempty"`
}

func (GrepTool) Def() tool.ToolDef {
	return tool.ToolDef{
		Name:        "grep",
		Description: "Search repository file contents with a Go regular expression. Optional `path` (subdirectory) and `glob` (filename filter, e.g. *.go). Returns matching lines as path:line: text.",
		Parameters: json.RawMessage(`{"type":"object","properties":{` +
			`"pattern":{"type":"string","description":"RE2 regular expression"},` +
			`"path":{"type":"string","description":"optional subdirectory to restrict the search"},` +
			`"glob":{"type":"string","description":"optional filename glob filter, e.g. *.go"},` +
			`"max_results":{"type":"integer","description":"optional cap on returned matches"}},` +
			`"required":["pattern"]}`),
	}
}

func (GrepTool) Invoke(ctx context.Context, tctx *tool.ToolCtx, args json.RawMessage) (tool.ToolOutput, error) {
	var p grepParams
	if err := json.Unmarshal(args, &p); err != nil {
		return tool.ToolOutput{}, fmt.Errorf("invalid arguments: %w", err)
	}
	if strings.TrimSpace(p.Pattern) == "" {
		return tool.ToolOutput{}, fmt.Errorf("pattern is required")
	}
	re, err := regexp.Compile(p.Pattern)
	if err != nil {
		return tool.ToolOutput{}, fmt.Errorf("invalid regular expression")
	}
	if strings.Contains(p.Path, "..") {
		return tool.ToolOutput{}, fmt.Errorf("path must not contain '..'")
	}
	limit := p.MaxResults
	if limit <= 0 || limit > maxGrepResults {
		limit = maxGrepResults
	}

	var results []string
	filesScanned := 0
	for _, root := range tctx.RepoPaths {
		absRoot, err := filepath.Abs(root)
		if err != nil {
			continue
		}
		searchRoot := absRoot
		if p.Path != "" {
			searchRoot = filepath.Clean(filepath.Join(absRoot, p.Path))
			if searchRoot != absRoot && !strings.HasPrefix(searchRoot, absRoot+string(os.PathSeparator)) {
				continue
			}
		}
		walkErr := filepath.WalkDir(searchRoot, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if d.IsDir() {
				if skipDirs[d.Name()] {
					return filepath.SkipDir
				}
				return nil
			}
			// Never follow symlinks: a symlinked file inside the checkout could point at
			// a mounted secret or host file outside the jail, and os.ReadFile follows it.
			if d.Type()&fs.ModeSymlink != 0 {
				return nil
			}
			if len(results) >= limit || filesScanned >= maxGrepFiles {
				return filepath.SkipAll
			}
			if p.Glob != "" {
				if ok, _ := filepath.Match(p.Glob, d.Name()); !ok {
					return nil
				}
			}
			info, err := d.Info()
			if err != nil || info.Size() > maxGrepFileBytes {
				return nil
			}
			filesScanned++
			appendMatches(re, path, tctx.RepoPaths, &results, limit)
			return nil
		})
		if walkErr != nil {
			break
		}
		if len(results) >= limit {
			break
		}
	}
	if len(results) == 0 {
		return tool.ToolOutput{Content: "(no matches)"}, nil
	}
	return tool.ToolOutput{Content: strings.Join(results, "\n")}, nil
}

func appendMatches(re *regexp.Regexp, path string, roots []string, results *[]string, limit int) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	rel := repoRel(roots, path)
	for i, line := range strings.Split(string(data), "\n") {
		if len(*results) >= limit {
			return
		}
		if re.MatchString(line) {
			trimmed := strings.TrimSpace(line)
			if len(trimmed) > maxLineLen {
				trimmed = trimmed[:maxLineLen] + "..."
			}
			*results = append(*results, fmt.Sprintf("%s:%d: %s", rel, i+1, trimmed))
		}
	}
}

// repoRel renders an absolute path relative to its jail root, prefixed with the root's
// base name so the model can tell repos apart.
func repoRel(roots []string, abs string) string {
	root := rootFor(roots, abs)
	if root == "" {
		return filepath.Base(abs)
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return filepath.Base(abs)
	}
	return filepath.Join(filepath.Base(root), rel)
}
