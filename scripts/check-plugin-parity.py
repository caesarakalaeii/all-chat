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
to read differently. It pins the things that must not diverge:

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
4.  **The version stamp is in every property inspector.** Three HTML files carry
    one hand-maintained Linking block that nothing else in the repo parses, so an
    element the shared script needs can silently exist in only one of them.

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
TS_LINKING = TS / "src" / "allchat" / "linking.ts"
PY_LINKING = PY / "allchat" / "linking.py"
TS_ACTION_BASE = TS / "src" / "actions" / "base.ts"
TS_UI = TS / "com.allchat.streamdeck.sdPlugin" / "ui"
TS_SEND_UI = TS_UI / "send-message.html"
TS_LINK_SCRIPT = TS_UI / "allchat-link.js"

#: The three property inspectors. They are three files carrying one Linking block,
#: which is the drift ADR-0049 names; anything the shared script reaches for has to
#: exist in all three or it works on one action and not the others.
TS_INSPECTORS: tuple[Path, ...] = (
    TS_SEND_UI,
    TS_UI / "poll-control.html",
    TS_UI / "prediction-control.html",
)

#: Element the shared script writes the running plugin version into. Pinned because
#: it is the only thing that tells a streamer, or a support reply, WHICH build they
#: are on — issue #816: the #797 client fix had shipped in the repo and never reached
#: anyone, and nothing in the panel could have revealed that.
VERSION_ELEMENT_ID = "allchat-plugin-version"

#: How the plugin refers to its own version in a log line. `streamDeck.info` carries
#: the registration info the Stream Deck app sends at connect, so this is the version
#: of the build that is loaded rather than of a manifest on disk.
PLUGIN_VERSION_EXPRESSION = "streamDeck.info.plugin.version"

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
    (TS_SETTINGS, PY_SETTINGS, "ACCOUNT_DEVICES_URL"),
    (TS_SETTINGS, PY_SETTINGS, "UPGRADE_URL"),
    (TS_SETTINGS, PY_SETTINGS, "PAT_PREFIX"),
    # Device linking (ADR-0049 steps 2-3). These five are a wire contract with
    # auth-service, not a preference: the two paths are routes the gateway mounts,
    # LOOPBACK_PATH is the single path the server's redirect validator accepts, and
    # LOOPBACK_HOST is the literal address it pins. A plugin that disagreed with the
    # other on any of them would fail to link on one platform only, which is the
    # exact failure mode this script exists to catch.
    (TS_SETTINGS, PY_SETTINGS, "DEVICE_PREFIX"),
    (TS_SETTINGS, PY_SETTINGS, "LINK_START_PATH"),
    (TS_SETTINGS, PY_SETTINGS, "LINK_EXCHANGE_PATH"),
    (TS_SETTINGS, PY_SETTINGS, "LOOPBACK_PATH"),
    (TS_SETTINGS, PY_SETTINGS, "LOOPBACK_HOST"),
)

# ---------------------------------------------------------------------------
# 2b. Linking parameters that must not drift.
#
#     The two plugins run the same state machine against the same server, so a
#     timeout or a scope set that differs between them means one platform gives up
#     while the other is still waiting, or one asks for a permission the other does
#     not. Compared as NUMBERS/LISTS rather than strings, because the two languages
#     spell them differently (`180_000` ms vs `180.0` s, an array vs a tuple).
# ---------------------------------------------------------------------------
LINKING_TIMEOUTS: tuple[tuple[str, str, float], ...] = (
    # (TS name in ms, Python name in seconds, expected seconds)
    ("LOOPBACK_TIMEOUT_MS", "LOOPBACK_TIMEOUT_SECONDS", 180.0),
    ("CODE_FLOW_TIMEOUT_MS", "CODE_FLOW_TIMEOUT_SECONDS", 600.0),
    ("REQUEST_TIMEOUT_MS", "REQUEST_TIMEOUT_SECONDS", 15.0),
)

#: Longest total silence the property inspector may present as progress before it
#: warns. Not a flow timeout: it bounds the case where the plugin never answers at
#: all, which is the shape of issue #816. Past roughly this long a streamer has
#: already concluded the button is broken, so a longer wait buys no information.
MAX_SILENCE_MS = 20_000

#: Scopes both plugins request at link time. The streamer narrows this on the
#: approve screen; asking for different sets per platform would mean the same button
#: works on Windows and 403s on Linux.
LINKING_SCOPES: tuple[str, ...] = ("chat:write", "engagement:write")

# ---------------------------------------------------------------------------
# 2c. The send surface.
#
#     `POST /api/v1/auth/chat/send` posts to Twitch, YouTube and Kick, and `all`
#     fans out to exactly those (handleStreamerSendToAll in
#     services/auth-service/handlers/chat_send.go). It is NARROWER than the set of
#     platforms All-Chat reads chat from, and that gap is the trap: both plugins
#     shipped `tiktok` in their pickers while auth-service answered 501 to it, so
#     the button silently did nothing and a streamer had to ask why.
#
#     Checked in three places rather than one, because the Elgato picker is a
#     fourth source of truth written in HTML that no compiler or test reads: the
#     TS constant, the Python constant, and the <option> list must agree.
# ---------------------------------------------------------------------------
SEND_PLATFORMS: tuple[str, ...] = ("all", "twitch", "youtube", "kick")

#: Platforms All-Chat reads but cannot post to. Both plugins must refuse these with
#: the REASON (see UNSENDABLE_PLATFORMS in either settings module) rather than as an
#: unknown value, because keys configured before the removal still carry them.
UNSENDABLE: tuple[str, ...] = ("tiktok", "discord")

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


def check_send_platforms() -> None:
    """The send surface, spelled in four places, must be one list.

    The picker is included deliberately. `tsc` never opens the HTML and no unit test
    asserts on it, so an <option> the server rejects is invisible until a streamer
    presses the key.
    """
    ts_platforms = _string_list(strip_ts_comments(read(TS_SETTINGS)), "PLATFORMS")
    py_platforms = _string_list(strip_py_comments(read(PY_SETTINGS)), "PLATFORMS")
    ui_platforms = tuple(re.findall(r'<option value="([a-z]+)"', read(TS_SEND_UI)))

    for label, found in (
        (f"{TS_SETTINGS.relative_to(REPO)} PLATFORMS", ts_platforms),
        (f"{PY_SETTINGS.relative_to(REPO)} PLATFORMS", py_platforms),
        (f"{TS_SEND_UI.relative_to(REPO)} <option> values", ui_platforms),
    ):
        if set(found) != set(SEND_PLATFORMS):
            fail(
                f"send-platform drift: {label} is {list(found)}, expected "
                f"{list(SEND_PLATFORMS)}. The server posts to Twitch/YouTube/Kick only; "
                f"anything else here is a key that fails on press."
            )

    # The refusal must name the reason. A platform merely absent from PLATFORMS gets
    # "not a platform All-Chat sends to", which reads as a typo for a value the plugin
    # itself offered until recently.
    for path in (TS_SETTINGS, PY_SETTINGS):
        # Keys only: `"tiktok": (` in Python, `tiktok:` in TS. The explanation text
        # itself is prose and deliberately not compared — it is allowed to be worded
        # for each plugin's UI, as long as both cover the same platforms.
        keys = set(re.findall(r'^\s*"?([a-z]+)"?:', _unsendable_literal(read(path)), re.MULTILINE))
        if keys != set(UNSENDABLE):
            fail(
                f"{path.relative_to(REPO)} UNSENDABLE_PLATFORMS covers {sorted(keys)}, "
                f"expected {sorted(UNSENDABLE)}. Each read-only platform needs an "
                f"explanation, not just an absence."
            )


def _string_list(src: str, name: str) -> tuple[str, ...]:
    """Values of `NAME = [...]` / `NAME = (...)`, in either language."""
    match = re.search(rf"^(?:export const )?{name}\s*[:=][^=\[(]*[\[(](.*?)[\])]", src, re.DOTALL | re.MULTILINE)
    return tuple(re.findall(r'"([a-z]+)"', match.group(1))) if match else ()


def _unsendable_literal(src: str) -> str:
    """The body of the UNSENDABLE_PLATFORMS mapping, in either language."""
    match = re.search(r"UNSENDABLE_PLATFORMS[^=]*=\s*\{(.*?)\n\}", src, re.DOTALL)
    return match.group(1) if match else ""


def check_inspector_watchdogs() -> None:
    """The panel's own give-up timers must bracket the flows they are waiting on.

    These live in JavaScript that nothing in this repo compiles or runs, and they are
    the difference between "linking failed" and the report that opened issue #816:
    "nothing actually happens, it just stays like that". Three properties, each of
    which has been wrong at some point:

      * The silence budget must be short enough that a human is still watching. It
        was 660s for every path, which is not a timeout, it is an abandonment.
      * The loopback watchdog must OUTLAST LOOPBACK_TIMEOUT_MS, or the panel gives
        up on a flow that is still running and would have succeeded.
      * The code watchdog must OUTLAST CODE_FLOW_TIMEOUT_MS, because the pairing
        code really is valid that long and the user is typing it on another device.

    Read out of linking.ts rather than restated here, so tuning the flow timeout
    cannot silently leave the panel giving up first.
    """
    ui_src = read(TS_LINK_SCRIPT)
    flow_src = strip_ts_comments(read(TS_LINKING))

    def milliseconds(src: str, name: str) -> float | None:
        """Value of `const NAME = 123_000;`, or None having recorded a failure."""
        match = re.search(rf"^\s*const {name}\s*=\s*([0-9_]+)", src, re.MULTILINE)
        if not match:
            fail(f"`{name}` is not declared as a plain millisecond constant")
            return None
        return float(match.group(1).replace("_", ""))

    silence = milliseconds(ui_src, "SILENCE_MS")
    watchdog = milliseconds(ui_src, "WATCHDOG_MS")
    code_watchdog = milliseconds(ui_src, "CODE_WATCHDOG_MS")
    loopback_flow = milliseconds(flow_src, "LOOPBACK_TIMEOUT_MS")
    code_flow = milliseconds(flow_src, "CODE_FLOW_TIMEOUT_MS")

    if silence is not None and silence > MAX_SILENCE_MS:
        fail(
            f"SILENCE_MS is {silence / 1000:g}s. A streamer will not watch a spinner that "
            f"long — they report it as 'nothing happens' (issue #816). Keep it at or under "
            f"{MAX_SILENCE_MS / 1000:g}s."
        )

    if watchdog is not None and loopback_flow is not None and watchdog <= loopback_flow:
        fail(
            f"WATCHDOG_MS ({watchdog / 1000:g}s) does not outlast LOOPBACK_TIMEOUT_MS "
            f"({loopback_flow / 1000:g}s) in {TS_LINKING.relative_to(REPO)}. The panel would "
            f"give up on a loopback flow that is still running."
        )

    if code_watchdog is not None and code_flow is not None and code_watchdog <= code_flow:
        fail(
            f"CODE_WATCHDOG_MS ({code_watchdog / 1000:g}s) does not outlast "
            f"CODE_FLOW_TIMEOUT_MS ({code_flow / 1000:g}s) in "
            f"{TS_LINKING.relative_to(REPO)}. The pairing code is valid that long and the "
            f"user is typing it on another device."
        )


def check_version_stamp() -> None:
    """The running plugin version must be identifiable from the panel and from a log.

    Checked here rather than in a unit test because no compiler or test runner in
    this repo opens these HTML files: the three inspectors are hand-maintained
    copies of one Linking block, so an id added to one and forgotten in the other
    two is invisible until a streamer on that action reports a blank panel.

    Both surfaces are asserted together because they answer one question. A streamer
    reading the panel and a maintainer reading the log they were sent both need the
    build number, and issue #816 is what happens when neither has it: the #797 fix
    sat unreleased for days and no report could have revealed that.
    """
    if VERSION_ELEMENT_ID not in read(TS_LINK_SCRIPT):
        fail(
            f"{TS_LINK_SCRIPT.relative_to(REPO)} does not write to `{VERSION_ELEMENT_ID}`. "
            f"The version stamp belongs in the shared script, not three times in the HTML."
        )

    for page in TS_INSPECTORS:
        if f'id="{VERSION_ELEMENT_ID}"' not in read(page):
            fail(
                f"{page.relative_to(REPO)} has no element with id=\"{VERSION_ELEMENT_ID}\". "
                f"All three property inspectors carry the version stamp, or a support "
                f"report from that action cannot say which build it came from."
            )

    # Every "linking failed" the plugin logs has to carry the build, so a log file a
    # streamer attaches identifies it without a follow-up question. Matched on the
    # line rather than on a helper name so it cannot be satisfied by declaring one.
    base_src = strip_ts_comments(read(TS_ACTION_BASE))
    failure_logs = re.findall(r"^.*logger\.error\(.*linking failed.*$", base_src, re.MULTILINE)
    if not failure_logs:
        fail(
            f"{TS_ACTION_BASE.relative_to(REPO)} logs no `linking failed` error. If the "
            f"wording changed, change it here too."
        )
    for line in failure_logs:
        if PLUGIN_VERSION_EXPRESSION not in line:
            fail(
                f"{TS_ACTION_BASE.relative_to(REPO)} logs a linking failure without "
                f"`{PLUGIN_VERSION_EXPRESSION}`: {line.strip()}. A log a streamer sends in "
                f"has to name the build, or the first reply is always 'which version?'."
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


def check_linking() -> None:
    """Both plugins must implement the same linking flow with the same numbers.

    ADR-0049 flags the pairing-code fallback as the path that will rot, because it
    is used rarely. Half of "rot" is drift: a timeout that was tuned on one plugin
    and not the other, or a scope set that grew on one side. Both are invisible in
    review and only show up as "linking works on my machine".
    """
    ts_src = strip_ts_comments(read(TS_LINKING))
    py_src = strip_py_comments(read(PY_LINKING))

    for ts_name, py_name, expected_seconds in LINKING_TIMEOUTS:
        ts_match = re.search(rf"^const {ts_name}\s*=\s*([0-9_]+)", ts_src, re.MULTILINE)
        py_match = re.search(rf"^{py_name}\s*=\s*([0-9_.]+)", py_src, re.MULTILINE)
        if not ts_match:
            fail(f"{TS_LINKING.relative_to(REPO)} does not define `{ts_name}`")
        if not py_match:
            fail(f"{PY_LINKING.relative_to(REPO)} does not define `{py_name}`")
        if not ts_match or not py_match:
            continue
        ts_seconds = float(ts_match.group(1).replace("_", "")) / 1000.0
        py_seconds = float(py_match.group(1).replace("_", ""))
        if ts_seconds != expected_seconds or py_seconds != expected_seconds:
            fail(
                f"linking timeout drift: {ts_name}={ts_seconds}s, {py_name}={py_seconds}s, "
                f"expected {expected_seconds}s on both. Change both plugins and the expectation "
                f"here in one commit."
            )

    # The requested scope set, spelled as an array on one side and a tuple on the
    # other. Compared as a set of quoted strings so formatting differences do not
    # register as drift.
    ts_scopes = set(
        re.findall(r'"([a-z]+:[a-z]+)"', _scope_literal(ts_src, "REQUESTED_SCOPES"))
    )
    py_scopes = set(
        re.findall(r'"([a-z]+:[a-z]+)"', _scope_literal(py_src, "REQUESTED_SCOPES"))
    )
    if ts_scopes != set(LINKING_SCOPES) or py_scopes != set(LINKING_SCOPES):
        fail(
            f"REQUESTED_SCOPES differ: {TS_LINKING.relative_to(REPO)} has {sorted(ts_scopes)}, "
            f"{PY_LINKING.relative_to(REPO)} has {sorted(py_scopes)}, expected "
            f"{sorted(LINKING_SCOPES)} on both."
        )

    # The loopback listener must never bind the wildcard address. This is a security
    # property, not a style one: 0.0.0.0 would expose the listener to the local
    # network for the duration of linking, and the point of a loopback redirect is
    # that the credential cannot leave the machine.
    for path, src in ((TS_LINKING, ts_src), (PY_LINKING, py_src)):
        if "0.0.0.0" in src:
            fail(
                f"{path.relative_to(REPO)} mentions 0.0.0.0. The loopback listener must bind "
                f"127.0.0.1 ONLY (ADR-0049): the wildcard address is reachable from the local "
                f"network while linking is in progress."
            )


def _scope_literal(src: str, name: str) -> str:
    """Returns the bracketed/parenthesised literal assigned to `name`, or ""."""
    match = re.search(rf"{name}\s*[:=][^=]*?[\[(](.*?)[\])]", src, re.DOTALL)
    return match.group(1) if match else ""


def main() -> int:
    check_action_list()
    check_constants()
    check_send_platforms()
    check_version_stamp()
    check_inspector_watchdogs()
    check_linking()
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
        f"{len(SHARED_CONSTANTS)} shared constants, {len(SEND_PLATFORMS)} send "
        f"platforms (picker included), {len(LINKING_TIMEOUTS)} linking timeouts, "
        f"version stamp in {len(TS_INSPECTORS)} inspectors, pointers mutual."
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
