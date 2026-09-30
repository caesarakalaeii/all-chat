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
	"sort"
	"strings"
	"time"

	"github.com/caesar/all-chat/services/support-bot/access"
	"github.com/caesar/all-chat/services/support-bot/grafana"
	"github.com/caesar/all-chat/services/support-bot/tool"
)

// GrafanaLogsTool queries Loki logs via Grafana. Support-mode results are summarized.
type GrafanaLogsTool struct {
	tool.BothModes
	client      *grafana.Client
	datasource  string
	defaultTail time.Duration
}

// NewGrafanaLogsTool builds the Loki tool.
func NewGrafanaLogsTool(c *grafana.Client, datasource string, defaultTail time.Duration) *GrafanaLogsTool {
	if defaultTail <= 0 {
		defaultTail = 15 * time.Minute
	}
	return &GrafanaLogsTool{client: c, datasource: datasource, defaultTail: defaultTail}
}

type grafanaLogsParams struct {
	Query string `json:"query"`
	Since string `json:"since,omitempty"`
	Limit int    `json:"limit,omitempty"`
}

func (t *GrafanaLogsTool) Def() tool.ToolDef {
	return tool.ToolDef{
		Name: "grafana_logs",
		Description: "Query recent service logs via Grafana Loki (LogQL). Example query: {namespace=\"allchat\",app=\"api-gateway\"} |= \"error\". " +
			"In support mode the result is an aggregate (counts + normalized patterns), not raw lines.",
		Parameters: json.RawMessage(`{"type":"object","properties":{` +
			`"query":{"type":"string","description":"LogQL query"},` +
			`"since":{"type":"string","description":"optional look-back window, e.g. 15m (default 15m)"},` +
			`"limit":{"type":"integer","description":"optional max log lines (<=1000)"}},` +
			`"required":["query"]}`),
	}
}

func (t *GrafanaLogsTool) Invoke(ctx context.Context, tctx *tool.ToolCtx, args json.RawMessage) (tool.ToolOutput, error) {
	var p grafanaLogsParams
	if err := json.Unmarshal(args, &p); err != nil {
		return tool.ToolOutput{}, fmt.Errorf("invalid arguments: %w", err)
	}
	if strings.TrimSpace(p.Query) == "" {
		return tool.ToolOutput{}, fmt.Errorf("query is required")
	}
	since := t.defaultTail
	if p.Since != "" {
		d, err := time.ParseDuration(p.Since)
		if err != nil || d <= 0 {
			return tool.ToolOutput{}, fmt.Errorf("since must be a duration like 15m")
		}
		since = d
	}
	lines, err := t.client.QueryLoki(ctx, t.datasource, p.Query, since, p.Limit)
	if err != nil {
		return tool.ToolOutput{}, err
	}
	if len(lines) == 0 {
		return tool.ToolOutput{Content: "(no log lines matched)"}, nil
	}
	joined := strings.Join(lines, "\n")
	if tctx.Mode == access.ModeSupport && tctx.Redactor != nil {
		return tool.ToolOutput{Content: tctx.Redactor.SummarizeLogs(joined, 8)}, nil
	}
	if len(joined) > maxKubeOutput {
		joined = joined[:maxKubeOutput] + "\n... (truncated)"
	}
	return tool.ToolOutput{Content: joined}, nil
}

// GrafanaMetricsTool queries Prometheus metrics via Grafana.
type GrafanaMetricsTool struct {
	tool.BothModes
	client     *grafana.Client
	datasource string
}

// NewGrafanaMetricsTool builds the Prometheus tool.
func NewGrafanaMetricsTool(c *grafana.Client, datasource string) *GrafanaMetricsTool {
	return &GrafanaMetricsTool{client: c, datasource: datasource}
}

type grafanaMetricsParams struct {
	Query string `json:"query"`
}

func (t *GrafanaMetricsTool) Def() tool.ToolDef {
	return tool.ToolDef{
		Name:        "grafana_metrics",
		Description: "Run an instant PromQL query via Grafana Prometheus, e.g. sum by (pod) (kube_pod_container_status_restarts_total{namespace=\"allchat\"}).",
		Parameters: json.RawMessage(`{"type":"object","properties":{` +
			`"query":{"type":"string","description":"instant PromQL query"}},` +
			`"required":["query"]}`),
	}
}

func (t *GrafanaMetricsTool) Invoke(ctx context.Context, tctx *tool.ToolCtx, args json.RawMessage) (tool.ToolOutput, error) {
	var p grafanaMetricsParams
	if err := json.Unmarshal(args, &p); err != nil {
		return tool.ToolOutput{}, fmt.Errorf("invalid arguments: %w", err)
	}
	if strings.TrimSpace(p.Query) == "" {
		return tool.ToolOutput{}, fmt.Errorf("query is required")
	}
	samples, err := t.client.QueryPrometheus(ctx, t.datasource, p.Query)
	if err != nil {
		return tool.ToolOutput{}, err
	}
	if len(samples) == 0 {
		return tool.ToolOutput{Content: "(no samples)"}, nil
	}
	var b strings.Builder
	for i, s := range samples {
		if i >= 100 {
			fmt.Fprintf(&b, "... and %d more series\n", len(samples)-i)
			break
		}
		b.WriteString(formatMetric(s))
		b.WriteString("\n")
	}
	return tool.ToolOutput{Content: strings.TrimRight(b.String(), "\n")}, nil
}

func formatMetric(s grafana.PromSample) string {
	name := s.Metric["__name__"]
	keys := make([]string, 0, len(s.Metric))
	for k := range s.Metric {
		if k == "__name__" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%q", k, s.Metric[k]))
	}
	return fmt.Sprintf("%s{%s} = %s", name, strings.Join(parts, ","), s.Value)
}
