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

	"github.com/caesar/all-chat/services/support-bot/ghclient"
	"github.com/caesar/all-chat/services/support-bot/ghsafe"
	"github.com/caesar/all-chat/services/support-bot/tool"
)

// defaultAllowedRepos is the fixed set of repositories the bot may touch.
var defaultAllowedRepos = []string{"all-chat", "all-chat-extension"}

type githubBase struct {
	client       *ghclient.Client
	owner        string
	botLogin     string
	allowedRepos []string
	blockedRepos []string
}

func (b *githubBase) checkRepo(repo string) error {
	if err := ghsafe.ValidateRepoCoords(b.owner, repo); err != nil {
		return err
	}
	allowed := false
	for _, r := range b.allowedRepos {
		if strings.EqualFold(r, repo) {
			allowed = true
			break
		}
	}
	if !allowed {
		return fmt.Errorf("repo %q is not in the allow-list", repo)
	}
	if ghsafe.IsBlockedRepo(b.owner, repo, b.blockedRepos) {
		return fmt.Errorf("repo %q is deny-listed", repo)
	}
	return nil
}

// redactBody scrubs secrets/topology from human-authored text before it is sent to
// GitHub. Applied in both modes because a model can paraphrase a secret it saw in an
// earlier tool result into an issue/PR/comment body.
func redactBody(tctx *tool.ToolCtx, s string) string {
	if tctx.Redactor != nil {
		return tctx.Redactor.Redact(s)
	}
	return s
}

// GitHubTool provides read + triage (issue read/create + comment) in BOTH modes.
type GitHubTool struct {
	tool.BothModes
	githubBase
}

// NewGitHubTool builds the both-modes GitHub tool.
func NewGitHubTool(c *ghclient.Client, owner, botLogin string, blockedRepos []string) *GitHubTool {
	return &GitHubTool{githubBase: githubBase{
		client: c, owner: owner, botLogin: botLogin,
		allowedRepos: defaultAllowedRepos, blockedRepos: blockedRepos,
	}}
}

type githubParams struct {
	Action string `json:"action"`
	Repo   string `json:"repo"`
	Number int    `json:"number,omitempty"`
	Title  string `json:"title,omitempty"`
	Body   string `json:"body,omitempty"`
}

func (t *GitHubTool) Def() tool.ToolDef {
	return tool.ToolDef{
		Name: "github",
		Description: "Read and triage GitHub issues/PRs. action is one of: get_issue (read an issue or PR by number), " +
			"comment (post a comment), create_issue (file a new issue). repo is 'all-chat' or 'all-chat-extension'. " +
			"These do not modify code.",
		Parameters: json.RawMessage(`{"type":"object","properties":{` +
			`"action":{"type":"string","enum":["get_issue","comment","create_issue"]},` +
			`"repo":{"type":"string","enum":["all-chat","all-chat-extension"]},` +
			`"number":{"type":"integer","description":"issue or PR number (get_issue, comment)"},` +
			`"title":{"type":"string","description":"issue title (create_issue)"},` +
			`"body":{"type":"string","description":"markdown body (comment, create_issue)"}},` +
			`"required":["action","repo"]}`),
	}
}

func (t *GitHubTool) Invoke(ctx context.Context, tctx *tool.ToolCtx, args json.RawMessage) (tool.ToolOutput, error) {
	var p githubParams
	if err := json.Unmarshal(args, &p); err != nil {
		return tool.ToolOutput{}, fmt.Errorf("invalid arguments: %w", err)
	}
	if err := t.checkRepo(p.Repo); err != nil {
		return tool.ToolOutput{}, err
	}
	switch p.Action {
	case "get_issue":
		if p.Number <= 0 {
			return tool.ToolOutput{}, fmt.Errorf("get_issue requires a positive number")
		}
		iss, err := t.client.GetIssue(ctx, t.owner, p.Repo, p.Number)
		if err != nil {
			return tool.ToolOutput{}, err
		}
		body := iss.Body
		if len(body) > 800 {
			body = body[:800] + "..."
		}
		return tool.ToolOutput{Content: fmt.Sprintf(
			"#%d [%s] %s\nby %s\n%s\n\n%s",
			iss.Number, iss.State, iss.Title, iss.User.Login, iss.HTMLURL, body)}, nil
	case "comment":
		if p.Number <= 0 || strings.TrimSpace(p.Body) == "" {
			return tool.ToolOutput{}, fmt.Errorf("comment requires number and body")
		}
		cm, err := t.client.CreateComment(ctx, t.owner, p.Repo, p.Number, redactBody(tctx, p.Body))
		if err != nil {
			return tool.ToolOutput{}, err
		}
		ref := fmt.Sprintf("%s#%d", p.Repo, p.Number)
		return tool.ToolOutput{
			Content: "posted comment: " + cm.HTMLURL,
			Effect:  &tool.ToolEffect{Tool: "github", Kind: "comment", URL: cm.HTMLURL, Ref: ref, Summary: "commented on " + ref},
		}, nil
	case "create_issue":
		if strings.TrimSpace(p.Title) == "" {
			return tool.ToolOutput{}, fmt.Errorf("create_issue requires a title")
		}
		if err := ghsafe.ValidateParam("title", p.Title, true); err != nil {
			return tool.ToolOutput{}, err
		}
		iss, err := t.client.CreateIssue(ctx, t.owner, p.Repo,
			redactBody(tctx, p.Title), redactBody(tctx, p.Body), []string{"bot-proposed", "needs-review"})
		if err != nil {
			return tool.ToolOutput{}, err
		}
		ref := fmt.Sprintf("%s#%d", p.Repo, iss.Number)
		return tool.ToolOutput{
			Content: "opened issue: " + iss.HTMLURL,
			Effect:  &tool.ToolEffect{Tool: "github", Kind: "issue", URL: iss.HTMLURL, Ref: ref, Summary: "opened issue " + ref},
		}, nil
	default:
		return tool.ToolOutput{}, fmt.Errorf("action %q not supported", p.Action)
	}
}

// GitHubWriteTool provides code-writing actions (branch+PR, review, close) — ADMIN ONLY.
type GitHubWriteTool struct {
	tool.AdminOnly
	githubBase
}

// NewGitHubWriteTool builds the admin-only GitHub write tool.
func NewGitHubWriteTool(c *ghclient.Client, owner, botLogin string, blockedRepos []string) *GitHubWriteTool {
	return &GitHubWriteTool{githubBase: githubBase{
		client: c, owner: owner, botLogin: botLogin,
		allowedRepos: defaultAllowedRepos, blockedRepos: blockedRepos,
	}}
}

type githubWriteParams struct {
	Action  string `json:"action"`
	Repo    string `json:"repo"`
	Branch  string `json:"branch,omitempty"`
	Path    string `json:"path,omitempty"`
	Content string `json:"content,omitempty"`
	Message string `json:"message,omitempty"`
	Head    string `json:"head,omitempty"`
	Base    string `json:"base,omitempty"`
	Title   string `json:"title,omitempty"`
	Body    string `json:"body,omitempty"`
	Number  int    `json:"number,omitempty"`
	Event   string `json:"event,omitempty"`
}

func (t *GitHubWriteTool) Def() tool.ToolDef {
	return tool.ToolDef{
		Name: "github_write",
		Description: "Write code changes and reviews to GitHub (maintainers only). action is one of: " +
			"push_file (commit a single file to a NON-protected feature branch, creating it from the default branch if needed), " +
			"open_pr (open a pull request from a feature branch into a base branch), " +
			"review (submit a PR review: COMMENT/APPROVE/REQUEST_CHANGES), " +
			"close_issue (close a bot-authored issue). Pushing to protected branches like main is refused; all changes go via a pull request.",
		Parameters: json.RawMessage(`{"type":"object","properties":{` +
			`"action":{"type":"string","enum":["push_file","open_pr","review","close_issue"]},` +
			`"repo":{"type":"string","enum":["all-chat","all-chat-extension"]},` +
			`"branch":{"type":"string","description":"feature branch (push_file)"},` +
			`"path":{"type":"string","description":"repo file path (push_file)"},` +
			`"content":{"type":"string","description":"full new file content (push_file)"},` +
			`"message":{"type":"string","description":"commit message (push_file)"},` +
			`"head":{"type":"string","description":"source feature branch (open_pr)"},` +
			`"base":{"type":"string","description":"target branch, default main (open_pr)"},` +
			`"title":{"type":"string","description":"PR title (open_pr)"},` +
			`"body":{"type":"string","description":"PR/review markdown body"},` +
			`"number":{"type":"integer","description":"PR or issue number (review, close_issue)"},` +
			`"event":{"type":"string","enum":["COMMENT","APPROVE","REQUEST_CHANGES"],"description":"review verdict"}},` +
			`"required":["action","repo"]}`),
	}
}

func (t *GitHubWriteTool) Invoke(ctx context.Context, tctx *tool.ToolCtx, args json.RawMessage) (tool.ToolOutput, error) {
	var p githubWriteParams
	if err := json.Unmarshal(args, &p); err != nil {
		return tool.ToolOutput{}, fmt.Errorf("invalid arguments: %w", err)
	}
	if err := t.checkRepo(p.Repo); err != nil {
		return tool.ToolOutput{}, err
	}
	switch p.Action {
	case "push_file":
		return t.pushFile(ctx, tctx, p)
	case "open_pr":
		return t.openPR(ctx, tctx, p)
	case "review":
		return t.review(ctx, tctx, p)
	case "close_issue":
		return t.closeIssue(ctx, p)
	default:
		return tool.ToolOutput{}, fmt.Errorf("action %q not supported", p.Action)
	}
}

func (t *GitHubWriteTool) pushFile(ctx context.Context, tctx *tool.ToolCtx, p githubWriteParams) (tool.ToolOutput, error) {
	if err := validateBranch(p.Branch); err != nil {
		return tool.ToolOutput{}, err
	}
	if ghsafe.IsProtectedBranch(p.Branch) {
		return tool.ToolOutput{}, fmt.Errorf("refusing to push to protected branch %q; use a feature branch", p.Branch)
	}
	if err := ghsafe.ValidateParam("path", p.Path, false); err != nil {
		return tool.ToolOutput{}, err
	}
	if p.Content == "" || strings.TrimSpace(p.Message) == "" {
		return tool.ToolOutput{}, fmt.Errorf("push_file requires content and message")
	}
	// Ensure the branch exists, creating it from the default branch if absent.
	if _, err := t.client.BranchSHA(ctx, t.owner, p.Repo, p.Branch); err != nil {
		def, derr := t.client.DefaultBranch(ctx, t.owner, p.Repo)
		if derr != nil {
			return tool.ToolOutput{}, derr
		}
		sha, serr := t.client.BranchSHA(ctx, t.owner, p.Repo, def)
		if serr != nil {
			return tool.ToolOutput{}, serr
		}
		if err := t.client.CreateBranch(ctx, t.owner, p.Repo, p.Branch, sha); err != nil {
			return tool.ToolOutput{}, err
		}
	}
	if err := t.client.PutFile(ctx, t.owner, p.Repo, p.Path, p.Branch, redactBody(tctx, p.Message), p.Content); err != nil {
		return tool.ToolOutput{}, err
	}
	return tool.ToolOutput{Content: fmt.Sprintf("committed %s to branch %s of %s", p.Path, p.Branch, p.Repo)}, nil
}

func (t *GitHubWriteTool) openPR(ctx context.Context, tctx *tool.ToolCtx, p githubWriteParams) (tool.ToolOutput, error) {
	if err := validateBranch(p.Head); err != nil {
		return tool.ToolOutput{}, fmt.Errorf("head: %w", err)
	}
	if ghsafe.IsProtectedBranch(p.Head) {
		return tool.ToolOutput{}, fmt.Errorf("head branch %q must not be a protected branch", p.Head)
	}
	base := p.Base
	if base == "" {
		base = "main"
	}
	if err := validateBranch(base); err != nil {
		return tool.ToolOutput{}, fmt.Errorf("base: %w", err)
	}
	if strings.TrimSpace(p.Title) == "" {
		return tool.ToolOutput{}, fmt.Errorf("open_pr requires a title")
	}
	pr, err := t.client.CreatePullRequest(ctx, t.owner, p.Repo,
		redactBody(tctx, p.Title), p.Head, base, redactBody(tctx, p.Body))
	if err != nil {
		return tool.ToolOutput{}, err
	}
	ref := fmt.Sprintf("%s#%d", p.Repo, pr.Number)
	return tool.ToolOutput{
		Content: "opened pull request: " + pr.HTMLURL,
		Effect:  &tool.ToolEffect{Tool: "github", Kind: "pr", URL: pr.HTMLURL, Ref: ref, Summary: "opened PR " + ref},
	}, nil
}

func (t *GitHubWriteTool) review(ctx context.Context, tctx *tool.ToolCtx, p githubWriteParams) (tool.ToolOutput, error) {
	if p.Number <= 0 {
		return tool.ToolOutput{}, fmt.Errorf("review requires a PR number")
	}
	switch p.Event {
	case "COMMENT", "APPROVE", "REQUEST_CHANGES":
	default:
		return tool.ToolOutput{}, fmt.Errorf("event must be COMMENT, APPROVE, or REQUEST_CHANGES")
	}
	rv, err := t.client.CreateReview(ctx, t.owner, p.Repo, p.Number, p.Event, redactBody(tctx, p.Body))
	if err != nil {
		return tool.ToolOutput{}, err
	}
	ref := fmt.Sprintf("%s#%d", p.Repo, p.Number)
	return tool.ToolOutput{
		Content: fmt.Sprintf("submitted %s review on %s: %s", p.Event, ref, rv.HTMLURL),
		Effect:  &tool.ToolEffect{Tool: "github", Kind: "review", URL: rv.HTMLURL, Ref: ref, Summary: p.Event + " review on " + ref},
	}, nil
}

func (t *GitHubWriteTool) closeIssue(ctx context.Context, p githubWriteParams) (tool.ToolOutput, error) {
	if p.Number <= 0 {
		return tool.ToolOutput{}, fmt.Errorf("close_issue requires a number")
	}
	// Only close issues the bot itself authored.
	iss, err := t.client.GetIssue(ctx, t.owner, p.Repo, p.Number)
	if err != nil {
		return tool.ToolOutput{}, err
	}
	if t.botLogin == "" || !strings.EqualFold(iss.User.Login, t.botLogin) {
		return tool.ToolOutput{}, fmt.Errorf("refusing to close %s#%d: not authored by the bot", p.Repo, p.Number)
	}
	if err := t.client.CloseIssue(ctx, t.owner, p.Repo, p.Number); err != nil {
		return tool.ToolOutput{}, err
	}
	ref := fmt.Sprintf("%s#%d", p.Repo, p.Number)
	return tool.ToolOutput{Content: "closed " + ref}, nil
}
