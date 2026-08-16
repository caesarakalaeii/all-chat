/**
 * Entry point for the All-Chat Stream Deck plugin.
 *
 * Registers the three action types and connects to the Stream Deck application.
 * Everything interesting lives in `src/actions/` and `src/allchat/`.
 */

import streamDeck from "@elgato/streamdeck";

import { PollControlAction } from "./actions/poll-control.js";
import { PredictionControlAction } from "./actions/prediction-control.js";
import { SendMessageAction } from "./actions/send-message.js";
import { DEFAULT_BASE_URL } from "./allchat/settings.js";

// INFO by default. The plugin log is written to disk, so nothing here — and
// nothing in any action — may ever contain a personal access token.
streamDeck.logger.setLevel("info");

streamDeck.actions.registerAction(new SendMessageAction());
streamDeck.actions.registerAction(new PollControlAction());
streamDeck.actions.registerAction(new PredictionControlAction());

streamDeck.logger.info(`All-Chat plugin starting; default host ${DEFAULT_BASE_URL}`);

await streamDeck.connect();
