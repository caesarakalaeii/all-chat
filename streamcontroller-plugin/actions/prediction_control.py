"""Action: prediction control -- start, lock, resolve or cancel a prediction.

The same shape as :mod:`actions.poll_control`, with four modes, and the same
premium boundary in the same place:

* ``start``   -> ``POST /api/v1/engagement/overlays/:id/predictions`` -- **premium**.
  HTTP 403 on a free account is expected and is surfaced as the premium state.
* ``lock``    -> ``…/predictions/:pid/lock``    -- free. No further wagers.
* ``resolve`` -> ``…/predictions/:pid/resolve`` -- free. Pays out a winner.
* ``cancel``  -> ``…/predictions/:pid/cancel``  -- free. Refunds every stake.

Three of the four are free on purpose: a round that is already open must always
be finishable, so a lapsed premium account can still pay its viewers out rather
than stranding their points in a prediction nobody can resolve.

When no prediction ID is set, the free modes look up the overlay's live
prediction, so one overlay ID on each key is the whole configuration.

KEEP IN SYNC with ``streamdeck-plugin/src/actions/prediction-control.ts`` (ADR-0049).
"""

from __future__ import annotations

from typing import Any, Mapping

from allchat.api import (
    active_prediction,
    cancel_prediction,
    extract_id,
    lock_prediction,
    resolve_prediction,
    start_prediction,
)
from allchat.settings import split_list, to_seconds

from .base import AllChatActionBase, ConfigurationError, connection_from, require

MODE_START = "start"
MODE_LOCK = "lock"
MODE_RESOLVE = "resolve"
MODE_CANCEL = "cancel"
MODES = (MODE_START, MODE_LOCK, MODE_RESOLVE, MODE_CANCEL)

#: The server accepts 2-10 prediction outcomes.
MIN_OUTCOMES = 2
MAX_OUTCOMES = 10


class PredictionControlAction(AllChatActionBase):
    """Starts (premium) or locks / resolves / cancels (free) a prediction."""

    ACTION_NAME = "Prediction control"

    def perform(self, settings: Mapping[str, Any]) -> str:
        conn = connection_from(settings)
        overlay_id = require(settings, "overlay_id", "the overlay ID the prediction belongs to")
        mode = str(settings.get("mode") or MODE_START).strip().lower()

        if mode == MODE_START:
            return self._start(conn, settings, overlay_id)
        if mode in (MODE_LOCK, MODE_RESOLVE, MODE_CANCEL):
            return self._transition(conn, settings, overlay_id, mode)
        raise ConfigurationError(
            f"\"{mode}\" is not a prediction mode. Choose one of: {', '.join(MODES)}."
        )

    def _start(self, conn: Any, settings: Mapping[str, Any], overlay_id: str) -> str:
        """Opens a prediction. Premium-gated; the 403 propagates deliberately."""
        title = require(settings, "title", "the prediction title")
        outcomes = split_list(settings.get("outcomes"))
        if not MIN_OUTCOMES <= len(outcomes) <= MAX_OUTCOMES:
            raise ConfigurationError(
                f"A prediction needs between {MIN_OUTCOMES} and {MAX_OUTCOMES} outcomes; this "
                f"key has {len(outcomes)}. List them in the key's settings, one per line."
            )

        result = start_prediction(
            conn, overlay_id, title, outcomes, to_seconds(settings.get("auto_lock_seconds"))
        )

        prediction_id = extract_id(result)
        if prediction_id:
            self.remember("last_prediction_id", prediction_id)
        return "Prediction"

    def _transition(
        self, conn: Any, settings: Mapping[str, Any], overlay_id: str, mode: str
    ) -> str:
        """Runs one of the three free transitions on an open prediction."""
        prediction_id = self._resolve_prediction_id(conn, settings, overlay_id)

        if mode == MODE_LOCK:
            lock_prediction(conn, overlay_id, prediction_id)
            return "Locked"

        if mode == MODE_CANCEL:
            cancel_prediction(conn, overlay_id, prediction_id)
            self.remember("last_prediction_id", "")
            return "Cancelled"

        # resolve: needs to be told which outcome won.
        winning_outcome_id = str(settings.get("winning_outcome_id") or "").strip()
        if not winning_outcome_id:
            raise ConfigurationError(
                "Resolving a prediction needs the winning outcome's ID, and this key has none. "
                "Paste it into the key's settings -- one resolve key per possible outcome is "
                "the usual layout, so the winner is a single press."
            )
        resolve_prediction(conn, overlay_id, prediction_id, winning_outcome_id)
        self.remember("last_prediction_id", "")
        return "Resolved"

    def _resolve_prediction_id(
        self, conn: Any, settings: Mapping[str, Any], overlay_id: str
    ) -> str:
        """Finds the prediction to act on: explicit, remembered, then live."""
        prediction_id = str(settings.get("prediction_id") or "").strip()
        if prediction_id:
            return prediction_id
        prediction_id = str(self.recall("last_prediction_id") or "")
        if prediction_id:
            return prediction_id
        prediction_id = extract_id(active_prediction(conn, overlay_id))
        if prediction_id:
            return prediction_id
        raise ConfigurationError(
            "There is no live prediction on this overlay, and this key has no prediction ID "
            "set. Start a prediction first, or paste a specific ID into the key's settings."
        )

    # -- small settings-store helpers ---------------------------------------

    def remember(self, key: str, value: str) -> None:
        try:
            settings = dict(self.get_settings_safe())
            settings[key] = value
            self.set_settings(settings)
        except Exception:  # pragma: no cover - depends on host internals
            pass

    def recall(self, key: str) -> str:
        return str(self.get_settings_safe().get(key) or "")
