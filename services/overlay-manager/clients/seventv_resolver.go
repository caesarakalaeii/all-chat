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
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"go.uber.org/zap"
)

const (
	sevenTVAPIBase         = "https://7tv.io"
	sevenTVResolverTimeout = 5 * time.Second
	// 7TV uses two ID formats in the wild:
	//   - 24-char lowercase-hex (legacy MongoDB ObjectIDs)
	//   - 26-char Crockford-base32 ULIDs (current scheme, e.g. 01K0BT1KXDYA24WQJD80CRZC75)
	// Both must be accepted so we can resolve user-pasted links and IDs.
	sevenTVHexIDPattern  = `^[0-9a-fA-F]{24}$`
	sevenTVULIDPattern   = `^[0-9A-HJKMNP-TV-Z]{26}$`
	sevenTVUserIDPattern = sevenTVULIDPattern // user IDs are ULIDs
)

var (
	sevenTVHexIDRegex  = regexp.MustCompile(sevenTVHexIDPattern)
	sevenTVULIDRegex   = regexp.MustCompile(sevenTVULIDPattern)
	sevenTVUserIDRegex = regexp.MustCompile(sevenTVUserIDPattern)
)

// sevenTVKnownPlatforms is the set of platform connection slugs that 7TV
// exposes under /v3/users/{platform}/{platform_id}. Anything outside this set
// is treated as a 7TV user ID instead (so /users/{ulid}/emote-sets and
// /users/{ulid}/... paths don't get mis-routed as platform lookups).
var sevenTVKnownPlatforms = map[string]struct{}{
	"twitch":  {},
	"youtube": {},
	"kick":    {},
	"discord": {},
}

func isSevenTVID(s string) bool {
	return sevenTVHexIDRegex.MatchString(s) || sevenTVULIDRegex.MatchString(s)
}

// SevenTVResolver validates and resolves user-supplied 7TV identifiers (raw
// emote-set IDs, 7tv.app emote-set URLs, or 7tv.app user profile URLs) to a
// canonical emote-set ID by validating against the 7TV API.
type SevenTVResolver struct {
	baseURL    string
	httpClient *http.Client
	logger     *zap.Logger
}

// NewSevenTVResolver constructs a resolver pointed at the public 7TV API.
func NewSevenTVResolver(logger *zap.Logger) *SevenTVResolver {
	return &SevenTVResolver{
		baseURL:    sevenTVAPIBase,
		httpClient: &http.Client{Timeout: sevenTVResolverTimeout},
		logger:     logger.With(zap.String("component", "seventv-resolver")),
	}
}

// ResolvedSet describes a successful resolution.
type ResolvedSet struct {
	EmoteSetID string `json:"emote_set_id"`
	Name       string `json:"name,omitempty"`
	EmoteCount int    `json:"emote_count,omitempty"`
}

// Resolve normalizes the input and returns the canonical emote-set descriptor.
// Empty input is valid and resolves to an empty ResolvedSet (caller treats this
// as "clear the override").
func (r *SevenTVResolver) Resolve(ctx context.Context, input string) (ResolvedSet, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return ResolvedSet{}, nil
	}

	setID, err := r.extractEmoteSetID(ctx, trimmed)
	if err != nil {
		return ResolvedSet{}, err
	}

	return r.fetchEmoteSet(ctx, setID)
}

// extractEmoteSetID parses the user input. Supported forms:
//   - bare 24-char hex or 26-char ULID emote-set ID
//   - URL pointing at /emote-sets/{id}                  (e.g. https://7tv.app/emote-sets/01K0...)
//   - URL pointing at /users/{user_id}                  (resolved to the user's active set)
//   - URL pointing at /users/{user_id}/emote-sets[/...] (same, the page the 7TV UI links to)
//   - URL pointing at /users/{platform}/{platform_id}   (resolved via connection)
func (r *SevenTVResolver) extractEmoteSetID(ctx context.Context, input string) (string, error) {
	if isSevenTVID(input) {
		return normalizeSevenTVID(input), nil
	}

	if strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://") {
		parsed, err := url.Parse(input)
		if err != nil {
			return "", fmt.Errorf("invalid URL: %w", err)
		}
		host := strings.ToLower(parsed.Host)
		if host != "7tv.app" && host != "www.7tv.app" && host != "7tv.io" {
			return "", fmt.Errorf("unsupported host %q (expected 7tv.app)", parsed.Host)
		}

		segments := splitPath(parsed.Path)
		if len(segments) >= 2 && segments[0] == "emote-sets" {
			if !isSevenTVID(segments[1]) {
				return "", fmt.Errorf("emote-set id in URL must be a 24-char hex or 26-char ULID")
			}
			return normalizeSevenTVID(segments[1]), nil
		}

		if len(segments) >= 2 && segments[0] == "users" {
			second := segments[1]
			// Disambiguate /users/{user_id}[/...] from /users/{platform}/{id}.
			// 7TV user IDs are 26-char ULIDs; platform slugs are short
			// lowercase keywords (twitch, youtube, kick, discord). The
			// 7TV UI links streamers' emote-set pages as
			// /users/{user_id}/emote-sets — the trailing segment is part
			// of the SPA route, not a platform id.
			if sevenTVUserIDRegex.MatchString(second) {
				return r.resolveUserToSetID(ctx, second)
			}
			if _, ok := sevenTVKnownPlatforms[strings.ToLower(second)]; ok && len(segments) >= 3 {
				return r.resolveUserConnectionToSetID(ctx, strings.ToLower(second), segments[2])
			}
			// Fallback: try as a user ID anyway (some legacy IDs may not match the ULID pattern).
			return r.resolveUserToSetID(ctx, second)
		}

		return "", fmt.Errorf("URL path %q does not point to a 7TV emote set or user", parsed.Path)
	}

	return "", fmt.Errorf("input %q is not a 7TV emote-set id or a 7tv.app URL", input)
}

// normalizeSevenTVID canonicalizes an ID for caching/storage: hex IDs are
// lowercased (they're case-insensitive); ULIDs are uppercased per the
// Crockford-base32 spec, which is how 7TV emits them.
func normalizeSevenTVID(id string) string {
	if sevenTVHexIDRegex.MatchString(id) {
		return strings.ToLower(id)
	}
	return strings.ToUpper(id)
}

func (r *SevenTVResolver) fetchEmoteSet(ctx context.Context, setID string) (ResolvedSet, error) {
	endpoint := fmt.Sprintf("%s/v3/emote-sets/%s", r.baseURL, setID)
	body, err := r.getJSON(ctx, endpoint)
	if err != nil {
		return ResolvedSet{}, fmt.Errorf("failed to fetch emote set: %w", err)
	}

	var resp struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Emotes []struct {
			ID string `json:"id"`
		} `json:"emotes"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return ResolvedSet{}, fmt.Errorf("failed to decode emote set response: %w", err)
	}
	if resp.ID == "" {
		return ResolvedSet{}, fmt.Errorf("7TV returned no emote set for id %s", setID)
	}

	return ResolvedSet{
		EmoteSetID: resp.ID,
		Name:       resp.Name,
		EmoteCount: len(resp.Emotes),
	}, nil
}

func (r *SevenTVResolver) resolveUserToSetID(ctx context.Context, userID string) (string, error) {
	endpoint := fmt.Sprintf("%s/v3/users/%s", r.baseURL, userID)
	body, err := r.getJSON(ctx, endpoint)
	if err != nil {
		return "", fmt.Errorf("failed to fetch 7TV user: %w", err)
	}
	return extractActiveSetIDFromUserBody(body)
}

func (r *SevenTVResolver) resolveUserConnectionToSetID(ctx context.Context, platform, platformID string) (string, error) {
	endpoint := fmt.Sprintf("%s/v3/users/%s/%s", r.baseURL, platform, platformID)
	body, err := r.getJSON(ctx, endpoint)
	if err != nil {
		return "", fmt.Errorf("failed to fetch 7TV user connection: %w", err)
	}
	return extractActiveSetIDFromConnectionBody(body)
}

// extractActiveSetIDFromUserBody parses a /v3/users/{user_id} response, which
// looks roughly like:
//
//	{
//	  "id": "...",
//	  "emote_sets": [{"id": "...", "name": "...", ...}],
//	  "connections": [{"platform":"TWITCH", "emote_set_id":"...", "emote_set":{"id":"..."}}]
//	}
//
// We prefer the active emote_set_id on the first Twitch connection (that's the
// set actually being used in chat), fall back to any connection, then to the
// first owned emote_set. 7TV does not currently include a top-level
// `emote_set` field on this endpoint despite what the older code assumed.
func extractActiveSetIDFromUserBody(body []byte) (string, error) {
	var resp struct {
		EmoteSets []struct {
			ID string `json:"id"`
		} `json:"emote_sets"`
		Connections []struct {
			Platform    string `json:"platform"`
			EmoteSetID  string `json:"emote_set_id"`
			EmoteSetObj struct {
				ID string `json:"id"`
			} `json:"emote_set"`
		} `json:"connections"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("failed to decode 7TV user response: %w", err)
	}

	// Prefer Twitch connection (most users link Twitch first / use it as canonical).
	for _, c := range resp.Connections {
		if strings.EqualFold(c.Platform, "TWITCH") {
			if id := firstNonEmpty(c.EmoteSetID, c.EmoteSetObj.ID); id != "" {
				return id, nil
			}
		}
	}
	// Any other connection with an active set.
	for _, c := range resp.Connections {
		if id := firstNonEmpty(c.EmoteSetID, c.EmoteSetObj.ID); id != "" {
			return id, nil
		}
	}
	// User has no platform connection but owns sets directly.
	for _, s := range resp.EmoteSets {
		if s.ID != "" {
			return s.ID, nil
		}
	}
	return "", fmt.Errorf("7TV user has no active emote set")
}

// extractActiveSetIDFromConnectionBody parses /v3/users/{platform}/{id}, which
// returns a single connection object (not wrapped in a user).
func extractActiveSetIDFromConnectionBody(body []byte) (string, error) {
	var resp struct {
		EmoteSetID  string `json:"emote_set_id"`
		EmoteSetObj struct {
			ID string `json:"id"`
		} `json:"emote_set"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("failed to decode 7TV connection response: %w", err)
	}
	if id := firstNonEmpty(resp.EmoteSetID, resp.EmoteSetObj.ID); id != "" {
		return id, nil
	}
	return "", fmt.Errorf("7TV user has no active emote set on this platform connection")
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func (r *SevenTVResolver) getJSON(ctx context.Context, endpoint string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "All-Chat/1.0")
	req.Header.Set("Accept", "application/json")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("7TV returned 404 (not found)")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("7TV returned status %d", resp.StatusCode)
	}

	const maxBytes = 1 << 20 // 1 MiB — emote sets are small
	limited := http.MaxBytesReader(nil, resp.Body, maxBytes)
	body := make([]byte, 0, 4096)
	buf := make([]byte, 4096)
	for {
		n, err := limited.Read(buf)
		if n > 0 {
			body = append(body, buf[:n]...)
		}
		if err != nil {
			break
		}
	}
	return body, nil
}

func splitPath(path string) []string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	out := parts[:0]
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
