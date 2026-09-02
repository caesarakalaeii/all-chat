#!/usr/bin/env python3
"""Lint machine-written comment slop, with a shrink-only suppressions ratchet.

A comment earns its place by saying *why*. A comment that restates the code is
deleted rather than reworded — the reader already has the code.

Four rules, run over every tracked `*.go`, `*.ts`, `*.tsx`, `*.py` file:
`banner`, `step`, `filler`, `restate`. See CONTRIBUTING.md for the policy and
`RULES` below for the exact definitions.

Standard library only, so it runs in the flake dev shell and in CI without an
install step (the same constraint scripts/check-plugin-parity.py works under):

    python3 scripts/comment-lint.py
    python3 scripts/comment-lint.py --selftest
"""

from __future__ import annotations

import sys

# ---------------------------------------------------------------------------
# Selftest
#
# Every case below is quoted from, or directly modelled on, the examples pinned
# in issue #822. It asserts BOTH directions, because a linter that only proves
# it flags things is a linter that flags everything: each `kept` case is a
# comment the repo wants to keep, and one of them (`feedAnchor.ts`-style prose,
# the license header, a godoc line) is why the corresponding exemption exists.
#
# The fixtures are inline on purpose — no framework, no fixtures directory.
# ---------------------------------------------------------------------------

# (rule, filename, source). The source is a whole file: `restate` needs the
# following line of code, the license-header exemption needs the top of a file,
# and the block exemption needs neighbouring comment lines.
FLAGGED: tuple[tuple[str, str, str], ...] = (
    (
        "banner",
        "quota.go",
        "package handlers\n\n// =============== CIRCUIT BREAKER VISIBILITY ===============\n",
    ),
    ("banner", "bare.go", "package p\n\n// ------------------------\n"),
    (
        "step",
        "event_flow_test.go",
        "package p\n\nfunc f() {\n\t// Step 8: Verify message doesn't have emotes\n\tg()\n}\n",
    ),
    (
        "step",
        "phase.go",
        "package p\n\nfunc f() {\n\t// Phase 2: drain the queue\n\tg()\n}\n",
    ),
    (
        "step",
        "numbered.ts",
        "function f() {\n  // 3. Publish the result\n  h();\n}\n",
    ),
    (
        "filler",
        "helper.go",
        "package p\n\n// helper function to build the URL\nfunc build() {}\n",
    ),
    (
        "filler",
        "clarity.ts",
        "function f() {\n  // split out for clarity\n  h();\n}\n",
    ),
    (
        "restate",
        "retry.ts",
        "function f() {\n  // set the retry limit\n  retryLimit = 5;\n}\n",
    ),
    (
        "restate",
        "proxy.go",
        "package p\n\nfunc f() {\n\t// Get the full request path\n\tfullPath := c.Request.URL.Path\n}\n",
    ),
    (
        "restate",
        "manager.go",
        "package p\n\nfunc f() {\n\t// Create new timer\n\ttimer := time.NewTimer(d)\n}\n",
    ),
    (
        "restate",
        "signing.py",
        "def f():\n    # Add headers\n    add_headers(req)\n",
    ),
)

LICENSE_HEADER_GO = """// This file is part of All-Chat.
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
"""

# (why this must survive, filename, source)
KEPT: tuple[tuple[str, str, str], ...] = (
    (
        "godoc doc comment restates by construction and has to stay",
        "user.go",
        "package p\n\n// GetUser returns the user\nfunc GetUser() {}\n",
    ),
    (
        "a block of two or more comment lines is prose",
        "block.go",
        "package p\n\nfunc f() {\n\t// Create new timer\n\t// because the old one cannot be reset while armed.\n\ttimer := time.NewTimer(d)\n}\n",
    ),
    (
        "the AGPL header is on every file in the repo",
        "licensed.go",
        LICENSE_HEADER_GO + "\npackage p\n",
    ),
    (
        "a comment carrying a URL is a citation, not slop",
        "url.go",
        "package p\n\nfunc f() {\n\t// Add headers per https://example.com/spec\n\taddHeaders(r)\n}\n",
    ),
    (
        "ponytail: comments are addressed to the ponytail tooling",
        "ponytail.ts",
        "function f() {\n  // ponytail: set the retry limit\n  retryLimit = 5;\n}\n",
    ),
    (
        "a comment giving the reason for a literal is exactly what we want",
        "pusher.ts",
        "function f() {\n  // 5 because Pusher drops the 6th silently\n  retryLimit = 5;\n}\n",
    ),
    (
        "an issue reference makes the comment a pointer to context",
        "issue.go",
        "package p\n\nfunc f() {\n\t// Create new timer (#728)\n\ttimer := time.NewTimer(d)\n}\n",
    ),
    (
        "machine directives are not addressed to a human at all",
        "directive.go",
        "package p\n\n//go:generate stringer -type=Kind\n//nolint:gochecknoglobals // 1. registry\nvar kinds []Kind\n",
    ),
    (
        "a TODO with an owner is tracked work, not commentary",
        "todo.go",
        "package p\n\nfunc f() {\n\t// TODO(caesar): Create new timer\n\ttimer := time.NewTimer(d)\n}\n",
    ),
    (
        "an ADR reference is a pointer to a decision record",
        "adr.ts",
        "function f() {\n  // set the retry limit, ADR-0033\n  retryLimit = 5;\n}\n",
    ),
    (
        "one content word left after stopwords is too weak a signal",
        "single.go",
        "package p\n\nfunc f() {\n\t// the timer\n\ttimer := time.NewTimer(d)\n}\n",
    ),
    (
        "a comment adding a word the code does not have is not a restatement",
        "why.go",
        "package p\n\nfunc f() {\n\t// Create new timer per request, never shared\n\ttimer := time.NewTimer(d)\n}\n",
    ),
)


def selftest() -> int:
    """Assert both directions on the inline fixtures. Returns an exit code."""
    failures: list[str] = []

    for rule, name, source in FLAGGED:
        rules = {v.rule for v in lint_source(name, source.splitlines())}
        if rule not in rules:
            failures.append(f"{name}: expected rule {rule!r}, got {sorted(rules)}")

    for why, name, source in KEPT:
        found = lint_source(name, source.splitlines())
        if found:
            got = ", ".join(f"{v.rule}: {v.text}" for v in found)
            failures.append(f"{name}: expected no violation ({why}), got {got}")

    for line in failures:
        print(f"selftest FAIL {line}", file=sys.stderr)
    if failures:
        print(f"{len(failures)} selftest failure(s)", file=sys.stderr)
        return 1
    print(f"selftest ok: {len(FLAGGED)} flagged, {len(KEPT)} kept")
    return 0


def main(argv: list[str]) -> int:
    if argv == ["--selftest"]:
        return selftest()
    raise NotImplementedError("comment-lint is not implemented yet")


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
