"""Device linking: how this plugin obtains a credential without the streamer
typing or pasting one (ADR-0049).

The primary path, RFC 8252 section 7.3:

1. Generate a PKCE verifier and its S256 challenge, plus a ``state`` value.
2. Bind an ephemeral port on ``127.0.0.1`` -- **only** ``127.0.0.1``, never
   ``0.0.0.0`` -- serving exactly one path, :data:`~allchat.settings.LOOPBACK_PATH`.
3. ``POST /device/link/start`` with that loopback URL as the redirect target.
4. Open the system browser at the returned verification URI. The streamer,
   normally already signed in, sees one approve screen and clicks Approve.
5. All-Chat redirects the browser to our loopback with ``code`` and ``state``. We
   compare ``state`` ourselves -- the server echoes it and never interprets it, so
   this comparison is our own CSRF check on our own listener.
6. Close the socket immediately and ``POST /device/link/exchange`` with the code
   and the verifier. The response carries the device token, exactly once.

The fallback, RFC 8628, for when step 2 or step 4 cannot happen -- a Stream Deck
driving a second PC, a host that will not let us bind a port, a machine with no
browser: start the flow with ``flow="code"``, show the streamer the XXXX-XXXX code
we get back, and poll the exchange endpoint. 428 means "still waiting".

WHAT THIS MODULE WILL NOT DO:

* It never binds ``0.0.0.0``. A listener on the wildcard address is reachable from
  the local network for the duration of linking, and the whole point of a loopback
  redirect is that the credential cannot leave the machine.
* It never logs the verifier, the code or the token. Not at debug level, not in an
  exception message. The only rendering of a credential anywhere in this plugin is
  :func:`allchat.settings.redact`, which emits a prefix and nothing more.
* It never keeps the socket open longer than it must. The listener closes the
  moment the code arrives or the timeout expires, whichever comes first.

Standard library only, like the rest of this plugin: ``http.server`` for the
listener, ``urllib.request`` for the two calls, ``webbrowser`` for step 4. A
StreamController plugin is loaded into the host's Python process, so every
dependency we declare is one the host has to satisfy.

KEEP IN SYNC with ``streamdeck-plugin/src/allchat/linking.ts`` (ADR-0049). Both
plugins implement one flow; a change to the request shape or the timeouts here
belongs in both files in one change.
"""

from __future__ import annotations

import base64
import hashlib
import json
import secrets
import socket
import threading
import time
import urllib.error
import urllib.parse
import urllib.request
import webbrowser
from http.server import BaseHTTPRequestHandler, HTTPServer
from typing import Any, NamedTuple

from . import errors
from .errors import AllChatError
from .settings import (
    DEVICE_PREFIX,
    LINK_EXCHANGE_PATH,
    LINK_START_PATH,
    LOOPBACK_HOST,
    LOOPBACK_PATH,
)

#: How long the loopback listener waits for the browser before giving up.
LOOPBACK_TIMEOUT_SECONDS = 180.0

#: How long the pairing-code flow polls the exchange endpoint before giving up.
CODE_FLOW_TIMEOUT_SECONDS = 600.0

#: Fallback poll interval when the server does not send one.
DEFAULT_POLL_SECONDS = 5

#: Per-request HTTP timeout for the two linking calls.
REQUEST_TIMEOUT_SECONDS = 15.0

#: Scopes this plugin asks for. The streamer may grant a subset on the approve
#: screen; the request is a request, not a grant.
REQUESTED_SCOPES = ("chat:write", "engagement:write")


class LinkResult(NamedTuple):
    """The outcome of a completed link, ready to be written into settings.

    ``token`` is a secret. Write it to the action's settings and nowhere else --
    never a log line, never an exception message.
    """

    token: str
    device_id: str
    overlay_id: str
    scopes: tuple[str, ...]
    expires_at: str


class PendingCodeLink(NamedTuple):
    """A code-flow link in progress: what to show, and how to finish it."""

    request_id: str
    #: The grouped XXXX-XXXX code to display to the streamer.
    user_code: str
    #: Where to tell the streamer to go.
    verification_uri: str
    #: Call to block until the streamer approves. Raises AllChatError on failure.
    await_completion: Any


def _generate_pkce() -> tuple[str, str]:
    """Returns ``(verifier, challenge)`` for PKCE S256 (RFC 7636).

    32 bytes from :mod:`secrets`, base64url unpadded -- 43 characters, the RFC's
    minimum length and comfortably beyond guessing. ``plain`` is not implemented at
    all, because the server rejects it: a plain challenge IS the verifier, so it
    provides none of the protection PKCE exists for in a public client.
    """
    verifier = base64.urlsafe_b64encode(secrets.token_bytes(32)).decode("ascii").rstrip("=")
    digest = hashlib.sha256(verifier.encode("ascii")).digest()
    challenge = base64.urlsafe_b64encode(digest).decode("ascii").rstrip("=")
    return verifier, challenge


def _post_json(url: str, payload: dict[str, Any]) -> tuple[int, Any]:
    """POSTs JSON and returns ``(status, parsed_body)``.

    Unlike :mod:`allchat.client`, an error status is RETURNED rather than raised:
    the code flow needs to see 428 ("still pending") as a normal outcome it keeps
    polling against, and turning that into an exception would make the loop read
    backwards.
    """
    body = json.dumps(payload).encode("utf-8")
    request = urllib.request.Request(
        url,
        data=body,
        method="POST",
        headers={"Content-Type": "application/json", "Accept": "application/json"},
    )
    try:
        with urllib.request.urlopen(request, timeout=REQUEST_TIMEOUT_SECONDS) as response:
            return response.status, _decode(response.read())
    except urllib.error.HTTPError as exc:
        return exc.code, _decode(exc.read())
    except (urllib.error.URLError, OSError, TimeoutError) as exc:
        raise AllChatError(
            errors.NETWORK, f"Could not reach All-Chat at {url}: {exc}"
        ) from exc


def _decode(raw: bytes) -> Any:
    """Decodes a JSON body, tolerating an empty or non-JSON one."""
    if not raw:
        return None
    try:
        return json.loads(raw.decode("utf-8"))
    except (ValueError, UnicodeDecodeError):
        return None


def _server_message(body: Any) -> str:
    """Extracts the server's ``error``/``message`` field, truncated for a log line."""
    if not isinstance(body, dict):
        return ""
    candidate = body.get("error") or body.get("message")
    text = candidate if isinstance(candidate, str) else ""
    return f"{text[:200]}\u2026" if len(text) > 200 else text


def _to_link_result(body: Any) -> LinkResult:
    """Validates an exchange response and shapes it for the caller."""
    token = ""
    if isinstance(body, dict):
        raw = body.get("token")
        if isinstance(raw, str):
            token = raw
    if not token.startswith(DEVICE_PREFIX):
        raise AllChatError(
            errors.HTTP_ERROR,
            "All-Chat's exchange response did not contain a device token.",
        )
    scopes = ()
    if isinstance(body, dict) and isinstance(body.get("scopes"), list):
        scopes = tuple(str(scope) for scope in body["scopes"])
    return LinkResult(
        token=token,
        device_id=str(body.get("device_id", "")) if isinstance(body, dict) else "",
        overlay_id=str(body.get("overlay_id", "")) if isinstance(body, dict) else "",
        scopes=scopes,
        expires_at=str(body.get("expires_at", "")) if isinstance(body, dict) else "",
    )


class _CallbackHandler(BaseHTTPRequestHandler):
    """Serves exactly one path and records the authorization code.

    Everything else 404s. A one-route listener is what lets the server's redirect
    validator pin a single path, and it means there is no second surface on this
    socket for anything to probe.
    """

    #: Set by the enclosing server instance before it starts.
    expected_state: str = ""
    received_code: str | None = None
    state_mismatch: bool = False

    def do_GET(self) -> None:  # noqa: N802 - BaseHTTPRequestHandler's naming
        parsed = urllib.parse.urlparse(self.path)
        if parsed.path != LOOPBACK_PATH:
            self.send_response(404)
            self.end_headers()
            return

        params = urllib.parse.parse_qs(parsed.query)
        state = params.get("state", [""])[0]
        code = params.get("code", [""])[0]

        server = self.server
        # We generated `state`, the server echoed it back untouched, and we compare
        # it here: it is our own CSRF check on our own listener, and a mismatch
        # means this request did not come from the flow we started.
        if not code or state != getattr(server, "expected_state", ""):
            setattr(server, "state_mismatch", True)
            self._reply(
                400,
                "All-Chat: this response did not match the link request that was started here.",
            )
            return

        setattr(server, "received_code", code)
        self._reply(200, "All-Chat: this control surface is linked. You can close this tab.")

    def _reply(self, status: int, text: str) -> None:
        payload = text.encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "text/plain; charset=utf-8")
        self.send_header("Content-Length", str(len(payload)))
        self.end_headers()
        self.wfile.write(payload)

    def log_message(self, format: str, *args: Any) -> None:  # noqa: A002
        """Silences BaseHTTPRequestHandler's stderr logging.

        The default implementation writes the full request line, which for this
        listener contains the authorization code. Nothing about this socket is
        logged, deliberately.
        """
        return


def open_browser(url: str) -> bool:
    """Opens the system browser at ``url``, returning False when it cannot.

    Failure is a normal outcome, not an error: a machine with no desktop session is
    exactly the case the typed pairing code exists for, so the caller falls back
    instead of failing.
    """
    try:
        return bool(webbrowser.open(url, new=2))
    except Exception:  # noqa: BLE001 - any browser-launch failure means "fall back"
        return False


def link_via_loopback(
    base_url: str,
    device_name: str,
    timeout_seconds: float = LOOPBACK_TIMEOUT_SECONDS,
) -> LinkResult:
    """Runs the loopback link end to end and returns the credential.

    Raises :class:`~allchat.errors.AllChatError` on every failure, including
    "could not bind a port" -- which the caller should treat as "offer the typed
    code instead", not as a fault.
    """
    verifier, challenge = _generate_pkce()
    state = base64.urlsafe_b64encode(secrets.token_bytes(16)).decode("ascii").rstrip("=")

    try:
        # Port 0 lets the OS pick: a plugin cannot reserve a port in advance and a
        # fixed one collides with whatever else the streamer runs. The server's
        # validator accepts any port precisely because the host is pinned to a
        # literal loopback address.
        httpd = HTTPServer((LOOPBACK_HOST, 0), _CallbackHandler)
    except OSError as exc:
        raise AllChatError(
            errors.NETWORK,
            f"Could not bind a loopback port ({exc}). Use the pairing code instead.",
        ) from exc

    setattr(httpd, "expected_state", state)
    setattr(httpd, "received_code", None)
    setattr(httpd, "state_mismatch", False)
    port = httpd.server_address[1]
    redirect_uri = f"http://{LOOPBACK_HOST}:{port}{LOOPBACK_PATH}"

    thread = threading.Thread(target=httpd.serve_forever, kwargs={"poll_interval": 0.2}, daemon=True)
    thread.start()

    try:
        status, body = _post_json(
            f"{base_url}{LINK_START_PATH}",
            {
                "flow": "loopback",
                "device_name": device_name,
                "scopes": list(REQUESTED_SCOPES),
                "code_challenge": challenge,
                "code_challenge_method": "S256",
                "redirect_uri": redirect_uri,
            },
        )
        request_id = body.get("request_id") if isinstance(body, dict) else None
        if status != 201 or not request_id:
            detail = _server_message(body)
            raise AllChatError(
                errors.HTTP_ERROR,
                f"All-Chat refused to start linking (HTTP {status}"
                + (f": {detail}" if detail else "")
                + ").",
                status,
            )

        verification_uri = _with_state(
            body.get("verification_uri", "") if isinstance(body, dict) else "", state
        )
        if not open_browser(verification_uri):
            raise AllChatError(
                errors.NETWORK,
                "Could not open a browser for the approve screen. Use the pairing code instead.",
            )

        code = _await_code(httpd, timeout_seconds)

        status, body = _post_json(
            f"{base_url}{LINK_EXCHANGE_PATH}",
            {"request_id": request_id, "code": code, "code_verifier": verifier},
        )
        if status != 200:
            detail = _server_message(body)
            raise AllChatError(
                errors.HTTP_ERROR,
                f"All-Chat refused the link exchange (HTTP {status}"
                + (f": {detail}" if detail else "")
                + ").",
                status,
            )
        return _to_link_result(body)
    finally:
        # Unconditional: the socket closes on success, on failure and on timeout. A
        # listening port that outlives the flow is exactly what ADR-0049 asks us not
        # to leave behind.
        httpd.shutdown()
        httpd.server_close()
        thread.join(timeout=2.0)


def _await_code(httpd: HTTPServer, timeout_seconds: float) -> str:
    """Waits for the listener to record a code, or raises."""
    deadline = time.monotonic() + timeout_seconds
    while time.monotonic() < deadline:
        code = getattr(httpd, "received_code", None)
        if code:
            return str(code)
        if getattr(httpd, "state_mismatch", False):
            raise AllChatError(
                errors.FORBIDDEN,
                "The approve response did not match the link request started here. "
                "Start linking again.",
            )
        time.sleep(0.1)
    raise AllChatError(
        errors.NETWORK,
        "Timed out waiting for the approve screen. Start linking again, or use the pairing code.",
    )


def link_via_code(
    base_url: str,
    device_name: str,
    timeout_seconds: float = CODE_FLOW_TIMEOUT_SECONDS,
) -> PendingCodeLink:
    """Starts the pairing-code flow.

    Returns the code to display plus a callable that blocks until the streamer
    approves. This is the path that will rot, because it is used rarely -- ADR-0049
    says so explicitly -- which is why it has tests of its own rather than a manual
    once-over.
    """
    verifier, challenge = _generate_pkce()
    status, body = _post_json(
        f"{base_url}{LINK_START_PATH}",
        {
            "flow": "code",
            "device_name": device_name,
            "scopes": list(REQUESTED_SCOPES),
            "code_challenge": challenge,
            "code_challenge_method": "S256",
        },
    )
    request_id = body.get("request_id") if isinstance(body, dict) else None
    user_code = body.get("user_code") if isinstance(body, dict) else None
    if status != 201 or not request_id or not user_code:
        detail = _server_message(body)
        raise AllChatError(
            errors.HTTP_ERROR,
            f"All-Chat refused to start linking (HTTP {status}"
            + (f": {detail}" if detail else "")
            + ").",
            status,
        )

    interval = DEFAULT_POLL_SECONDS
    if isinstance(body, dict):
        raw_interval = body.get("interval")
        if isinstance(raw_interval, int) and raw_interval > 0:
            interval = raw_interval

    def await_completion() -> LinkResult:
        deadline = time.monotonic() + timeout_seconds
        while True:
            status, payload = _post_json(
                f"{base_url}{LINK_EXCHANGE_PATH}",
                {
                    "request_id": request_id,
                    "user_code": user_code,
                    "code_verifier": verifier,
                },
            )
            if status == 200:
                return _to_link_result(payload)
            # 428 is the server's "still pending" -- the streamer has not clicked
            # Approve yet. Anything else is terminal and the plugin must stop rather
            # than hammer an endpoint that will keep refusing.
            if status != 428:
                detail = _server_message(payload)
                raise AllChatError(
                    errors.HTTP_ERROR,
                    f"All-Chat refused the link exchange (HTTP {status}"
                    + (f": {detail}" if detail else "")
                    + ").",
                    status,
                )
            if time.monotonic() + interval > deadline:
                raise AllChatError(
                    errors.HTTP_ERROR,
                    "The pairing code expired before it was approved. Start linking again.",
                )
            time.sleep(interval)

    return PendingCodeLink(
        request_id=str(request_id),
        user_code=str(user_code),
        verification_uri=str(body.get("verification_uri", "")) if isinstance(body, dict) else "",
        await_completion=await_completion,
    )


def _with_state(verification_uri: str, state: str) -> str:
    """Appends our ``state`` to the verification URI so it survives the round trip."""
    if not verification_uri:
        return verification_uri
    separator = "&" if "?" in verification_uri else "?"
    return f"{verification_uri}{separator}state={urllib.parse.quote(state, safe='')}"


def loopback_available() -> bool:
    """Reports whether a loopback port can be bound at all.

    Called before offering the primary path, so a sandboxed host offers the typed
    code straight away instead of failing halfway through and confusing the
    streamer. Binds and immediately releases; it never leaves a socket behind.
    """
    try:
        with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as probe:
            probe.bind((LOOPBACK_HOST, 0))
        return True
    except OSError:
        return False
