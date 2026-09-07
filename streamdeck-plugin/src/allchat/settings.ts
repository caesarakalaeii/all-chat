/**
 * Shared settings shapes for every All-Chat action, plus the small amount of
 * normalisation the plugin does before a value reaches the HTTP client.
 *
 * The property inspector writes these verbatim into Stream Deck's per-action
 * settings store, so every field is optional: a freshly dropped key has `{}`.
 *
 * KEEP IN SYNC with `streamcontroller-plugin/allchat/settings.py` (ADR-0049).
 * The constants below are user-visible on both plugins and are compared by
 * `scripts/check-plugin-parity.py`.
 */

/**
 * Production host for All-Chat. Confirmed as `FRONTEND_URL` in
 * `deployments/k8s/base/configmap.yaml`. Self-hosters override this per key in
 * the property inspector; everyone else should leave it blank.
 */
export const DEFAULT_BASE_URL = "https://allch.at";

/** Where a user mints a personal access token, and where they upgrade. */
export const ACCOUNT_TOKENS_URL = `${DEFAULT_BASE_URL}/settings/api-tokens`;

/**
 * Where a user sees and revokes their paired control surfaces. This is the page
 * to point at once linking has succeeded — it is where the credential this plugin
 * now holds can be cut off.
 */
export const ACCOUNT_DEVICES_URL = `${DEFAULT_BASE_URL}/settings/devices`;

/**
 * Page advertised when the server reports the premium engagement gate. This is
 * `/upgrade`, not the homepage: a user who just pressed a key and was told they
 * need premium should land on the page that explains it, and the
 * StreamController plugin has always pointed there.
 */
export const UPGRADE_URL = `${DEFAULT_BASE_URL}/upgrade`;

/**
 * Every All-Chat personal access token carries this prefix; the server routes a
 * bearer on the prefix (hash-and-look-up) instead of parsing it as a JWT. We use
 * it only to tell the user early that they pasted the wrong string — never to
 * validate the secret itself, which only the server can do.
 */
export const PAT_PREFIX = "allchat_pat_";

/**
 * Prefix of a PAIRED-DEVICE token (ADR-0049), the credential the Link flow
 * produces. Deliberately a different prefix from {@link PAT_PREFIX} rather than a
 * flag inside the same namespace: the server switches on the prefix, secret
 * scanners can tell the two apart, and a support log line says which credential a
 * streamer is using without disclosing either.
 */
export const DEVICE_PREFIX = "allchat_dev_";

/** Opens a device-link request. Unauthenticated: this plugin has no credential yet. */
export const LINK_START_PATH = "/api/v1/auth/device/link/start";

/** Trades an approved link for the device token. Answers 428 while still pending. */
export const LINK_EXCHANGE_PATH = "/api/v1/auth/device/link/exchange";

/**
 * The ONE path the loopback listener serves, and the only path the server accepts
 * in a redirect_uri. Fixed on both sides so the listener has one route and the
 * server's validator has one string to compare — see
 * `services/auth-service/handlers/loopback_redirect.go`.
 */
export const LOOPBACK_PATH = "/allchat/device-callback";

/**
 * The loopback host. `127.0.0.1` ONLY, never `0.0.0.0` and never `localhost`:
 * binding the wildcard address would expose the listener to the local network
 * during linking, and `localhost` is a DNS name the server refuses precisely
 * because it can be pointed elsewhere.
 */
export const LOOPBACK_HOST = "127.0.0.1";

/**
 * Platforms accepted by `POST /api/v1/auth/chat/send`. `all` fans the message out
 * to every connected platform and returns a per-platform result — and fans out to
 * exactly these three, see `handleStreamerSendToAll` in
 * `services/auth-service/handlers/chat_send.go`.
 *
 * This list is the SEND surface, which is narrower than the set of platforms
 * All-Chat reads chat from. Keep it that way: an entry here that the server cannot
 * post to is a button that fails on press, which is how `tiktok` shipped in both
 * plugins' pickers while auth-service answered 501 to it.
 *
 * The picker in `ui/send-message.html` offers these and nothing else; the parity
 * script asserts that, because the picker is where the drift actually happened.
 */
export const PLATFORMS = ["all", "twitch", "youtube", "kick"] as const;

/**
 * Platforms All-Chat READS but cannot POST to, with the reason. These are offered
 * nowhere, but a key configured before they were removed still has one saved, so the
 * value must be explained rather than reported as a typo.
 *
 * KEEP IN SYNC with `streamcontroller-plugin/allchat/settings.py` (ADR-0049).
 */
export const UNSENDABLE_PLATFORMS: Readonly<Record<string, string>> = {
	tiktok:
		"TikTok publishes no API for posting into a live chat, so All-Chat can show " +
		"TikTok chat on your overlay but cannot send to it.",
	discord:
		"A Discord source is a one-way relay into your overlay. All-Chat has no send " +
		"path back into a Discord channel.",
};

/** Settings shared by all three action types. */
export type ConnectionSettings = {
	/**
	 * The credential this key authenticates with: either a paired-device token
	 * (`allchat_dev_…`, written by the Link flow) or a personal access token
	 * (`allchat_pat_…`, pasted by the user for a headless box or a second PC).
	 * Stored by Stream Deck, never logged. The field name is historical — it
	 * predates device tokens, and renaming it would silently unconfigure every
	 * existing key.
	 */
	apiToken?: string;
	/** Base URL override for self-hosters. Blank means {@link DEFAULT_BASE_URL}. */
	baseUrl?: string;
};

/** Settings for the "send chat message" action. */
export type SendMessageSettings = ConnectionSettings & {
	/** Message text sent on key-down. */
	message?: string;
	/** Target platform: one of {@link PLATFORMS}. */
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
 * True when `token` has the shape of EITHER All-Chat credential.
 *
 * A prefix test only. It catches the common paste error (a session JWT, a
 * truncated copy) before a pointless round-trip; it says nothing about whether
 * the token is valid, revoked, expired or in scope, all of which only the server
 * knows. Both prefixes are accepted because both are legitimate here: a device
 * token arrives from the Link flow, a PAT from the user.
 */
export function looksLikeToken(token: string): boolean {
	return looksLikeDeviceToken(token) || looksLikePat(token);
}

/**
 * True when `token` is specifically a paired-device token (from linking).
 *
 * Counterpart of `looks_like_device_token` in
 * `streamcontroller-plugin/allchat/settings.py`. The narrow pair exists so a
 * message can name the right remedy: a device token is re-linked, a pasted token
 * is re-pasted.
 */
export function looksLikeDeviceToken(token: string): boolean {
	return token.startsWith(DEVICE_PREFIX) && token.length > DEVICE_PREFIX.length;
}

/** True when `token` is specifically a personal access token (pasted). */
export function looksLikePat(token: string): boolean {
	return token.startsWith(PAT_PREFIX) && token.length > PAT_PREFIX.length;
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
