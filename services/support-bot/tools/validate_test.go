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
	"os"
	"path/filepath"
	"testing"
)

func TestResolveInRootsJail(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sub", "a.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := resolveInRoots([]string{root}, "sub/a.txt"); err != nil {
		t.Fatalf("valid path should resolve: %v", err)
	}
	// Escape attempts must be rejected.
	for _, bad := range []string{"../../../etc/passwd", "sub/../../outside", "/etc/passwd"} {
		if _, err := resolveInRoots([]string{root}, bad); err == nil {
			t.Errorf("path %q should have been rejected", bad)
		}
	}
}

func TestResolveInRootsRejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	secret := filepath.Join(outside, "token")
	if err := os.WriteFile(secret, []byte("SUPER_SECRET"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "creds")
	if err := os.Symlink(secret, link); err != nil {
		t.Skipf("symlinks unsupported on this platform: %v", err)
	}
	// A symlink inside the jail pointing outside it must be rejected (real path escapes).
	if got, err := resolveInRoots([]string{root}, "creds"); err == nil {
		t.Fatalf("symlink escape should be rejected, resolved to %q", got)
	}
}

func TestValidateKubeName(t *testing.T) {
	ok := []string{"pods", "api-gateway", "deployment.apps", "pod_name"}
	for _, s := range ok {
		if err := validateKubeName("f", s); err != nil {
			t.Errorf("%q should be valid: %v", s, err)
		}
	}
	bad := []string{"", "-x", "a b", "a;b", "a$b"}
	for _, s := range bad {
		if err := validateKubeName("f", s); err == nil {
			t.Errorf("%q should be invalid", s)
		}
	}
}

func TestIsBlockedResource(t *testing.T) {
	blocked := []string{"secret", "secrets", "secrets.v1.core", "SECRETS"}
	for _, r := range blocked {
		if !isBlockedResource(r) {
			t.Errorf("%q should be blocked", r)
		}
	}
	allowed := []string{"pods", "deployments", "events", "configmaps"}
	for _, r := range allowed {
		if isBlockedResource(r) {
			t.Errorf("%q should be allowed", r)
		}
	}
}

func TestKubectlBuildArgsRejectsWrites(t *testing.T) {
	var kt KubectlTool
	// Only read actions are recognized; anything else is rejected.
	for _, action := range []string{"delete", "apply", "edit", "scale", "exec", "patch", "create"} {
		if _, err := kt.buildArgs(kubectlParams{Action: action, Resource: "pods"}, "allchat"); err == nil {
			t.Errorf("write action %q must be rejected", action)
		}
	}
}

func TestKubectlBuildArgsReadOnly(t *testing.T) {
	var kt KubectlTool
	argv, err := kt.buildArgs(kubectlParams{Action: "get", Resource: "pods", Selector: "app=api-gateway"}, "allchat")
	if err != nil {
		t.Fatalf("get should build: %v", err)
	}
	// -n allchat must come before the resource positional (auth/scope first).
	if argv[0] != "get" || argv[1] != "-n" || argv[2] != "allchat" {
		t.Fatalf("namespace not pinned first: %v", argv)
	}
	// Secret resource is blocked.
	if _, err := kt.buildArgs(kubectlParams{Action: "get", Resource: "secrets"}, "allchat"); err == nil {
		t.Fatal("get secrets must be blocked")
	}
}
