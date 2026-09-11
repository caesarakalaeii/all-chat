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

// Package client is a thin HTTP client for the Facebook Graph API. Shapes are
// pinned against the v26.0 reference docs:
//   - Page live videos: https://developers.facebook.com/docs/graph-api/reference/page/live_videos/
//   - Live video comments: https://developers.facebook.com/docs/graph-api/reference/live-video/comments/
//   - Comment node: https://developers.facebook.com/docs/graph-api/reference/comment/
//
// Graph errors carry a JSON body with an `error` object (code, message,
// fbtrace_id); see https://developers.facebook.com/docs/graph-api/guides/error-handling/.
package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Base URL and version. Graph versions stay valid for two years after the next
// release, so a pinned version is safer than an unversioned call (which uses
// whatever the app dashboard has configured).
const (
	DefaultBaseURL    = "https://graph.facebook.com"
	DefaultAPIVersion = "v26.0"
)

// Live video statuses. LIVE_NOW is the one we poll.
const (
	LiveStatusLiveNow = "LIVE_NOW"
)

// Errors classified from the HTTP status, mirroring the moderation
// clients' sentinels so callers can refresh-and-retry a 401.
var (
	ErrUnauthorized = fmt.Errorf("facebook: access token unauthorized")
	ErrForbidden    = fmt.Errorf("facebook: forbidden (token invalid or permission revoked)")
	ErrRateLimited  = fmt.Errorf("facebook: rate limited")
)

// Client calls the Facebook Graph API with a Page access token.
type Client struct {
	httpClient *http.Client
	baseURL    string
	apiVersion string
}

// NewClient builds a Graph API client. baseAPI/version may be empty for defaults.
func NewClient(baseAPI, version string) *Client {
	if baseAPI == "" {
		baseAPI = DefaultBaseURL
	}
	if version == "" {
		version = DefaultAPIVersion
	}
	return &Client{
		httpClient: &http.Client{Timeout: 15 * time.Second},
		baseURL:    strings.TrimRight(baseAPI, "/"),
		apiVersion: version,
	}
}

// Error is the Graph API error envelope.
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Type    string `json:"type"`
	FBTrace string `json:"fbtrace_id"`
}

func (e *Error) Error() string {
	return fmt.Sprintf("facebook: graph error %d: %s", e.Code, e.Message)
}

// graphError decodes the error envelope and maps >=400 statuses that carry
// known throttle codes (4, 17, 32, 613) onto ErrRateLimited, auth/permission
// responses onto the sentinels, and everything else onto a descriptive error.
// See https://developers.facebook.com/docs/graph-api/overview/rate-limiting.
func (c *Client) graphError(body []byte, status int) error {
	var env struct {
		Error *Error `json:"error"`
	}
	if err := json.Unmarshal(body, &env); err == nil && env.Error != nil {
		switch env.Error.Code {
		case 4, 17, 32, 613, 80001:
			return ErrRateLimited
		case 190, 102:
			return ErrUnauthorized
		case 200:
			return ErrForbidden
		}
		return env.Error
	}
	return fmt.Errorf("facebook: api returned status %d: %s", status, truncate(body, 512))
}

// get performs a GET against a Graph path with query params, a token, and
// decodes the response into out.
func (c *Client) get(ctx context.Context, path string, params url.Values, token string, out interface{}, usage *Usage) error {
	endpoint := fmt.Sprintf("%s/%s/%s", c.baseURL, c.apiVersion, strings.TrimPrefix(path, "/"))
	if params == nil {
		params = url.Values{}
	}
	params.Set("access_token", token)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return fmt.Errorf("facebook: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("facebook: request failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return fmt.Errorf("facebook: read body: %w", err)
	}
	*usage = readUsage(resp.Header)

	if resp.StatusCode >= 400 {
		return c.graphError(body, resp.StatusCode)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("facebook: decode response: %w", err)
	}
	return nil
}

// Usage is the rate-limit picture from the response headers. See the Rate
// Limits doc: page-token requests carry X-Business-Use-Case-Usage, other
// tokens X-App-Usage; both report the same call_count percentage shape.
type Usage struct {
	CallCount int64 // percentage of the rolling window consumed
}

func readUsage(h http.Header) Usage {
	var u Usage
	if raw := h.Get("X-Business-Use-Case-Usage"); raw != "" {
		u.callCountFromBUC(raw)
	} else if raw := h.Get("X-App-Usage"); raw != "" {
		var hdr struct {
			CallCount int64 `json:"call_count"`
		}
		if json.Unmarshal([]byte(raw), &hdr) == nil {
			u.CallCount = hdr.CallCount
		}
	}
	return u
}

func (u *Usage) callCountFromBUC(raw string) {
	// X-Business-Use-Case-Usage is a JSON object keyed by business id, each
	// holding a list of usage entries. We surface the max call_count
	// percentage across entries — throttling is per business object.
	var parsed map[string][]struct {
		CallCount int64 `json:"call_count"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return
	}
	for _, entries := range parsed {
		for _, e := range entries {
			if e.CallCount > u.CallCount {
				u.CallCount = e.CallCount
			}
		}
	}
}

func truncate(b []byte, n int) string {
	s := string(b)
	if len(s) > n {
		return s[:n]
	}
	return s
}


// LiveVideoSummary is the subset of a LiveVideo node the listener needs.
type LiveVideoSummary struct {
	ID          string    `json:"id"`
	Status      string    `json:"status"`
	Title       string    `json:"title"`
	CreatedTime time.Time `json:"created_time"`
}

// liveVideosResponse is the paged shape of GET /{page-id}/live_videos.
type liveVideosResponse struct {
	Data []LiveVideoSummary `json:"data"`
}

// LiveVideosWithUsage returns the Page's live videos, newest first as Graph
// returns them. status filtering is done client-side because the edge accepts
// no server-side status param (see the page/live_videos reference: Reading is
// a plain paged edge; status values LIVE_NOW / SCHEDULED_* / UNPUBLISHED are
// defined on the LiveVideo node).
func (c *Client) LiveVideosWithUsage(ctx context.Context, pageID, pageToken string) ([]LiveVideoSummary, Usage, error) {
	params := url.Values{}
	params.Set("fields", "id,status,title,created_time")
	params.Set("limit", "25")
	var out liveVideosResponse
	usage := Usage{}
	if err := c.get(ctx, pageID+"/live_videos", params, pageToken, &out, &usage); err != nil {
		return nil, usage, fmt.Errorf("fetch live videos for page %s: %w", pageID, err)
	}
	return out.Data, usage, nil
}

// LiveVideos is LiveVideosWithUsage without the usage surface (test seam).
func (c *Client) LiveVideos(ctx context.Context, pageID, pageToken string) ([]LiveVideoSummary, error) {
	videos, _, err := c.LiveVideosWithUsage(ctx, pageID, pageToken)
	return videos, err
}


// Comment is the subset of a Comment node read from a live video.
// from.id is the app-scoped user id; from.name the display name.
type Comment struct {
	ID          string `json:"id"`
	Message     string `json:"message"`
	CreatedTime string `json:"created_time"` // RFC3339
	From        struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"from"`
	Parent struct {
		ID string `json:"id"`
	} `json:"parent"`
}

// commentsResponse is the paged shape of GET /{live-video-id}/comments.
type commentsResponse struct {
	Data   []Comment `json:"data"`
	Paging struct {
		Cursors struct {
			After  string `json:"after"`
			Before string `json:"before"`
		} `json:"cursors"`
		Next string `json:"next"`
	} `json:"paging"`
}

// CommentsParams carries the fixed comment-poll query.
//
// Per the live-video/comments reference, the best practice for a live video is
// to poll with order=reverse_chronological and cursor pagination; `since` is
// also accepted as a lower time bound. We drive the cursor from the newest
// comment's created_time + a defensive id-set, so both mechanisms stay
// consistent even if Graph ignores one of them.
type CommentsParams struct {
	Order string // "reverse_chronological" recommended for live polling
	Since string // optional created_time lower bound ("since" param)
	After string // optional cursor from the previous page
	Limit string // page size, e.g. "100"
}

// CommentsWithUsage fetches one page of comments on a live video.
func (c *Client) CommentsWithUsage(ctx context.Context, liveVideoID, token string, p CommentsParams) ([]Comment, string, Usage, error) {
	params := url.Values{}
	params.Set("fields", "id,message,created_time,from{id,name},parent{id}")
	if p.Order != "" {
		params.Set("order", p.Order)
	}
	if p.Since != "" {
		params.Set("since", p.Since)
	}
	if p.After != "" {
		params.Set("after", p.After)
	}
	if p.Limit != "" {
		params.Set("limit", p.Limit)
	}
	var out commentsResponse
	usage := Usage{}
	if err := c.get(ctx, liveVideoID+"/comments", params, token, &out, &usage); err != nil {
		return nil, "", usage, fmt.Errorf("fetch comments for live video %s: %w", liveVideoID, err)
	}
	return out.Data, out.Paging.Cursors.After, usage, nil
}

// Comments is CommentsWithUsage without the usage surface (test seam).
func (c *Client) Comments(ctx context.Context, liveVideoID, token string, p CommentsParams) ([]Comment, string, error) {
	data, after, _, err := c.CommentsWithUsage(ctx, liveVideoID, token, p)
	return data, after, err
}

// Me returns the token's user identity (app-scoped id + name).
func (c *Client) Me(ctx context.Context, token string) (id string, name string, err error) {
	var out struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	usage := Usage{}
	if err := c.get(ctx, "me", url.Values{"fields": {"id,name"}}, token, &out, &usage); err != nil {
		return "", "", err
	}
	return out.ID, out.Name, nil
}

// MyPages returns the Pages the (long-lived) user token can access, with their
// page tokens and task lists. Used by the login/add-source flows in
// auth-service to resolve which Page the streamer connected.
// See https://developers.facebook.com/docs/graph-api/reference/user/accounts.
func (c *Client) MyPages(ctx context.Context, userToken string) ([]PageSummary, error) {
	type pageResponse struct {
		Data []PageSummary `json:"data"`
	}
	var out pageResponse
	params := url.Values{}
	params.Set("fields", "id,name,access_token,tasks")
	params.Set("limit", "100")
	usage := Usage{}
	if err := c.get(ctx, "me/accounts", params, userToken, &out, &usage); err != nil {
		return nil, fmt.Errorf("fetch pages: %w", err)
	}
	return out.Data, nil
}

// PageSummary is one entry of GET /me/accounts.
type PageSummary struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	AccessToken string   `json:"access_token"`
	Tasks       []string `json:"tasks"`
}

// HasTask reports whether the user holds one of the Page tasks relevant for
// reading + moderating (MODERATE covers comment writes; MANAGE covers blocks).
func (p *PageSummary) HasModerateTask() bool {
	for _, t := range p.Tasks {
		if t == "MODERATE" || t == "MANAGE" {
			return true
		}
	}
	return false
}

// itoa avoids a strconv import at each call site that formats a page param.
func itoa(n int) string { return strconv.Itoa(n) }
