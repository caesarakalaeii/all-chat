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

// Package client is a thin HTTP client for the Instagram Graph API (the
// Instagram API with Facebook Login flavor, served from graph.facebook.com).
// Shapes are pinned against the v26.0 references:
//   - Live comments (the ingest edge): https://developers.facebook.com/docs/instagram-platform/instagram-graph-api/live-comments/
//   - IG User live media (broadcast discovery): https://developers.facebook.com/docs/instagram-platform/instagram-graph-api/reference/ig-user/live_media/
//   - IG Comment node (fields): https://developers.facebook.com/docs/instagram-platform/instagram-graph-api/reference/ig-comment/
//
// Graph errors carry a JSON body with an `error` object (code, message,
// fbtrace_id), the same envelope Facebook's Graph API returns; see
// https://developers.facebook.com/docs/graph-api/guides/error-handling/.
package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Base URL and version. The Instagram API with Facebook Login is served from
// graph.facebook.com (https://developers.facebook.com/documentation/instagram-platform/reference).
// Graph versions stay valid for two years after the next release, so a pinned
// version is safer than an unversioned call.
const (
	DefaultBaseURL    = "https://graph.facebook.com"
	DefaultAPIVersion = "v26.0"
)

// Errors classified from the HTTP status, mirroring the facebook-listener's
// sentinels so callers can treat an expired/revoked token as a credential
// problem (the streamer re-auths) rather than a transport problem.
var (
	ErrUnauthorized = fmt.Errorf("instagram: access token unauthorized")
	ErrForbidden    = fmt.Errorf("instagram: forbidden (token invalid or permission revoked)")
	ErrRateLimited  = fmt.Errorf("instagram: rate limited")
)

// Client calls the Instagram Graph API with a Page access token (the token
// auth-service stores for the Page linked to the streamer's IG account).
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

// Error is the Graph API error envelope (shared shape with Facebook).
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Type    string `json:"type"`
	FBTrace string `json:"fbtrace_id"`
}

func (e *Error) Error() string {
	return fmt.Sprintf("instagram: graph error %d: %s", e.Code, e.Message)
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
	return fmt.Errorf("instagram: api returned status %d: %s", status, truncate(body, 512))
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
		return fmt.Errorf("instagram: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("instagram: request failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return fmt.Errorf("instagram: read body: %w", err)
	}
	*usage = readUsage(resp.Header)

	if resp.StatusCode >= 400 {
		return c.graphError(body, resp.StatusCode)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("instagram: decode response: %w", err)
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

// LiveMediaSummary is the subset of a live-video IG Media node the listener
// needs. media_type BROADCAST with media_product_type LIVE is the shape
// GET /{ig-user-id}/live_media returns for an in-progress broadcast; the edge
// returns ONLY media being broadcast at request time
// (https://developers.facebook.com/docs/instagram-platform/instagram-graph-api/reference/ig-user/live_media/
// — "Only live video IG Media being broadcast at the time of the request will
// be returned"), so an empty data array IS the not-live signal.
type LiveMediaSummary struct {
	ID               string `json:"id"`
	MediaType        string `json:"media_type"`         // "BROADCAST" for live video
	MediaProductType string `json:"media_product_type"` // "LIVE"
}

// liveMediaResponse is the paged shape of GET /{ig-user-id}/live_media.
type liveMediaResponse struct {
	Data []LiveMediaSummary `json:"data"`
}

// LiveMediaWithUsage returns the IG user's currently-broadcast live media.
// See https://developers.facebook.com/docs/instagram-platform/instagram-graph-api/reference/ig-user/live_media/.
func (c *Client) LiveMediaWithUsage(ctx context.Context, igUserID, token string) ([]LiveMediaSummary, Usage, error) {
	params := url.Values{}
	params.Set("fields", "id,media_type,media_product_type")
	var out liveMediaResponse
	usage := Usage{}
	if err := c.get(ctx, igUserID+"/live_media", params, token, &out, &usage); err != nil {
		return nil, usage, fmt.Errorf("fetch live media for ig user %s: %w", igUserID, err)
	}
	return out.Data, usage, nil
}

// LiveMedia is LiveMediaWithUsage without the usage surface (test seam).
func (c *Client) LiveMedia(ctx context.Context, igUserID, token string) ([]LiveMediaSummary, error) {
	media, _, err := c.LiveMediaWithUsage(ctx, igUserID, token)
	return media, err
}

// IsLiveBroadcast reports whether the summary is an in-progress live video.
func (m LiveMediaSummary) IsLiveBroadcast() bool {
	return m.MediaType == "BROADCAST" && m.MediaProductType == "LIVE"
}

// Comment is the subset of an IG Comment node read from a live broadcast.
// `from.id` is the Instagram-scoped ID (IGSID) of the commenter; `username`
// their @handle (requires instagram_manage_comments per the IG Comment node
// reference). `parent_id` is set when the comment is a reply.
// See https://developers.facebook.com/docs/instagram-platform/instagram-graph-api/reference/ig-comment/.
type Comment struct {
	ID        string `json:"id"`
	Text      string `json:"text"`
	Timestamp string `json:"timestamp"` // ISO 8601, e.g. 2017-05-19T23:27:28+0000
	From      struct {
		ID       string `json:"id"`
		Username string `json:"username"`
	} `json:"from"`
	User     *string `json:"user"` // present only when the commenter is the IG user itself
	ParentID string  `json:"parent_id"`
	Media    struct {
		ID string `json:"id"`
	} `json:"media"`
}

// commentsResponse is the paged shape of the live-comments edge.
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

// LiveCommentsParams carries the fixed live-comment-poll query.
//
// Per the live-comments reference
// (https://developers.facebook.com/docs/instagram-platform/instagram-graph-api/live-comments/),
// the edge is GET /{ig-user-id}/live_comments with fields id,text,timestamp,
// username,user and pagination via the `comment_id` query parameter, which
// returns the comments made AFTER the comment with that id — an id cursor,
// NOT a `since` datetime (the IG Comment node reference pins `timestamp` as
// read-only metadata; unlike Facebook's live-video/comments edge there is no
// `since` filter: "Comments cannot be filtered by timestamp",
// https://developers.facebook.com/docs/instagram-platform/instagram-graph-api/reference/ig-media/comments).
type LiveCommentsParams struct {
	CommentID string // last-seen comment id; the next fetch returns comments after it
	After     string // optional Graph paging cursor from the previous page
	Limit     string // page size, e.g. "50"
}

// LiveCommentsWithUsage fetches one page of comments on the IG user's live
// broadcast. Only meaningful while the IG user is live: per the IG Comment
// node reference, "Comments on live video IG Media can only be read while the
// IG Media upon which the comment was created is being broadcast"
// (https://developers.facebook.com/docs/instagram-platform/instagram-graph-api/reference/ig-comment/).
func (c *Client) LiveCommentsWithUsage(ctx context.Context, igUserID, token string, p LiveCommentsParams) ([]Comment, string, Usage, error) {
	params := url.Values{}
	params.Set("fields", "id,text,timestamp,username,user{id},parent_id,media{id}")
	if p.CommentID != "" {
		params.Set("comment_id", p.CommentID)
	}
	if p.After != "" {
		params.Set("after", p.After)
	}
	if p.Limit != "" {
		params.Set("limit", p.Limit)
	}
	var out commentsResponse
	usage := Usage{}
	if err := c.get(ctx, igUserID+"/live_comments", params, token, &out, &usage); err != nil {
		return nil, "", usage, fmt.Errorf("fetch live comments for ig user %s: %w", igUserID, err)
	}
	return out.Data, out.Paging.Cursors.After, usage, nil
}

// LiveComments is LiveCommentsWithUsage without the usage surface (test seam).
func (c *Client) LiveComments(ctx context.Context, igUserID, token string, p LiveCommentsParams) ([]Comment, string, error) {
	data, after, _, err := c.LiveCommentsWithUsage(ctx, igUserID, token, p)
	return data, after, err
}
