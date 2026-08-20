#!/usr/bin/env python3
"""Parity gate for the two desktop plugins (ADR-0049).

Two plugins ship the same buttons in two languages: `streamdeck-plugin/`
(TypeScript, Elgato SDK, Windows + macOS) and `streamcontroller-plugin/`
(Python, StreamController, Linux). ADR-0049 names the resulting risk exactly:

    The real cost of two plugins is drift, not effort. [...] the *button set*
    now exists twice and will diverge unless it is defined once. Treat the
    action list (name, endpoint, payload, and what the button surfaces on
    failure) as the single source both implementations follow.

This script is that single source, made executable. It is deliberately not a
string-diff of the two files: they are different languages and are *supposed*
to read differently. It pins the three things that must not diverge:

1.  **The action list.** Every entry below must exist in both API modules, and
    neither module may carry a route-bearing public function the other lacks —
    that is precisely "a button that exists on Linux but not Windows".
2.  **The shared constants.** Host, token prefix and the two URLs a user is
    sent to. These are user-visible, and a mismatch here is invisible in review
    (it already happened: UPGRADE_URL pointed at the homepage on one side and
    at /upgrade on the other).
3.  **The keep-in-sync pointers are mutual.** The convention CLAUDE.md uses for
    OnboardingChecklist.tsx / upgrade only works when both ends point at each
    other; a one-way pointer rots the moment someone edits the unmarked file.

Standard library only, so it runs in the flake dev shell, in CI, and in the
Caterpillar sandbox without an install step:

    python3 scripts/check-plugin-parity.py
"""

from __future__ import annotations

import re
import sys
from pathlib import Path

REPO = Path(__file__).resolve().parent.parent
TS = REPO / "streamdeck-plugin"
PY = REPO / "streamcontroller-plugin"

TS_API = TS / "src" / "allchat" / "api.ts"
PY_API = PY / "allchat" / "api.py"
TS_SETTINGS = TS / "src" / "allchat" / "settings.ts"
PY_SETTINGS = PY / "allchat" / "settings.py"

# ---------------------------------------------------------------------------
# 1. The action list. THIS is the contract ADR-0049 asks for: adding a button
#    means adding a row here and implementing it on both sides, in one change.
#    The route column is documentation for the reader, not something this script
#    can verify against a running server.
# ---------------------------------------------------------------------------
ACTIONS: tuple[tuple[str, str, str], ...] = (
    ("sendChatMessage", "send_chat_message", "POST /api/v1/auth/chat/send"),
    ("startPoll", "start_poll", "POST /api/v1/engagement/overlays/:id/polls  (premium)"),
    ("closePoll", "close_poll", "POST /api/v1/engagement/overlays/:id/polls/:pollId/close"),
    ("activePoll", "active_poll", "GET  /api/v1/engagement/overlays/:id/active-poll"),
    ("startPrediction", "start_prediction", "POST /api/v1/engagement/overlays/:id/predictions  (premium)"),
    ("lockPrediction", "lock_prediction", "POST /api/v1/engagement/overlays/:id/predictions/:pid/lock"),
    ("resolvePrediction", "resolve_prediction", "POST /api/v1/engagement/overlays/:id/predictions/:pid/resolve"),
    ("cancelPrediction", "cancel_prediction", "POST /api/v1/engagement/overlays/:id/predictions/:pid/cancel"),
    ("activePrediction", "active_prediction", "GET  /api/v1/engagement/overlays/:id/active-prediction"),
)

# Public helpers that legitimately exist on one side only. Every entry needs a
# reason, because "add it to the allowlist" is the easy way to defeat this gate.
TS_ONLY_ALLOWED: dict[str, str] = {}
PY_ONLY_ALLOWED: dict[str, str] = {
    # The TS actions read `poll.id` off the typed response directly; Python has
    # no such type, so it needs a runtime shape-sniffer. It drives no route.
    "extract_id": "response-shape helper with no TypeScript counterpart (TS has typed responses)",
}

# ---------------------------------------------------------------------------
# 2. Constants that are user-visible and must be identical in both plugins.
# ---------------------------------------------------------------------------
SHARED_CONSTANTS: tuple[tuple[Path, Path, str], ...] = (
    (TS_API, PY_API, "CHAT_SEND_PATH"),
    (TS_SETTINGS, PY_SETTINGS, "DEFAULT_BASE_URL"),
    (TS_SETTINGS, PY_SETTINGS, "ACCOUNT_TOKENS_URL"),
    (TS_SETTINGS, PY_SETTINGS, "UPGRADE_URL"),
    (TS_SETTINGS, PY_SETTINGS, "PAT_PREFIX"),
)

# ---------------------------------------------------------------------------
# 3. Keep-in-sync pointers, e.g.
#       KEEP IN SYNC with ``streamdeck-plugin/src/allchat/api.ts`` (ADR-0049).
#    Matched in either language's comment syntax; the quoting around the path is
#    whatever the surrounding docstring/JSDoc uses, so it is skipped over.
# ---------------------------------------------------------------------------
POINTER = re.compile(r"KEEP IN SYNC with[`'\"\s]+([A-Za-z0-9_./-]+\.(?:ts|py))")

PLUGIN_SOURCES = sorted(
    [p for p in (TS / "src").rglob("*.ts")] + [p for p in PY.rglob("*.py") if "tests" not in p.parts]
)


def read(path: Path) -> str:
    if not path.is_file():
        fail(f"expected file is missing: {path.relative_to(REPO)}")
        return ""
    return path.read_text(encoding="utf-8")


errors: list[str] = []


def fail(message: str) -> None:
    errors.append(message)


def strip_ts_comments(src: str) -> str:
    """Remove // and /* */ comments so documentation prose is never mistaken for code.

    The TS api module documents routes in its header block, so a naive scan
    would 'find' functions and paths that are only being described.
    """
    src = re.sub(r"/\*.*?\*/", "", src, flags=re.DOTALL)
    return re.sub(r"^\s*//.*$", "", src, flags=re.MULTILINE)


def strip_py_comments(src: str) -> str:
    """Same idea for Python: drop docstrings and # comments."""
    src = re.sub(r'"""(?:.|\n)*?"""', "", src)
    src = re.sub(r"'''(?:.|\n)*?'''", "", src)
    return re.sub(r"^\s*#.*$", "", src, flags=re.MULTILINE)


def ts_exported_functions(src: str) -> set[str]:
    return set(re.findall(r"^export\s+(?:async\s+)?function\s+(\w+)", strip_ts_comments(src), re.MULTILINE))


def py_public_functions(src: str) -> set[str]:
    """Top-level, non-underscore defs. Indented defs are methods, not API."""
    return set(re.findall(r"^def\s+([a-z][A-Za-z0-9_]*)\s*\(", strip_py_comments(src), re.MULTILINE))


def ts_constant(src: str, name: str) -> str | None:
    """Value of `export const NAME = "..."` or a backtick template of one.

    `${DEFAULT_BASE_URL}` is resolved so a composed URL can be compared against
    the Python f-string that composes it the same way.
    """
    match = re.search(rf"^export const {name}\s*=\s*([`\"'])(.*?)\1\s*;", strip_ts_comments(src), re.MULTILINE)
    if not match:
        return None
    return resolve_base_url_placeholder(src, match.group(2), ts=True)


def py_constant(src: str, name: str) -> str | None:
    """Value of `NAME = "..."` or `NAME = f"..."`, with the same placeholder resolution."""
    match = re.search(rf"^{name}\s*=\s*f?([\"'])(.*?)\1\s*$", strip_py_comments(src), re.MULTILINE)
    if not match:
        return None
    return resolve_base_url_placeholder(src, match.group(2), ts=False)


def resolve_base_url_placeholder(src: str, value: str, *, ts: bool) -> str:
    """Substitute the one interpolation both plugins use: the base URL."""
    if "DEFAULT_BASE_URL" not in value:
        return value
    pattern = r"^export const DEFAULT_BASE_URL\s*=\s*\"(.*?)\"" if ts else r"^DEFAULT_BASE_URL\s*=\s*\"(.*?)\""
    base = re.search(pattern, src, re.MULTILINE)
    if not base:
        return value
    return value.replace("${DEFAULT_BASE_URL}" if ts else "{DEFAULT_BASE_URL}", base.group(1))


def check_action_list() -> None:
    ts_funcs = ts_exported_functions(read(TS_API))
    py_funcs = py_public_functions(read(PY_API))

    for ts_name, py_name, route in ACTIONS:
        if ts_name not in ts_funcs:
            fail(f"{TS_API.relative_to(REPO)} does not export `{ts_name}` ({route})")
        if py_name not in py_funcs:
            fail(f"{PY_API.relative_to(REPO)} does not define `{py_name}` ({route})")

    known_ts = {a[0] for a in ACTIONS} | set(TS_ONLY_ALLOWED)
    known_py = {a[1] for a in ACTIONS} | set(PY_ONLY_ALLOWED)

    for extra in sorted(ts_funcs - known_ts):
        fail(
            f"{TS_API.relative_to(REPO)} exports `{extra}`, which is not in the ADR-0049 action list. "
            f"Add it to ACTIONS here and implement it in {PY_API.relative_to(REPO)}, or allowlist it with a reason."
        )
    for extra in sorted(py_funcs - known_py):
        fail(
            f"{PY_API.relative_to(REPO)} defines `{extra}`, which is not in the ADR-0049 action list. "
            f"Add it to ACTIONS here and implement it in {TS_API.relative_to(REPO)}, or allowlist it with a reason."
        )


def check_constants() -> None:
    for ts_path, py_path, name in SHARED_CONSTANTS:
        ts_value = ts_constant(read(ts_path), name)
        py_value = py_constant(read(py_path), name)
        if ts_value is None:
            fail(f"{ts_path.relative_to(REPO)} does not export a string constant `{name}`")
        if py_value is None:
            fail(f"{py_path.relative_to(REPO)} does not define a string constant `{name}`")
        if ts_value is not None and py_value is not None and ts_value != py_value:
            fail(
                f"`{name}` differs between the plugins: "
                f"{ts_path.relative_to(REPO)} has {ts_value!r}, {py_path.relative_to(REPO)} has {py_value!r}"
            )


def check_sync_pointers() -> None:
    pointers: dict[Path, set[str]] = {}
    for path in PLUGIN_SOURCES:
        targets = set(POINTER.findall(path.read_text(encoding="utf-8")))
        if targets:
            pointers[path] = targets

    if not pointers:
        fail("no KEEP IN SYNC pointers found at all; the drift convention has been deleted")

    for path, targets in pointers.items():
        source_rel = path.relative_to(REPO).as_posix()
        for target_rel in sorted(targets):
            target = REPO / target_rel
            if not target.is_file():
                fail(f"{source_rel} points at {target_rel}, which does not exist")
                continue
            back = POINTER.findall(target.read_text(encoding="utf-8"))
            if source_rel not in back:
                fail(
                    f"one-way KEEP IN SYNC pointer: {source_rel} -> {target_rel}, but {target_rel} "
                    f"does not point back. Add `KEEP IN SYNC with {source_rel} (ADR-0049).` to it."
                )


def main() -> int:
    check_action_list()
    check_constants()
    check_sync_pointers()

    if errors:
        print("Desktop plugin parity check FAILED (ADR-0049):\n", file=sys.stderr)
        for message in errors:
            print(f"  - {message}", file=sys.stderr)
        print(
            "\nBoth plugins implement one action list. If a button changed on one side, "
            "change it on the other in the same PR.",
            file=sys.stderr,
        )
        return 1

    print(
        f"Desktop plugin parity OK: {len(ACTIONS)} actions, "
        f"{len(SHARED_CONSTANTS)} shared constants, pointers mutual."
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
