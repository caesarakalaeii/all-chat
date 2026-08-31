/**
 * "Send chat message" — posts a fixed, per-key message to the streamer's chat.
 *
 * Route: `POST /api/v1/auth/chat/send`, authenticated with the key's personal
 * access token. This action is **not** premium-gated; it works on every account
 * that has a connected platform.
 *
 * KEEP IN SYNC with `streamcontroller-plugin/actions/send_message.py` (ADR-0049).
 */

import { action, type KeyDownEvent } from "@elgato/streamdeck";

import { sendChatMessage } from "../allchat/api.js";
import { AllChatError } from "../allchat/errors.js";
import { PLATFORMS, UNSENDABLE_PLATFORMS, type SendMessageSettings } from "../allchat/settings.js";
import { AllChatAction } from "./base.js";

/** Platform used when the key does not name one: fan out everywhere. */
const DEFAULT_PLATFORM = "all";

@action({ UUID: "com.allchat.streamdeck.send-message" })
export class SendMessageAction extends AllChatAction<SendMessageSettings> {
	protected readonly label = "send chat message";

	override async onKeyDown(ev: KeyDownEvent<SendMessageSettings>): Promise<void> {
		await this.run(ev.action, async () => {
			const settings = ev.payload.settings;
			const conn = this.connection(settings);

			const message = settings.message?.trim() ?? "";
			if (!message) {
				throw new AllChatError(
					"not-configured",
					"This key has no message text. Type the message to send in the key's settings.",
				);
			}

			const platform = (settings.platform?.trim() || DEFAULT_PLATFORM).toLowerCase();

			// A read-only platform is answered before the generic "not a platform"
			// text. Both are refusals, but only this one is a property of the
			// platform rather than a typo, and a streamer who picked TikTok deserves
			// the reason instead of a list it is missing from. Keys configured while
			// TikTok was still in the picker land here on their next press.
			const unsendable = UNSENDABLE_PLATFORMS[platform];
			if (unsendable) {
				throw new AllChatError(
					"not-configured",
					`${unsendable} Point this key at one of: ${PLATFORMS.join(", ")}.`,
				);
			}
			if (!(PLATFORMS as readonly string[]).includes(platform)) {
				throw new AllChatError(
					"not-configured",
					`"${platform}" is not a platform All-Chat sends to. Choose one of: ${PLATFORMS.join(", ")}.`,
				);
			}

			await sendChatMessage(conn, message, platform);

			// Log the length rather than the text: a key can carry anything, and
			// the log is written to disk.
			return `sent ${message.length} characters to ${platform}`;
		});
	}
}
