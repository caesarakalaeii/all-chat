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

// Package redact is the code-level leak-prevention layer. In support mode it runs
// over every tool output and over the final answer before it reaches Discord, so a
// leak cannot depend on the model "following instructions". It strips credentials,
// internal hostnames and pod/cluster IPs, and — for log-bearing tools — replaces raw
// log bodies and stack traces with aggregated counts and normalized patterns, since a
// whole log line or stack frame is itself the leak and would survive token redaction.
package redact

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// rule is an ordered (pattern, replacement) redaction step.
type rule struct {
	re   *regexp.Regexp
	repl string
}

// Redactor applies an ordered set of secret and topology redactions.
type Redactor struct {
	rules []rule
}

// placeholder is emitted in place of a redacted secret.
const placeholder = "[REDACTED]"

// NewRedactor builds the default redactor. Order matters: multi-line PEM blocks and
// keyword=value pairs are collapsed before the standalone token patterns run, and
// hostnames/IPs are neutralized last.
func NewRedactor() *Redactor {
	rules := []rule{
		// PEM private key blocks (multi-line).
		{regexp.MustCompile(`(?s)-----BEGIN [A-Z ]*PRIVATE KEY-----.*?-----END [A-Z ]*PRIVATE KEY-----`), placeholder},
		// bearer <token>
		{regexp.MustCompile(`(?i)\b(bearer)\s+[A-Za-z0-9._~+/=-]{12,}`), "$1 " + placeholder},
		// x-api-key: <token> / authorization headers
		{regexp.MustCompile(`(?i)\b(x-api-key)\s*[:=]\s*[A-Za-z0-9._~+/=-]{8,}`), "$1: " + placeholder},
		// keyword = value (quoted or bare)
		{regexp.MustCompile(`(?i)\b(api[_-]?key|access[_-]?token|token|password|passwd|secret|client[_-]?secret|credential|private[_-]?key|aws_secret_access_key)\b\s*[:=]\s*("[^"]*"|'[^']*'|` + "`[^`]*`" + `|[^\s"'` + "`" + `,;]+)`), "$1=" + placeholder},
		// GitHub tokens
		{regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{20,}\b`), placeholder},
		{regexp.MustCompile(`\bgithub_pat_[A-Za-z0-9_]{20,}\b`), placeholder},
		// OpenAI / Anthropic keys
		{regexp.MustCompile(`\bsk-ant-[A-Za-z0-9_-]{20,}\b`), placeholder},
		{regexp.MustCompile(`\bsk-(?:proj-)?[A-Za-z0-9_-]{20,}\b`), placeholder},
		// Slack tokens
		{regexp.MustCompile(`\bxox[baprs]-[A-Za-z0-9-]{10,}\b`), placeholder},
		{regexp.MustCompile(`\bxapp-[A-Za-z0-9-]{10,}\b`), placeholder},
		// Grafana service-account tokens
		{regexp.MustCompile(`\bglsa_[A-Za-z0-9_]{20,}\b`), placeholder},
		{regexp.MustCompile(`\bglc_[A-Za-z0-9+/_=-]{20,}\b`), placeholder},
		// AWS access key id
		{regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`), placeholder},
		// Discord bot token (three base64url segments)
		{regexp.MustCompile(`\b[MNO][A-Za-z0-9_-]{23,}\.[A-Za-z0-9_-]{6,}\.[A-Za-z0-9_-]{27,}\b`), placeholder},
		// JWT
		{regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\b`), placeholder},
		// Kubernetes in-cluster DNS names
		{regexp.MustCompile(`\b[a-z0-9]([-a-z0-9.]*[a-z0-9])?\.svc\.cluster\.local\b`), "[internal-host]"},
		// Private / loopback / CGNAT IPv4 (pod & node IPs)
		{regexp.MustCompile(`\b(?:10\.\d{1,3}\.\d{1,3}\.\d{1,3}|127\.\d{1,3}\.\d{1,3}\.\d{1,3}|192\.168\.\d{1,3}\.\d{1,3}|172\.(?:1[6-9]|2\d|3[01])\.\d{1,3}\.\d{1,3}|100\.(?:6[4-9]|[7-9]\d|1[01]\d|12[0-7])\.\d{1,3}\.\d{1,3})\b`), "[internal-ip]"},
	}
	return &Redactor{rules: rules}
}

// Redact strips stack traces, then applies every credential/topology rule in order.
// It runs on every support-mode tool output and on the final answer, so a leaked stack
// frame or secret cannot depend on the model omitting it.
func (r *Redactor) Redact(s string) string {
	s = stripStackTraces(s)
	for _, rl := range r.rules {
		s = rl.re.ReplaceAllString(s, rl.repl)
	}
	return s
}

// stackFrameRe matches a single line that is part of a stack trace / traceback across
// the common runtimes (Go, Python, Node/JS, Java, Rust).
var stackFrameRe = regexp.MustCompile(`^\s*(?:panic:|goroutine \d+ \[|at [\w.$<>]+\(|File "[^"]+", line \d+|Traceback \(most recent call last\)|[\w./-]+\.(?:go|py|js|ts|rb|java|rs|kt|cpp|cc):\d+|/[^ ]+\+0x[0-9a-fA-F]+|\+0x[0-9a-fA-F]+)`)

// frameArgRe matches a Go frame's function-call line whose arguments are raw hex
// addresses (e.g. `main.handler(0xc0000b6000, 0x1)`) — distinctive of a stack trace and
// very unlikely in normal code/log text.
var frameArgRe = regexp.MustCompile(`\(0x[0-9a-fA-F]`)

// stripStackTraces removes contiguous stack-frame lines, collapsing each run into a
// single "[stack trace omitted]" marker. A whole frame is itself a leak (it reveals
// file paths and internal structure) that per-token redaction cannot neutralize.
func stripStackTraces(s string) string {
	if !HasStackTrace(s) {
		return s
	}
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	omitted := false
	for _, ln := range lines {
		if stackFrameRe.MatchString(ln) || frameArgRe.MatchString(ln) {
			if !omitted {
				out = append(out, "[stack trace omitted]")
				omitted = true
			}
			continue
		}
		omitted = false
		out = append(out, ln)
	}
	return strings.Join(out, "\n")
}

var (
	stackMarkers = []*regexp.Regexp{
		regexp.MustCompile(`(?m)^panic:`),
		regexp.MustCompile(`goroutine \d+ \[`),
		regexp.MustCompile(`(?m)^\s+at [\w.$<>]+\(`),
		regexp.MustCompile(`File "[^"]+", line \d+`),
		regexp.MustCompile(`Traceback \(most recent call last\)`),
		regexp.MustCompile(`(?m)\n\s+at .+:\d+`),
	}
	levelRe = regexp.MustCompile(`(?i)\b(fatal|panic|error|err|warn(?:ing)?|info|debug|trace)\b`)

	normSteps = []rule{
		{regexp.MustCompile(`\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:?\d{2})?`), "<ts>"},
		{regexp.MustCompile(`\d{2}:\d{2}:\d{2}(?:\.\d+)?`), "<ts>"},
		{regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`), "<uuid>"},
		{regexp.MustCompile(`0x[0-9a-fA-F]+`), "<hex>"},
		{regexp.MustCompile(`\b[0-9a-fA-F]{16,}\b`), "<hex>"},
		{regexp.MustCompile(`\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`), "<ip>"},
		{regexp.MustCompile(`\b\d+\b`), "<n>"},
		{regexp.MustCompile(`\s+`), " "},
	}
)

// HasStackTrace reports whether s looks like it contains a stack trace / traceback.
func HasStackTrace(s string) bool {
	for _, re := range stackMarkers {
		if re.MatchString(s) {
			return true
		}
	}
	return false
}

// SummarizeLogs collapses a raw multi-line log/query result into aggregate counts and
// the most frequent normalized patterns (each redacted). This is what the support-mode
// log tools return in place of raw lines. maxPatterns bounds how many templates are
// listed. If raw is short and contains no stack trace, it is redacted and returned
// verbatim — a two-line answer needn't be summarized.
func (r *Redactor) SummarizeLogs(raw string, maxPatterns int) string {
	lines := splitNonEmpty(raw)
	if len(lines) == 0 {
		return "(no output)"
	}
	if len(lines) <= 3 && !HasStackTrace(raw) {
		return r.Redact(strings.TrimSpace(raw))
	}
	if maxPatterns <= 0 {
		maxPatterns = 8
	}

	levels := map[string]int{}
	tmplCount := map[string]int{}
	var tmplOrder []string
	for _, ln := range lines {
		levels[classifyLevel(ln)]++
		t := normalizeLine(ln)
		if t == "" {
			continue
		}
		if _, seen := tmplCount[t]; !seen {
			tmplOrder = append(tmplOrder, t)
		}
		tmplCount[t]++
	}

	var b strings.Builder
	fmt.Fprintf(&b, "[summarized: %d log lines; %s]\n", len(lines), formatLevels(levels))
	b.WriteString("top patterns:\n")

	sort.SliceStable(tmplOrder, func(i, j int) bool {
		return tmplCount[tmplOrder[i]] > tmplCount[tmplOrder[j]]
	})
	limit := maxPatterns
	if limit > len(tmplOrder) {
		limit = len(tmplOrder)
	}
	for _, t := range tmplOrder[:limit] {
		fmt.Fprintf(&b, "  %dx  %s\n", tmplCount[t], r.Redact(t))
	}
	if len(tmplOrder) > limit {
		fmt.Fprintf(&b, "  ... and %d more distinct patterns\n", len(tmplOrder)-limit)
	}
	return strings.TrimRight(b.String(), "\n")
}

func classifyLevel(line string) string {
	if m := levelRe.FindString(line); m != "" {
		u := strings.ToUpper(m)
		switch u {
		case "ERR":
			return "ERROR"
		case "WARNING":
			return "WARN"
		default:
			return u
		}
	}
	return "OTHER"
}

func normalizeLine(line string) string {
	s := line
	for _, st := range normSteps {
		s = st.re.ReplaceAllString(s, st.repl)
	}
	return strings.TrimSpace(s)
}

func formatLevels(levels map[string]int) string {
	order := []string{"FATAL", "PANIC", "ERROR", "WARN", "INFO", "DEBUG", "TRACE", "OTHER"}
	var parts []string
	for _, k := range order {
		if n, ok := levels[k]; ok && n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, k))
		}
	}
	if len(parts) == 0 {
		return "0 classified"
	}
	return strings.Join(parts, ", ")
}

func splitNonEmpty(raw string) []string {
	var out []string
	for _, ln := range strings.Split(raw, "\n") {
		if strings.TrimSpace(ln) != "" {
			out = append(out, ln)
		}
	}
	return out
}
