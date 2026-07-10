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

// Package grafana is a thin client over the Grafana datasource-proxy API for Loki
// (log) and Prometheus (metric) range/instant queries. It replaces the mcp-grafana
// subprocess the former bot shelled out to. It never returns the service-account
// token and reduces upstream error bodies to a capped message.
package grafana

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Client queries Grafana datasources by name via the proxy API.
type Client struct {
	base  string
	token string
	http  *http.Client
	log   *zap.Logger

	mu      sync.Mutex
	dsCache map[string]string // datasource name -> uid
}

// New builds a Grafana client.
func New(baseURL, token string, log *zap.Logger) *Client {
	return &Client{
		base:  strings.TrimRight(baseURL, "/"),
		token: token,
		http: &http.Client{
			Timeout:       20 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
		log:     log,
		dsCache: make(map[string]string),
	}
}

// QueryLoki runs a LogQL range query over the last `since` window and returns the raw
// log lines (newest first), capped at limit. The caller decides whether to summarize.
func (c *Client) QueryLoki(ctx context.Context, datasource, logql string, since time.Duration, limit int) ([]string, error) {
	uid, err := c.resolveDS(ctx, datasource)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 1000 {
		limit = 1000
	}
	end := time.Now()
	start := end.Add(-since)
	q := url.Values{}
	q.Set("query", logql)
	q.Set("start", strconv.FormatInt(start.UnixNano(), 10))
	q.Set("end", strconv.FormatInt(end.UnixNano(), 10))
	q.Set("limit", strconv.Itoa(limit))
	q.Set("direction", "backward")

	raw, err := c.get(ctx, fmt.Sprintf("/api/datasources/proxy/uid/%s/loki/api/v1/query_range?%s", uid, q.Encode()))
	if err != nil {
		return nil, err
	}

	var resp struct {
		Data struct {
			Result []struct {
				Values [][]string `json:"values"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("decode loki response: %w", err)
	}
	var lines []string
	for _, stream := range resp.Data.Result {
		for _, v := range stream.Values {
			if len(v) == 2 {
				lines = append(lines, v[1])
			}
		}
	}
	return lines, nil
}

// PromSample is a single Prometheus result series value.
type PromSample struct {
	Metric map[string]string
	Value  string
}

// QueryPrometheus runs an instant PromQL query and returns the result samples.
func (c *Client) QueryPrometheus(ctx context.Context, datasource, promql string) ([]PromSample, error) {
	uid, err := c.resolveDS(ctx, datasource)
	if err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Set("query", promql)
	raw, err := c.get(ctx, fmt.Sprintf("/api/datasources/proxy/uid/%s/api/v1/query?%s", uid, q.Encode()))
	if err != nil {
		return nil, err
	}

	var resp struct {
		Data struct {
			Result []struct {
				Metric map[string]string `json:"metric"`
				Value  []json.RawMessage `json:"value"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("decode prometheus response: %w", err)
	}
	var out []PromSample
	for _, r := range resp.Data.Result {
		val := ""
		if len(r.Value) == 2 {
			// value is [<unix ts>, "<sample>"]; the sample is a quoted string.
			_ = json.Unmarshal(r.Value[1], &val)
		}
		out = append(out, PromSample{Metric: r.Metric, Value: val})
	}
	return out, nil
}

// resolveDS looks up (and caches) a datasource UID by name.
func (c *Client) resolveDS(ctx context.Context, name string) (string, error) {
	c.mu.Lock()
	if uid, ok := c.dsCache[name]; ok {
		c.mu.Unlock()
		return uid, nil
	}
	c.mu.Unlock()

	raw, err := c.get(ctx, "/api/datasources/name/"+url.PathEscape(name))
	if err != nil {
		return "", fmt.Errorf("resolve datasource %q: %w", name, err)
	}
	var ds struct {
		UID string `json:"uid"`
	}
	if err := json.Unmarshal(raw, &ds); err != nil {
		return "", err
	}
	if ds.UID == "" {
		return "", fmt.Errorf("datasource %q has no uid", name)
	}
	c.mu.Lock()
	c.dsCache[name] = ds.UID
	c.mu.Unlock()
	return ds.UID, nil
}

// APIError is a Grafana API error reduced to status + capped body.
type APIError struct {
	Status  int
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("grafana api error (status %d): %s", e.Status, e.Message)
}

func (c *Client) get(ctx context.Context, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "allchat-support-bot")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(raw))
		if len(msg) > 300 {
			msg = msg[:300]
		}
		return nil, &APIError{Status: resp.StatusCode, Message: msg}
	}
	return raw, nil
}
