/**
 * "Prediction control" — one key that starts, locks, resolves or cancels a
 * prediction, chosen by the key's `mode` setting.
 *
 * Same premium boundary as the poll action, and for the same reason:
 *
 * - `start` → `POST /api/v1/engagement/overlays/:id/predictions`. Premium-gated;
 *   a free account gets an expected **403** that we report as "requires premium"
 *   while naming the upgrade page.
 * - `lock` / `resolve` / `cancel` → `.../predictions/:pid/{lock,resolve,cancel}`.
 *   Deliberately free, so a round can always be finished or refunded.
 *
 * A typical premium streamer binds four keys to this action, one per mode. A
 * free account binds three: the start key will keep telling them, informatively,
 * what it would unlock.
 */

import { action, type KeyDownEvent } from "@elgato/streamdeck";

import {
	activePrediction,
	cancelPrediction,
	lockPrediction,
	resolvePrediction,
	startPrediction,
	type Prediction,
} from "../allchat/api.js";
import { AllChatError } from "../allchat/errors.js";
import { splitList, toSeconds, type PredictionSettings } from "../allchat/settings.js";
import { AllChatAction } from "./base.js";

type Conn = { baseUrl: string; token: string };

@action({ UUID: "com.allchat.streamdeck.prediction-control" })
export class PredictionControlAction extends AllChatAction<PredictionSettings> {
	protected readonly label = "prediction control";

	protected override premiumSubject(): "poll" | "prediction" {
		return "prediction";
	}

	override async onKeyDown(ev: KeyDownEvent<PredictionSettings>): Promise<void> {
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

			switch (settings.mode ?? "start") {
				case "lock":
					return this.lock(conn, settings, overlayId);
				case "resolve":
					return this.resolve(conn, settings, overlayId);
				case "cancel":
					return this.cancel(conn, settings, overlayId);
				default:
					return this.start(conn, settings, overlayId);
			}
		});
	}

	/**
	 * Opens a prediction. The premium-gated call — `startPrediction` marks the
	 * route, so a 403 surfaces as the upgrade prompt, never as a generic error.
	 */
	private async start(
		conn: Conn,
		settings: PredictionSettings,
		overlayId: string,
	): Promise<string> {
		const title = settings.title?.trim() ?? "";
		const outcomes = splitList(settings.outcomes);

		if (!title) {
			throw new AllChatError(
				"not-configured",
				"This key has no prediction title. Add one in the key's settings.",
			);
		}
		if (outcomes.length < 2 || outcomes.length > 10) {
			throw new AllChatError(
				"not-configured",
				`A prediction needs between 2 and 10 outcomes; this key has ${outcomes.length}. ` +
					"List them one per line in the key's settings.",
			);
		}

		const pred = await startPrediction(
			conn,
			overlayId,
			title,
			outcomes,
			toSeconds(settings.autoLockSeconds),
		);
		return `started prediction ${pred.id ?? "(id unknown)"} with ${outcomes.length} outcomes`;
	}

	/** Stops accepting wagers. Free on every account. */
	private async lock(conn: Conn, settings: PredictionSettings, overlayId: string): Promise<string> {
		const pid = await this.targetId(conn, settings, overlayId);
		const pred = await lockPrediction(conn, overlayId, pid);
		return `locked prediction ${pred.id ?? pid}`;
	}

	/**
	 * Settles a prediction and pays out. Free, but it needs to know which outcome
	 * won — the server takes an outcome **id**, not a label, so the key must carry
	 * one. The outcome ids are visible on the prediction in the dashboard.
	 */
	private async resolve(
		conn: Conn,
		settings: PredictionSettings,
		overlayId: string,
	): Promise<string> {
		const winning = settings.winningOutcomeId?.trim() ?? "";
		if (!winning) {
			throw new AllChatError(
				"not-configured",
				"A resolve key needs the winning outcome's id. Copy it from the prediction in " +
					"your All-Chat dashboard into the key's settings.",
			);
		}
		const pid = await this.targetId(conn, settings, overlayId);
		const pred = await resolvePrediction(conn, overlayId, pid, winning);
		return `resolved prediction ${pred.id ?? pid}`;
	}

	/** Voids a prediction and refunds every stake. Free on every account. */
	private async cancel(
		conn: Conn,
		settings: PredictionSettings,
		overlayId: string,
	): Promise<string> {
		const pid = await this.targetId(conn, settings, overlayId);
		const pred = await cancelPrediction(conn, overlayId, pid);
		return `cancelled prediction ${pred.id ?? pid}`;
	}

	/**
	 * Resolves which prediction a lock / resolve / cancel key acts on: the id
	 * pinned in the key's settings, or else whichever prediction is live on the
	 * overlay right now.
	 */
	private async targetId(
		conn: Conn,
		settings: PredictionSettings,
		overlayId: string,
	): Promise<string> {
		const pinned = settings.predictionId?.trim() ?? "";
		if (pinned) {
			return pinned;
		}
		const live: Prediction = await activePrediction(conn, overlayId);
		const id = live.id ?? "";
		if (!id) {
			throw new AllChatError(
				"not-found",
				"No prediction is currently live on that overlay, and this key has no " +
					"prediction id of its own.",
			);
		}
		return id;
	}
}
