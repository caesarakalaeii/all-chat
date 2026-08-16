/**
 * Shared settings shapes for every All-Chat action, plus the small amount of
 * normalisation the plugin does before a value reaches the HTTP client.
 *
 * The property inspector writes these verbatim into Stream Deck's per-action
 * settings store, so every field is optional: a freshly dropped key has `{}`.
 */

/**
 * Production host for All-Chat. Confirmed as `FRONTEND_URL` in
 * `deployments/k8s/base/configmap.yaml`. Self-hosters override this per key in
 * the property inspector; everyone else should leave it blank.
 */
export const DEFAULT_BASE_URL = "https://allch.at";

/** Where a user mints a personal access token, and where they upgrade. */
export const ACCOUNT_TOKENS_URL = `${DEFAULT_BASE_URL}/settings/api-tokens`;

/** Page advertised when the server reports the premium engagement gate. */
export const UPGRADE_URL = `${DEFAULT_BASE_URL}`;

/**
 * Every All-Chat personal access token carries this prefix; the server routes a
 * bearer on the prefix (hash-and-look-up) instead of parsing it as a JWT. We use
 * it only to tell the user early that they pasted the wrong string — never to
 * validate the secret itself, which only the server can do.
 */
export const PAT_PREFIX = "allchat_pat_";

/** Settings shared by all three action types. */
export type ConnectionSettings = {
	/** Personal access token, `allchat_pat_…`. Stored by Stream Deck, never logged. */
	apiToken?: string;
	/** Base URL override for self-hosters. Blank means {@link DEFAULT_BASE_URL}. */
	baseUrl?: string;
};

/** Settings for the "send chat message" action. */
export type SendMessageSettings = ConnectionSettings & {
	/** Message text sent on key-down. */
	message?: string;
	/** Target platform: `twitch` | `youtube` | `kick` | `tiktok` | `all`. */
	platform?: string;
};

/** Which button of the poll action this key is. */
export type PollMode = "start" | "close";

/** Settings for the "poll control" action. */
export type PollSettings = ConnectionSettings & {
	/** `start` opens a poll (premium), `close` ends the live one (free). */
	mode?: PollMode;
	/** Overlay UUID the poll belongs to. */
	overlayId?: string;
	/** Question asked when `mode` is `start`. */
	question?: string;
	/** Newline- or comma-separated options; the server requires 2–5. */
	options?: string;
	/** Auto-close after this many seconds; 0 / blank means "until closed". */
	durationSeconds?: number;
	/** Poll id to close. Blank means "whichever poll is currently active". */
	pollId?: string;
};

/** Which button of the prediction action this key is. */
export type PredictionMode = "start" | "lock" | "resolve" | "cancel";

/** Settings for the "prediction control" action. */
export type PredictionSettings = ConnectionSettings & {
	/** `start` opens a prediction (premium); lock/resolve/cancel are free. */
	mode?: PredictionMode;
	/** Overlay UUID the prediction belongs to. */
	overlayId?: string;
	/** Title used when `mode` is `start`. */
	title?: string;
	/** Newline- or comma-separated outcomes; the server requires 2–10. */
	outcomes?: string;
	/** Auto-lock after this many seconds; 0 / blank means "until locked". */
	autoLockSeconds?: number;
	/** Prediction id for lock/resolve/cancel. Blank means "the active one". */
	predictionId?: string;
	/** Winning outcome id, required by `resolve`. */
	winningOutcomeId?: string;
};

/**
 * Resolves the base URL for a key: the trimmed override when present, otherwise
 * the production host. Trailing slashes are dropped so path concatenation cannot
 * produce a double slash.
 */
export function resolveBaseUrl(settings: ConnectionSettings | undefined): string {
	const raw = settings?.baseUrl?.trim();
	const base = raw && raw.length > 0 ? raw : DEFAULT_BASE_URL;
	return base.replace(/\/+$/, "");
}

/** Resolves the token for a key, trimmed. Empty string when unset. */
export function resolveToken(settings: ConnectionSettings | undefined): string {
	return settings?.apiToken?.trim() ?? "";
}

/**
 * Splits a user-typed option/outcome list into entries. Accepts newlines or
 * commas as separators, drops blanks, and preserves order.
 */
export function splitList(raw: string | undefined): string[] {
	if (!raw) {
		return [];
	}
	return raw
		.split(/[\n,]/)
		.map((entry) => entry.trim())
		.filter((entry) => entry.length > 0);
}

/** Coerces a possibly-string duration field into a non-negative integer. */
export function toSeconds(value: number | string | undefined): number {
	if (value === undefined || value === null || value === "") {
		return 0;
	}
	const n = typeof value === "number" ? value : Number.parseInt(value, 10);
	return Number.isFinite(n) && n > 0 ? Math.floor(n) : 0;
}
