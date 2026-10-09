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

package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Facebook moderation write-path (ADR-0060), all calls with the streamer's
// stored Page access token:
//
//   - Delete a comment:  DELETE /{comment-id}                    ({success: true})
//   - Hide a comment:    POST /{comment-id}   is_hidden=true     ({success: true})
//   - Unhide a comment:  POST /{comment-id}   is_hidden=false
//   - Ban a page user:   POST /{page-id}/blocked   user=<uid>     (map uid->bool)
//   - Unban a page user: DELETE /{page-id}/blocked user=<uid>     ({success: true})
//
// Endpoints pinned against the Graph API v26.0 references:
//   - https://developers.facebook.com/docs/graph-api/reference/comment/
//   - https://developers.facebook.com/docs/graph-api/reference/page/blocked/
//
// Facebook has no timeout action (comment deletion is the only "remove
// message" verb), so `timeout` is reported unsupported rather than mapped onto
// a ban. Ban/unban key on the commenter's user id (app-scoped id from the
// comment's from.id).
var (
	// ErrFacebookUnauthorized indicates the page token is invalid/expired/revoked (HTTP 401).
	ErrFacebookUnauthorized = errors.New("facebook: access token unauthorized")
	// ErrFacebookForbidden indicates the token lacks the permission or the
	// actor does not moderate the Page (HTTP 403 / graph code 200).
	ErrFacebookForbidden = errors.New("facebook: forbidden (missing permission or not a page moderator)")
)

const (
	defaultFacebookBaseURL    = "https://graph.facebook.com"
	defaultFacebookAPIVersion = "v26.0"
)

// FacebookClient calls Facebook's Graph moderation endpoints as the Page.
type FacebookClient struct {
	httpClient *http.Client
	baseURL    string
	apiVersion string
}

// NewFacebookClient builds a Facebook moderation client. baseURL/version are
// overridable for tests.
func NewFacebookClient(baseURL, apiVersion string) *FacebookClient {
	if baseURL == "" {
		baseURL = defaultFacebookBaseURL
	}
	if apiVersion == "" {
		apiVersion = defaultFacebookAPIVersion
	}
	return &FacebookClient{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiVersion: apiVersion,
	}
}

// facebookAPI is the subset the dispatcher calls; an interface keeps dispatch
// unit-testable with a fake (same pattern as kickAPI/youtubeAPI).
type facebookAPI interface {
	DeleteComment(ctx context.Context, token, commentID string) error
	HideComment(ctx context.Context, token, commentID string, hidden bool) error
	BanUser(ctx context.Context, token, pageID, userID string) error
	UnbanUser(ctx context.Context, token, pageID, userID string) error
}

// DeleteComment removes a single comment on the Page's content.
// DELETE /{comment-id} — needs pages_manage_engagement (ADR-0060).
func (c *FacebookClient) DeleteComment(ctx context.Context, token, commentID string) error {
	if commentID == "" {
		return errors.New("facebook: empty comment id")
	}
	return c.do(ctx, http.MethodDelete, "/"+url.PathEscape(commentID), token, nil)
}

// HideComment hides (true) or unhides (false) a comment.
// POST /{comment-id} with is_hidden — only valid on Page-owned comments.
func (c *FacebookClient) HideComment(ctx context.Context, token, commentID string, hidden bool) error {
	if commentID == "" {
		return errors.New("facebook: empty comment id")
	}
	body, err := json.Marshal(map[string]bool{"is_hidden": hidden})
	if err != nil {
		return fmt.Errorf("facebook: marshal hide body: %w", err)
	}
	return c.do(ctx, http.MethodPost, "/"+url.PathEscape(commentID), token, body)
}

// BanUser permanently blocks a user from the Page.
// POST /{page-id}/blocked with user=<id>; returns a map {uid: bool}.
func (c *FacebookClient) BanUser(ctx context.Context, token, pageID, userID string) error {
	if pageID == "" || userID == "" {
		return errors.New("facebook: ban requires page id and user id")
	}
	body, err := json.Marshal(map[string]string{"user": userID})
	if err != nil {
		return fmt.Errorf("facebook: marshal ban body: %w", err)
	}
	return c.do(ctx, http.MethodPost, "/"+url.PathEscape(pageID)+"/blocked", token, body)
}

// UnbanUser lifts a block.
// DELETE /{page-id}/blocked with user=<id>; returns {success: bool}.
func (c *FacebookClient) UnbanUser(ctx context.Context, token, pageID, userID string) error {
	if pageID == "" || userID == "" {
		return errors.New("facebook: unban requires page id and user id")
	}
	params := url.Values{"user": {userID}}
	return c.do(ctx, http.MethodDelete, "/"+url.PathEscape(pageID)+"/blocked?"+params.Encode(), token, nil)
}

// do issues the request and maps the status: 2xx OK, 401 → sentinel, 403 →
// sentinel, 4 (throttle codes 4/17/32/613 ride in graph error bodies with 400)
// → descriptive error carrying the body for the audit log.
func (c *FacebookClient) do(ctx context.Context, method, path, token string, body []byte) error {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	endpoint := c.baseURL + "/" + c.apiVersion + path
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return fmt.Errorf("facebook: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("facebook: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return ErrFacebookUnauthorized
	case http.StatusForbidden:
		return ErrFacebookForbidden
	default:
		// Graph returns 400 for throttled calls with code 4/17/32/613 in the
		// body; surface those distinctly so the dispatcher can back off.
		if code, ok := graphThrottleCode(snippet); ok {
			return fmt.Errorf("facebook: api rate limited (graph code %d): %s", code, string(snippet))
		}
		return fmt.Errorf("facebook: api returned %s: %s", strconv.Itoa(resp.StatusCode), string(snippet))
	}
}

// graphThrottleCode extracts the graph error code from a 400 body and reports
// whether it is one of the throttle codes.
func graphThrottleCode(body []byte) (int, bool) {
	var env struct {
		Error struct {
			Code int `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return 0, false
	}
	switch env.Error.Code {
	case 4, 17, 32, 613, 80001:
		return env.Error.Code, true
	}
	return 0, false
}

// Compile-time check that FacebookClient satisfies the dispatcher's view.
var _ facebookAPI = (*FacebookClient)(nil)
