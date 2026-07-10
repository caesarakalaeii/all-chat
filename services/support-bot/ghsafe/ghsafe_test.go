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

package ghsafe

import "testing"

func TestIsProtectedBranch(t *testing.T) {
	protected := []string{"main", "MAIN", "master", "prod", "production", "release/1.2", "hotfix-x", "develop"}
	for _, b := range protected {
		if !IsProtectedBranch(b) {
			t.Errorf("%q should be protected", b)
		}
	}
	allowed := []string{"feature/foo", "bot/fix-123", "chore-x", "fix/support-bot"}
	for _, b := range allowed {
		if IsProtectedBranch(b) {
			t.Errorf("%q should NOT be protected", b)
		}
	}
}

func TestIsBlockedRepo(t *testing.T) {
	blocked := []string{"caesarakalaeii/secret-repo"}
	if !IsBlockedRepo("caesarakalaeii", "secret-repo", blocked) {
		t.Error("expected deny-listed repo to be blocked")
	}
	if !IsBlockedRepo("CaesarAkalaeii", "Secret-Repo", blocked) {
		t.Error("deny-list should be case-insensitive")
	}
	if IsBlockedRepo("caesarakalaeii", "all-chat", blocked) {
		t.Error("non-listed repo should be allowed")
	}
}

func TestValidateParam(t *testing.T) {
	bad := []string{"", "../etc/passwd", "a//b", "a?b", "with space"}
	for _, v := range bad {
		if err := ValidateParam("p", v, false); err == nil {
			t.Errorf("expected %q to be rejected", v)
		}
	}
	if err := ValidateParam("title", "A normal title with spaces", true); err != nil {
		t.Errorf("spaces should be allowed when spacesOK: %v", err)
	}
	if err := ValidateParam("path", "services/support-bot/README.md", false); err != nil {
		t.Errorf("valid path rejected: %v", err)
	}
}

func TestValidateRepoCoords(t *testing.T) {
	if err := ValidateRepoCoords("owner", "repo"); err != nil {
		t.Errorf("valid coords rejected: %v", err)
	}
	if err := ValidateRepoCoords("own/er", "repo"); err == nil {
		t.Error("owner with slash should be rejected")
	}
}
