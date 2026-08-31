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

import { UPGRADE_URL, ACCOUNT_TOKENS_URL, DEFAULT_BASE_URL } from "./settings.js";

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
		`All-Chat rejected this key's credential (HTTP 401). It is expired, revoked or ` +
		`mistyped. If this key was linked, press "Link with All-Chat" again — a paired ` +
		`device lapses if it goes unused. If you pasted a token, mint a fresh one at ` +
		`${ACCOUNT_TOKENS_URL}. (The credential itself is never logged.)`
	);
}

/** The message shown/logged when the key has no credential yet. */
export function missingTokenMessage(): string {
	return (
		`This key is not connected to All-Chat yet. Press "Link with All-Chat" in its ` +
		`settings — your browser opens an approve screen and nothing needs to be copied. ` +
		`On a second machine or a headless host, paste a personal access token from ` +
		`${ACCOUNT_TOKENS_URL} instead.`
	);
}

/**
 * The message the property inspector shows when a link attempt fails.
 *
 * The Link button used to render `error.message` verbatim. Most of those strings are
 * built for the plugin log, not for a streamer looking at a settings panel: they carry a
 * URL, an errno, an HTTP status and sometimes a server-supplied string. The worst case
 * was a raw JavaScript error message for anything that was not an AllChatError at all.
 *
 * This translates only the messages that leak detail, and deliberately passes the others
 * through — several of them are already good user-facing copy ("The pairing code expired
 * before it was approved. Start linking again.") and replacing them with something
 * generic would be a downgrade. The rule is: an authored message with no status attached
 * is copy and survives; anything carrying transport or HTTP detail gets replaced.
 *
 * The raw message is still logged by the caller, so nothing is lost for debugging.
 *
 * Mirrored by `link_failure_message` in the StreamController plugin's `errors.py` (this
 * module's header carries the file-level sync pointer). ADR-0049 counts "what the button
 * surfaces on failure" as part of the action contract, so the two plugins must not tell a
 * user two different things about one failure.
 */
export function linkFailureMessage(error: unknown): string {
    if (!(error instanceof AllChatError)) {
        // A TypeError or similar: implementation detail with no user-actionable content.
        return (
            `Linking did not complete. Press "Link with All-Chat" to try again, or paste a ` +
            `personal access token from ${ACCOUNT_TOKENS_URL} instead.`
        );
    }
    switch (error.kind) {
        case "network":
            // Covers an unreachable host, DNS, TLS, a timeout and a loopback port that
            // could not be bound. All of them carry an errno or a URL in the message.
            return (
                `Could not reach All-Chat. Check your internet connection and try again. If you ` +
                `self-host, check the Server field in this key's settings.`
            );
        case "forbidden":
            return (
                `The approval could not be verified, so nothing was linked. Press "Link with ` +
                `All-Chat" and approve the device again.`
            );
        case "unauthorized":
            return unauthorizedMessage();
        default:
            break;
    }
    if (error.status !== undefined && error.status >= 500) {
        return (
            `All-Chat could not complete the link because the server returned an error (HTTP ` +
            `${error.status}). That is a fault on the server, not on this machine: try again in ` +
            `a few minutes, and report it if it keeps happening.`
        );
    }
    if (error.status !== undefined) {
        return (
            `All-Chat refused the link (HTTP ${error.status}). Make sure you are signed in at ` +
            `${DEFAULT_BASE_URL} as the account you want this key to control, then press "Link ` +
            `with All-Chat" again.`
        );
    }
    // No status: an authored message, e.g. the expired pairing code. Show it as written.
    return error.message;
}

/** The message shown/logged when the configured string is not an All-Chat credential. */
export function malformedTokenMessage(): string {
	return (
		`The value in this key's token field is not an All-Chat credential. A linked ` +
		`device token starts with "allchat_dev_" and a pasted personal access token with ` +
		`"allchat_pat_". Press "Link with All-Chat", or mint a token at ` +
		`${ACCOUNT_TOKENS_URL} and paste the whole string including the prefix.`
	);
}
