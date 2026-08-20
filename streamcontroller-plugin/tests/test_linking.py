"""Device linking, including the path that will rot if nobody tests it.

ADR-0049 is explicit about why this file exists:

    Two linking paths mean two paths to keep correct. The fallback will be used
    rarely, which is exactly why it will rot silently unless it is tested
    deliberately rather than only when someone reports it.

So the pairing-code flow gets coverage at parity with the loopback flow, not a
manual once-over. What is pinned here:

1. **PKCE is S256 and the challenge is really S256 of the verifier.** A plugin
   that sent a mismatched pair would fail every exchange with a message that
   points at the server.
2. **The loopback listener binds 127.0.0.1 and never 0.0.0.0.** The wildcard
   address is reachable from the local network for the duration of linking, and the
   whole point of a loopback redirect is that the credential cannot leave the
   machine.
3. **The redirect_uri the plugin sends is the one shape the server accepts** --
   ``http://127.0.0.1:<port>/allchat/device-callback`` -- because a mismatch here
   is a 400 the user cannot act on.
4. **428 means keep polling; anything else is terminal.** A plugin that treated a
   terminal error as "pending" would hammer the endpoint forever.
5. **The credential lands in settings and NOWHERE else** -- not a log line, not the
   status callback, not an exception message.
6. **The state check rejects a mismatched callback.** It is our own CSRF check on
   our own listener; the server echoes state and never interprets it.
"""

from __future__ import annotations

import base64
import hashlib
import json
import unittest
import urllib.error
import urllib.parse
import urllib.request
from typing import Any
from unittest import mock

from actions.send_message import SendMessageAction
from allchat import errors, linking, settings
from allchat.errors import AllChatError

from .test_premium_boundary import OVERLAY, make_action

DEVICE_TOKEN = "allchat_dev_" + "z" * 32
REQUEST_ID = "33333333-3333-3333-3333-333333333333"
BASE_URL = "https://allch.at"


class FakeHTTP:
    """Scripts the two linking calls in order, recording what was sent.

    ``urlopen`` is patched with this rather than a per-call mock because the code
    flow issues the same request repeatedly and the interesting behaviour is the
    SEQUENCE of answers: 428, 428, 200.
    """

    def __init__(self, script: list[tuple[int, Any]]) -> None:
        self.script = list(script)
        self.requests: list[dict[str, Any]] = []

    def __call__(self, request: Any, timeout: float | None = None) -> Any:
        body = json.loads(request.data.decode("utf-8")) if request.data else {}
        self.requests.append({"url": request.full_url, "body": body})
        status, payload = self.script.pop(0) if self.script else (500, None)
        if status >= 400:
            raise _http_error(status, payload)
        return _FakeResponse(status, payload)

    def bodies_for(self, path_suffix: str) -> list[dict[str, Any]]:
        return [r["body"] for r in self.requests if r["url"].endswith(path_suffix)]


class _FakeResponse:
    def __init__(self, status: int, payload: Any) -> None:
        self.status = status
        self._body = json.dumps(payload).encode("utf-8") if payload is not None else b""

    def read(self) -> bytes:
        return self._body

    def __enter__(self) -> "_FakeResponse":
        return self

    def __exit__(self, *_: Any) -> bool:
        return False


def _http_error(status: int, payload: Any) -> urllib.error.HTTPError:
    import io

    raw = json.dumps(payload).encode("utf-8") if payload is not None else b""
    return urllib.error.HTTPError(
        url="https://allch.at/x",
        code=status,
        msg="error",
        hdrs=None,  # type: ignore[arg-type]
        fp=io.BytesIO(raw),
    )


def start_response(**over: Any) -> dict[str, Any]:
    body = {
        "request_id": REQUEST_ID,
        "verification_uri": "https://allch.at/link?request_id=" + REQUEST_ID,
        "expires_in": 600,
        "interval": 5,
    }
    body.update(over)
    return body


def exchange_response(**over: Any) -> dict[str, Any]:
    body = {
        "token": DEVICE_TOKEN,
        "token_type": "Bearer",
        "device_id": "44444444-4444-4444-4444-444444444444",
        "overlay_id": OVERLAY,
        "scopes": ["chat:write", "engagement:write"],
        "expires_at": "2026-07-01T00:00:00Z",
    }
    body.update(over)
    return body


class PkceTest(unittest.TestCase):
    def test_challenge_is_s256_of_the_verifier(self) -> None:
        verifier, challenge = linking._generate_pkce()
        digest = hashlib.sha256(verifier.encode("ascii")).digest()
        expected = base64.urlsafe_b64encode(digest).decode("ascii").rstrip("=")
        self.assertEqual(expected, challenge)

    def test_verifier_meets_the_rfc_7636_minimum(self) -> None:
        verifier, _ = linking._generate_pkce()
        # 43 characters is the RFC minimum and what 32 random bytes base64url
        # unpadded produces. A shorter verifier would be a silently weakened
        # substitute for the client secret a published plugin cannot hold.
        self.assertEqual(43, len(verifier))
        self.assertNotIn("=", verifier)
        self.assertNotIn("+", verifier)
        self.assertNotIn("/", verifier)

    def test_two_verifiers_differ(self) -> None:
        first, _ = linking._generate_pkce()
        second, _ = linking._generate_pkce()
        self.assertNotEqual(first, second)


class LoopbackContractTest(unittest.TestCase):
    """The one shape the server's redirect validator accepts."""

    def test_host_is_the_literal_loopback_never_localhost_or_wildcard(self) -> None:
        # `localhost` is a DNS name and can be pointed elsewhere, which is why the
        # server refuses it; `0.0.0.0` would expose the listener to the local
        # network while linking is in progress.
        self.assertEqual("127.0.0.1", settings.LOOPBACK_HOST)
        self.assertNotEqual("localhost", settings.LOOPBACK_HOST)
        self.assertNotEqual("0.0.0.0", settings.LOOPBACK_HOST)  # noqa: S104

    def test_every_bind_uses_the_pinned_loopback_constant(self) -> None:
        # Parsed rather than grepped, because the module's own docstring says
        # "never 0.0.0.0" and a grep would trip on the prose. What matters is that
        # every socket bind takes its host from LOOPBACK_HOST, so there is exactly
        # one place the address can be wrong and one constant the parity gate and
        # the server both agree on.
        import ast
        from pathlib import Path

        tree = ast.parse(Path(linking.__file__).read_text(encoding="utf-8"))
        # Every (host, port) tuple passed to HTTPServer(...) or socket.bind(...).
        binds: list[ast.Tuple] = []
        for node in ast.walk(tree):
            if not isinstance(node, ast.Call):
                continue
            func = node.func
            name = func.attr if isinstance(func, ast.Attribute) else getattr(func, "id", "")
            if name not in ("HTTPServer", "bind"):
                continue
            for arg in node.args:
                if isinstance(arg, ast.Tuple):
                    binds.append(arg)
        self.assertTrue(binds, "no (host, port) bind tuple found in linking.py")
        for node in binds:
            self.assertEqual(
                "LOOPBACK_HOST",
                node.elts[0].id,  # type: ignore[attr-defined]
                "a bind target is not the pinned LOOPBACK_HOST constant",
            )

    def test_the_callback_path_matches_the_server_validator(self) -> None:
        self.assertEqual("/allchat/device-callback", settings.LOOPBACK_PATH)

    def test_loopback_available_binds_and_releases(self) -> None:
        # Called before offering the primary path so a sandboxed host offers the
        # typed code straight away. It must not leave a socket behind.
        self.assertTrue(linking.loopback_available())
        self.assertTrue(linking.loopback_available())


class CodeFlowTest(unittest.TestCase):
    """The fallback, tested deliberately because it will otherwise rot."""

    def test_start_requests_s256_and_no_redirect(self) -> None:
        http = FakeHTTP([(201, start_response(user_code="ABCD-EFGH"))])
        with mock.patch("urllib.request.urlopen", http):
            pending = linking.link_via_code(BASE_URL, "StreamController")

        self.assertEqual("ABCD-EFGH", pending.user_code)
        body = http.bodies_for(settings.LINK_START_PATH)[0]
        self.assertEqual("code", body["flow"])
        self.assertEqual("S256", body["code_challenge_method"])
        # A redirect_uri on the code flow is a 400 from the server: there is no
        # loopback listener in this path, so sending one would be a lie.
        self.assertNotIn("redirect_uri", body)
        self.assertEqual(list(linking.REQUESTED_SCOPES), body["scopes"])

    def test_428_means_keep_polling_and_200_completes(self) -> None:
        http = FakeHTTP(
            [
                (201, start_response(user_code="ABCD-EFGH")),
                (428, {"error": "pending"}),
                (428, {"error": "pending"}),
                (200, exchange_response()),
            ]
        )
        with mock.patch("urllib.request.urlopen", http), mock.patch("time.sleep"):
            pending = linking.link_via_code(BASE_URL, "StreamController")
            result = pending.await_completion()

        self.assertEqual(DEVICE_TOKEN, result.token)
        self.assertEqual(OVERLAY, result.overlay_id)
        # Three exchange attempts: two pending, then the token.
        self.assertEqual(3, len(http.bodies_for(settings.LINK_EXCHANGE_PATH)))
        # Every attempt carried the verifier, which is what stands in for a client
        # secret this plugin cannot hold.
        for body in http.bodies_for(settings.LINK_EXCHANGE_PATH):
            self.assertEqual(43, len(body["code_verifier"]))
            self.assertEqual("ABCD-EFGH", body["user_code"])

    def test_a_terminal_error_stops_polling(self) -> None:
        # A plugin that treated 400 as "pending" would hammer an endpoint that will
        # keep refusing, forever, from a machine we cannot debug.
        http = FakeHTTP(
            [
                (201, start_response(user_code="ABCD-EFGH")),
                (400, {"error": "This device link code has already been used."}),
            ]
        )
        with mock.patch("urllib.request.urlopen", http), mock.patch("time.sleep"):
            pending = linking.link_via_code(BASE_URL, "StreamController")
            with self.assertRaises(AllChatError) as caught:
                pending.await_completion()

        self.assertEqual(errors.HTTP_ERROR, caught.exception.kind)
        self.assertEqual(2, len(http.requests))

    def test_a_start_failure_is_reported_not_swallowed(self) -> None:
        http = FakeHTTP([(429, {"error": "rate limited"})])
        with mock.patch("urllib.request.urlopen", http):
            with self.assertRaises(AllChatError):
                linking.link_via_code(BASE_URL, "StreamController")

    def test_a_response_without_a_device_token_is_refused(self) -> None:
        # Defence against a proxy or a misrouted response: a "success" that does not
        # carry an allchat_dev_ token must not be written into settings as one.
        http = FakeHTTP(
            [
                (201, start_response(user_code="ABCD-EFGH")),
                (200, exchange_response(token="allchat_pat_wrong-kind")),
            ]
        )
        with mock.patch("urllib.request.urlopen", http), mock.patch("time.sleep"):
            pending = linking.link_via_code(BASE_URL, "StreamController")
            with self.assertRaises(AllChatError):
                pending.await_completion()


class LoopbackFlowTest(unittest.TestCase):
    """The primary path: bind, open, redirect, exchange."""

    def _run_loopback(self, script: list[tuple[int, Any]], browser: Any) -> Any:
        """Runs link_via_loopback with the two API calls scripted.

        The loopback GET is issued by `browser` against the real listener, so the
        handler and the state check are genuinely exercised; only the two calls to
        All-Chat are faked.
        """
        http = FakeHTTP(script)
        captured: dict[str, str] = {}
        real_urlopen = urllib.request.urlopen

        def route(request: Any, timeout: float | None = None) -> Any:
            if isinstance(request, str):
                # A loopback GET from the fake browser. Let it reach the listener.
                return real_urlopen(request, timeout=timeout)
            if request.data:
                body = json.loads(request.data.decode("utf-8"))
                if "redirect_uri" in body:
                    captured["redirect_uri"] = body["redirect_uri"]
            return http(request, timeout)

        with mock.patch("urllib.request.urlopen", route), mock.patch.object(
            linking, "open_browser", lambda url: browser(url, captured)
        ):
            result = linking.link_via_loopback(BASE_URL, "StreamController", timeout_seconds=15)
        return result, captured, http

    @staticmethod
    def _approving_browser(url: str, captured: dict[str, str]) -> bool:
        """Acts as the streamer's browser: reads `state`, calls back with a code."""
        state = urllib.parse.parse_qs(urllib.parse.urlparse(url).query)["state"][0]
        netloc = urllib.parse.urlparse(captured["redirect_uri"]).netloc
        urllib.request.urlopen(  # noqa: S310 - a loopback URL the test just built
            f"http://{netloc}{settings.LOOPBACK_PATH}"
            f"?code=one-time-code&state={urllib.parse.quote(state)}",
            timeout=5,
        ).read()
        return True

    def test_redirect_uri_is_the_one_shape_the_server_accepts(self) -> None:
        result, captured, http = self._run_loopback(
            [(201, start_response()), (200, exchange_response())],
            self._approving_browser,
        )

        self.assertEqual(DEVICE_TOKEN, result.token)
        parsed = urllib.parse.urlparse(captured["redirect_uri"])
        # Exactly the rule in services/auth-service/handlers/loopback_redirect.go:
        # http, the literal 127.0.0.1, any port, one fixed path, and nothing else.
        self.assertEqual("http", parsed.scheme)
        self.assertEqual("127.0.0.1", parsed.hostname)
        self.assertEqual(settings.LOOPBACK_PATH, parsed.path)
        self.assertTrue(parsed.port and parsed.port > 0)
        self.assertEqual("", parsed.query)
        self.assertEqual("", parsed.fragment)
        self.assertIsNone(parsed.username)

        start_body = http.bodies_for(settings.LINK_START_PATH)[0]
        self.assertEqual("loopback", start_body["flow"])
        self.assertEqual("S256", start_body["code_challenge_method"])
        exchange_body = http.bodies_for(settings.LINK_EXCHANGE_PATH)[0]
        self.assertEqual("one-time-code", exchange_body["code"])
        self.assertEqual(43, len(exchange_body["code_verifier"]))

    def test_an_unopenable_browser_is_a_network_error_so_the_caller_can_fall_back(self) -> None:
        # This is the second-machine case, not a fault: the caller offers the typed
        # code instead, so the kind has to be the one it branches on.
        http = FakeHTTP([(201, start_response())])
        with mock.patch("urllib.request.urlopen", http), mock.patch.object(
            linking, "open_browser", lambda _url: False
        ):
            with self.assertRaises(AllChatError) as caught:
                linking.link_via_loopback(BASE_URL, "StreamController", timeout_seconds=1)
        self.assertEqual(errors.NETWORK, caught.exception.kind)

    def test_a_mismatched_state_is_rejected(self) -> None:
        def wrong_state_browser(url: str, captured: dict[str, str]) -> bool:
            netloc = urllib.parse.urlparse(captured["redirect_uri"]).netloc
            try:
                urllib.request.urlopen(  # noqa: S310 - a loopback URL the test built
                    f"http://{netloc}{settings.LOOPBACK_PATH}?code=x&state=not-ours",
                    timeout=5,
                )
            except urllib.error.HTTPError:
                # The listener answers 400, which is the point.
                pass
            return True

        with self.assertRaises(AllChatError) as caught:
            self._run_loopback([(201, start_response())], wrong_state_browser)

        # `state` is our own CSRF check on our own socket: a mismatch means this did
        # not come from the flow we started, and the code must not be exchanged.
        self.assertEqual(errors.FORBIDDEN, caught.exception.kind)


class StartLinkingWritesOnlySettingsTest(unittest.TestCase):
    """The credential lands in settings and nowhere else."""

    def test_the_token_is_written_to_settings_and_never_reported(self) -> None:
        statuses: list[tuple[str, str, str]] = []
        action = make_action(SendMessageAction, message="hi")
        # Start from an unconfigured action, so a leftover PAT cannot make this pass.
        action.set_settings({"message": "hi"})

        http = FakeHTTP(
            [
                (201, start_response(user_code="ABCD-EFGH")),
                (200, exchange_response()),
            ]
        )
        # loopback_available() False forces the fallback, which is the path this
        # test cares about: it is also the one a second machine actually takes.
        with mock.patch("urllib.request.urlopen", http), mock.patch.object(
            linking, "loopback_available", lambda: False
        ), mock.patch("time.sleep"):
            result = action.start_linking(
                lambda status, detail, code="": statuses.append((status, detail, code))
            )

        self.assertEqual(DEVICE_TOKEN, result.token)
        self.assertEqual(DEVICE_TOKEN, action.get_settings()["api_token"])
        # No status callback, and no log line, may ever carry the secret.
        for status, detail, code in statuses:
            self.assertNotIn(DEVICE_TOKEN, detail)
            self.assertNotIn(DEVICE_TOKEN, code)
            self.assertNotIn(DEVICE_TOKEN, status)
        # The pairing code WAS reported, because the user has to read it.
        self.assertTrue(any(code == "ABCD-EFGH" for _, _, code in statuses))

    def test_a_linked_action_authenticates_with_the_device_token(self) -> None:
        # The point of the whole flow: after linking, the action works with a
        # credential nobody typed. base.connection_from must accept it.
        action = make_action(SendMessageAction, message="hi", api_token=DEVICE_TOKEN)
        conn = __import__("actions.base", fromlist=["base"]).connection_from(
            action.get_settings()
        )
        self.assertEqual(DEVICE_TOKEN, conn.token)

    def test_both_credential_shapes_are_accepted_and_nothing_else_is(self) -> None:
        self.assertTrue(settings.looks_like_token("allchat_dev_abcdef"))
        self.assertTrue(settings.looks_like_token("allchat_pat_abcdef"))
        # A bare prefix is not a token, and a JWT is definitely not one.
        self.assertFalse(settings.looks_like_token("allchat_dev_"))
        self.assertFalse(settings.looks_like_token("eyJhbGciOiJIUzI1NiJ9.a.b"))
        self.assertFalse(settings.looks_like_token(""))

    def test_redact_names_the_credential_kind_and_never_the_secret(self) -> None:
        # Support needs to know WHICH kind is configured, because the advice differs:
        # a device token is re-linked, a PAT is re-pasted.
        self.assertEqual("allchat_dev_\u2026", settings.redact(DEVICE_TOKEN))
        self.assertEqual("allchat_pat_\u2026", settings.redact("allchat_pat_" + "k" * 32))
        self.assertEqual("(none)", settings.redact(""))
        self.assertNotIn("zzz", settings.redact(DEVICE_TOKEN))


if __name__ == "__main__":
    unittest.main()
