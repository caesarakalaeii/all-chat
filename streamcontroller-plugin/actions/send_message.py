"""Action: send a preset chat message.

One key press posts a canned message to one platform, or to every connected
platform at once with ``platform = "all"``. This is the action ADR-0049 opens
with: "a physical button that fans a message out to Twitch, YouTube, Kick, TikTok
and Discord at once is the shortest path between an intent and five platforms."

Free on every account. It needs a PAT carrying ``chat:write``, which the gateway
checks at the edge and auth-service checks again behind it, so a token minted
without that scope fails with a 403 that is *not* the premium gate -- see
:mod:`allchat.errors`.

KEEP IN SYNC with ``streamdeck-plugin/src/actions/send-message.ts`` (ADR-0049).
"""

from __future__ import annotations

from typing import Any, Mapping

from allchat.api import send_chat_message
from allchat.settings import PLATFORMS

from .base import AllChatActionBase, ConfigurationError, connection_from, require

#: Used when the key has no platform set. "all" matches the action's reason for
#: existing -- one press, every chat -- so it is the useful default rather than
#: an arbitrary single platform.
DEFAULT_PLATFORM = "all"

#: Mirrors `maxMessageLength` enforced by auth-service. Checked here only to give
#: a better message than a 400 would; the server remains the authority.
MAX_MESSAGE_LENGTH = 500


class SendMessageAction(AllChatActionBase):
    """Posts a fixed message to chat on key-down."""

    ACTION_NAME = "Send chat message"

    def perform(self, settings: Mapping[str, Any]) -> str:
        conn = connection_from(settings)
        message = require(settings, "message", "the message text")

        if len(message) > MAX_MESSAGE_LENGTH:
            raise ConfigurationError(
                f"This message is {len(message)} characters; All-Chat accepts at most "
                f"{MAX_MESSAGE_LENGTH}. Shorten it in the key's settings."
            )

        platform = str(settings.get("platform") or DEFAULT_PLATFORM).strip().lower()
        if platform not in PLATFORMS:
            raise ConfigurationError(
                f"\"{platform}\" is not a platform All-Chat sends to. Choose one of: "
                f"{', '.join(PLATFORMS)}."
            )

        send_chat_message(conn, message, platform)

        # Deliberately does not echo the message text back into the log: a canned
        # message is not secret, but the log line is more useful short.
        return "Sent" if platform == "all" else f"Sent {platform}"
