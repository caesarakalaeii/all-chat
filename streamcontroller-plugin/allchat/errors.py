"""Error taxonomy for All-Chat requests.

This module exists to preserve one distinction, and it is the reason the plugin
is worth reviewing carefully:

* **401** means the personal access token is missing, malformed, expired or
  revoked. The user must mint a new one and re-paste it. The fix is entirely on
  the user's side.

* **403 on a start action** is *not* a failure. The server deliberately gates
  *starting* a poll or a prediction behind the premium ``engagement`` feature,
  while close / lock / resolve / cancel are deliberately free. A non-premium
  account therefore gets a perfectly correct 403 from exactly two routes while
  every other key keeps working. We surface that as its own state and name the
  upgrade page, because the point of the gate is to advertise the feature at the
  moment somebody reaches for it -- not to look like a broken button.

**Three different things return 403, and conflating them produces bad advice.**
Telling a user to buy premium when their real problem is a mis-scoped token is
worse than a generic error, so we branch on the server's ``error`` string:

===================================  ==========================================
Server response body                 Meaning / what the user must actually do
===================================  ==========================================
``{"error": "Premium feature        The premium gate
required", "upgrade_url": ...}``     (``shared/middleware/premium.go``).
                                     -> Upgrade. This is REQUIRES_PREMIUM.
``{"error": "insufficient token     The PAT lacks a scope
scope", "required_scopes": [...]}``  (``shared/middleware/apitoken.go``).
                                     -> Mint a new token WITH that scope.
                                     Premium would NOT help. This is
                                     INSUFFICIENT_SCOPE.
``{"error": "your account is        The account is banned.
banned"}``                           -> Neither upgrading nor re-minting helps.
===================================  ==========================================

Anything else 403 (typically "not your overlay") falls through to FORBIDDEN.

No function here ever puts the token in a message, and no caller may log one.

KEEP IN SYNC with ``streamdeck-plugin/src/allchat/errors.ts`` (ADR-0049).
"""

from __future__ import annotations

from typing import Any, Mapping

from .settings import (
    ACCOUNT_TOKENS_URL,
    DEFAULT_BASE_URL,
    SCOPE_CHAT_WRITE,
    SCOPE_ENGAGEMENT_WRITE,
    UPGRADE_URL,
)

# Error kinds. Plain strings rather than an enum so a kind survives a round-trip
# through StreamController's JSON settings store unchanged.

#: No token configured on the action at all.
NO_TOKEN = "no-token"
#: Credential present but neither the ``allchat_dev_…`` nor ``allchat_pat_…`` shape.
MALFORMED_TOKEN = "malformed-token"
#: A required setting (overlay id, message, ...) is missing.
NOT_CONFIGURED = "not-configured"
#: HTTP 401 -- token rejected by the server.
UNAUTHORIZED = "unauthorized"
#: HTTP 403 on a premium-gated route. EXPECTED on a free account, not a bug.
REQUIRES_PREMIUM = "requires-premium"
#: HTTP 403 because the credential lacks a scope. Premium is NOT the fix.
INSUFFICIENT_SCOPE = "insufficient-scope"
#: HTTP 403 where neither premium nor scope is the explanation.
FORBIDDEN = "forbidden"
#: HTTP 404 -- overlay / poll / prediction not found.
NOT_FOUND = "not-found"
#: HTTP 409 -- e.g. an active poll already exists on this overlay.
CONFLICT = "conflict"
#: HTTP 429 -- rate limited. A physical button invites mashing.
RATE_LIMITED = "rate-limited"
#: Any other non-2xx response.
HTTP_ERROR = "http-error"
#: Transport failure: host unreachable, DNS, TLS, timeout.
NETWORK = "network"


class AllChatError(Exception):
    """An All-Chat failure carrying the kind a UI branches on.

    ``message`` is always safe to log verbatim: it never contains the token.
    """

    def __init__(self, kind: str, message: str, status: int | None = None) -> None:
        super().__init__(message)
        self.kind = kind
        self.message = message
        self.status = status

    @property
    def is_premium_gate(self) -> bool:
        """True when this is the deliberate premium gate, not a real fault.

        Actions use this to pick the "requires premium" key state instead of the
        error state, which is the whole point of the taxonomy.
        """
        return self.kind == REQUIRES_PREMIUM

    def __repr__(self) -> str:  # pragma: no cover - debugging aid
        return f"AllChatError(kind={self.kind!r}, status={self.status!r})"


def _server_error_text(body: Any) -> str:
    """Pulls the server's ``error`` field out of a decoded JSON body."""
    if isinstance(body, Mapping):
        for key in ("error", "message"):
            value = body.get(key)
            if isinstance(value, str) and value:
                return value
    return ""


def classify_forbidden(body: Any) -> str:
    """Decides which of the three 403 meanings a response carries.

    Branching on the server's own ``error`` string is what keeps "you need
    premium" from being shown to somebody whose token is merely mis-scoped.
    Matching is case-insensitive and substring-based so a future rewording of the
    server message degrades to the generic FORBIDDEN rather than to a wrong one.
    """
    text = _server_error_text(body).lower()
    if "insufficient token scope" in text or "scope" in text:
        return INSUFFICIENT_SCOPE
    if "premium" in text:
        return REQUIRES_PREMIUM
    return FORBIDDEN


def premium_gate_message(what: str) -> str:
    """The message shown when the premium ``engagement`` gate answers 403.

    Deliberately names the feature, states plainly that the *other* keys still
    work, and points at the upgrade page. It must never read like a generic
    "request failed" -- a free user reaching this has found the feature, and this
    string is the entire pitch.
    """
    return (
        f"Starting a {what} is part of All-Chat premium (the \"engagement\" feature), "
        f"and this account does not have it. The server answered HTTP 403, which is "
        f"expected here rather than a bug. Closing a poll and locking / resolving / "
        f"cancelling a prediction stay free, so those keys keep working. "
        f"Upgrade at {UPGRADE_URL} to start polls and predictions from your Stream Deck."
    )


def insufficient_scope_message(required: str) -> str:
    """403 because the PAT lacks a scope -- NOT the premium gate.

    Kept textually distinct from :func:`premium_gate_message` on purpose:
    upgrading would not fix this, and telling the user to pay for something that
    will not help them is the worst outcome this module can produce.
    """
    return (
        f"This personal access token does not carry the \"{required}\" scope, so All-Chat "
        f"refused the request with HTTP 403. This is NOT the premium gate and upgrading "
        f"will not change it. Mint a replacement token at {ACCOUNT_TOKENS_URL} with "
        f"\"{SCOPE_CHAT_WRITE}\" for the chat action and \"{SCOPE_ENGAGEMENT_WRITE}\" for "
        f"the poll and prediction actions, then paste it into this key's settings."
    )


def unauthorized_message() -> str:
    """The message shown when the server rejects the credential with 401."""
    return (
        f"All-Chat rejected this action's credential (HTTP 401): it is expired, revoked "
        f"or mistyped. If this action was linked, press Link with All-Chat again -- a "
        f"paired device lapses if it goes unused. If you pasted a token, mint a fresh one "
        f"at {ACCOUNT_TOKENS_URL}. (The credential itself is never logged.)"
    )


def missing_token_message() -> str:
    """The message shown when the action has no credential yet."""
    return (
        f"This action is not connected to All-Chat yet. Press \"Link with All-Chat\" in "
        f"its settings -- your browser opens an approve screen and nothing needs to be "
        f"copied. On a second machine or a headless host, paste a personal access token "
        f"from {ACCOUNT_TOKENS_URL} instead."
    )


def malformed_token_message() -> str:
    """The message shown when the configured string is not an All-Chat credential."""
    return (
        f"The value in this action's token field is not an All-Chat credential. A linked "
        f"device token starts with \"allchat_dev_\" and a pasted personal access token with "
        f"\"allchat_pat_\". Press \"Link with All-Chat\", or mint a token at "
        f"{ACCOUNT_TOKENS_URL} and paste the whole string including the prefix."
    )


def forbidden_message(body: Any) -> str:
    """403 that is neither premium nor scope -- usually "not your overlay"."""
    detail = _server_error_text(body) or "no reason given"
    return (
        f"All-Chat refused this request with HTTP 403 ({detail}). This is not the premium "
        f"gate. The usual cause is an overlay ID that belongs to a different account -- "
        f"check the overlay ID in this key's settings."
    )


def link_failure_message(exc: BaseException) -> str:
    """The subtitle shown under the Link button when a link attempt fails.

    The row used to render ``exc.message`` verbatim, and anything that was not an
    :class:`AllChatError` as ``f"Linking failed: {type(exc).__name__}: {exc}"`` --
    an exception class name is not something to put in front of a streamer.

    Most of those messages are written for the plugin log: they carry a URL, an
    errno, an HTTP status or a server-supplied string. This translates the ones
    that leak detail and deliberately passes the rest through, because several
    are already good user-facing copy ("The pairing code expired before it was
    approved. Start linking again.") and replacing those with something generic
    would be a downgrade. The rule: an authored message with no status attached
    is copy and survives; anything carrying transport or HTTP detail is replaced.

    Mirrored by ``linkFailureMessage`` in the Stream Deck plugin's ``errors.ts``
    (this module's header carries the file-level sync pointer). ADR-0049 counts
    "what the button surfaces on failure" as part of the action contract, so the
    two plugins must not tell a user two different things about one failure.
    """
    if not isinstance(exc, AllChatError):
        # A TypeError or similar: implementation detail, nothing user-actionable.
        return (
            f"Linking did not complete. Press \"Link with All-Chat\" to try again, or "
            f"paste a personal access token from {ACCOUNT_TOKENS_URL} instead."
        )
    if exc.kind == NETWORK:
        # Unreachable host, DNS, TLS, a timeout, or a loopback port that could not
        # be bound. All of them carry an errno or a URL in the message.
        return (
            "Could not reach All-Chat. Check your internet connection and try again. "
            "If you self-host, check the Server field in this action's settings."
        )
    if exc.kind == FORBIDDEN:
        return (
            "The approval could not be verified, so nothing was linked. Press "
            "\"Link with All-Chat\" and approve the device again."
        )
    if exc.kind == UNAUTHORIZED:
        return unauthorized_message()
    if exc.status is not None and exc.status >= 500:
        return (
            f"All-Chat could not complete the link because the server returned an error "
            f"(HTTP {exc.status}). That is a fault on the server, not on this machine: "
            f"try again in a few minutes, and report it if it keeps happening."
        )
    if exc.status is not None:
        return (
            f"All-Chat refused the link (HTTP {exc.status}). Make sure you are signed in "
            f"at {DEFAULT_BASE_URL} as the account you want this action to control, then "
            f"press \"Link with All-Chat\" again."
        )
    # No status: an authored message, e.g. the expired pairing code. Show it as written.
    return exc.message
