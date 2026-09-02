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

import json
import re
import subprocess
import sys
from collections import Counter
from pathlib import Path
from typing import NamedTuple

REPO = Path(__file__).resolve().parent.parent
SUPPRESSIONS = REPO / ".comment-lint.suppressions.json"

LINTED_SUFFIXES = (".go", ".ts", ".tsx", ".py")
EXCLUDED_DIRS = ("vendor", "node_modules")

# Only LINE comments are linted. Block comments (`/* */`, `"""`) are prose by
# construction and are where the repo's genuine why-docs live — feedAnchor.ts is
# entirely `/** */` and must never appear in the suppressions file. Restricting
# the scan this way also sidesteps CSS-in-string-literal false positives in
# frontend/src/lib/theme-marketplace/bundled-themes.generated.ts, whose banners
# live inside a Go/TS string and are not comments at all.
COMMENT_PREFIX = {".go": "//", ".ts": "//", ".tsx": "//", ".py": "#"}

# ---------------------------------------------------------------------------
# Exemptions. These are checked before any rule: without them the linter
# deletes the good comments.
# ---------------------------------------------------------------------------

# A citation, a tracked issue, a decision record or a note to the ponytail
# tooling is a pointer to context that is not in the code.
CONTEXT_MARKERS = re.compile(r"https?://|#\d|ADR-|ponytail:", re.IGNORECASE)

# Addressed to a compiler or a linter, not to a human reader.
MACHINE_DIRECTIVES = (
    "//go:",
    "// +build",
    "//nolint",
    "// eslint-",
    "// @ts-",
    "// prettier-",
    "// istanbul ",
    "# noqa",
    "# type:",
    "# pylint",
)

# An unowned TODO is slop; an owned one is tracked work.
OWNED_MARKER = re.compile(r"\b(TODO|FIXME|XXX)\(")

LICENSE_FIRST_LINE = "This file is part of All-Chat."

# ---------------------------------------------------------------------------
# Rules
# ---------------------------------------------------------------------------

# Decoration: six or more repeats of one character, alone or wrapping a title.
# Deleting the banner keeps the title as an ordinary comment when it says
# something the code does not, which is why the run is matched rather than the
# whole line.
BANNER = re.compile(r"([=\-*#_~])\1{5,}")

# `Step 3`, `Phase 2`, `4.`, `5)` as the leading token.
STEP = re.compile(r"^(?:step|phase)\s+\d+\b|^\d+[.)](?:\s|$)", re.IGNORECASE)

# Pinned list. Growing it is a policy change, not a bug fix: every addition
# retroactively reddens files, so it belongs in a reviewed commit of its own.
FILLER = (
    "helper function to",
    "utility function",
    "as mentioned above",
    "this function is responsible for",
    "this function will",
    "for clarity",
    "we need to",
    "note that we",
    "magic number",
)

# Dropped before comparing a comment against the code below it: these words
# carry no information about what the line does, so leaving them in would let
# `// the timer` escape by "the" not appearing in `timer`.
STOPWORDS = frozenset(
    "a an the this that these those to for of in on at by is are was be "
    "will we it its if then and or not new".split()
)

# Verbs that name the assignment or call the line already performs, so they add
# nothing even though they do not appear in the identifier. `retryLimit = 5` IS
# the set; `timer := time.NewTimer(d)` IS the create. Without these, the two
# examples issue #822 pins as flagged (`// set the retry limit`, `// Get the
# full request path`) escape on their leading verb alone — which would make the
# rule trivially avoidable by prefixing any restatement with "Get".
#
# Kept separate from STOPWORDS because that list is pinned by the issue and this
# one is a judgement call that a later reader may want to revisit on its own.
SYNTAX_VERBS = frozenset("get set create build make return call add".split())

# Splits on non-alphanumerics (which covers snake_case and `.`) and on a
# camelCase boundary. Applied BEFORE lowercasing — lowercase first and the case
# boundary is gone, so `retryLimit` stays one word and `// set the retry limit`
# escapes.
IDENTIFIER_SPLIT = re.compile(r"[^0-9A-Za-z]+|(?<=[a-z0-9])(?=[A-Z])")


class Violation(NamedTuple):
    path: str
    line: int
    rule: str
    text: str

    def __str__(self) -> str:
        return f"{self.path}:{self.line}: {self.rule}: {self.text}"


class Comment(NamedTuple):
    """A single-line comment, with what the rules need to judge it."""

    line: int
    raw: str  # the whole source line, for directive prefixes
    content: str  # comment text with the marker and whitespace stripped
    in_block: bool  # has a comment line directly above or below
    indented: bool  # the comment is indented, i.e. inside a body and not a
    # top-level declaration doc comment
    next_code: str | None  # next non-blank, non-comment source line


def comment_prefix(path: str) -> str:
    return COMMENT_PREFIX[Path(path).suffix]


def license_header_end(lines: list[str], prefix: str) -> int:
    """Index one past the AGPL header, or 0 when the file does not start with it.

    The header is 17 identical comment lines on every file in the repo. Matching
    it by its first line and then consuming the contiguous comment block is
    cheaper and less brittle than pinning the exact text, which would need
    updating on every copyright-year bump.
    """
    for index, line in enumerate(lines[:3]):
        stripped = line.strip()
        if not stripped.startswith(prefix):
            continue
        if LICENSE_FIRST_LINE not in stripped:
            continue
        end = index
        while end < len(lines) and lines[end].strip().startswith(prefix):
            end += 1
        return end
    return 0


def scan_comments(path: str, lines: list[str]) -> list[Comment]:
    prefix = comment_prefix(path)
    start = license_header_end(lines, prefix)
    is_comment = [line.strip().startswith(prefix) for line in lines]

    comments: list[Comment] = []
    for index in range(start, len(lines)):
        if not is_comment[index]:
            continue
        raw = lines[index]
        above = index > start and is_comment[index - 1]
        below = index + 1 < len(lines) and is_comment[index + 1]

        next_code = None
        for candidate in lines[index + 1 :]:
            stripped = candidate.strip()
            if not stripped or stripped.startswith(prefix):
                continue
            next_code = stripped
            break

        comments.append(
            Comment(
                line=index + 1,
                raw=raw,
                content=raw.strip()[len(prefix) :].strip(),
                in_block=above or below,
                indented=raw[:1].isspace(),
                next_code=next_code,
            )
        )
    return comments


def is_exempt(comment: Comment) -> bool:
    """True when no rule may fire on this comment."""
    stripped = comment.raw.strip()
    if any(stripped.startswith(d) or d in stripped for d in MACHINE_DIRECTIVES):
        return True
    if CONTEXT_MARKERS.search(comment.content):
        return True
    if OWNED_MARKER.search(comment.content):
        return True
    return False


def content_words(text: str) -> list[str]:
    """Lowercase words of `text`, split on identifier boundaries, minus stopwords."""
    words = [w.lower() for w in IDENTIFIER_SPLIT.split(text) if w]
    return [w for w in words if w not in STOPWORDS]


def restates_next_line(comment: Comment) -> bool:
    """True when every content word of the comment is already in the next line.

    Fires only on a lone comment inside a body. A block of two or more comment
    lines is prose, and a declaration doc comment has to restate its subject —
    godoc *requires* `// GetUser returns the user`.
    """
    if comment.in_block or not comment.indented or comment.next_code is None:
        return False
    words = content_words(comment.content)
    if len(words) < 2:
        return False
    code_words = set(content_words(comment.next_code))
    # A syntax verb counts as covered whether or not the identifier spells it
    # out: `retryLimit = 5` IS the set, `timer := time.NewTimer(d)` IS the
    # create. It still counts toward the two-word floor above, so a bare
    # `// create` is left alone.
    return all(word in code_words or word in SYNTAX_VERBS for word in words)


def lint_source(path: str, lines: list[str]) -> list[Violation]:
    violations: list[Violation] = []
    for comment in scan_comments(path, lines):
        if is_exempt(comment):
            continue
        lowered = comment.content.lower()
        if BANNER.search(comment.content):
            rule = "banner"
        elif STEP.match(comment.content):
            rule = "step"
        elif any(phrase in lowered for phrase in FILLER):
            rule = "filler"
        elif restates_next_line(comment):
            rule = "restate"
        else:
            continue
        violations.append(Violation(path, comment.line, rule, comment.content))
    return violations


def tracked_files() -> list[str]:
    listing = subprocess.run(
        ["git", "ls-files", "-z", "--", *(f"*{s}" for s in LINTED_SUFFIXES)],
        cwd=REPO,
        check=True,
        capture_output=True,
        text=True,
    ).stdout
    paths = [p for p in listing.split("\0") if p]
    return sorted(
        p for p in paths if not any(part in EXCLUDED_DIRS for part in p.split("/"))
    )


def lint_repo() -> list[Violation]:
    violations: list[Violation] = []
    for path in tracked_files():
        text = (REPO / path).read_text(encoding="utf-8", errors="replace")
        violations.extend(lint_source(path, text.splitlines()))
    return violations


def load_suppressions() -> dict[str, dict[str, int]]:
    if not SUPPRESSIONS.exists():
        return {}
    return json.loads(SUPPRESSIONS.read_text(encoding="utf-8"))


def counts_by_file(violations: list[Violation]) -> dict[str, Counter[str]]:
    counts: dict[str, Counter[str]] = {}
    for violation in violations:
        counts.setdefault(violation.path, Counter())[violation.rule] += 1
    return counts


def check(violations: list[Violation]) -> int:
    """Report violations against the ratchet. Returns an exit code.

    The ratchet is enforced inside the script, with no diff against `main`: a
    count *below* its allowance is an error naming the lower number, so a fix
    is not silently absorbed and the file cannot drift back up later.
    """
    allowances = load_suppressions()
    actual = counts_by_file(violations)
    errors: list[str] = []

    for violation in violations:
        allowed = allowances.get(violation.path, {}).get(violation.rule, 0)
        if actual[violation.path][violation.rule] > allowed:
            print(violation)

    for path, rules in sorted(allowances.items()):
        for rule, allowed in sorted(rules.items()):
            found = actual.get(path, {}).get(rule, 0)
            if found > allowed:
                errors.append(
                    f"{path}: {rule}: {found} violations, {allowed} suppressed"
                )
            elif found < allowed:
                fix = (
                    f"lower it to {found}"
                    if found
                    else f"remove the {rule!r} entry"
                )
                errors.append(
                    f"{path}: {rule}: suppressions allow {allowed} but only {found} "
                    f"remain — {fix} (the ratchet may only shrink)"
                )

    for path, rules in sorted(actual.items()):
        for rule, found in sorted(rules.items()):
            if rule not in allowances.get(path, {}):
                errors.append(f"{path}: {rule}: {found} new violations")

    for error in errors:
        print(error, file=sys.stderr)
    if errors:
        print(
            f"\n{len(errors)} comment-lint error(s). A comment earns its place by "
            "saying why; see CONTRIBUTING.md.",
            file=sys.stderr,
        )
        return 1
    return 0


def seed() -> int:
    """Rewrite the suppressions file with today's counts. Not run by CI."""
    counts = counts_by_file(lint_repo())
    seeded = {
        path: dict(sorted(rules.items()))
        for path, rules in sorted(counts.items())
    }
    SUPPRESSIONS.write_text(json.dumps(seeded, indent=2) + "\n", encoding="utf-8")
    print(f"seeded {SUPPRESSIONS.name}: {len(seeded)} files")
    return 0


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


USAGE = "usage: comment-lint.py [--selftest | --seed]"


def main(argv: list[str]) -> int:
    if argv == ["--selftest"]:
        return selftest()
    if argv == ["--seed"]:
        return seed()
    if argv:
        print(USAGE, file=sys.stderr)
        return 2
    return check(lint_repo())


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
