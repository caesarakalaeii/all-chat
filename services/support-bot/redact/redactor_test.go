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

package redact

import (
	"strings"
	"testing"
)

func TestRedactSecrets(t *testing.T) {
	r := NewRedactor()
	cases := []string{
		"token=ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789",
		"key: sk-ant-api03-abcdefghijklmnopqrstuvwxyz",
		"Authorization: Bearer eyJhbGciabcdefghijklmnop.qrstuvwx.yz1234567890",
		"password = hunter2secretvalue",
		"AKIAIOSFODNN7EXAMPLE",
	}
	for _, in := range cases {
		got := r.Redact(in)
		if !strings.Contains(got, "[REDACTED]") {
			t.Errorf("expected redaction for %q, got %q", in, got)
		}
	}
}

func TestRedactInternalTopology(t *testing.T) {
	r := NewRedactor()
	got := r.Redact("connecting to allchat-cluster-rw.allchat.svc.cluster.local at 10.42.3.9")
	if strings.Contains(got, "svc.cluster.local") {
		t.Errorf("internal host survived: %q", got)
	}
	if strings.Contains(got, "10.42.3.9") {
		t.Errorf("internal ip survived: %q", got)
	}
	if !strings.Contains(got, "[internal-host]") || !strings.Contains(got, "[internal-ip]") {
		t.Errorf("expected topology placeholders, got %q", got)
	}
}

func TestRedactLeavesPublicIP(t *testing.T) {
	r := NewRedactor()
	// A public IP should not be treated as internal.
	got := r.Redact("upstream 8.8.8.8 responded")
	if !strings.Contains(got, "8.8.8.8") {
		t.Errorf("public IP wrongly redacted: %q", got)
	}
}

func TestHasStackTrace(t *testing.T) {
	if !HasStackTrace("panic: runtime error\ngoroutine 1 [running]:") {
		t.Error("go panic not detected")
	}
	if !HasStackTrace("Traceback (most recent call last):\n  File \"x.py\", line 3") {
		t.Error("python traceback not detected")
	}
	if HasStackTrace("just a normal sentence") {
		t.Error("false positive on normal text")
	}
}

func TestSummarizeLogsAggregates(t *testing.T) {
	r := NewRedactor()
	var b strings.Builder
	for i := 0; i < 20; i++ {
		b.WriteString("2026-07-10T12:00:00Z ERROR connection timeout to db-host:5432 attempt 7\n")
	}
	for i := 0; i < 5; i++ {
		b.WriteString("2026-07-10T12:00:00Z WARN retrying request 12\n")
	}
	got := r.SummarizeLogs(b.String(), 8)
	if !strings.Contains(got, "summarized: 25 log lines") {
		t.Errorf("missing line count: %q", got)
	}
	if !strings.Contains(got, "ERROR") || !strings.Contains(got, "WARN") {
		t.Errorf("missing level breakdown: %q", got)
	}
	// Raw values must be normalized away, so the specific numbers should not appear.
	if strings.Contains(got, "attempt 7") {
		t.Errorf("raw value leaked into summary: %q", got)
	}
}

func TestRedactStripsStackTraces(t *testing.T) {
	r := NewRedactor()
	in := "handling request\n" +
		"panic: runtime error: index out of range\n" +
		"goroutine 42 [running]:\n" +
		"main.handler(0xc0000b6000, 0x1)\n" +
		"\t/app/services/api-gateway/main.go:88 +0x1d\n" +
		"done"
	got := r.Redact(in)
	for _, leak := range []string{"main.go:88", "goroutine 42", "0xc0000b6000", "runtime error"} {
		if strings.Contains(got, leak) {
			t.Fatalf("stack detail %q survived: %q", leak, got)
		}
	}
	if !strings.Contains(got, "[stack trace omitted]") {
		t.Fatalf("expected stack-omitted marker: %q", got)
	}
	if !strings.Contains(got, "handling request") || !strings.Contains(got, "done") {
		t.Fatalf("surrounding non-stack lines should survive: %q", got)
	}
}

func TestSummarizeLogsShortPassthrough(t *testing.T) {
	r := NewRedactor()
	got := r.SummarizeLogs("just one line", 8)
	if got != "just one line" {
		t.Errorf("short non-stack log should pass through, got %q", got)
	}
}
