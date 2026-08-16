/**
 * "Send chat message" — posts a fixed, per-key message to the streamer's chat.
 *
 * Route: `POST /api/v1/auth/chat/send`, authenticated with the key's personal
 * access token. This action is **not** premium-gated; it works on every account
 * that has a connected platform.
 */

import { action, type KeyDownEvent } from "@elgato/streamdeck";

import { sendChatMessage } from "../allchat/api.js";
import { AllChatError } from "../allchat/errors.js";
import type { SendMessageSettings } from "../allchat/settings.js";
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

			const platform = settings.platform?.trim() || DEFAULT_PLATFORM;
			await sendChatMessage(conn, message, platform);

			// Log the length rather than the text: a key can carry anything, and
			// the log is written to disk.
			return `sent ${message.length} characters to ${platform}`;
		});
	}
}
