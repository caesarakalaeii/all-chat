"""Minimal HTTP client for the All-Chat API, standard library only.

``urllib.request`` rather than ``requests`` on purpose: a StreamController plugin
is loaded into the host's own Python process, so every dependency we declare is a
dependency the *host* has to satisfy. The three actions here issue small JSON
POSTs and two GETs, which ``urllib`` does perfectly well. See ``requirements.txt``.

Two rules this module enforces so callers cannot get them wrong:

1. **The token goes into the ``Authorization`` header and nowhere else.** It is
   never placed in a URL, never returned in an exception, and never logged. The
   only rendering of a token anywhere in the plugin is
   ``settings.redact()``.
2. **Every non-2xx becomes an** :class:`~allchat.errors.AllChatError` **with a
   kind**, so an action never has to look at a status code. In particular a 403
   is classified into premium / scope / other by
   :func:`~allchat.errors.classify_forbidden` before it reaches an action.
"""

from __future__ import annotations

import json
import urllib.error
import urllib.request
from typing import Any, Mapping

from . import errors
from .errors import AllChatError

#: Sent so All-Chat's logs can attribute traffic to this plugin. Carries no
#: version-specific behaviour; the server does not branch on it.
USER_AGENT = "all-chat-streamcontroller-plugin/0.1 (+https://allch.at)"

#: Per-request timeout. Generous enough for a chat send that fans out to five
#: platforms server-side, short enough that a wedged key recovers within one
#: stream segment rather than hanging until the host is restarted.
DEFAULT_TIMEOUT_SECONDS = 15.0


def _decode(raw: bytes) -> Any:
    """Decodes a JSON body, tolerating an empty or non-JSON one.

    Several of these endpoints answer 200 with an empty body, and an error page
    from a misconfigured reverse proxy is HTML. Neither should turn into a
    traceback, so both degrade to ``None``.
    """
    if not raw:
        return None
    try:
        return json.loads(raw.decode("utf-8"))
    except (ValueError, UnicodeDecodeError):
        return None


def _status_error(status: int, body: Any) -> AllChatError:
    """Maps a non-2xx response onto the error taxonomy.

    The 403 branch is the one that matters: it asks
    :func:`errors.classify_forbidden` which of the three distinct 403 meanings
    this is, so the premium gate keeps its own state and a mis-scoped token is
    never reported as "buy premium".
    """
    if status == 401:
        return AllChatError(errors.UNAUTHORIZED, errors.unauthorized_message(), status)

    if status == 403:
        kind = errors.classify_forbidden(body)
        if kind == errors.REQUIRES_PREMIUM:
            # The caller knows whether it was starting a poll or a prediction and
            # rewrites this with the right noun; this generic wording is only a
            # fallback for a 403 on a route we did not flag as premium-gated.
            return AllChatError(kind, errors.premium_gate_message("poll or prediction"), status)
        if kind == errors.INSUFFICIENT_SCOPE:
            required = ""
            if isinstance(body, Mapping):
                scopes = body.get("required_scopes")
                if isinstance(scopes, list) and scopes:
                    required = ", ".join(str(scope) for scope in scopes)
            return AllChatError(
                kind,
                errors.insufficient_scope_message(required or "required"),
                status,
            )
        return AllChatError(errors.FORBIDDEN, errors.forbidden_message(body), status)

    if status == 404:
        return AllChatError(
            errors.NOT_FOUND,
            "All-Chat could not find that overlay, poll or prediction (HTTP 404). Check the "
            "overlay ID in this key's settings, and whether the round is still open.",
            status,
        )

    if status == 409:
        detail = ""
        if isinstance(body, Mapping) and isinstance(body.get("error"), str):
            detail = f" ({body['error']})"
        return AllChatError(
            errors.CONFLICT,
            f"All-Chat rejected this as conflicting with the current state (HTTP 409){detail}. "
            f"Typically there is already an active poll or prediction on this overlay, or the "
            f"round has already moved past this step.",
            status,
        )

    if status == 429:
        return AllChatError(
            errors.RATE_LIMITED,
            "All-Chat is rate limiting this key (HTTP 429). A chat send fans out to every "
            "connected platform, and the per-platform limits bind fastest. Wait a moment "
            "before pressing again.",
            status,
        )

    detail = ""
    if isinstance(body, Mapping) and isinstance(body.get("error"), str):
        detail = f": {body['error']}"
    return AllChatError(
        errors.HTTP_ERROR,
        f"All-Chat returned HTTP {status}{detail}.",
        status,
    )


def request(
    *,
    base_url: str,
    token: str,
    path: str,
    method: str = "POST",
    body: Mapping[str, Any] | None = None,
    timeout: float = DEFAULT_TIMEOUT_SECONDS,
) -> Any:
    """Issues one authenticated JSON request and returns the decoded body.

    Raises :class:`AllChatError` for every failure, transport or HTTP, so callers
    have exactly one exception type to catch.
    """
    url = f"{base_url}{path}"
    data = json.dumps(dict(body)).encode("utf-8") if body is not None else None

    req = urllib.request.Request(url=url, data=data, method=method)
    # The token lives here and only here.
    req.add_header("Authorization", f"Bearer {token}")
    req.add_header("Accept", "application/json")
    req.add_header("User-Agent", USER_AGENT)
    if data is not None:
        req.add_header("Content-Type", "application/json")

    try:
        with urllib.request.urlopen(req, timeout=timeout) as response:  # noqa: S310
            return _decode(response.read())
    except urllib.error.HTTPError as exc:
        # An HTTPError IS the response, so the body carries the server's reason.
        # Reading it is what makes the premium/scope 403 split possible.
        raw = b""
        try:
            raw = exc.read()
        except Exception:  # pragma: no cover - body already consumed/closed
            pass
        raise _status_error(exc.code, _decode(raw)) from None
    except urllib.error.URLError as exc:
        # DNS, TLS, refused connection, timeout. `exc.reason` never contains the
        # token: it is derived from the host and socket, not from the headers.
        raise AllChatError(
            errors.NETWORK,
            f"Could not reach All-Chat at {base_url} ({exc.reason}). Check the machine's "
            f"network connection, and the base URL in this key's settings if you self-host.",
        ) from None
    except TimeoutError:
        raise AllChatError(
            errors.NETWORK,
            f"All-Chat at {base_url} did not respond within {timeout:.0f}s.",
        ) from None


def post(
    *,
    base_url: str,
    token: str,
    path: str,
    body: Mapping[str, Any] | None = None,
    timeout: float = DEFAULT_TIMEOUT_SECONDS,
) -> Any:
    """Convenience wrapper for a JSON POST."""
    return request(
        base_url=base_url, token=token, path=path, method="POST", body=body, timeout=timeout
    )


def get(
    *,
    base_url: str,
    token: str,
    path: str,
    timeout: float = DEFAULT_TIMEOUT_SECONDS,
) -> Any:
    """Convenience wrapper for a JSON GET."""
    return request(
        base_url=base_url, token=token, path=path, method="GET", timeout=timeout
    )
