/**
 * The All-Chat endpoints this plugin drives, and which of them the server gates
 * behind premium.
 *
 * Route shapes mirror `services/api-gateway/cmd/main.go`. The premium flag
 * mirrors `services/engagement-service/cmd/main.go`, where `requireEngagementPremium`
 * sits on exactly two routes:
 *
 *     POST /overlays/:id/polls              ← premium
 *     POST /overlays/:id/polls/:pollId/close        free
 *     POST /overlays/:id/predictions        ← premium
 *     POST /overlays/:id/predictions/:pid/lock      free
 *     POST /overlays/:id/predictions/:pid/resolve   free
 *     POST /overlays/:id/predictions/:pid/cancel    free
 *
 * That asymmetry is deliberate: a free account can always finish or unwind a
 * round it already started, it just cannot open a new one. Keeping the flag next
 * to the path is what stops the plugin from ever reporting a premium 403 as a
 * generic failure.
 */

import { get, post } from "./client.js";
import type { ConnectionSettings } from "./settings.js";

/** Path of the chat-send route (auth-service, proxied by the gateway). */
export const CHAT_SEND_PATH = "/api/v1/auth/chat/send";

/** Base path for an overlay's engagement routes. */
function overlayPath(overlayId: string): string {
	return `/api/v1/engagement/overlays/${encodeURIComponent(overlayId)}`;
}

/** Everything a request needs beyond its own body. */
export type Connection = {
	baseUrl: string;
	token: string;
};

/** Shape of a poll as returned by engagement-service. */
export type Poll = {
	id?: string;
	question?: string;
	status?: string;
};

/** Shape of a prediction as returned by engagement-service. */
export type Prediction = {
	id?: string;
	title?: string;
	status?: string;
	outcomes?: { id?: string; label?: string }[];
};

/**
 * Sends a chat message as the token's owner.
 *
 * `platform` is one of `twitch`, `youtube`, `kick`, `tiktok`, or `all` to fan
 * out to every connected platform. Not premium-gated.
 */
export async function sendChatMessage(
	conn: Connection,
	message: string,
	platform: string,
): Promise<unknown> {
	return post({
		...conn,
		path: CHAT_SEND_PATH,
		body: { message, platform },
	});
}

/**
 * Starts a poll. **Premium-gated** — a free account gets a legitimate 403 here,
 * which the caller must surface as "requires premium", never as a failure.
 */
export async function startPoll(
	conn: Connection,
	overlayId: string,
	question: string,
	options: string[],
	durationSeconds: number,
): Promise<Poll> {
	const body: Record<string, unknown> = { question, options };
	if (durationSeconds > 0) {
		body["duration_seconds"] = durationSeconds;
	}
	return post<Poll>({
		...conn,
		path: `${overlayPath(overlayId)}/polls`,
		body,
		premiumGated: true,
	});
}

/** Closes a poll. Free on every account, by design. */
export async function closePoll(
	conn: Connection,
	overlayId: string,
	pollId: string,
): Promise<Poll> {
	return post<Poll>({
		...conn,
		path: `${overlayPath(overlayId)}/polls/${encodeURIComponent(pollId)}/close`,
	});
}

/**
 * Reads the overlay's currently active poll so a "close" key can work without
 * the user pasting an id.
 *
 * This route is public — `publicAPI.GET /engagement/overlays/:id/active-poll` in
 * the gateway — because the OBS render path holds no token. We send the bearer
 * anyway: it is harmless on a public route and keeps one code path.
 */
export async function activePoll(conn: Connection, overlayId: string): Promise<Poll> {
	return get<Poll>({ ...conn, path: `${overlayPath(overlayId)}/active-poll` });
}

/**
 * Starts a prediction. **Premium-gated**, exactly like {@link startPoll}: 403
 * here means "not premium", not "broken".
 */
export async function startPrediction(
	conn: Connection,
	overlayId: string,
	title: string,
	outcomes: string[],
	autoLockSeconds: number,
): Promise<Prediction> {
	const body: Record<string, unknown> = { title, outcomes };
	if (autoLockSeconds > 0) {
		body["auto_lock_seconds"] = autoLockSeconds;
	}
	return post<Prediction>({
		...conn,
		path: `${overlayPath(overlayId)}/predictions`,
		body,
		premiumGated: true,
	});
}

/** Locks a prediction (no further wagers). Free on every account. */
export async function lockPrediction(
	conn: Connection,
	overlayId: string,
	predictionId: string,
): Promise<Prediction> {
	return post<Prediction>({
		...conn,
		path: `${overlayPath(overlayId)}/predictions/${encodeURIComponent(predictionId)}/lock`,
	});
}

/** Resolves a prediction, paying out the winning outcome. Free. */
export async function resolvePrediction(
	conn: Connection,
	overlayId: string,
	predictionId: string,
	winningOutcomeId: string,
): Promise<Prediction> {
	return post<Prediction>({
		...conn,
		path: `${overlayPath(overlayId)}/predictions/${encodeURIComponent(predictionId)}/resolve`,
		body: { winning_outcome_id: winningOutcomeId },
	});
}

/** Cancels a prediction, refunding every stake. Free. */
export async function cancelPrediction(
	conn: Connection,
	overlayId: string,
	predictionId: string,
): Promise<Prediction> {
	return post<Prediction>({
		...conn,
		path: `${overlayPath(overlayId)}/predictions/${encodeURIComponent(predictionId)}/cancel`,
	});
}

/**
 * Reads the overlay's live prediction so lock / resolve / cancel keys can work
 * without the user pasting an id. Public route, same as {@link activePoll}.
 */
export async function activePrediction(conn: Connection, overlayId: string): Promise<Prediction> {
	return get<Prediction>({ ...conn, path: `${overlayPath(overlayId)}/active-prediction` });
}

/** Narrowing helper so actions can hand settings straight to a request. */
export type WithConnection = ConnectionSettings;
