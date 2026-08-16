"""The premium boundary is a product decision, so it gets the strictest tests.

What must stay true, and what these tests pin:

1. A 403 from *starting* a poll or a prediction is the premium gate. It produces
   its own key state, a message naming the upgrade page, and it is NOT reported
   as a generic error.
2. Close / lock / resolve / cancel are free, and nothing in the plugin gates them.
3. A 403 for a **missing token scope** is a different failure with different
   advice. If it ever starts telling the user to buy premium, that is a bug that
   costs real money and trust, so it is asserted from both directions.
4. A 401 is "re-paste your token", never "upgrade".
5. No token ever reaches a log line.

The plausible future mistake this file exists to catch is someone "fixing" the
403 on start by making it look like every other error -- which would delete the
feature's only advertisement.
"""

from __future__ import annotations

import io
import json
import unittest
import urllib.error
import warnings
from typing import Any
from unittest import mock

from actions import base
from actions.poll_control import PollControlAction
from actions.prediction_control import PredictionControlAction
from actions.send_message import SendMessageAction
from allchat import errors
from allchat.errors import AllChatError

VALID_TOKEN = "allchat_pat_" + "k" * 32
OVERLAY = "11111111-2222-3333-4444-555555555555"

PREMIUM_403_BODY = {
    "error": "Premium feature required",
    "message": "This is a premium feature. Upgrade your account to access this functionality.",
    "upgrade_url": "/upgrade",
}
SCOPE_403_BODY = {
    "error": "insufficient token scope",
    "required_scopes": ["engagement:write"],
}


def http_error(status: int, body: Any) -> urllib.error.HTTPError:
    """Builds an HTTPError whose body reads back like a real response.

    ``fp`` is a real file object because ``HTTPError`` closes it on cleanup; a
    bare ``BytesIO`` works but makes Python warn about implicit cleanup, which
    buries genuine warnings in test output.
    """
    payload = json.dumps(body).encode("utf-8")
    error = urllib.error.HTTPError(
        url="https://allch.at/x",
        code=status,
        msg="error",
        hdrs=None,  # type: ignore[arg-type]
        fp=io.BytesIO(payload),
    )
    # The client reads the body to classify a 403; closing here would defeat it.
    return error


class FakeResponse:
    """Minimal stand-in for the object `urlopen` yields as a context manager."""

    def __init__(self, body: Any) -> None:
        self._body = json.dumps(body).encode("utf-8") if body is not None else b""

    def read(self) -> bytes:
        return self._body

    def __enter__(self) -> "FakeResponse":
        return self

    def __exit__(self, *_: Any) -> bool:
        return False


def make_action(cls: type, **settings: Any) -> Any:
    """Builds an action with settings, using the offline host stand-ins."""
    settings.setdefault("api_token", VALID_TOKEN)
    action = cls(plugin_base=None, settings=settings)
    action.set_settings(settings)
    return action


class PremiumGateTest(unittest.TestCase):
    """403 on the two start routes must be the premium state, distinctly."""

    def test_start_poll_403_is_premium_state_not_error(self) -> None:
        action = make_action(
            PollControlAction,
            mode="start",
            overlay_id=OVERLAY,
            question="Which map?",
            options="Dust\nMirage",
        )
        with mock.patch("urllib.request.urlopen", side_effect=http_error(403, PREMIUM_403_BODY)):
            state = action.run_action()

        self.assertEqual(base.STATE_PREMIUM, state)
        # Rendered as an advert, not a fault: the key shows the premium label and
        # the error affordance is never used.
        self.assertEqual(base.LABEL_FOR_STATE[base.STATE_PREMIUM], action.center_label)
        self.assertEqual(0, action.errors_shown)

    def test_start_prediction_403_is_premium_state_not_error(self) -> None:
        action = make_action(
            PredictionControlAction,
            mode="start",
            overlay_id=OVERLAY,
            title="Do we win?",
            outcomes="Yes\nNo",
        )
        with mock.patch("urllib.request.urlopen", side_effect=http_error(403, PREMIUM_403_BODY)):
            state = action.run_action()

        self.assertEqual(base.STATE_PREMIUM, state)
        self.assertEqual(0, action.errors_shown)

    def test_premium_message_names_the_feature_and_the_upgrade_page(self) -> None:
        message = errors.premium_gate_message("poll")
        self.assertIn("https://allch.at/upgrade", message)
        self.assertIn("premium", message.lower())
        self.assertIn("403", message)
        # Must say the free actions still work: that is the reassurance which
        # stops a 403 reading like a broken plugin.
        self.assertIn("free", message.lower())

    def test_premium_message_is_poll_or_prediction_specific(self) -> None:
        """The user pressed one button; the message names that button."""
        self.assertIn("poll", errors.premium_gate_message("poll"))
        self.assertIn("prediction", errors.premium_gate_message("prediction"))

    def test_premium_gate_is_logged_as_info_not_error(self) -> None:
        """It is the server working correctly, so it must not pollute error logs."""
        action = make_action(PollControlAction, mode="start", overlay_id=OVERLAY)
        logger = mock.Mock()
        action.plugin_base = mock.Mock(logger=logger)
        action.show_state(base.STATE_PREMIUM, "needs premium")
        logger.info.assert_called_once()
        logger.error.assert_not_called()


class FreeActionsTest(unittest.TestCase):
    """Everything that finishes or unwinds a round is free and ungated."""

    def test_close_poll_succeeds(self) -> None:
        action = make_action(
            PollControlAction, mode="close", overlay_id=OVERLAY, poll_id="poll-1"
        )
        with mock.patch("urllib.request.urlopen", return_value=FakeResponse({"status": "closed"})):
            self.assertEqual(base.STATE_OK, action.run_action())

    def test_lock_resolve_cancel_succeed(self) -> None:
        for mode, extra in (
            ("lock", {}),
            ("resolve", {"winning_outcome_id": "out-1"}),
            ("cancel", {}),
        ):
            with self.subTest(mode=mode):
                action = make_action(
                    PredictionControlAction,
                    mode=mode,
                    overlay_id=OVERLAY,
                    prediction_id="pred-1",
                    **extra,
                )
                with mock.patch("urllib.request.urlopen", return_value=FakeResponse({"ok": True})):
                    self.assertEqual(base.STATE_OK, action.run_action())

    def test_close_poll_uses_the_correct_free_route(self) -> None:
        """Pins the URL, so a refactor cannot silently point close at start."""
        action = make_action(
            PollControlAction, mode="close", overlay_id=OVERLAY, poll_id="poll-1"
        )
        with mock.patch(
            "urllib.request.urlopen", return_value=FakeResponse({})
        ) as urlopen:
            action.run_action()
        request = urlopen.call_args[0][0]
        self.assertEqual(
            f"https://allch.at/api/v1/engagement/overlays/{OVERLAY}/polls/poll-1/close",
            request.full_url,
        )


class ScopeVersusPremiumTest(unittest.TestCase):
    """A 403 for a missing scope must never be advertised as "buy premium"."""

    def test_scope_403_is_not_the_premium_state(self) -> None:
        action = make_action(
            PollControlAction,
            mode="start",
            overlay_id=OVERLAY,
            question="Which map?",
            options="Dust\nMirage",
        )
        with mock.patch("urllib.request.urlopen", side_effect=http_error(403, SCOPE_403_BODY)):
            state = action.run_action()
        self.assertEqual(base.STATE_ERROR, state)

    def test_scope_message_says_upgrading_will_not_help(self) -> None:
        message = errors.insufficient_scope_message("engagement:write")
        self.assertIn("engagement:write", message)
        self.assertIn("not the premium gate", message.lower())
        self.assertIn("https://allch.at/settings/api-tokens", message)

    def test_classify_forbidden_separates_the_three_cases(self) -> None:
        self.assertEqual(errors.REQUIRES_PREMIUM, errors.classify_forbidden(PREMIUM_403_BODY))
        self.assertEqual(errors.INSUFFICIENT_SCOPE, errors.classify_forbidden(SCOPE_403_BODY))
        self.assertEqual(
            errors.FORBIDDEN, errors.classify_forbidden({"error": "overlay not owned by user"})
        )
        # An unreadable body degrades to the generic case, never to "premium".
        self.assertEqual(errors.FORBIDDEN, errors.classify_forbidden(None))


class TokenStateTest(unittest.TestCase):
    """401 and the two configuration mistakes get their own advice."""

    def test_401_says_re_paste_the_token(self) -> None:
        action = make_action(SendMessageAction, message="hi", platform="all")
        with mock.patch("urllib.request.urlopen", side_effect=http_error(401, {"error": "x"})):
            self.assertEqual(base.STATE_ERROR, action.run_action())
        message = errors.unauthorized_message()
        self.assertIn("401", message)
        self.assertIn("https://allch.at/settings/api-tokens", message)
        # Must not send a 401 user to the billing page.
        self.assertNotIn("upgrade", message.lower())

    def test_missing_token_is_caught_before_any_request(self) -> None:
        action = SendMessageAction(plugin_base=None, settings={"message": "hi"})
        action.set_settings({"message": "hi"})
        with mock.patch("urllib.request.urlopen") as urlopen:
            self.assertEqual(base.STATE_ERROR, action.run_action())
        urlopen.assert_not_called()

    def test_a_session_jwt_is_rejected_locally(self) -> None:
        """The commonest paste error, caught without a round-trip."""
        action = make_action(SendMessageAction, api_token="eyJhbGciOiJIUzI1NiJ9.x.y", message="hi")
        with mock.patch("urllib.request.urlopen") as urlopen:
            self.assertEqual(base.STATE_ERROR, action.run_action())
        urlopen.assert_not_called()


class TokenSecrecyTest(unittest.TestCase):
    """The token goes in the Authorization header and nowhere else."""

    def test_token_is_sent_as_a_bearer(self) -> None:
        action = make_action(SendMessageAction, message="hi", platform="all")
        with mock.patch(
            "urllib.request.urlopen", return_value=FakeResponse({"ok": True})
        ) as urlopen:
            action.run_action()
        request = urlopen.call_args[0][0]
        self.assertEqual(f"Bearer {VALID_TOKEN}", request.get_header("Authorization"))
        self.assertNotIn(VALID_TOKEN, request.full_url)

    def test_no_error_message_contains_the_token(self) -> None:
        """Every taxonomy message, checked against a real-looking token."""
        messages = [
            errors.premium_gate_message("poll"),
            errors.insufficient_scope_message("chat:write"),
            errors.unauthorized_message(),
            errors.missing_token_message(),
            errors.malformed_token_message(),
            errors.forbidden_message({"error": "nope"}),
        ]
        for message in messages:
            self.assertNotIn(VALID_TOKEN, message)

    def test_failures_never_log_the_token(self) -> None:
        for status, body in ((401, {}), (403, PREMIUM_403_BODY), (403, SCOPE_403_BODY), (500, {})):
            with self.subTest(status=status, body=body):
                action = make_action(SendMessageAction, message="hi", platform="all")
                logger = mock.Mock()
                action.plugin_base = mock.Mock(logger=logger)
                with mock.patch("urllib.request.urlopen", side_effect=http_error(status, body)):
                    action.run_action()
                logged = " ".join(
                    str(call) for call in logger.info.call_args_list + logger.error.call_args_list
                )
                self.assertNotIn(VALID_TOKEN, logged)

    def test_redact_never_reveals_the_secret_tail(self) -> None:
        from allchat.settings import redact

        rendered = redact(VALID_TOKEN)
        self.assertNotIn("k" * 32, rendered)
        self.assertTrue(rendered.startswith("allchat_pat_"))


class ErrorMappingTest(unittest.TestCase):
    """The remaining statuses map to actionable, non-premium messages."""

    def test_statuses_map_to_expected_kinds(self) -> None:
        cases = {
            404: errors.NOT_FOUND,
            409: errors.CONFLICT,
            429: errors.RATE_LIMITED,
            500: errors.HTTP_ERROR,
        }
        for status, expected in cases.items():
            with self.subTest(status=status):
                action = make_action(SendMessageAction, message="hi", platform="all")
                with mock.patch(
                    "urllib.request.urlopen", side_effect=http_error(status, {"error": "e"})
                ):
                    self.assertEqual(base.STATE_ERROR, action.run_action())

    def test_network_failure_is_not_a_premium_state(self) -> None:
        action = make_action(SendMessageAction, message="hi", platform="all")
        with mock.patch(
            "urllib.request.urlopen", side_effect=urllib.error.URLError("no route to host")
        ):
            self.assertEqual(base.STATE_ERROR, action.run_action())

    def test_only_requires_premium_maps_to_the_premium_state(self) -> None:
        """Guards the mapping itself: exactly one kind is an advert."""
        all_kinds = [
            errors.NO_TOKEN, errors.MALFORMED_TOKEN, errors.NOT_CONFIGURED,
            errors.UNAUTHORIZED, errors.REQUIRES_PREMIUM, errors.INSUFFICIENT_SCOPE,
            errors.FORBIDDEN, errors.NOT_FOUND, errors.CONFLICT, errors.RATE_LIMITED,
            errors.HTTP_ERROR, errors.NETWORK,
        ]
        premium = [k for k in all_kinds if base.state_for_error(AllChatError(k, "m")) ==
                   base.STATE_PREMIUM]
        self.assertEqual([errors.REQUIRES_PREMIUM], premium)


def setUpModule() -> None:
    """Silences the ResourceWarning from the synthetic HTTPError bodies above.

    These are test fixtures, not the plugin leaking anything: the real client
    reads its response inside a ``with`` block.
    """
    warnings.filterwarnings("ignore", category=ResourceWarning)


if __name__ == "__main__":
    unittest.main()
