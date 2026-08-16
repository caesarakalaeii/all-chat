"""Action: poll control -- start or close a poll.

One action with a ``mode`` setting rather than two separate actions, so a user
drops the same thing twice and flips a dropdown. This is also where the premium
boundary becomes visible, and the asymmetry is the product working as designed:

* ``start`` -> ``POST /api/v1/engagement/overlays/:id/polls`` -- **premium**.
  On a free account this returns HTTP 403. That is correct server behaviour, and
  the key shows a distinct "premium" state naming the upgrade page, never a
  generic error.
* ``close`` -> ``POST …/polls/:pollId/close`` -- **free, always**. A free account
  is never left holding a poll it cannot close, including one started while
  premium or from the web dashboard.

When no poll ID is configured, ``close`` looks up the overlay's active poll
first, so the common setup is one overlay ID on both keys and nothing else.

KEEP IN SYNC with ``streamdeck-plugin/src/actions/poll-control.ts`` (ADR-0049).
"""

from __future__ import annotations

from typing import Any, Mapping

from allchat.api import active_poll, close_poll, extract_id, start_poll
from allchat.settings import split_list, to_seconds

from .base import AllChatActionBase, ConfigurationError, connection_from, require

MODE_START = "start"
MODE_CLOSE = "close"
MODES = (MODE_START, MODE_CLOSE)

#: The server accepts 2-5 poll options. Checked here so the user gets a message
#: about the field they typed into instead of a 400 from a service they cannot see.
MIN_OPTIONS = 2
MAX_OPTIONS = 5


class PollControlAction(AllChatActionBase):
    """Starts (premium) or closes (free) a poll on one overlay."""

    ACTION_NAME = "Poll control"

    def perform(self, settings: Mapping[str, Any]) -> str:
        conn = connection_from(settings)
        overlay_id = require(settings, "overlay_id", "the overlay ID the poll belongs to")
        mode = str(settings.get("mode") or MODE_START).strip().lower()

        if mode == MODE_START:
            return self._start(conn, settings, overlay_id)
        if mode == MODE_CLOSE:
            return self._close(conn, settings, overlay_id)
        raise ConfigurationError(
            f"\"{mode}\" is not a poll mode. Choose one of: {', '.join(MODES)}."
        )

    def _start(self, conn: Any, settings: Mapping[str, Any], overlay_id: str) -> str:
        """Opens a poll. Premium-gated -- a 403 here is expected on a free plan.

        The 403 is not caught: :func:`allchat.api.start_poll` re-words it as the
        poll-shaped premium message and the base class routes it to the premium
        key state. Swallowing it here would hide the feature at the exact moment
        the user reached for it.
        """
        question = require(settings, "question", "the poll question")
        options = split_list(settings.get("options"))
        if not MIN_OPTIONS <= len(options) <= MAX_OPTIONS:
            raise ConfigurationError(
                f"A poll needs between {MIN_OPTIONS} and {MAX_OPTIONS} options; this key has "
                f"{len(options)}. List them in the key's settings, one per line."
            )

        result = start_poll(
            conn, overlay_id, question, options, to_seconds(settings.get("duration_seconds"))
        )

        # Remember the new poll's ID so a paired "close" key on the same overlay
        # needs no configuration. Best-effort: close falls back to the active-poll
        # lookup when this is absent.
        poll_id = extract_id(result)
        if poll_id:
            self.remember("last_poll_id", poll_id)
        return "Poll open"

    def _close(self, conn: Any, settings: Mapping[str, Any], overlay_id: str) -> str:
        """Closes a poll. Free on every account, by design."""
        poll_id = str(settings.get("poll_id") or "").strip()
        if not poll_id:
            poll_id = str(self.recall("last_poll_id") or "")
        if not poll_id:
            poll_id = extract_id(active_poll(conn, overlay_id))
        if not poll_id:
            raise ConfigurationError(
                "There is no active poll on this overlay, and this key has no poll ID set. "
                "Start a poll first, or paste a specific poll ID into the key's settings."
            )

        close_poll(conn, overlay_id, poll_id)
        self.remember("last_poll_id", "")
        return "Poll closed"

    # -- small settings-store helpers ---------------------------------------

    def remember(self, key: str, value: str) -> None:
        """Persists a value into this key's own settings, best-effort."""
        try:
            settings = dict(self.get_settings_safe())
            settings[key] = value
            self.set_settings(settings)
        except Exception:  # pragma: no cover - depends on host internals
            pass

    def recall(self, key: str) -> str:
        return str(self.get_settings_safe().get(key) or "")
