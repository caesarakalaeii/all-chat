"""Route shapes, defaults and input normalisation.

The routes are pinned as literal strings because they are a contract with a
server this plugin cannot see at build time. If somebody renames a path here, the
only signal is a 404 during a live stream, so the assertions are deliberately
exact rather than pattern-based.
"""

from __future__ import annotations

import json
import unittest
from typing import Any
from unittest import mock

from actions import base
from actions.poll_control import PollControlAction
from actions.prediction_control import PredictionControlAction
from actions.send_message import SendMessageAction
from allchat import api, settings

from .test_premium_boundary import FakeResponse, OVERLAY, VALID_TOKEN, make_action


def captured_request(action: Any, payload: Any = None) -> Any:
    """Runs an action against a stubbed transport and returns the Request."""
    with mock.patch("urllib.request.urlopen", return_value=FakeResponse(payload)) as urlopen:
        action.run_action()
    return urlopen.call_args[0][0]


class DefaultsTest(unittest.TestCase):
    def test_production_host_is_the_default(self) -> None:
        """Confirmed as FRONTEND_URL in deployments/k8s/base/configmap.yaml."""
        self.assertEqual("https://allch.at", settings.DEFAULT_BASE_URL)
        self.assertEqual("https://allch.at", settings.resolve_base_url({}))
        self.assertEqual("https://allch.at", settings.resolve_base_url(None))
        self.assertEqual("https://allch.at", settings.resolve_base_url({"base_url": "  "}))

    def test_self_hosters_can_override_the_base_url(self) -> None:
        self.assertEqual(
            "https://chat.example.org",
            settings.resolve_base_url({"base_url": "https://chat.example.org/"}),
        )
        # Repeated trailing slashes must not produce a double slash in a path.
        self.assertEqual(
            "https://chat.example.org",
            settings.resolve_base_url({"base_url": "https://chat.example.org///"}),
        )

    def test_pat_prefix_matches_the_server_contract(self) -> None:
        """ADR-0051 freezes this prefix; the server routes on it."""
        self.assertEqual("allchat_pat_", settings.PAT_PREFIX)
        self.assertTrue(settings.looks_like_pat("allchat_pat_abcdef"))
        self.assertFalse(settings.looks_like_pat("allchat_pat_"))  # prefix alone is not a token
        self.assertFalse(settings.looks_like_pat("eyJhbGciOiJIUzI1NiJ9.a.b"))
        self.assertFalse(settings.looks_like_pat(""))


class RouteTest(unittest.TestCase):
    """Every endpoint this plugin calls, pinned exactly."""

    def test_chat_send_route(self) -> None:
        self.assertEqual("/api/v1/auth/chat/send", api.CHAT_SEND_PATH)
        action = make_action(SendMessageAction, message="hello", platform="twitch")
        request = captured_request(action, {"ok": True})
        self.assertEqual("https://allch.at/api/v1/auth/chat/send", request.full_url)
        self.assertEqual("POST", request.get_method())
        body = json.loads(request.data)
        # Both fields are `binding:"required"` in auth-service.
        self.assertEqual({"message": "hello", "platform": "twitch"}, body)

    def test_start_poll_route(self) -> None:
        action = make_action(
            PollControlAction, mode="start", overlay_id=OVERLAY,
            question="Which map?", options="Dust\nMirage", duration_seconds="60",
        )
        request = captured_request(action, {"id": "poll-9"})
        self.assertEqual(
            f"https://allch.at/api/v1/engagement/overlays/{OVERLAY}/polls", request.full_url
        )
        self.assertEqual(
            {"question": "Which map?", "options": ["Dust", "Mirage"], "duration_seconds": 60},
            json.loads(request.data),
        )

    def test_start_prediction_route(self) -> None:
        action = make_action(
            PredictionControlAction, mode="start", overlay_id=OVERLAY,
            title="Do we win?", outcomes="Yes, No", auto_lock_seconds="120",
        )
        request = captured_request(action, {"id": "pred-9"})
        self.assertEqual(
            f"https://allch.at/api/v1/engagement/overlays/{OVERLAY}/predictions",
            request.full_url,
        )
        self.assertEqual(
            {"title": "Do we win?", "outcomes": ["Yes", "No"], "auto_lock_seconds": 120},
            json.loads(request.data),
        )

    def test_prediction_transition_routes(self) -> None:
        for mode, verb, extra in (
            ("lock", "lock", {}),
            ("resolve", "resolve", {"winning_outcome_id": "out-1"}),
            ("cancel", "cancel", {}),
        ):
            with self.subTest(mode=mode):
                action = make_action(
                    PredictionControlAction, mode=mode, overlay_id=OVERLAY,
                    prediction_id="pred-1", **extra,
                )
                request = captured_request(action, {"ok": True})
                self.assertEqual(
                    f"https://allch.at/api/v1/engagement/overlays/{OVERLAY}"
                    f"/predictions/pred-1/{verb}",
                    request.full_url,
                )

    def test_resolve_sends_the_winning_outcome(self) -> None:
        action = make_action(
            PredictionControlAction, mode="resolve", overlay_id=OVERLAY,
            prediction_id="pred-1", winning_outcome_id="out-7",
        )
        request = captured_request(action, {"ok": True})
        self.assertEqual({"winning_outcome_id": "out-7"}, json.loads(request.data))

    def test_ids_are_percent_encoded_into_paths(self) -> None:
        """A stray slash in a pasted ID must not invent a new route."""
        self.assertEqual(
            "/api/v1/engagement/overlays/a%2Fb%20c", api._overlay_path("a/b c")
        )

    def test_self_hosted_base_url_is_used_for_requests(self) -> None:
        action = make_action(
            SendMessageAction, message="hi", platform="all",
            base_url="https://chat.example.org/",
        )
        request = captured_request(action, {"ok": True})
        self.assertEqual("https://chat.example.org/api/v1/auth/chat/send", request.full_url)


class ValidationTest(unittest.TestCase):
    """Bad configuration is caught locally, with a message about the field."""

    def test_poll_option_count_is_bounded(self) -> None:
        for options in ("Only one", "a\nb\nc\nd\ne\nf"):
            with self.subTest(options=options):
                action = make_action(
                    PollControlAction, mode="start", overlay_id=OVERLAY,
                    question="Q", options=options,
                )
                with mock.patch("urllib.request.urlopen") as urlopen:
                    self.assertEqual(base.STATE_ERROR, action.run_action())
                urlopen.assert_not_called()

    def test_prediction_outcome_count_is_bounded(self) -> None:
        action = make_action(
            PredictionControlAction, mode="start", overlay_id=OVERLAY,
            title="T", outcomes="only-one",
        )
        with mock.patch("urllib.request.urlopen") as urlopen:
            self.assertEqual(base.STATE_ERROR, action.run_action())
        urlopen.assert_not_called()

    def test_unknown_platform_is_rejected_locally(self) -> None:
        action = make_action(SendMessageAction, message="hi", platform="myspace")
        with mock.patch("urllib.request.urlopen") as urlopen:
            self.assertEqual(base.STATE_ERROR, action.run_action())
        urlopen.assert_not_called()

    def test_missing_overlay_id_is_rejected_locally(self) -> None:
        action = make_action(PollControlAction, mode="close")
        with mock.patch("urllib.request.urlopen") as urlopen:
            self.assertEqual(base.STATE_ERROR, action.run_action())
        urlopen.assert_not_called()

    def test_resolve_without_a_winner_is_rejected_locally(self) -> None:
        action = make_action(
            PredictionControlAction, mode="resolve", overlay_id=OVERLAY,
            prediction_id="pred-1",
        )
        with mock.patch("urllib.request.urlopen") as urlopen:
            self.assertEqual(base.STATE_ERROR, action.run_action())
        urlopen.assert_not_called()

    def test_platform_defaults_to_all(self) -> None:
        """One press, every chat -- the reason the action exists."""
        action = make_action(SendMessageAction, message="hi")
        request = captured_request(action, {"ok": True})
        self.assertEqual("all", json.loads(request.data)["platform"])


class NormalisationTest(unittest.TestCase):
    def test_split_list_accepts_commas_and_newlines(self) -> None:
        self.assertEqual(["a", "b", "c"], settings.split_list("a, b\n c "))
        self.assertEqual([], settings.split_list(""))
        self.assertEqual([], settings.split_list(None))
        self.assertEqual(["a"], settings.split_list(",,a,,"))

    def test_to_seconds_rejects_junk_and_negatives(self) -> None:
        self.assertEqual(60, settings.to_seconds("60"))
        self.assertEqual(60, settings.to_seconds(60))
        for junk in ("", None, "abc", "-5", 0):
            with self.subTest(junk=junk):
                self.assertEqual(0, settings.to_seconds(junk))

    def test_zero_duration_is_omitted_from_the_body(self) -> None:
        """0 means "run until closed by hand", so the field must not be sent."""
        action = make_action(
            PollControlAction, mode="start", overlay_id=OVERLAY,
            question="Q", options="a\nb", duration_seconds="",
        )
        request = captured_request(action, {"id": "p"})
        self.assertNotIn("duration_seconds", json.loads(request.data))

    def test_extract_id_handles_bare_and_wrapped_shapes(self) -> None:
        self.assertEqual("x", api.extract_id({"id": "x"}))
        self.assertEqual("x", api.extract_id({"poll": {"id": "x"}}))
        self.assertEqual("x", api.extract_id({"prediction": {"id": "x"}}))
        self.assertEqual("x", api.extract_id({"data": {"id": "x"}}))
        self.assertEqual("", api.extract_id({"unexpected": 1}))
        self.assertEqual("", api.extract_id(None))


class ConvenienceLookupTest(unittest.TestCase):
    """Close/lock/resolve fall back to the overlay's live round."""

    def test_close_without_an_id_looks_up_the_active_poll(self) -> None:
        action = make_action(PollControlAction, mode="close", overlay_id=OVERLAY)
        calls: list[str] = []

        def fake_urlopen(request: Any, **_: Any) -> FakeResponse:
            calls.append(request.full_url)
            if request.full_url.endswith("/active-poll"):
                return FakeResponse({"id": "live-poll"})
            return FakeResponse({"ok": True})

        with mock.patch("urllib.request.urlopen", side_effect=fake_urlopen):
            self.assertEqual(base.STATE_OK, action.run_action())

        self.assertTrue(calls[0].endswith(f"/overlays/{OVERLAY}/active-poll"))
        self.assertTrue(calls[1].endswith("/polls/live-poll/close"))

    def test_lock_without_an_id_looks_up_the_active_prediction(self) -> None:
        action = make_action(PredictionControlAction, mode="lock", overlay_id=OVERLAY)
        calls: list[str] = []

        def fake_urlopen(request: Any, **_: Any) -> FakeResponse:
            calls.append(request.full_url)
            if request.full_url.endswith("/active-prediction"):
                return FakeResponse({"id": "live-pred"})
            return FakeResponse({"ok": True})

        with mock.patch("urllib.request.urlopen", side_effect=fake_urlopen):
            self.assertEqual(base.STATE_OK, action.run_action())

        self.assertTrue(calls[-1].endswith("/predictions/live-pred/lock"))

    def test_starting_a_poll_remembers_its_id_for_the_close_key(self) -> None:
        action = make_action(
            PollControlAction, mode="start", overlay_id=OVERLAY,
            question="Q", options="a\nb",
        )
        with mock.patch("urllib.request.urlopen", return_value=FakeResponse({"id": "poll-42"})):
            self.assertEqual(base.STATE_OK, action.run_action())
        self.assertEqual("poll-42", action.get_settings()["last_poll_id"])


class PluginRegistrationTest(unittest.TestCase):
    """The host discovers three actions with stable IDs."""

    def test_three_actions_are_registered(self) -> None:
        import main

        plugin = main.AllChatPlugin()
        self.assertEqual(
            {
                "com_allchat_streamcontroller::SendMessage",
                "com_allchat_streamcontroller::PollControl",
                "com_allchat_streamcontroller::PredictionControl",
            },
            set(plugin.action_holders),
        )

    def test_plugin_id_matches_the_manifest(self) -> None:
        """A mismatch here makes the host refuse to load the plugin."""
        import pathlib

        import main

        manifest = json.loads(
            (pathlib.Path(__file__).resolve().parent.parent / "manifest.json").read_text()
        )
        self.assertEqual(manifest["id"], main.PLUGIN_ID)


if __name__ == "__main__":
    unittest.main()
