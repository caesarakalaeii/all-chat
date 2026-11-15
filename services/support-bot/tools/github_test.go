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
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/caesar/all-chat/services/support-bot/access"
	"github.com/caesar/all-chat/services/support-bot/ghclient"
	"github.com/caesar/all-chat/services/support-bot/tool"
	"go.uber.org/zap"
)

// fakeGitHub records issue creations and serves a canned search result.
type fakeGitHub struct {
	created     []map[string]any
	searchQuery string
}

func newGitHubTool(t *testing.T) (*GitHubTool, *fakeGitHub) {
	t.Helper()
	f := &fakeGitHub{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/repos/caesarakalaeii/all-chat/issues":
			raw, _ := io.ReadAll(r.Body)
			var body map[string]any
			_ = json.Unmarshal(raw, &body)
			f.created = append(f.created, body)
			_, _ = io.WriteString(w, `{"number":42,"html_url":"https://github.com/caesarakalaeii/all-chat/issues/42"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/search/issues":
			f.searchQuery = r.URL.Query().Get("q")
			_, _ = io.WriteString(w, `{"items":[{"number":7,"title":"kick-listener: reconnect storm","state":"open",
				"html_url":"https://github.com/caesarakalaeii/all-chat/issues/7","labels":[{"name":"agent"}]}]}`)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	gh := ghclient.NewWithBase("token", srv.URL, zap.NewNop())
	return NewGitHubTool(gh, "caesarakalaeii", "", nil), f
}

func invokeGitHub(t *testing.T, gt *GitHubTool, args string) (tool.ToolOutput, error) {
	t.Helper()
	return gt.Invoke(context.Background(), &tool.ToolCtx{Mode: access.ModeSupport}, json.RawMessage(args))
}

func labelsOf(body map[string]any) []string {
	var out []string
	for _, l := range body["labels"].([]any) {
		out = append(out, l.(string))
	}
	return out
}

func hasLabel(labels []string, want string) bool {
	for _, l := range labels {
		if l == want {
			return true
		}
	}
	return false
}

const agentTaskBody = "## Context\nkick-listener reconnects every 2s.\n\n" +
	"```agent\nrepos:\n  - caesarakalaeii/all-chat\nacceptance:\n  - \"cd services/kick-listener && go test -short ./...\"\n```\n"

func TestCreateIssue_AgentTaskAddsAgentLabel(t *testing.T) {
	gt, f := newGitHubTool(t)
	args, _ := json.Marshal(map[string]any{
		"action": "create_issue", "repo": "all-chat", "title": "kick-listener: stop the reconnect storm",
		"body": agentTaskBody, "agent_task": true,
	})
	out, err := invokeGitHub(t, gt, string(args))
	if err != nil {
		t.Fatalf("create_issue: %v", err)
	}
	if len(f.created) != 1 {
		t.Fatalf("want 1 issue created, got %d", len(f.created))
	}
	labels := labelsOf(f.created[0])
	if !hasLabel(labels, "agent") || !hasLabel(labels, "bot-proposed") {
		t.Errorf("labels = %v, want agent + bot-proposed", labels)
	}
	if !strings.Contains(out.Content, "agent") {
		t.Errorf("tool output should say the issue was queued for Caterpillar: %q", out.Content)
	}
}

func TestCreateIssue_PlainIssueHasNoAgentLabel(t *testing.T) {
	gt, f := newGitHubTool(t)
	_, err := invokeGitHub(t, gt, `{"action":"create_issue","repo":"all-chat","title":"question","body":"how?"}`)
	if err != nil {
		t.Fatalf("create_issue: %v", err)
	}
	if labels := labelsOf(f.created[0]); hasLabel(labels, "agent") {
		t.Errorf("a plain issue must not be claimable by Caterpillar, labels = %v", labels)
	}
}

func TestCreateIssue_AgentTaskWithoutAgentBlockIsRefused(t *testing.T) {
	gt, f := newGitHubTool(t)
	for _, body := range []string{
		"## Context\nno machine-checkable block here",
		"```agent\nrepos:\n  - caesarakalaeii/all-chat\n```", // no acceptance commands
	} {
		args, _ := json.Marshal(map[string]any{
			"action": "create_issue", "repo": "all-chat", "title": "x", "body": body, "agent_task": true,
		})
		if _, err := invokeGitHub(t, gt, string(args)); err == nil {
			t.Errorf("agent task without a complete agent block was accepted: %q", body)
		}
	}
	if len(f.created) != 0 {
		t.Errorf("refused agent tasks must not reach GitHub, %d were created", len(f.created))
	}
}

func TestSearchIssues_ScopesQueryToRepo(t *testing.T) {
	gt, f := newGitHubTool(t)
	out, err := invokeGitHub(t, gt, `{"action":"search_issues","repo":"all-chat","query":"kick reconnect"}`)
	if err != nil {
		t.Fatalf("search_issues: %v", err)
	}
	if !strings.Contains(f.searchQuery, "repo:caesarakalaeii/all-chat") || !strings.Contains(f.searchQuery, "kick reconnect") {
		t.Errorf("search query = %q", f.searchQuery)
	}
	if !strings.Contains(out.Content, "#7") || !strings.Contains(out.Content, "reconnect storm") {
		t.Errorf("search output = %q", out.Content)
	}
}

func TestSearchIssues_RequiresQuery(t *testing.T) {
	gt, _ := newGitHubTool(t)
	if _, err := invokeGitHub(t, gt, `{"action":"search_issues","repo":"all-chat"}`); err == nil {
		t.Error("search_issues without a query was accepted")
	}
}
