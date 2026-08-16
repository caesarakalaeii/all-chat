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
    SCOPE_CHAT_WRITE,
    SCOPE_ENGAGEMENT_WRITE,
    UPGRADE_URL,
)

# --- kinds -----------------------------------------------------------------
# Plain strings rather than an enum so a kind survives a round-trip through
# StreamController's JSON settings store unchanged.

#: No token configured on the action at all.
NO_TOKEN = "no-token"
#: Token present but not of the ``allchat_pat_…`` shape.
MALFORMED_TOKEN = "malformed-token"
#: A required setting (overlay id, message, ...) is missing.
NOT_CONFIGURED = "not-configured"
#: HTTP 401 -- token rejected by the server.
UNAUTHORIZED = "unauthorized"
#: HTTP 403 on a premium-gated route. EXPECTED on a free account, not a bug.
REQUIRES_PREMIUM = "requires-premium"
#: HTTP 403 because the PAT lacks a scope. Premium is NOT the fix.
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
    """The message shown when the server rejects the token with 401."""
    return (
        f"All-Chat rejected the personal access token (HTTP 401): it is expired, revoked "
        f"or mistyped. Mint a fresh token at {ACCOUNT_TOKENS_URL} and re-paste it into "
        f"this key's settings. (The token itself is never logged.)"
    )


def missing_token_message() -> str:
    """The message shown when no token has been pasted onto the action yet."""
    return (
        f"No All-Chat personal access token configured for this key. Create one at "
        f"{ACCOUNT_TOKENS_URL} and paste it into the key's settings."
    )


def malformed_token_message() -> str:
    """The message shown when the pasted string is not a PAT."""
    return (
        f"The value in this key's token field is not an All-Chat personal access token "
        f"(they start with \"allchat_pat_\"). Mint one at {ACCOUNT_TOKENS_URL} and paste "
        f"the whole string, including the prefix."
    )


def forbidden_message(body: Any) -> str:
    """403 that is neither premium nor scope -- usually "not your overlay"."""
    detail = _server_error_text(body) or "no reason given"
    return (
        f"All-Chat refused this request with HTTP 403 ({detail}). This is not the premium "
        f"gate. The usual cause is an overlay ID that belongs to a different account -- "
        f"check the overlay ID in this key's settings."
    )
