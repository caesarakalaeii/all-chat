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

// Package ghclient is a minimal GitHub REST client covering exactly the operations the
// support-bot needs: read/create issues, comment, create branches, commit single-file
// changes, open pull requests, and submit reviews. It holds the token internally and
// never returns it; upstream error bodies are reduced to the GitHub "message" field.
package ghclient

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.uber.org/zap"
)

const apiBase = "https://api.github.com"

// Client is a thin GitHub REST client.
type Client struct {
	token string
	base  string
	http  *http.Client
	log   *zap.Logger
}

// New builds a client. token may be empty (read-only unauthenticated), though the
// support-bot always supplies one.
func New(token string, log *zap.Logger) *Client {
	return &Client{
		token: token,
		base:  apiBase,
		http: &http.Client{
			Timeout:       20 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
		log: log,
	}
}

// User is a GitHub account (only the login is used).
type User struct {
	Login string `json:"login"`
}

// Issue is an issue or PR (GitHub models PRs as issues for comments).
type Issue struct {
	Number  int    `json:"number"`
	Title   string `json:"title"`
	Body    string `json:"body"`
	State   string `json:"state"`
	HTMLURL string `json:"html_url"`
	User    User   `json:"user"`
}

// Comment is a created issue/PR comment.
type Comment struct {
	HTMLURL string `json:"html_url"`
}

// PullRequest is a created pull request.
type PullRequest struct {
	Number  int    `json:"number"`
	HTMLURL string `json:"html_url"`
	User    User   `json:"user"`
}

// Review is a submitted PR review.
type Review struct {
	HTMLURL string `json:"html_url"`
	State   string `json:"state"`
}

// --- request payloads (typed, no untyped maps) ---

type issueCreateReq struct {
	Title  string   `json:"title"`
	Body   string   `json:"body"`
	Labels []string `json:"labels,omitempty"`
}

type commentReq struct {
	Body string `json:"body"`
}

type refCreateReq struct {
	Ref string `json:"ref"`
	SHA string `json:"sha"`
}

type putFileReq struct {
	Message string `json:"message"`
	Content string `json:"content"`
	Branch  string `json:"branch"`
	SHA     string `json:"sha,omitempty"`
}

type prCreateReq struct {
	Title string `json:"title"`
	Head  string `json:"head"`
	Base  string `json:"base"`
	Body  string `json:"body"`
}

type reviewReq struct {
	Event string `json:"event"`
	Body  string `json:"body"`
}

type issuePatchReq struct {
	State string `json:"state"`
}

type repoInfo struct {
	DefaultBranch string `json:"default_branch"`
}

type gitRef struct {
	Object struct {
		SHA string `json:"sha"`
	} `json:"object"`
}

type contentInfo struct {
	SHA string `json:"sha"`
}

// GetIssue fetches an issue or PR by number.
func (c *Client) GetIssue(ctx context.Context, owner, repo string, number int) (*Issue, error) {
	raw, err := c.do(ctx, http.MethodGet, fmt.Sprintf("/repos/%s/%s/issues/%d", owner, repo, number), nil)
	if err != nil {
		return nil, err
	}
	out := &Issue{}
	return out, json.Unmarshal(raw, out)
}

// CreateIssue opens a new issue.
func (c *Client) CreateIssue(ctx context.Context, owner, repo, title, body string, labels []string) (*Issue, error) {
	payload, _ := json.Marshal(issueCreateReq{Title: title, Body: body, Labels: labels})
	raw, err := c.do(ctx, http.MethodPost, fmt.Sprintf("/repos/%s/%s/issues", owner, repo), payload)
	if err != nil {
		return nil, err
	}
	out := &Issue{}
	return out, json.Unmarshal(raw, out)
}

// CreateComment posts a comment on an issue or PR.
func (c *Client) CreateComment(ctx context.Context, owner, repo string, number int, body string) (*Comment, error) {
	payload, _ := json.Marshal(commentReq{Body: body})
	raw, err := c.do(ctx, http.MethodPost,
		fmt.Sprintf("/repos/%s/%s/issues/%d/comments", owner, repo, number), payload)
	if err != nil {
		return nil, err
	}
	out := &Comment{}
	return out, json.Unmarshal(raw, out)
}

// DefaultBranch returns the repository's default branch name.
func (c *Client) DefaultBranch(ctx context.Context, owner, repo string) (string, error) {
	raw, err := c.do(ctx, http.MethodGet, fmt.Sprintf("/repos/%s/%s", owner, repo), nil)
	if err != nil {
		return "", err
	}
	var info repoInfo
	if err := json.Unmarshal(raw, &info); err != nil {
		return "", err
	}
	return info.DefaultBranch, nil
}

// BranchSHA returns the commit SHA a branch points at.
func (c *Client) BranchSHA(ctx context.Context, owner, repo, branch string) (string, error) {
	raw, err := c.do(ctx, http.MethodGet,
		fmt.Sprintf("/repos/%s/%s/git/ref/heads/%s", owner, repo, url.PathEscape(branch)), nil)
	if err != nil {
		return "", err
	}
	var ref gitRef
	if err := json.Unmarshal(raw, &ref); err != nil {
		return "", err
	}
	return ref.Object.SHA, nil
}

// CreateBranch creates refs/heads/<branch> pointing at fromSHA.
func (c *Client) CreateBranch(ctx context.Context, owner, repo, branch, fromSHA string) error {
	payload, _ := json.Marshal(refCreateReq{Ref: "refs/heads/" + branch, SHA: fromSHA})
	_, err := c.do(ctx, http.MethodPost, fmt.Sprintf("/repos/%s/%s/git/refs", owner, repo), payload)
	return err
}

// fileSHA returns the blob SHA of an existing file on a branch, or "" if it does not
// exist (404).
func (c *Client) fileSHA(ctx context.Context, owner, repo, path, branch string) (string, error) {
	raw, err := c.do(ctx, http.MethodGet,
		fmt.Sprintf("/repos/%s/%s/contents/%s?ref=%s", owner, repo, escapePath(path), url.QueryEscape(branch)), nil)
	if err != nil {
		var apiErr *APIError
		if errors.As(err, &apiErr) && apiErr.Status == http.StatusNotFound {
			return "", nil
		}
		return "", err
	}
	var info contentInfo
	if err := json.Unmarshal(raw, &info); err != nil {
		return "", err
	}
	return info.SHA, nil
}

// PutFile creates or updates a single file on a branch (Contents API).
func (c *Client) PutFile(ctx context.Context, owner, repo, path, branch, message, content string) error {
	sha, err := c.fileSHA(ctx, owner, repo, path, branch)
	if err != nil {
		return err
	}
	payload, _ := json.Marshal(putFileReq{
		Message: message,
		Content: base64.StdEncoding.EncodeToString([]byte(content)),
		Branch:  branch,
		SHA:     sha,
	})
	_, err = c.do(ctx, http.MethodPut,
		fmt.Sprintf("/repos/%s/%s/contents/%s", owner, repo, escapePath(path)), payload)
	return err
}

// CreatePullRequest opens a PR from head into base.
func (c *Client) CreatePullRequest(ctx context.Context, owner, repo, title, head, base, body string) (*PullRequest, error) {
	payload, _ := json.Marshal(prCreateReq{Title: title, Head: head, Base: base, Body: body})
	raw, err := c.do(ctx, http.MethodPost, fmt.Sprintf("/repos/%s/%s/pulls", owner, repo), payload)
	if err != nil {
		return nil, err
	}
	out := &PullRequest{}
	return out, json.Unmarshal(raw, out)
}

// CloseIssue sets an issue's state to closed.
func (c *Client) CloseIssue(ctx context.Context, owner, repo string, number int) error {
	payload, _ := json.Marshal(issuePatchReq{State: "closed"})
	_, err := c.do(ctx, http.MethodPatch, fmt.Sprintf("/repos/%s/%s/issues/%d", owner, repo, number), payload)
	return err
}

// CreateReview submits a review on a PR. event is COMMENT, APPROVE, or REQUEST_CHANGES.
func (c *Client) CreateReview(ctx context.Context, owner, repo string, number int, event, body string) (*Review, error) {
	payload, _ := json.Marshal(reviewReq{Event: event, Body: body})
	raw, err := c.do(ctx, http.MethodPost, fmt.Sprintf("/repos/%s/%s/pulls/%d/reviews", owner, repo, number), payload)
	if err != nil {
		return nil, err
	}
	out := &Review{}
	return out, json.Unmarshal(raw, out)
}

// APIError is a GitHub API error reduced to status + message.
type APIError struct {
	Status  int
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("github api error (status %d): %s", e.Status, e.Message)
}

// do performs a request and returns the 2xx body bytes, or an *APIError on non-2xx.
func (c *Client) do(ctx context.Context, method, path string, reqBody []byte) ([]byte, error) {
	var reader io.Reader
	if reqBody != nil {
		reader = bytes.NewReader(reqBody)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "allchat-support-bot")
	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode >= 300 {
		return nil, &APIError{Status: resp.StatusCode, Message: extractMessage(raw)}
	}
	return raw, nil
}

// extractMessage pulls the "message" field out of a GitHub error body, falling back to
// a capped raw string.
func extractMessage(raw []byte) string {
	var m struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(raw, &m); err == nil && m.Message != "" {
		return m.Message
	}
	s := strings.TrimSpace(string(raw))
	if len(s) > 300 {
		s = s[:300]
	}
	return s
}

// escapePath escapes each path segment while preserving slashes.
func escapePath(p string) string {
	parts := strings.Split(p, "/")
	for i, seg := range parts {
		parts[i] = url.PathEscape(seg)
	}
	return strings.Join(parts, "/")
}
