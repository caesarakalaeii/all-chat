"""Settings shared by every All-Chat action, and the normalisation applied
before a value reaches the HTTP client.

StreamController stores per-action settings as a plain JSON dict, so every field
here is optional: a freshly dropped key has ``{}``. Nothing in this module
touches the network, which is what makes it the easy part to unit-test.

KEEP IN SYNC with ``streamdeck-plugin/src/allchat/settings.ts``. ADR-0049 calls
out drift between the two plugins as the real cost of shipping both, so the
action list, the endpoints and the failure states are defined once and mirrored
deliberately rather than reinvented per vendor.
"""

from __future__ import annotations

from typing import Any, Mapping

#: Production host for All-Chat. Confirmed as ``FRONTEND_URL`` in
#: ``deployments/k8s/base/configmap.yaml``. Self-hosters override this per action
#: in the settings UI; everyone else leaves it blank and gets this.
DEFAULT_BASE_URL = "https://allch.at"

#: Where a user mints a personal access token.
ACCOUNT_TOKENS_URL = f"{DEFAULT_BASE_URL}/settings/api-tokens"

#: Where a user sees and revokes their paired control surfaces. This is the page to
#: point at once linking has succeeded -- it is where the credential this plugin now
#: holds can be cut off.
ACCOUNT_DEVICES_URL = f"{DEFAULT_BASE_URL}/settings/devices"

#: Page advertised when the server reports the premium engagement gate.
UPGRADE_URL = f"{DEFAULT_BASE_URL}/upgrade"

#: Every All-Chat personal access token carries this prefix. The server routes on
#: the prefix (hash-and-look-up) instead of parsing the bearer as a JWT, per
#: ADR-0051. We use it only to tell the user early that they pasted the wrong
#: string -- never to validate the secret itself, which only the server can do.
PAT_PREFIX = "allchat_pat_"

#: Prefix of a PAIRED-DEVICE token (ADR-0049), the credential the Link flow
#: produces. Deliberately a different prefix from :data:`PAT_PREFIX` rather than a
#: flag inside the same namespace: the server switches on the prefix, secret
#: scanners can tell the two apart, and a support log line says which credential a
#: streamer is using without disclosing either.
DEVICE_PREFIX = "allchat_dev_"

#: Opens a device-link request. Unauthenticated: this plugin has no credential yet.
LINK_START_PATH = "/api/v1/auth/device/link/start"

#: Trades an approved link for the device token. Answers 428 while still pending.
LINK_EXCHANGE_PATH = "/api/v1/auth/device/link/exchange"

#: The ONE path the loopback listener serves, and the only path the server accepts in
#: a ``redirect_uri``. Fixed on both sides so the listener has one route and the
#: server's validator has one string to compare -- see
#: ``services/auth-service/handlers/loopback_redirect.go``.
LOOPBACK_PATH = "/allchat/device-callback"

#: The loopback host. ``127.0.0.1`` ONLY, never ``0.0.0.0`` and never ``localhost``:
#: binding the wildcard address would expose the listener to the local network during
#: linking, and ``localhost`` is a DNS name the server refuses precisely because it
#: can be pointed elsewhere.
LOOPBACK_HOST = "127.0.0.1"

#: Platforms accepted by ``POST /api/v1/auth/chat/send``. ``all`` fans the message
#: out to every connected platform and returns a per-platform result.
PLATFORMS = ("all", "twitch", "youtube", "kick", "tiktok")

#: Scopes a PAT must carry for these actions, mirroring the constants in
#: ``shared/middleware/apitoken.go``. Shown in error text so a user who minted a
#: chat-only token understands why engagement actions refuse -- that refusal is a
#: 403 too, but it is NOT the premium gate. See ``errors.py``.
SCOPE_CHAT_WRITE = "chat:write"
SCOPE_ENGAGEMENT_WRITE = "engagement:write"


def _text(settings: Mapping[str, Any] | None, key: str) -> str:
    """Returns a trimmed string setting, or ``""`` when absent/not a string."""
    if not settings:
        return ""
    value = settings.get(key)
    if value is None:
        return ""
    return str(value).strip()


def resolve_base_url(settings: Mapping[str, Any] | None) -> str:
    """Resolves the base URL for an action.

    The trimmed override when the user set one, otherwise the production host.
    Trailing slashes are dropped so joining a path cannot produce ``//``.
    """
    base = _text(settings, "base_url") or DEFAULT_BASE_URL
    return base.rstrip("/")


def resolve_token(settings: Mapping[str, Any] | None) -> str:
    """Resolves the credential for an action, trimmed.

    Either a paired-device token (``allchat_dev_...``, written by the Link flow) or
    a personal access token (``allchat_pat_...``, pasted by the user for a headless
    box or a second machine). Returns ``""`` when unset. The value is a secret:
    callers pass it to the ``Authorization`` header and nowhere else, and never into
    a log line.

    The setting key is ``api_token`` for historical reasons -- it predates device
    tokens, and renaming it would silently unconfigure every existing action.
    """
    return _text(settings, "api_token")


def looks_like_token(token: str) -> bool:
    """True when ``token`` has the shape of either All-Chat credential.

    A prefix test only. It catches the common paste error (a session JWT, or a
    truncated copy) before a pointless round-trip; it says nothing about whether the
    token is valid, revoked, expired or in scope, all of which only the server knows.
    Both prefixes are accepted because both are legitimate here: a device token
    arrives from the Link flow, a PAT from the user.
    """
    return looks_like_device_token(token) or looks_like_pat(token)


def looks_like_device_token(token: str) -> bool:
    """True when ``token`` is specifically a paired-device token (from linking).

    Counterpart of ``looksLikeDeviceToken`` in
    ``streamdeck-plugin/src/allchat/settings.ts``. The narrow pair exists so a message
    can name the right remedy: a device token is re-linked, a pasted token is
    re-pasted.
    """
    return token.startswith(DEVICE_PREFIX) and len(token) > len(DEVICE_PREFIX)


def looks_like_pat(token: str) -> bool:
    """True when ``token`` has the shape of an All-Chat PAT.

    A prefix test only. It catches the common paste error (a session JWT, or a
    truncated copy) before a pointless round-trip; it says nothing about whether
    the token is valid, revoked or in-scope, all of which only the server knows.
    """
    return token.startswith(PAT_PREFIX) and len(token) > len(PAT_PREFIX)


def redact(token: str) -> str:
    """Renders a token safe to display.

    Used only where a user needs to tell two configured actions apart, and to say
    WHICH KIND of credential is configured -- a paired device token or a pasted PAT
    -- because that changes the advice when something goes wrong. The tail is
    deliberately dropped rather than kept: a suffix is still secret material, and the
    server stores only a SHA-256 precisely so the plaintext exists in as few places
    as possible. Never log the result of *not* calling this.
    """
    if not token:
        return "(none)"
    for prefix in (DEVICE_PREFIX, PAT_PREFIX):
        if token.startswith(prefix) and len(token) > len(prefix):
            return f"{prefix}\u2026"
    return "(not an All-Chat token)"


def split_list(raw: str | None) -> list[str]:
    """Splits a user-typed option/outcome list into entries.

    Accepts newlines or commas as separators, drops blanks, preserves order. The
    server requires 2-5 poll options and 2-10 prediction outcomes; validating the
    count is the caller's job, because the message differs per action.
    """
    if not raw:
        return []
    entries: list[str] = []
    for chunk in str(raw).replace(",", "\n").split("\n"):
        entry = chunk.strip()
        if entry:
            entries.append(entry)
    return entries


def to_seconds(value: Any) -> int:
    """Coerces a possibly-string duration field into a non-negative int.

    Blank, unparseable and negative all mean 0, which every caller reads as
    "omit the field and let the round run until it is closed by hand".
    """
    if value is None or value == "":
        return 0
    try:
        seconds = int(str(value).strip())
    except (TypeError, ValueError):
        return 0
    return seconds if seconds > 0 else 0
