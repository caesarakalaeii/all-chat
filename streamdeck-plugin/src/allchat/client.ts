/**
 * Thin HTTP client for the All-Chat API.
 *
 * Authentication is a fixed contract: the user pastes a personal access token of
 * the form `allchat_pat_…` into the property inspector and every request carries
 * it as `Authorization: Bearer allchat_pat_…`. There is no OAuth or cookie flow
 * here — these tokens exist precisely because a desktop client has no browser
 * session to borrow.
 *
 * Two rules this file enforces on behalf of every caller:
 *
 * 1. **The token never leaves this file in readable form.** It goes into one
 *    header and is not logged, not echoed into an error message, and not put in
 *    a URL. {@link redact} exists for the rare case a caller wants to prove
 *    which token is configured without disclosing it.
 * 2. **401 and 403 are separated at the source.** 403 on a premium-gated route
 *    is classified `requires-premium`, everything else becomes a specific
 *    {@link AllChatError} kind. See `errors.ts` for why that distinction is
 *    load-bearing.
 */

import { AllChatError } from "./errors.js";
import { PAT_PREFIX } from "./settings.js";

/** Options for a single All-Chat request. */
export type RequestOptions = {
	/** Base URL, already trimmed of trailing slashes. */
	baseUrl: string;
	/** Personal access token, `allchat_pat_…`. */
	token: string;
	/** Path below the base URL, starting with a slash, e.g. `/api/v1/…`. */
	path: string;
	/** JSON body; omitted for bodyless POSTs. */
	body?: unknown;
	/**
	 * When true, a 403 from this route is classified as `requires-premium`
	 * rather than a plain `forbidden`. Set it only on the two routes the server
	 * actually gates: starting a poll and starting a prediction.
	 */
	premiumGated?: boolean;
	/** Abort the request after this many milliseconds. */
	timeoutMs?: number;
};

/** Default request timeout — a Stream Deck key should never hang visibly. */
const DEFAULT_TIMEOUT_MS = 10_000;

/**
 * Renders a token safely for diagnostics: the `allchat_pat_` prefix plus the
 * last four characters. Never the secret. Used only when a log line genuinely
 * needs to distinguish two configured keys.
 */
export function redact(token: string): string {
	if (!token) {
		return "<none>";
	}
	const tail = token.slice(-4);
	return `${PAT_PREFIX}…${tail}`;
}

/** True when the string has the shape of an All-Chat personal access token. */
export function looksLikePat(token: string): boolean {
	return token.startsWith(PAT_PREFIX) && token.length > PAT_PREFIX.length;
}

/**
 * Performs a POST against All-Chat and returns the decoded JSON body.
 *
 * Throws {@link AllChatError} for every non-2xx response and for transport
 * failures, with the kind already classified.
 */
export async function post<T = unknown>(options: RequestOptions): Promise<T> {
	const { baseUrl, token, path, body, premiumGated = false } = options;
	const url = `${baseUrl}${path}`;

	const headers: Record<string, string> = {
		// The fixed auth contract. Nothing else authenticates this plugin.
		Authorization: `Bearer ${token}`,
		Accept: "application/json",
	};
	let payload: string | undefined;
	if (body !== undefined) {
		payload = JSON.stringify(body);
		headers["Content-Type"] = "application/json";
	}

	const controller = new AbortController();
	const timeout = setTimeout(() => controller.abort(), options.timeoutMs ?? DEFAULT_TIMEOUT_MS);

	let response: Response;
	try {
		response = await fetch(url, {
			method: "POST",
			headers,
			body: payload,
			signal: controller.signal,
		});
	} catch (cause) {
		// Transport-level failure. The URL is safe to include; the token is not
		// in it, and never will be.
		const reason = cause instanceof Error ? cause.message : String(cause);
		throw new AllChatError("network", `Could not reach All-Chat at ${url}: ${reason}`);
	} finally {
		clearTimeout(timeout);
	}

	if (response.ok) {
		return (await readJson(response)) as T;
	}

	throw classify(response, await readServerMessage(response), premiumGated, path);
}

/**
 * Performs a GET against All-Chat. Used to look up the currently active poll or
 * prediction so a "close"/"lock" key can work without the user pasting an id.
 */
export async function get<T = unknown>(options: Omit<RequestOptions, "body">): Promise<T> {
	const { baseUrl, token, path, premiumGated = false } = options;
	const url = `${baseUrl}${path}`;

	const controller = new AbortController();
	const timeout = setTimeout(() => controller.abort(), options.timeoutMs ?? DEFAULT_TIMEOUT_MS);

	let response: Response;
	try {
		response = await fetch(url, {
			method: "GET",
			headers: {
				Authorization: `Bearer ${token}`,
				Accept: "application/json",
			},
			signal: controller.signal,
		});
	} catch (cause) {
		const reason = cause instanceof Error ? cause.message : String(cause);
		throw new AllChatError("network", `Could not reach All-Chat at ${url}: ${reason}`);
	} finally {
		clearTimeout(timeout);
	}

	if (response.ok) {
		return (await readJson(response)) as T;
	}

	throw classify(response, await readServerMessage(response), premiumGated, path);
}

/**
 * Maps an HTTP status onto an {@link AllChatError} kind.
 *
 * The 403 branch is the interesting one. `premiumGated` is set by the caller for
 * exactly the two routes the server gates behind the premium `engagement`
 * feature — starting a poll and starting a prediction. On those, 403 means "this
 * account is not premium", which is expected product behaviour rather than a
 * fault, and gets its own kind so the action can advertise the upgrade instead
 * of printing a generic failure. A 403 anywhere else means something genuinely
 * went wrong, most likely the overlay belongs to somebody else.
 */
function classify(
	response: Response,
	serverMessage: string,
	premiumGated: boolean,
	path: string,
): AllChatError {
	const status = response.status;
	const detail = serverMessage ? ` (server said: ${serverMessage})` : "";

	switch (status) {
		case 401:
			return new AllChatError("unauthorized", `HTTP 401 from ${path}${detail}`, status);
		case 403:
			return premiumGated
				? new AllChatError("requires-premium", `HTTP 403 from ${path}${detail}`, status)
				: new AllChatError(
						"forbidden",
						`HTTP 403 from ${path}${detail}. This token's account does not own that overlay, ` +
							`or the token lacks the required scope.`,
						status,
					);
		case 404:
			return new AllChatError(
				"not-found",
				`HTTP 404 from ${path}${detail}. Check the overlay id, and that the poll or ` +
					`prediction you are targeting still exists.`,
				status,
			);
		case 409:
			return new AllChatError("conflict", `HTTP 409 from ${path}${detail}`, status);
		case 429:
			return new AllChatError(
				"rate-limited",
				`HTTP 429 from ${path}${detail}. Slow down and try again shortly.`,
				status,
			);
		default:
			return new AllChatError("http-error", `HTTP ${status} from ${path}${detail}`, status);
	}
}

/** Reads a JSON body, tolerating an empty one (some routes return no content). */
async function readJson(response: Response): Promise<unknown> {
	const text = await response.text();
	if (!text) {
		return {};
	}
	try {
		return JSON.parse(text) as unknown;
	} catch {
		return { raw: text };
	}
}

/**
 * Extracts the server's `error` field for inclusion in a log line. Truncated,
 * because an unbounded server string should not be able to flood the plugin log.
 */
async function readServerMessage(response: Response): Promise<string> {
	let text: string;
	try {
		text = await response.text();
	} catch {
		return "";
	}
	if (!text) {
		return "";
	}
	let message = text;
	try {
		const parsed = JSON.parse(text) as { error?: unknown; message?: unknown };
		const candidate = parsed.error ?? parsed.message;
		if (typeof candidate === "string" && candidate.length > 0) {
			message = candidate;
		}
	} catch {
		// Not JSON — fall through to the raw text.
	}
	return message.length > 200 ? `${message.slice(0, 200)}…` : message;
}
