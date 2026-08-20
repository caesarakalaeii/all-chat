/**
 * Device linking: how this plugin obtains a credential without the streamer
 * typing or pasting one (ADR-0049).
 *
 * The primary path, RFC 8252 §7.3:
 *
 *   1. Generate a PKCE verifier and its S256 challenge, plus a `state` value.
 *   2. Bind an ephemeral port on 127.0.0.1 — **only** 127.0.0.1, never 0.0.0.0 —
 *      serving exactly one path, {@link LOOPBACK_PATH}.
 *   3. POST /device/link/start with that loopback URL as the redirect target.
 *   4. Open the system browser at the returned verification URI. The streamer,
 *      normally already signed in, sees one approve screen and clicks Approve.
 *   5. All-Chat redirects the browser to our loopback with `code` and `state`.
 *      We compare `state` ourselves — the server echoes it and never interprets
 *      it, so this comparison is our own CSRF check on our own listener.
 *   6. Close the socket immediately and POST /device/link/exchange with the code
 *      and the verifier. The response carries the device token, exactly once.
 *
 * The fallback, RFC 8628, for when step 2 or step 4 cannot happen — a Stream Deck
 * driving a second PC, a host that will not let us bind a port, a machine with no
 * browser: start the flow with `flow: "code"`, show the streamer the XXXX-XXXX
 * code we get back, and poll the exchange endpoint. 428 means "still waiting".
 *
 * WHAT THIS MODULE WILL NOT DO:
 *
 *   - It never binds 0.0.0.0. A listener on the wildcard address is reachable
 *     from the local network for the duration of linking, and the whole point of
 *     a loopback redirect is that the credential cannot leave the machine.
 *   - It never logs the verifier, the code or the token. Not at debug level, not
 *     in an error message. The only rendering of a credential anywhere in this
 *     plugin is `settings.redact`, which emits a prefix and nothing more.
 *   - It never keeps the socket open longer than it must. The listener closes the
 *     moment the code arrives or the timeout expires, whichever comes first.
 *
 * KEEP IN SYNC with `streamcontroller-plugin/allchat/linking.py` (ADR-0049). Both
 * plugins implement one flow; a change to the request shape or the timeouts here
 * belongs in both files in one change.
 */

import { createHash, randomBytes } from "node:crypto";
import { createServer, type Server } from "node:http";
import { spawn } from "node:child_process";
import { AllChatError } from "./errors.js";
import {
	DEVICE_PREFIX,
	LINK_EXCHANGE_PATH,
	LINK_START_PATH,
	LOOPBACK_HOST,
	LOOPBACK_PATH,
} from "./settings.js";

/** How long the loopback listener waits for the browser before giving up. */
const LOOPBACK_TIMEOUT_MS = 180_000;

/** How long the pairing-code flow polls the exchange endpoint before giving up. */
const CODE_FLOW_TIMEOUT_MS = 600_000;

/** Fallback poll interval when the server does not send one. */
const DEFAULT_POLL_SECONDS = 5;

/** Per-request HTTP timeout for the two linking calls. */
const REQUEST_TIMEOUT_MS = 15_000;

/** Scopes this plugin asks for. The streamer may grant a subset on the approve screen. */
export const REQUESTED_SCOPES = ["chat:write", "engagement:write"] as const;

/** The server's answer to `POST /device/link/start`. */
type StartResponse = {
	request_id?: string;
	user_code?: string;
	verification_uri?: string;
	expires_in?: number;
	interval?: number;
};

/** The server's answer to a successful `POST /device/link/exchange`. */
type ExchangeResponse = {
	token?: string;
	device_id?: string;
	overlay_id?: string;
	scopes?: string[];
	expires_at?: string;
};

/** The outcome of a completed link, ready to be written into the key's settings. */
export type LinkResult = {
	/** The device token. Write it to settings and never log it. */
	token: string;
	deviceId: string;
	overlayId: string;
	scopes: string[];
	expiresAt: string;
};

/** What the caller must show the streamer while a code-flow link is pending. */
export type PendingCodeLink = {
	requestId: string;
	/** The grouped XXXX-XXXX code to display. */
	userCode: string;
	/** Where to tell the streamer to go. */
	verificationUri: string;
	/** Resolves with the credential once the streamer approves, or throws. */
	completion: Promise<LinkResult>;
};

/** A PKCE pair. The verifier never leaves this process except in the exchange body. */
type Pkce = { verifier: string; challenge: string };

/**
 * Generates a PKCE verifier and its S256 challenge (RFC 7636).
 *
 * 32 bytes of `crypto.randomBytes`, base64url unpadded — 43 characters, the RFC's
 * minimum length and comfortably beyond guessing. `plain` is not implemented at
 * all, because the server rejects it: a plain challenge IS the verifier, so it
 * provides none of the protection PKCE exists for in a public client.
 */
function generatePkce(): Pkce {
	const verifier = randomBytes(32).toString("base64url");
	const challenge = createHash("sha256").update(verifier).digest("base64url");
	return { verifier, challenge };
}

/** Generates the `state` value we echo-check ourselves. */
function generateState(): string {
	return randomBytes(16).toString("base64url");
}

/** POSTs JSON and returns the parsed body, classifying failures. */
async function postJson<T>(url: string, body: unknown, timeoutMs = REQUEST_TIMEOUT_MS): Promise<{ status: number; body: T }> {
	const controller = new AbortController();
	const timer = setTimeout(() => controller.abort(), timeoutMs);
	let response: Response;
	try {
		response = await fetch(url, {
			method: "POST",
			headers: { "Content-Type": "application/json", Accept: "application/json" },
			body: JSON.stringify(body),
			signal: controller.signal,
		});
	} catch (cause) {
		const reason = cause instanceof Error ? cause.message : String(cause);
		throw new AllChatError("network", `Could not reach All-Chat at ${url}: ${reason}`);
	} finally {
		clearTimeout(timer);
	}
	const text = await response.text();
	let parsed: unknown = {};
	if (text) {
		try {
			parsed = JSON.parse(text) as unknown;
		} catch {
			parsed = {};
		}
	}
	return { status: response.status, body: parsed as T };
}

/** Extracts the server's `error`/`message` field for a log line, truncated. */
function serverMessage(body: unknown): string {
	if (typeof body !== "object" || body === null) {
		return "";
	}
	const record = body as { error?: unknown; message?: unknown };
	const candidate = record.error ?? record.message;
	const text = typeof candidate === "string" ? candidate : "";
	return text.length > 200 ? `${text.slice(0, 200)}…` : text;
}

/**
 * Opens the system browser at `url`, resolving false when it cannot.
 *
 * No dependency and no shell: `spawn` with an argument array, so a URL can never
 * be interpreted as anything but one argument. Failure is a normal outcome here,
 * not an error — a machine with no desktop session is exactly the case the typed
 * code exists for, so the caller falls back rather than failing.
 */
export function openBrowser(url: string): boolean {
	const [command, args] =
		process.platform === "win32"
			? ["cmd", ["/c", "start", "", url]]
			: process.platform === "darwin"
				? ["open", [url]]
				: ["xdg-open", [url]];
	try {
		const child = spawn(command as string, args as string[], {
			detached: true,
			stdio: "ignore",
		});
		child.unref();
		return true;
	} catch {
		return false;
	}
}

/**
 * Binds a one-shot loopback listener and resolves with the `code` the browser
 * delivers.
 *
 * Bound to {@link LOOPBACK_HOST} with port 0, so the OS picks a free port: a
 * plugin cannot reserve one in advance and a fixed port collides with whatever
 * else the streamer runs. The server's validator accepts any port precisely
 * because the host is pinned to a literal address.
 *
 * The listener answers exactly one path and closes as soon as it has an answer.
 */
function listenForCode(expectedState: string): {
	redirectUri: Promise<string>;
	code: Promise<string>;
	close: () => void;
} {
	let server: Server | undefined;
	let settle: ((code: string) => void) | undefined;
	let fail: ((err: Error) => void) | undefined;

	const code = new Promise<string>((resolve, reject) => {
		settle = resolve;
		fail = reject;
	});

	const redirectUri = new Promise<string>((resolve, reject) => {
		server = createServer((request, response) => {
			const url = new URL(request.url ?? "/", `http://${LOOPBACK_HOST}`);
			if (url.pathname !== LOOPBACK_PATH) {
				response.writeHead(404).end();
				return;
			}
			const received = url.searchParams.get("state") ?? "";
			const authCode = url.searchParams.get("code") ?? "";
			// We generated `state`, the server echoed it back untouched, and we
			// compare it here: it is our own CSRF check on our own listener, and a
			// mismatch means this request did not come from the flow we started.
			if (received !== expectedState || authCode === "") {
				response
					.writeHead(400, { "Content-Type": "text/plain; charset=utf-8" })
					.end("All-Chat: this response did not match the link request that was started here.");
				fail?.(new AllChatError("forbidden", "Loopback callback failed the state check"));
				return;
			}
			response
				.writeHead(200, { "Content-Type": "text/plain; charset=utf-8" })
				.end("All-Chat: this control surface is linked. You can close this tab.");
			settle?.(authCode);
		});

		server.on("error", (err) => {
			// Cannot bind — a sandboxed host, a firewall, a locked-down container.
			// A normal outcome, and the reason the typed-code fallback exists.
			reject(new AllChatError("network", `Could not bind a loopback port: ${err.message}`));
		});

		server.listen(0, LOOPBACK_HOST, () => {
			const address = server?.address();
			if (address === null || typeof address !== "object") {
				reject(new AllChatError("network", "Loopback listener reported no port"));
				return;
			}
			resolve(`http://${LOOPBACK_HOST}:${address.port}${LOOPBACK_PATH}`);
		});
	});

	return {
		redirectUri,
		code,
		close: () => {
			server?.close();
		},
	};
}

/** Asserts the exchange response actually carries a device token. */
function toLinkResult(body: ExchangeResponse): LinkResult {
	const token = body.token ?? "";
	if (!token.startsWith(DEVICE_PREFIX)) {
		throw new AllChatError(
			"http-error",
			"All-Chat's exchange response did not contain a device token",
		);
	}
	return {
		token,
		deviceId: body.device_id ?? "",
		overlayId: body.overlay_id ?? "",
		scopes: body.scopes ?? [],
		expiresAt: body.expires_at ?? "",
	};
}

/**
 * Runs the loopback link end to end and resolves with the credential.
 *
 * Throws {@link AllChatError} on every failure, including "could not bind a
 * port" — which the caller should treat as "offer the typed code instead", not as
 * a fault.
 */
export async function linkViaLoopback(
	baseUrl: string,
	deviceName: string,
	timeoutMs = LOOPBACK_TIMEOUT_MS,
): Promise<LinkResult> {
	const pkce = generatePkce();
	const state = generateState();
	const listener = listenForCode(state);

	try {
		const redirectUri = await listener.redirectUri;

		const start = await postJson<StartResponse>(`${baseUrl}${LINK_START_PATH}`, {
			flow: "loopback",
			device_name: deviceName,
			scopes: REQUESTED_SCOPES,
			code_challenge: pkce.challenge,
			code_challenge_method: "S256",
			redirect_uri: redirectUri,
		});
		if (start.status !== 201 || !start.body.request_id) {
			throw new AllChatError(
				"http-error",
				`All-Chat refused to start linking (HTTP ${start.status}${
					serverMessage(start.body) ? `: ${serverMessage(start.body)}` : ""
				})`,
				start.status,
			);
		}

		const verificationUri = withState(start.body.verification_uri ?? "", state);
		if (!openBrowser(verificationUri)) {
			throw new AllChatError(
				"network",
				"Could not open a browser for the approve screen. Use the pairing code instead.",
			);
		}

		const code = await withTimeout(
			listener.code,
			timeoutMs,
			"Timed out waiting for the approve screen. Start linking again, or use the pairing code.",
		);

		const exchange = await postJson<ExchangeResponse>(`${baseUrl}${LINK_EXCHANGE_PATH}`, {
			request_id: start.body.request_id,
			code,
			code_verifier: pkce.verifier,
		});
		if (exchange.status !== 200) {
			throw new AllChatError(
				"http-error",
				`All-Chat refused the link exchange (HTTP ${exchange.status}${
					serverMessage(exchange.body) ? `: ${serverMessage(exchange.body)}` : ""
				})`,
				exchange.status,
			);
		}
		return toLinkResult(exchange.body);
	} finally {
		// Unconditional: the socket closes on success, on failure and on timeout.
		// A listening port that outlives the flow is exactly what ADR-0049 asks us
		// not to leave behind.
		listener.close();
	}
}

/**
 * Starts the pairing-code flow and returns the code to display plus a promise
 * that resolves when the streamer approves.
 *
 * This is the path that will rot, because it is used rarely. It has tests of its
 * own for that reason (see the parity gate and the Python plugin's tests).
 */
export async function linkViaCode(
	baseUrl: string,
	deviceName: string,
	timeoutMs = CODE_FLOW_TIMEOUT_MS,
): Promise<PendingCodeLink> {
	const pkce = generatePkce();
	const start = await postJson<StartResponse>(`${baseUrl}${LINK_START_PATH}`, {
		flow: "code",
		device_name: deviceName,
		scopes: REQUESTED_SCOPES,
		code_challenge: pkce.challenge,
		code_challenge_method: "S256",
	});
	if (start.status !== 201 || !start.body.request_id || !start.body.user_code) {
		throw new AllChatError(
			"http-error",
			`All-Chat refused to start linking (HTTP ${start.status}${
				serverMessage(start.body) ? `: ${serverMessage(start.body)}` : ""
			})`,
			start.status,
		);
	}

	const requestId = start.body.request_id;
	const userCode = start.body.user_code;
	const intervalMs = Math.max(1, start.body.interval ?? DEFAULT_POLL_SECONDS) * 1000;

	const completion = (async (): Promise<LinkResult> => {
		const deadline = Date.now() + timeoutMs;
		for (;;) {
			const exchange = await postJson<ExchangeResponse>(`${baseUrl}${LINK_EXCHANGE_PATH}`, {
				request_id: requestId,
				user_code: userCode,
				code_verifier: pkce.verifier,
			});
			if (exchange.status === 200) {
				return toLinkResult(exchange.body);
			}
			// 428 is the server's "still pending" — the streamer has not clicked
			// Approve yet. Anything else is terminal and the plugin must stop rather
			// than hammer an endpoint that will keep refusing.
			if (exchange.status !== 428) {
				throw new AllChatError(
					"http-error",
					`All-Chat refused the link exchange (HTTP ${exchange.status}${
						serverMessage(exchange.body) ? `: ${serverMessage(exchange.body)}` : ""
					})`,
					exchange.status,
				);
			}
			if (Date.now() + intervalMs > deadline) {
				throw new AllChatError(
					"http-error",
					"The pairing code expired before it was approved. Start linking again.",
				);
			}
			await sleep(intervalMs);
		}
	})();

	return {
		requestId,
		userCode,
		verificationUri: start.body.verification_uri ?? "",
		completion,
	};
}

/**
 * Reports whether a loopback port can be bound at all.
 *
 * Called before offering the primary path, so a sandboxed host offers the typed
 * code straight away instead of failing halfway through and confusing the
 * streamer. Binds and immediately releases; it never leaves a socket behind.
 */
export async function loopbackAvailable(): Promise<boolean> {
	return new Promise<boolean>((resolve) => {
		const probe = createServer();
		probe.once("error", () => resolve(false));
		probe.listen(0, LOOPBACK_HOST, () => {
			probe.close(() => resolve(true));
		});
	});
}

/** Appends our `state` to the verification URI so it survives the round trip. */
function withState(verificationUri: string, state: string): string {
	if (!verificationUri) {
		return verificationUri;
	}
	const separator = verificationUri.includes("?") ? "&" : "?";
	return `${verificationUri}${separator}state=${encodeURIComponent(state)}`;
}

function sleep(ms: number): Promise<void> {
	return new Promise((resolve) => setTimeout(resolve, ms));
}

/** Rejects with a usable message when `promise` outlives `ms`. */
function withTimeout<T>(promise: Promise<T>, ms: number, message: string): Promise<T> {
	return new Promise<T>((resolve, reject) => {
		const timer = setTimeout(() => reject(new AllChatError("network", message)), ms);
		promise.then(
			(value) => {
				clearTimeout(timer);
				resolve(value);
			},
			(error: unknown) => {
				clearTimeout(timer);
				reject(error instanceof Error ? error : new Error(String(error)));
			},
		);
	});
}
