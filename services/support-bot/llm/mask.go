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

import "regexp"

// maskRules mask credentials in an upstream error body before it is logged. Kept
// self-contained (no dependency on the redact package) so the LLM client is a leaf.
var maskRules = []struct {
	re   *regexp.Regexp
	repl string
}{
	{regexp.MustCompile(`(?i)\b(bearer)\s+[A-Za-z0-9._~+/=-]{8,}`), "$1 [REDACTED]"},
	{regexp.MustCompile(`(?i)\b(x-api-key)\s*[:=]\s*[A-Za-z0-9._~+/=-]{6,}`), "$1: [REDACTED]"},
	{regexp.MustCompile(`\bsk-ant-[A-Za-z0-9_-]{10,}`), "[REDACTED]"},
	{regexp.MustCompile(`\bsk-(?:proj-)?[A-Za-z0-9_-]{10,}`), "[REDACTED]"},
	{regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{20,}`), "[REDACTED]"},
	{regexp.MustCompile(`(?i)\b(api[_-]?key|token|password|secret)\b\s*[:=]\s*("[^"]*"|'[^']*'|[^\s"',;]+)`), "$1=[REDACTED]"},
}

// maskCredentials redacts common credential shapes from an error body.
func maskCredentials(s string) string {
	for _, r := range maskRules {
		s = r.re.ReplaceAllString(s, r.repl)
	}
	return s
}
