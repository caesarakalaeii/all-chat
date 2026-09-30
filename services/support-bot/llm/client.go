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

package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
)

// ChatClient is the provider-agnostic non-streaming chat interface the agent loop
// depends on.
type ChatClient interface {
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
}

// Config configures the OpenAI-compatible client.
type Config struct {
	BaseURL        string        // e.g. http://localhost:8000 (no trailing /v1)
	APIKey         string        // optional; empty => no Authorization header
	Model          string        // default model when a request leaves Model empty
	MaxRetries     int           // additional attempts after the first
	RetryBaseDelay time.Duration // base backoff
	MaxBackoff     time.Duration // backoff cap
	RequestTimeout time.Duration // per-attempt deadline
	ConnectTimeout time.Duration // TCP connect deadline
}

// withDefaults fills unset tuning knobs.
func (c Config) withDefaults() Config {
	if c.MaxRetries == 0 {
		c.MaxRetries = 3
	}
	if c.RetryBaseDelay <= 0 {
		c.RetryBaseDelay = 500 * time.Millisecond
	}
	if c.MaxBackoff <= 0 {
		c.MaxBackoff = 30 * time.Second
	}
	if c.RequestTimeout <= 0 {
		c.RequestTimeout = 120 * time.Second
	}
	if c.ConnectTimeout <= 0 {
		c.ConnectTimeout = 10 * time.Second
	}
	return c
}

// Client is an OpenAI-compatible chat-completions client. It carries no per-request
// mutable state, so a single instance is safe to share across concurrent Chat calls.
type Client struct {
	cfg  Config
	http *http.Client
	log  *zap.Logger
	url  string
}

// New builds a Client. It fails if an API key is set but contains non-printable
// characters (header-injection guard).
func New(cfg Config, log *zap.Logger) (*Client, error) {
	cfg = cfg.withDefaults()
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("llm: BaseURL is required")
	}
	if cfg.APIKey != "" && !isPrintableASCII(cfg.APIKey) {
		return nil, fmt.Errorf("llm: API key contains non-printable characters")
	}
	transport := &http.Transport{
		DialContext:       (&net.Dialer{Timeout: cfg.ConnectTimeout}).DialContext,
		MaxIdleConns:      100,
		IdleConnTimeout:   90 * time.Second,
		ForceAttemptHTTP2: true,
	}
	hc := &http.Client{
		Transport: transport,
		// Disable redirect following (redirect::Policy::none equivalent): an LLM POST
		// must never chase a redirect (SSRF / credential-leak risk).
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	return &Client{
		cfg:  cfg,
		http: hc,
		log:  log,
		url:  strings.TrimRight(cfg.BaseURL, "/") + "/v1/chat/completions",
	}, nil
}

// Chat performs a single (retried) chat-completions round-trip.
func (c *Client) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	if req.Model == "" {
		req.Model = c.cfg.Model
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, &APIError{Kind: KindOther, Message: "marshal request: " + err.Error()}
	}

	for attempt := 0; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err // caller cancelled / overall deadline hit
		}

		resp, status, retryAfter, masked, doErr := c.doOnce(ctx, body)
		if doErr != nil {
			if attempt < c.cfg.MaxRetries {
				c.log.Warn("llm transport error, retrying",
					zap.Int("attempt", attempt), zap.Error(doErr))
				if serr := c.sleep(ctx, backoff(attempt, c.cfg.RetryBaseDelay, c.cfg.MaxBackoff)); serr != nil {
					return nil, serr
				}
				continue
			}
			return nil, &APIError{Kind: KindUnavailable, Message: "transport: " + doErr.Error()}
		}
		if status < 300 {
			return resp, nil
		}
		// error path: masked body is a local, so concurrent Chat calls never cross-talk
		if attempt < c.cfg.MaxRetries && isRetryableStatus(status) {
			d := backoff(attempt, c.cfg.RetryBaseDelay, c.cfg.MaxBackoff)
			if retryAfter > d {
				d = retryAfter
			}
			c.log.Warn("llm http error, retrying",
				zap.Int("attempt", attempt), zap.Int("status", status))
			if serr := c.sleep(ctx, d); serr != nil {
				return nil, serr
			}
			continue
		}
		return nil, mapStatus(status, masked)
	}
}

// doOnce performs a single attempt. On a non-2xx it returns the (credential-masked)
// error body as the 4th value so Chat can build the final APIError from a local — the
// Client holds no per-request state.
func (c *Client) doOnce(parent context.Context, body []byte) (*ChatResponse, int, time.Duration, string, error) {
	ctx, cancel := context.WithTimeout(parent, c.cfg.RequestTimeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return nil, 0, 0, "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	if c.cfg.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	}

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, 0, 0, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 300 {
		var out ChatResponse
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			return nil, 0, 0, "", fmt.Errorf("decode response: %w", err)
		}
		return &out, resp.StatusCode, 0, "", nil
	}

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	masked := maskCredentials(string(raw))
	c.log.Warn("llm http error", zap.Int("status", resp.StatusCode), zap.String("body", truncate(masked, 500)))
	ra, _ := parseRetryAfter(resp.Header)
	return nil, resp.StatusCode, ra, masked, nil
}

func (c *Client) sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func backoff(attempt int, base, max time.Duration) time.Duration {
	d := base << uint(attempt)
	if d <= 0 || d > max {
		d = max
	}
	if j := int64(d) / 3; j > 0 {
		d += time.Duration(rand.Int63n(j + 1))
	}
	if d > max {
		d = max
	}
	return d
}

func parseRetryAfter(h http.Header) (time.Duration, bool) {
	v := h.Get("Retry-After")
	if v == "" {
		return 0, false
	}
	if secs, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && secs >= 0 {
		return cap300(time.Duration(secs) * time.Second), true
	}
	if t, err := http.ParseTime(v); err == nil {
		d := time.Until(t)
		if d < 0 {
			d = 0
		}
		return cap300(d), true
	}
	return 0, false
}

func cap300(d time.Duration) time.Duration {
	if d > 300*time.Second {
		return 300 * time.Second
	}
	return d
}

func isPrintableASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < 0x20 || s[i] > 0x7e {
			return false
		}
	}
	return true
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
