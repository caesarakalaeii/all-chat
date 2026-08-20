/**
 * "Poll control" — one key that either starts a poll or closes the live one,
 * chosen by the key's `mode` setting.
 *
 * The premium boundary runs straight through this action and is the reason the
 * two modes are worth spelling out:
 *
 * - `start` → `POST /api/v1/engagement/overlays/:id/polls`. Gated behind the
 *   premium `engagement` feature. On a free account the server answers **403**,
 *   which is correct and expected; the base class turns that into the
 *   "requires premium" state and names the upgrade page.
 * - `close` → `POST /api/v1/engagement/overlays/:id/polls/:pollId/close`.
 *   Deliberately free, so a streamer who started a poll while premium (or via
 *   the web app) can always end it.
 *
 * KEEP IN SYNC with `streamcontroller-plugin/actions/poll_control.py` (ADR-0049).
 */

import { action, type KeyDownEvent } from "@elgato/streamdeck";

import { activePoll, closePoll, startPoll } from "../allchat/api.js";
import { AllChatError } from "../allchat/errors.js";
import { splitList, toSeconds, type PollSettings } from "../allchat/settings.js";
import { AllChatAction } from "./base.js";

@action({ UUID: "com.allchat.streamdeck.poll-control" })
export class PollControlAction extends AllChatAction<PollSettings> {
	protected readonly label = "poll control";

	protected override premiumSubject(): "poll" | "prediction" {
		return "poll";
	}

	override async onKeyDown(ev: KeyDownEvent<PollSettings>): Promise<void> {
		await this.run(ev.action, async () => {
			const settings = ev.payload.settings;
			const conn = this.connection(settings);

			const overlayId = settings.overlayId?.trim() ?? "";
			if (!overlayId) {
				throw new AllChatError(
					"not-configured",
					"This key has no overlay id. Paste the overlay's id from your All-Chat " +
						"dashboard into the key's settings.",
				);
			}

			const mode = settings.mode ?? "start";
			if (mode === "close") {
				return this.close(conn, settings, overlayId);
			}
			return this.start(conn, settings, overlayId);
		});
	}

	/**
	 * Opens a poll. This is the premium-gated call: `startPoll` marks the route
	 * `premiumGated`, so a 403 arrives here classified as `requires-premium` and
	 * is reported as the upgrade prompt rather than a failure.
	 */
	private async start(
		conn: { baseUrl: string; token: string },
		settings: PollSettings,
		overlayId: string,
	): Promise<string> {
		const question = settings.question?.trim() ?? "";
		const options = splitList(settings.options);

		if (!question) {
			throw new AllChatError(
				"not-configured",
				"This key has no poll question. Add one in the key's settings.",
			);
		}
		if (options.length < 2 || options.length > 5) {
			throw new AllChatError(
				"not-configured",
				`A poll needs between 2 and 5 options; this key has ${options.length}. ` +
					"List them one per line in the key's settings.",
			);
		}

		const poll = await startPoll(
			conn,
			overlayId,
			question,
			options,
			toSeconds(settings.durationSeconds),
		);
		return `started poll ${poll.id ?? "(id unknown)"} with ${options.length} options`;
	}

	/**
	 * Closes a poll. Free on every account. When the key carries no explicit poll
	 * id we look up whichever poll is currently active on the overlay, so the
	 * common "one key, ends whatever is running" setup needs no configuration.
	 */
	private async close(
		conn: { baseUrl: string; token: string },
		settings: PollSettings,
		overlayId: string,
	): Promise<string> {
		let pollId = settings.pollId?.trim() ?? "";
		if (!pollId) {
			const live = await activePoll(conn, overlayId);
			pollId = live.id ?? "";
			if (!pollId) {
				throw new AllChatError(
					"not-found",
					"No poll is currently active on that overlay, and this key has no poll id " +
						"to close.",
				);
			}
		}

		const poll = await closePoll(conn, overlayId, pollId);
		return `closed poll ${poll.id ?? pollId}`;
	}
}
