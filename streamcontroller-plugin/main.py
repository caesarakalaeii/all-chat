"""StreamController plugin entry point for All-Chat.

StreamController loads a plugin by importing ``main.py`` from the plugin
directory and instantiating the :class:`PluginBase` subclass it finds. Each
action is registered with an ``ActionHolder``, which pairs an action ID with the
class that implements it and the name shown in the action picker.

Install location (see ``README.md``)::

    ~/.var/app/com.core447.StreamController/data/plugins/com_allchat_streamcontroller/

The three actions mirror the Elgato plugin in ``streamdeck-plugin/`` one-for-one.
ADR-0049 requires that: the two plugins exist because no single format reaches
both Linux and Windows/macOS, and the button set drifting between them is the
cost the ADR calls out by name. Every action module carries a KEEP IN SYNC
pointer at its counterpart.

Premium boundary, restated here because this is the file a reviewer opens first:
starting a poll or a prediction is premium and legitimately answers **HTTP 403**
on a free account; closing a poll and locking / resolving / cancelling a
prediction are free. The 403 is rendered as its own "requires premium" key state
pointing at https://allch.at/upgrade -- never as a generic error. See
``allchat/errors.py``, which also explains why a 403 for a missing token *scope*
is a different thing that must not be reported as "buy premium".
"""

from __future__ import annotations

from actions.host import ActionHolder, PluginBase
from actions.poll_control import PollControlAction
from actions.prediction_control import PredictionControlAction
from actions.send_message import SendMessageAction

#: Must match ``id`` in ``manifest.json``. StreamController namespaces an
#: action's ID under the plugin's, so these strings are part of the on-disk
#: format: changing one orphans every key a user has already configured.
PLUGIN_ID = "com_allchat_streamcontroller"

ACTION_SEND_MESSAGE = f"{PLUGIN_ID}::SendMessage"
ACTION_POLL_CONTROL = f"{PLUGIN_ID}::PollControl"
ACTION_PREDICTION_CONTROL = f"{PLUGIN_ID}::PredictionControl"


class AllChatPlugin(PluginBase):
    """Registers the All-Chat actions with StreamController.

    Holds no credentials itself: a personal access token is stored per action by
    the host, so one deck can drive two accounts, and no token is ever written to
    a file this plugin owns.
    """

    def __init__(self) -> None:
        super().__init__()

        self.send_message_holder = ActionHolder(
            plugin_base=self,
            action_base=SendMessageAction,
            action_id=ACTION_SEND_MESSAGE,
            action_name="Send chat message",
        )
        self.add_action_holder(self.send_message_holder)

        self.poll_control_holder = ActionHolder(
            plugin_base=self,
            action_base=PollControlAction,
            action_id=ACTION_POLL_CONTROL,
            action_name="Poll control",
        )
        self.add_action_holder(self.poll_control_holder)

        self.prediction_control_holder = ActionHolder(
            plugin_base=self,
            action_base=PredictionControlAction,
            action_id=ACTION_PREDICTION_CONTROL,
            action_name="Prediction control",
        )
        self.add_action_holder(self.prediction_control_holder)

        # Publishes the plugin to the host. Guarded because the signature has
        # moved between StreamController releases and a mismatch here would take
        # the whole plugin down rather than one action.
        try:
            self.register()
        except Exception:  # pragma: no cover - depends on host version
            pass
