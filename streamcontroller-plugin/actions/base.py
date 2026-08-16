"""Shared behaviour for every All-Chat action.

StreamController actions subclass ``ActionBase`` from the host
(``src.backend.PluginManager.ActionBase``) and are registered through an
``ActionHolder``. That import only resolves *inside* a running StreamController
process, so :mod:`actions.host` provides a stand-in when it is absent, letting
this file be imported -- and the logic below exercised -- on a machine with no
Stream Deck attached.

What this module owns:

* turning an action's settings dict into a validated
  :class:`~allchat.api.Connection`, and
* turning an :class:`~allchat.errors.AllChatError` into a key state.

The second is the interesting half. The mapping deliberately gives the premium
gate its **own** state, distinct from the error state:

======================  ===========================================================
Error kind              What the key shows
======================  ===========================================================
``requires-premium``    PREMIUM -- an advert, not a fault. The 403 is expected on a
                        free account and the message names the upgrade page.
``insufficient-scope``  ERROR -- but with re-mint-the-token advice, never "upgrade".
``unauthorized``        ERROR -- "token invalid, re-paste it".
everything else         ERROR -- with whatever the taxonomy produced.
======================  ===========================================================

Nothing here logs a token, and :func:`connection_from` is the only path that
reads one out of settings.
"""

from __future__ import annotations

from typing import Any, Mapping

from allchat import errors
from allchat.api import Connection
from allchat.errors import AllChatError
from allchat.settings import (
    looks_like_pat,
    resolve_base_url,
    resolve_token,
)

from .host import ActionBase

#: Key states an action can be in. ``PREMIUM`` exists so that "you need premium"
#: is never rendered with the same affordance as "something broke": the premium
#: boundary is a product feature being advertised at the point of use.
STATE_OK = "ok"
STATE_BUSY = "busy"
STATE_PREMIUM = "premium"
STATE_ERROR = "error"

#: Shown on the key itself. Kept to a couple of glyphs plus a word because a
#: Stream Deck key is ~72x72 px and long strings render illegibly.
LABEL_FOR_STATE = {
    STATE_OK: "\u2713",
    STATE_BUSY: "\u2026",
    STATE_PREMIUM: "\u2b50 Premium",
    STATE_ERROR: "\u26a0 Error",
}


def state_for_error(exc: AllChatError) -> str:
    """Maps an error onto a key state.

    Only the premium gate gets :data:`STATE_PREMIUM`. A missing *scope* is also a
    403 but maps to :data:`STATE_ERROR`, because upgrading would not fix it and
    showing the upgrade affordance would be actively misleading.
    """
    if exc.is_premium_gate:
        return STATE_PREMIUM
    return STATE_ERROR


class ConfigurationError(AllChatError):
    """A setting is missing or malformed, so no request was attempted.

    Separate from a transport failure so an action can say "fill this field in"
    rather than "All-Chat is unreachable".
    """

    def __init__(self, message: str, kind: str = errors.NOT_CONFIGURED) -> None:
        super().__init__(kind, message)


def connection_from(settings: Mapping[str, Any] | None) -> Connection:
    """Builds a :class:`Connection` from an action's settings.

    Raises :class:`ConfigurationError` when the token is absent or is not a PAT,
    which catches the two commonest setup mistakes before a pointless round-trip.
    The token is read here and passed straight to the client; it is never logged.
    """
    token = resolve_token(settings)
    if not token:
        raise ConfigurationError(errors.missing_token_message(), errors.NO_TOKEN)
    if not looks_like_pat(token):
        raise ConfigurationError(errors.malformed_token_message(), errors.MALFORMED_TOKEN)
    return Connection(base_url=resolve_base_url(settings), token=token)


def require(settings: Mapping[str, Any] | None, key: str, human_name: str) -> str:
    """Returns a required free-text setting, or raises with a usable message."""
    value = ""
    if settings:
        raw = settings.get(key)
        if raw is not None:
            value = str(raw).strip()
    if not value:
        raise ConfigurationError(
            f"This key needs {human_name} before it can do anything. Open the key's "
            f"settings and fill it in."
        )
    return value


class AllChatActionBase(ActionBase):
    """Base for the plugin's three actions.

    Subclasses implement :meth:`perform`, which runs the request and returns a
    short success label for the key. Everything around it -- catching
    :class:`AllChatError`, choosing the state, showing it, logging a message that
    never contains the token -- happens here so the three actions cannot drift in
    how they report the premium boundary.
    """

    #: Overridden per action for the settings UI.
    ACTION_NAME = "All-Chat"

    # -- StreamController lifecycle -----------------------------------------

    def on_ready(self) -> None:
        """Called by the host once the key is on screen."""
        self.set_bottom_label(self.ACTION_NAME)

    def on_key_down(self) -> None:
        """Called by the host when the physical key is pressed."""
        self.run_action()

    # -- plumbing -----------------------------------------------------------

    def get_settings_safe(self) -> Mapping[str, Any]:
        """Returns this key's settings, tolerating a host that has none yet."""
        try:
            return self.get_settings() or {}
        except Exception:  # pragma: no cover - depends on host internals
            return {}

    def perform(self, settings: Mapping[str, Any]) -> str:
        """Runs the action. Returns a short label to show on success."""
        raise NotImplementedError

    def run_action(self) -> str:
        """Executes :meth:`perform` and renders the outcome on the key.

        Returns the state it settled in, which is what the tests assert on.
        """
        settings = self.get_settings_safe()
        try:
            label = self.perform(settings)
        except AllChatError as exc:
            state = state_for_error(exc)
            self.show_state(state, exc.message)
            return state
        except Exception as exc:  # noqa: BLE001
            # A bug in the plugin, not a server response. Reported without the
            # settings dict, which holds the token.
            self.show_state(STATE_ERROR, f"All-Chat plugin error: {type(exc).__name__}: {exc}")
            return STATE_ERROR

        self.show_state(STATE_OK, label)
        return STATE_OK

    def show_state(self, state: str, message: str) -> None:
        """Renders a state on the key and logs the explanation.

        The premium state is logged at INFO rather than ERROR: it is the server
        behaving correctly and the user meeting a paid feature, not a fault, and
        filing it under errors would make it look like one in the host's log.
        """
        if state == STATE_PREMIUM:
            self.log_info(message)
            self.set_key_label(LABEL_FOR_STATE[STATE_PREMIUM])
            self.show_premium_hint()
        elif state == STATE_ERROR:
            self.log_error(message)
            self.set_key_label(LABEL_FOR_STATE[STATE_ERROR])
            self.show_error_hint()
        else:
            self.log_info(message)
            self.set_key_label(message[:12] if message else LABEL_FOR_STATE[STATE_OK])

    # -- thin wrappers over host APIs ---------------------------------------
    # Wrapped rather than called directly so the stand-in host in host.py can
    # record them, and so a host-version rename lands in one place.

    def set_key_label(self, text: str) -> None:
        try:
            self.set_center_label(text)
        except Exception:  # pragma: no cover - depends on host internals
            pass

    def show_premium_hint(self) -> None:
        """Marks the key as "premium feature" rather than "failed"."""
        try:
            self.show_info(LABEL_FOR_STATE[STATE_PREMIUM])
        except Exception:  # pragma: no cover
            pass

    def show_error_hint(self) -> None:
        try:
            self.show_error()
        except Exception:  # pragma: no cover
            pass

    def log_info(self, message: str) -> None:
        try:
            self.plugin_base.logger.info(message)
        except Exception:  # pragma: no cover
            pass

    def log_error(self, message: str) -> None:
        try:
            self.plugin_base.logger.error(message)
        except Exception:  # pragma: no cover
            pass
