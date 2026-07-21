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
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// ErrNotFound signals that a provider has no emotes for the requested channel — an HTTP
// 404 / unknown-channel response. This is the NORMAL case for most channels on BTTV and
// FFZ (few streamers have those sets) and for channels without a 7TV set, so callers
// classify it as a benign "not_found" miss rather than a real API failure. Wrap it with
// %w so callers can detect it via errors.Is.
var ErrNotFound = errors.New("emotes not found for channel")

// ErrRateLimited signals that a provider throttled us (HTTP 429). This is a transient,
// provider-wide condition, not a per-channel fact: retrying the same request only keeps
// the provider throttling, so the handler opens a short cooldown for the provider and
// skips upstream fetches until it expires. Detect with errors.Is(err, ErrRateLimited);
// the provider's Retry-After hint is available via errors.As with *RateLimitedError.
var ErrRateLimited = errors.New("provider rate limited")

// RateLimitedError wraps ErrRateLimited and carries the provider's Retry-After hint.
// RetryAfter is zero when the provider sent no (or an unparseable) header.
type RateLimitedError struct {
	Provider   string
	RetryAfter time.Duration
}

func (e *RateLimitedError) Error() string {
	if e.RetryAfter > 0 {
		return fmt.Sprintf("%s: rate limited (429), retry after %s", e.Provider, e.RetryAfter)
	}
	return fmt.Sprintf("%s: rate limited (429)", e.Provider)
}

func (e *RateLimitedError) Unwrap() error { return ErrRateLimited }

// rateLimited builds the error for a 429 response, honoring a numeric Retry-After
// header when present (the delta-seconds form; the rare HTTP-date form is ignored).
func rateLimited(provider string, resp *http.Response) error {
	var retryAfter time.Duration
	if s := resp.Header.Get("Retry-After"); s != "" {
		if secs, err := strconv.Atoi(s); err == nil && secs > 0 {
			retryAfter = time.Duration(secs) * time.Second
		}
	}
	return &RateLimitedError{Provider: provider, RetryAfter: retryAfter}
}
