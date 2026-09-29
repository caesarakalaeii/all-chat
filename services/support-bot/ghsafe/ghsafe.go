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

// Package ghsafe holds the GitHub write-safety guards used by the admin-only GitHub
// tools: protected-branch and repo deny-lists, and parameter validation. Code changes
// always land on a fresh feature branch and open a pull request; a direct push to a
// protected branch is structurally rejected. This is how admin "push to the cluster"
// works — via a PR against the manifests repo that the GitOps pipeline then deploys —
// so no kubectl write access is ever needed.
package ghsafe

import (
	"fmt"
	"strings"
)

var protectedBranches = map[string]bool{
	"main": true, "master": true, "prod": true, "production": true, "live": true,
	"stage": true, "staging": true, "dev": true, "develop": true, "development": true,
}

var protectedPrefixes = []string{"release/", "release-", "hotfix/", "hotfix-"}

// IsProtectedBranch reports whether pushing directly to b is forbidden.
func IsProtectedBranch(b string) bool {
	lb := strings.ToLower(strings.TrimSpace(b))
	if protectedBranches[lb] {
		return true
	}
	for _, p := range protectedPrefixes {
		if strings.HasPrefix(lb, p) {
			return true
		}
	}
	return false
}

// IsBlockedRepo reports whether owner/repo is on the deny-list (case-insensitive).
func IsBlockedRepo(owner, repo string, blocked []string) bool {
	full := strings.ToLower(owner + "/" + repo)
	for _, b := range blocked {
		if strings.ToLower(strings.TrimSpace(b)) == full {
			return true
		}
	}
	return false
}

// forbiddenParamChars are rejected in any path/ref/owner/repo parameter to prevent
// path traversal, query/fragment injection, protocol-relative URLs, and CRLF.
const forbiddenParamChars = "\x00\n\r\t?#%@\\"

// ValidateParam rejects an empty or structurally dangerous GitHub parameter. spacesOK
// controls whether spaces are permitted (titles/bodies yes; refs/owners no).
func ValidateParam(name, val string, spacesOK bool) error {
	if strings.TrimSpace(val) == "" {
		return fmt.Errorf("%s must not be empty", name)
	}
	if strings.Contains(val, "..") {
		return fmt.Errorf("%s must not contain '..'", name)
	}
	if strings.Contains(val, "//") {
		return fmt.Errorf("%s must not contain '//'", name)
	}
	if strings.ContainsAny(val, forbiddenParamChars) {
		return fmt.Errorf("%s contains a forbidden character", name)
	}
	if !spacesOK && strings.Contains(val, " ") {
		return fmt.Errorf("%s must not contain spaces", name)
	}
	return nil
}

// ValidateRepoCoords validates owner and repo names (no slashes, no dangerous chars).
func ValidateRepoCoords(owner, repo string) error {
	if err := ValidateParam("owner", owner, false); err != nil {
		return err
	}
	if strings.Contains(owner, "/") {
		return fmt.Errorf("owner must not contain '/'")
	}
	if err := ValidateParam("repo", repo, false); err != nil {
		return err
	}
	if strings.Contains(repo, "/") {
		return fmt.Errorf("repo must not contain '/'")
	}
	return nil
}
