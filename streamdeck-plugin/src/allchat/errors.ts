/**
 * Error taxonomy for All-Chat requests.
 *
 * The important distinction this file exists to preserve is between HTTP 401 and
 * HTTP 403 on the two "start" actions:
 *
 * - **401** means the personal access token is missing, malformed, expired or
 *   revoked. The user must mint a new one and re-paste it. Actionable, and the
 *   fix is entirely on the user's side.
 *
 * - **403** on `POST …/polls` or `POST …/predictions` is *not* a failure of the
 *   plugin. The server deliberately gates *starting* a poll or a prediction
 *   behind the premium `engagement` feature; close / lock / resolve / cancel are
 *   deliberately free. A non-premium account therefore gets a perfectly correct
 *   403 from those two routes while every other key keeps working. We surface
 *   that as its own state and name the upgrade page, because the point of the
 *   gate is to advertise the feature at the moment somebody reaches for it —
 *   not to look like a broken button.
 *
 * Neither path ever puts the token in a message or a log line.
 *
 * KEEP IN SYNC with `streamcontroller-plugin/allchat/errors.py` (ADR-0049). The
 * 401-vs-403 split is the part that must not drift: one says "re-paste your
 * token", the other says "this is premium", and swapping them costs money.
 */

import { UPGRADE_URL, ACCOUNT_TOKENS_URL } from "./settings.js";

/** Discriminator for the states a caller may want to react to differently. */
export type AllChatErrorKind =
	/** No token configured on the key at all. */
	| "no-token"
	/** Token present but not of the `allchat_pat_…` shape. */
	| "malformed-token"
	/** A required setting (overlay id, message, …) is missing. */
	| "not-configured"
	/** HTTP 401 — token rejected by the server. */
	| "unauthorized"
	/** HTTP 403 on a premium-gated route — expected on a free account. */
	| "requires-premium"
	/** HTTP 403 somewhere premium is not the explanation (e.g. not your overlay). */
	| "forbidden"
	/** HTTP 404 — overlay / poll / prediction not found. */
	| "not-found"
	/** HTTP 409 — e.g. an active poll already exists on this overlay. */
	| "conflict"
	/** HTTP 429 — rate limited. */
	| "rate-limited"
	/** Any other non-2xx response. */
	| "http-error"
	/** Transport failure: host unreachable, DNS, TLS, timeout. */
	| "network";

/**
 * An All-Chat failure carrying the kind a UI should branch on, a message safe to
 * log verbatim, and the raw status when there was one.
 */
export class AllChatError extends Error {
	/** What went wrong, in terms the actions branch on. */
	readonly kind: AllChatErrorKind;
	/** HTTP status when the failure came from a response. */
	readonly status?: number;

	constructor(kind: AllChatErrorKind, message: string, status?: number) {
		super(message);
		this.name = "AllChatError";
		this.kind = kind;
		this.status = status;
	}

	/** True when this is the deliberate premium gate rather than a real fault. */
	get isPremiumGate(): boolean {
		return this.kind === "requires-premium";
	}
}

/**
 * The message shown/logged when the premium `engagement` gate answers 403.
 *
 * Deliberately names the feature, states plainly that the *other* keys still
 * work, and points at the upgrade page. It must never read like a generic
 * "request failed".
 */
export function premiumGateMessage(what: "poll" | "prediction"): string {
	return (
		`Starting a ${what} is part of All-Chat premium (the "engagement" feature), ` +
		`and this account does not have it — the server answered 403, which is expected ` +
		`rather than a bug. Closing a poll and locking / resolving / cancelling a ` +
		`prediction stay free, so those keys keep working. Upgrade at ${UPGRADE_URL} to ` +
		`start polls and predictions from your Stream Deck.`
	);
}

/** The message shown/logged when the server rejects the token with 401. */
export function unauthorizedMessage(): string {
	return (
		`All-Chat rejected the personal access token (HTTP 401). It is expired, revoked ` +
		`or mistyped. Mint a fresh token at ${ACCOUNT_TOKENS_URL} and paste it into this ` +
		`key's settings. (The token itself is never logged.)`
	);
}

/** The message shown/logged when no token has been pasted onto the key yet. */
export function missingTokenMessage(): string {
	return (
		`No All-Chat personal access token configured for this key. Create one at ` +
		`${ACCOUNT_TOKENS_URL} and paste it into the key's settings.`
	);
}

/** The message shown/logged when the pasted string is not a PAT. */
export function malformedTokenMessage(): string {
	return (
		`The value in this key's token field is not an All-Chat personal access token ` +
		`(they start with "allchat_pat_"). Mint one at ${ACCOUNT_TOKENS_URL} and paste ` +
		`the whole string, including the prefix.`
	);
}
