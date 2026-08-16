/**
 * Shared key-press plumbing for every All-Chat action.
 *
 * The whole point of this file is `run()`: it wraps an action's real work so
 * that the outcome always reaches the user through the same, deliberate set of
 * states. In particular it is where 401 and 403 stop being "an error" and become
 * two different, specific pieces of advice:
 *
 * - **401** → "your token was rejected, paste a fresh one".
 * - **403 on a premium-gated route** → "starting polls/predictions is premium;
 *   the free keys still work; upgrade at https://allch.at". This is a real
 *   product state, not a fault, and it is advertised rather than hidden.
 *
 * Both show Stream Deck's alert (a key cannot show two different glyphs, so the
 * distinction lives in the log line and the key title), and neither ever writes
 * the token anywhere.
 */

import streamDeck, { SingletonAction, type KeyAction, type DialAction } from "@elgato/streamdeck";

import { looksLikePat } from "../allchat/client.js";
import {
	AllChatError,
	malformedTokenMessage,
	missingTokenMessage,
	premiumGateMessage,
	unauthorizedMessage,
} from "../allchat/errors.js";
import {
	resolveBaseUrl,
	resolveToken,
	type ConnectionSettings,
} from "../allchat/settings.js";

/**
 * The settings constraint `SingletonAction` imposes, restated locally.
 *
 * The SDK's own `JsonObject` is declared in `@elgato/utils` and is not
 * re-exported from `@elgato/streamdeck`, so importing it would mean depending
 * directly on a package the SDK owns and may bump underneath us. It is a plain
 * index-signature type, so this structurally identical copy satisfies the
 * constraint with no extra coupling.
 */
type JsonPrimitive = boolean | number | string | null | undefined;
type JsonValue = JsonBag | JsonPrimitive | JsonValue[];
type JsonBag = { [key: string]: JsonValue };

/** Anything a key press can be attached to. */
type AnyAction<T extends JsonBag> = KeyAction<T> | DialAction<T>;

/** Resolved connection plus the logger the action should use. */
export type ActionContext = {
	baseUrl: string;
	token: string;
};

const logger = streamDeck.logger.createScope("all-chat");

/**
 * Base class carrying the connection resolution, the outcome reporting and the
 * error taxonomy every All-Chat action shares.
 */
export abstract class AllChatAction<
	T extends ConnectionSettings & JsonBag,
> extends SingletonAction<T> {
	/** Human name of this action, used in log lines and key titles. */
	protected abstract readonly label: string;

	/**
	 * Resolves the connection for a key, or throws a classified
	 * {@link AllChatError} when the key is not configured yet.
	 *
	 * The token is only inspected for its `allchat_pat_` prefix — the plugin
	 * cannot and must not try to validate the secret; only the server can.
	 */
	protected connection(settings: T | undefined): ActionContext {
		const token = resolveToken(settings);
		if (!token) {
			throw new AllChatError("no-token", missingTokenMessage());
		}
		if (!looksLikePat(token)) {
			throw new AllChatError("malformed-token", malformedTokenMessage());
		}
		return { baseUrl: resolveBaseUrl(settings), token };
	}

	/**
	 * Runs an action body and reports the outcome on the key.
	 *
	 * Success shows Stream Deck's tick. Every failure shows the alert, but the
	 * *log line* is what carries the meaning, and it is specific in every branch.
	 */
	protected async run(
		action: AnyAction<T>,
		work: () => Promise<string | void>,
	): Promise<void> {
		try {
			const note = await work();
			await this.ok(action);
			logger.info(`${this.label}: ${note ?? "ok"}`);
		} catch (error) {
			await this.report(action, error);
		}
	}

	/**
	 * Turns a thrown value into the right key state and the right log line.
	 *
	 * The `requires-premium` branch is deliberately the loudest and the most
	 * informative: the server gated *starting* a poll or prediction behind the
	 * premium `engagement` feature, and the moment somebody presses that key is
	 * exactly when telling them about the upgrade is useful. It is never
	 * collapsed into a generic "request failed".
	 */
	protected async report(action: AnyAction<T>, error: unknown): Promise<void> {
		await this.alert(action);

		if (error instanceof AllChatError) {
			switch (error.kind) {
				case "requires-premium":
					// Expected on a free account. Advertise, do not apologise, and
					// certainly do not try to work around the gate.
					logger.warn(`${this.label}: ${premiumGateMessage(this.premiumSubject())}`);
					logger.info(`${this.label}: server detail — ${error.message}`);
					await this.setTitleSafely(action, "Premium");
					return;

				case "unauthorized":
					logger.error(`${this.label}: ${unauthorizedMessage()}`);
					await this.setTitleSafely(action, "Token?");
					return;

				case "no-token":
				case "malformed-token":
				case "not-configured":
					logger.error(`${this.label}: ${error.message}`);
					await this.setTitleSafely(action, "Setup");
					return;

				default:
					logger.error(`${this.label}: ${error.message}`);
					return;
			}
		}

		const reason = error instanceof Error ? error.message : String(error);
		logger.error(`${this.label}: unexpected failure — ${reason}`);
	}

	/**
	 * What the premium message should call the thing being started. Overridden by
	 * the poll and prediction actions; chat send never hits the gate.
	 */
	protected premiumSubject(): "poll" | "prediction" {
		return "poll";
	}

	/** Shows the success tick, where the hardware supports one. */
	private async ok(action: AnyAction<T>): Promise<void> {
		if ("showOk" in action) {
			await action.showOk();
		}
	}

	/** Shows the alert glyph, where the hardware supports one. */
	private async alert(action: AnyAction<T>): Promise<void> {
		if ("showAlert" in action) {
			await action.showAlert();
		}
	}

	/**
	 * Best-effort title hint. Stream Deck ignores this when the user set their own
	 * title, and a failure here must never mask the original error.
	 */
	private async setTitleSafely(action: AnyAction<T>, title: string): Promise<void> {
		try {
			if ("setTitle" in action) {
				await action.setTitle(title);
			}
		} catch {
			// A cosmetic title is not worth reporting over the real failure.
		}
	}
}

/** The scoped logger, so individual actions can log without re-scoping. */
export { logger };
