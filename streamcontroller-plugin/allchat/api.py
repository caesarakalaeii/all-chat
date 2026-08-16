"""The All-Chat endpoints this plugin drives, and which of them are premium.

Route shapes mirror ``services/api-gateway/cmd/main.go``. The premium flag mirrors
``services/engagement-service/cmd/main.go``, where ``requireEngagementPremium``
sits on exactly two routes::

    POST /overlays/:id/polls                        <- premium
    POST /overlays/:id/polls/:pollId/close             free
    POST /overlays/:id/predictions                  <- premium
    POST /overlays/:id/predictions/:pid/lock           free
    POST /overlays/:id/predictions/:pid/resolve        free
    POST /overlays/:id/predictions/:pid/cancel         free

That asymmetry is deliberate product behaviour: a free account can always finish
or unwind a round it already has open, it just cannot open a new one. Nobody is
ever left holding a poll they cannot close. Keeping the premium flag next to the
path is what stops the plugin from reporting a premium 403 as a generic failure.

Every function raises :class:`~allchat.errors.AllChatError` and nothing else.

KEEP IN SYNC with ``streamdeck-plugin/src/allchat/api.ts`` (ADR-0049).
"""

from __future__ import annotations

from typing import Any, NamedTuple
from urllib.parse import quote

from . import errors
from .client import get, post
from .errors import AllChatError

#: Path of the chat-send route (auth-service, proxied by the gateway).
#: Requires the ``chat:write`` scope; NOT premium-gated.
CHAT_SEND_PATH = "/api/v1/auth/chat/send"

#: Prefix the gateway mounts engagement-service under.
ENGAGEMENT_PREFIX = "/api/v1/engagement"


class Connection(NamedTuple):
    """Everything a request needs beyond its own body."""

    base_url: str
    token: str


def _overlay_path(overlay_id: str) -> str:
    """Base path for one overlay's engagement routes."""
    return f"{ENGAGEMENT_PREFIX}/overlays/{quote(overlay_id, safe='')}"


def _premium(exc: AllChatError, what: str) -> AllChatError:
    """Re-words a premium 403 with the noun the user actually pressed.

    The client cannot know whether a given 403 came from a poll or a prediction
    route, so the two premium-gated calls below pass through here to name it.
    Anything that is not the premium gate -- including a 403 for a missing scope
    -- is returned untouched, because those need different advice.
    """
    if exc.is_premium_gate:
        return AllChatError(errors.REQUIRES_PREMIUM, errors.premium_gate_message(what), exc.status)
    return exc


# --- chat ------------------------------------------------------------------


def send_chat_message(conn: Connection, message: str, platform: str) -> Any:
    """Sends a chat message as the token's owner.

    ``platform`` is one of ``twitch``, ``youtube``, ``kick``, ``tiktok``, or
    ``all`` to fan out to every connected platform. Both fields are required by
    the server. Not premium-gated; needs the ``chat:write`` scope.
    """
    return post(
        base_url=conn.base_url,
        token=conn.token,
        path=CHAT_SEND_PATH,
        body={"message": message, "platform": platform},
    )


# --- polls -----------------------------------------------------------------


def start_poll(
    conn: Connection,
    overlay_id: str,
    question: str,
    options: list[str],
    duration_seconds: int = 0,
) -> Any:
    """Starts a poll. **Premium-gated.**

    A free account gets a legitimate 403 here. It is re-worded as the poll-shaped
    premium message and must be surfaced as "requires premium", never as a
    failure.
    """
    body: dict[str, Any] = {"question": question, "options": options}
    if duration_seconds > 0:
        body["duration_seconds"] = duration_seconds
    try:
        return post(
            base_url=conn.base_url,
            token=conn.token,
            path=f"{_overlay_path(overlay_id)}/polls",
            body=body,
        )
    except AllChatError as exc:
        raise _premium(exc, "poll") from None


def close_poll(conn: Connection, overlay_id: str, poll_id: str) -> Any:
    """Closes a poll. Free on every account, by design."""
    return post(
        base_url=conn.base_url,
        token=conn.token,
        path=f"{_overlay_path(overlay_id)}/polls/{quote(poll_id, safe='')}/close",
    )


def active_poll(conn: Connection, overlay_id: str) -> Any:
    """Reads the overlay's currently active poll.

    Lets a "close" key work without the user pasting an ID. This route is public
    (``pubGroup.GET /overlays/:id/active-poll``) because the OBS render path holds
    no token; we send the bearer anyway, which is harmless on a public route and
    keeps one code path.
    """
    return get(
        base_url=conn.base_url,
        token=conn.token,
        path=f"{_overlay_path(overlay_id)}/active-poll",
    )


# --- predictions -----------------------------------------------------------


def start_prediction(
    conn: Connection,
    overlay_id: str,
    title: str,
    outcomes: list[str],
    auto_lock_seconds: int = 0,
) -> Any:
    """Starts a prediction. **Premium-gated**, exactly like :func:`start_poll`."""
    body: dict[str, Any] = {"title": title, "outcomes": outcomes}
    if auto_lock_seconds > 0:
        body["auto_lock_seconds"] = auto_lock_seconds
    try:
        return post(
            base_url=conn.base_url,
            token=conn.token,
            path=f"{_overlay_path(overlay_id)}/predictions",
            body=body,
        )
    except AllChatError as exc:
        raise _premium(exc, "prediction") from None


def _prediction_action(conn: Connection, overlay_id: str, prediction_id: str, verb: str,
                       body: dict[str, Any] | None = None) -> Any:
    """Shared shape for the three free prediction transitions."""
    return post(
        base_url=conn.base_url,
        token=conn.token,
        path=f"{_overlay_path(overlay_id)}/predictions/{quote(prediction_id, safe='')}/{verb}",
        body=body,
    )


def lock_prediction(conn: Connection, overlay_id: str, prediction_id: str) -> Any:
    """Locks a prediction so no further wagers land. Free on every account."""
    return _prediction_action(conn, overlay_id, prediction_id, "lock")


def resolve_prediction(
    conn: Connection, overlay_id: str, prediction_id: str, winning_outcome_id: str
) -> Any:
    """Resolves a prediction, paying out the winning outcome. Free."""
    return _prediction_action(
        conn, overlay_id, prediction_id, "resolve",
        {"winning_outcome_id": winning_outcome_id},
    )


def cancel_prediction(conn: Connection, overlay_id: str, prediction_id: str) -> Any:
    """Cancels a prediction, refunding every stake. Free."""
    return _prediction_action(conn, overlay_id, prediction_id, "cancel")


def active_prediction(conn: Connection, overlay_id: str) -> Any:
    """Reads the overlay's live prediction, so lock / resolve / cancel keys work
    without the user pasting an ID. Public route, same as :func:`active_poll`."""
    return get(
        base_url=conn.base_url,
        token=conn.token,
        path=f"{_overlay_path(overlay_id)}/active-prediction",
    )


def extract_id(payload: Any) -> str:
    """Pulls an ``id`` out of a poll/prediction response.

    The engagement endpoints have returned the object either bare or wrapped in
    ``{"poll": …}`` / ``{"prediction": …}`` / ``{"data": …}``, so all three shapes
    are accepted and an unrecognised one yields ``""`` rather than a traceback.
    """
    if isinstance(payload, dict):
        for key in ("id", "ID"):
            value = payload.get(key)
            if isinstance(value, str) and value:
                return value
        for key in ("poll", "prediction", "data", "result"):
            nested = payload.get(key)
            if isinstance(nested, dict):
                found = extract_id(nested)
                if found:
                    return found
    return ""
