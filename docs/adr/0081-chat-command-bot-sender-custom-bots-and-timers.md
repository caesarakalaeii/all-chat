# ADR-0081: Chat Command Replies From an All-Chat Bot, Premium Custom Bots, and Timers

**Date**: 2026-10-23
**Status**: Accepted
**Revised**: 2026-11-15 (released: `chat_commands` is open to everyone and `custom_chat_bot` is premium without early access)
**Deciders**: caesarakalaeii

## Context

ADR-0079 posts every chat command reply from the streamer's own account and leaves timers
out. Streamers asked for a branded All-Chat Bot sender, an (opt-in, premium) custom
bot name like Wizebot sells, and timers posting every X minutes, every X chat messages, or
both. Constraints: each platform has its own way to let a third account post, and the
design follows them rather than pretending they are the same; the send path is reused
(auth-service, with its rate limit, YouTube quota accounting, ADR-0006, and error
classification); a missing grant or bot row must not break an existing command; no leader
election (engagement-service coordinates with Redis and the database); and a timer must
not spend YouTube quota discovering a channel is offline.

## Considered options

- All-Chat Bot by default, streamer's own account as opt-out (chosen), or the reverse
  (nobody would see the brand).
- Timer coordination: per-timer leader election (rejected, out of proportion for a 15 s
  tick) versus a conditional `UPDATE` claim (chosen): every replica ticks and claims each
  due timer with `UPDATE ... WHERE id = $1 AND last_fired_at IS NOT DISTINCT FROM $seen`,
  so only one replica's update matches.

## Decision

**Data.** `chat_bot_accounts` (migration 115, auth-service-owned), one row per bot
account (`owner_user_id` NULL = All-Chat Bot), unique per owner and platform, tokens
encrypted like every OAuth token; `chat_bot_settings` (115, engagement-service reads/writes)
with `sender` `bot` or `self` (no row = `bot`); `chat_timers` (116): name, message
template, optional `interval_minutes` (5-1440), optional `min_messages` (1-10000), at
least one required, plus fire bookkeeping; at most 20 per streamer. `custom_chat_bot` is
premium (early access off since 2026-11-15); timer and sender-setting writes use the
`chat_commands` gate.

**Platform mechanics.** Twitch: Helix Send Chat Message with an app access token; the
bot account has `user:bot`/`user:write:chat`, the broadcaster grants `channel:bot` (the
`bot` re-consent action, ADR-0017), which yields the Chat Bot badge and works for a
custom bot too, so a Twitch bot row needs only the bot's user id. An undelivered message
returns 200 with `is_sent=false`, counted as `message_dropped` with Twitch's
`drop_reason` logged. Kick: `POST /public/v1/chat` with `type: "bot"` and the streamer's
token posts as the bot on our Kick app; a custom Kick bot posts with its own token as
`type: "user"` into the streamer's `broadcaster_user_id`. YouTube is an ordinary Google
account whose token inserts into the streamer's live chat; quota and premium rules are
unchanged.

**Sending.** engagement-service posts command replies and timer messages through
auth-service `POST /internal/chat/bot-send` (`purpose` `command` or `timer`); poll and
prediction announcements stay on `/internal/chat/announce` as the streamer. auth-service
picks per platform: streamer chose `self`, else premium custom bot, else the All-Chat
Bot. A missing `channel:bot` grant or bot row falls back to the streamer's account
(`sender: "self"`) so existing commands keep working. Timer sends to YouTube read the
live chat id only from the Redis stream state the YouTube listeners keep, never the Data
API, so an offline channel costs nothing. A post that went out but was not delivered does
not fall back to self: `sender`, `error_kind` and the platform `detail` are stored
(`chatcmd:sendfail:{owner}`, 7 days) and returned by `GET /chat-commands` as
`send_failures`, which `/commands` explains, naming who must act.

**Connecting bots.** `GET /auth/{twitch,kick,youtube}/bot-connect?scope=custom|allchat`
returns an authorize URL, mirroring the delegated-moderator consent (ADR-0048): it reuses
each platform's callback, stores the bot row, never signs the bot account in. `custom`
needs the `custom_chat_bot` gate; `allchat` needs an admin (Twitch and YouTube only).
Admin page `/admin/chat-bots`; streamers manage custom bots on `/commands`.

**Timers.** The chatcmd consumer increments `chattimer:lines:{owner}` once per unique chat
message on the streamer's own channels (Kick copies deduplicated by message id, echoes
skipped); streamers with a message-counting timer join `chatcmd:channels` even without
commands. Every replica ticks every 15 s; a timer is due when its interval elapsed, when
the counter grew by `min_messages` since `lines_at_last_fire`, or both if both set; a
messages-only timer also waits at least 60 s between posts. The claim winner renders the
message with the ADR-0079 engine without chatter context (`{count}` is the fire count),
registers the echo so keyword commands cannot fire on it, and posts to Twitch and Kick,
plus YouTube when the owner is premium.

## Consequences

- Operator setup: create the bot accounts, connect Twitch and YouTube on
  `/admin/chat-bots`, configure the Kick app bot; until then Twitch and YouTube replies
  fall back to the streamer, and each streamer must grant `channel:bot` once.
- No cross-platform live signal: a time-only timer posts into offline Twitch and Kick
  chat; a minimum message count avoids that and the editor says so.
- auth-service and engagement-service run from `:main` only.
