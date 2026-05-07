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
	sevenTVAPIBase           = "https://7tv.io"
	sevenTVResolverTimeout   = 5 * time.Second
	sevenTVEmoteSetIDPattern = `^[0-9a-fA-F]{24}$`
)

var sevenTVIDRegex = regexp.MustCompile(sevenTVEmoteSetIDPattern)

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
//   - bare 24-char hex emote-set ID
//   - URL pointing at /emote-sets/{id}     (e.g. https://7tv.app/emote-sets/abc...)
//   - URL pointing at /users/{user_id}     (resolved to the user's active set)
//   - URL pointing at /users/{platform}/{platform_id} (resolved via connection)
func (r *SevenTVResolver) extractEmoteSetID(ctx context.Context, input string) (string, error) {
	if sevenTVIDRegex.MatchString(input) {
		return strings.ToLower(input), nil
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
			if !sevenTVIDRegex.MatchString(segments[1]) {
				return "", fmt.Errorf("emote-set id in URL must be a 24-char hex string")
			}
			return strings.ToLower(segments[1]), nil
		}

		if len(segments) >= 2 && segments[0] == "users" {
			// /users/{platform}/{id} or /users/{user_id}
			if len(segments) >= 3 {
				platform := segments[1]
				platformID := segments[2]
				return r.resolveUserConnectionToSetID(ctx, platform, platformID)
			}
			return r.resolveUserToSetID(ctx, segments[1])
		}

		return "", fmt.Errorf("URL path %q does not point to a 7TV emote set or user", parsed.Path)
	}

	return "", fmt.Errorf("input %q is not a 24-char emote-set id or a 7tv.app URL", input)
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
	return r.extractActiveSetIDFromUser(ctx, endpoint)
}

func (r *SevenTVResolver) resolveUserConnectionToSetID(ctx context.Context, platform, platformID string) (string, error) {
	endpoint := fmt.Sprintf("%s/v3/users/%s/%s", r.baseURL, platform, platformID)
	return r.extractActiveSetIDFromUser(ctx, endpoint)
}

func (r *SevenTVResolver) extractActiveSetIDFromUser(ctx context.Context, endpoint string) (string, error) {
	body, err := r.getJSON(ctx, endpoint)
	if err != nil {
		return "", fmt.Errorf("failed to fetch 7TV user: %w", err)
	}

	var resp struct {
		EmoteSet struct {
			ID string `json:"id"`
		} `json:"emote_set"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("failed to decode 7TV user response: %w", err)
	}
	if resp.EmoteSet.ID == "" {
		return "", fmt.Errorf("7TV user has no active emote set")
	}
	return resp.EmoteSet.ID, nil
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
