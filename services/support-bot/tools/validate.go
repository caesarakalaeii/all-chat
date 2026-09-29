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

// Package tools holds the concrete Tool implementations wired into the support-bot's
// registry: read-only filesystem/kubectl/grafana access (both modes, redacted for
// support), GitHub read + issue/comment (both modes), GitHub write (admin only), and
// the bot memory store. Every external command is executed with an argument slice
// (never a shell string) and every user-supplied value is validated before use.
package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	kubeNameRe   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]*$`)
	selectorRe   = regexp.MustCompile(`^[A-Za-z0-9/_.=!,*-]+$`)
	sinceRe      = regexp.MustCompile(`^(\d+[smhd])+$`)
	branchNameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]*$`)
)

// resolveInRoots resolves a repo-relative path within one of the jailed roots. It
// rejects any path that escapes a root (via .. or absolute components) and returns the
// first existing regular file across the roots.
func resolveInRoots(roots []string, rel string) (string, error) {
	if strings.ContainsRune(rel, 0) {
		return "", fmt.Errorf("path contains a null byte")
	}
	rel = strings.TrimPrefix(rel, "/")
	for _, root := range roots {
		abs, err := filepath.Abs(root)
		if err != nil {
			continue
		}
		candidate := filepath.Clean(filepath.Join(abs, rel))
		if candidate != abs && !strings.HasPrefix(candidate, abs+string(os.PathSeparator)) {
			continue // escaped the root lexically
		}
		if fi, err := os.Stat(candidate); err == nil && !fi.IsDir() {
			// Re-verify against the symlink-resolved real path so a symlink inside the
			// checkout cannot point read_file at a file outside the jail.
			if realWithinRoot(abs, candidate) {
				return candidate, nil
			}
		}
	}
	return "", fmt.Errorf("path %q not found within the readable repositories", rel)
}

// realWithinRoot reports whether candidate's symlink-resolved real path stays within
// root's real path. Used to defeat symlink jail escapes on file reads.
func realWithinRoot(root, candidate string) bool {
	real, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return false
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		realRoot = root
	}
	return real == realRoot || strings.HasPrefix(real, realRoot+string(os.PathSeparator))
}

// rootFor returns the jail root that contains an already-resolved absolute path, for
// rendering repo-relative results.
func rootFor(roots []string, abs string) string {
	for _, root := range roots {
		r, err := filepath.Abs(root)
		if err != nil {
			continue
		}
		if abs == r || strings.HasPrefix(abs, r+string(os.PathSeparator)) {
			return r
		}
	}
	return ""
}

func validateKubeName(field, s string) error {
	if s == "" {
		return fmt.Errorf("%s must not be empty", field)
	}
	if len(s) > 253 {
		return fmt.Errorf("%s too long", field)
	}
	if !kubeNameRe.MatchString(s) {
		return fmt.Errorf("%s %q is not a valid Kubernetes name", field, s)
	}
	return nil
}

func validateSelector(s string) error {
	if s == "" {
		return nil
	}
	if !selectorRe.MatchString(s) {
		return fmt.Errorf("selector %q contains invalid characters", s)
	}
	return nil
}

func validateSince(s string) error {
	if s == "" {
		return nil
	}
	if !sinceRe.MatchString(s) {
		return fmt.Errorf("since %q must be a duration like 15m or 1h30m", s)
	}
	return nil
}

// rejectFlag guards a positional value from being interpreted as a flag.
func rejectFlag(field, s string) error {
	if strings.HasPrefix(s, "-") {
		return fmt.Errorf("%s must not start with '-'", field)
	}
	return nil
}

func validateBranch(s string) error {
	if s == "" {
		return fmt.Errorf("branch must not be empty")
	}
	if len(s) > 255 || !branchNameRe.MatchString(s) || strings.Contains(s, "..") {
		return fmt.Errorf("branch %q is not a valid branch name", s)
	}
	return nil
}
