# ADR-0081: Chat Command Replies From an All-Chat Bot, Premium Custom Bots, and Timers

**Date**: 2026-10-23
**Status**: Accepted
**Deciders**: caesarakalaeii

## Context and Problem Statement

ADR-0079 posts every chat command reply from the streamer's own account and
leaves timers out. Streamers coming from Nightbot, Wizebot and StreamElements
asked for three things:

- **A bot account.** Replies should come from an account named after All-Chat,
  not from the streamer. A reply from the streamer reads as if they typed it,
  and a branded bot shows the All-Chat name in every chat it answers in.
- **A custom bot name.** Wizebot sells this: the streamer connects their own
  bot account and replies carry that name. It is a natural premium feature.
- **Timers.** A message posted automatically every X minutes, every X chat
  messages, or both ("every 15 minutes, but only if 20 messages came in").

The questions are how each platform lets a third account post into the
streamer's chat, where the credentials live, and how a timer fires exactly once
across engagement-service replicas without a live/offline signal.

## Decision Drivers

- **Platform rules decide the mechanism.** Each platform has its own way to let
  a bot post in someone else's chat. The design follows them rather than
  pretending the three are the same.
- **Reuse the send path.** auth-service already sends as the streamer, with the
  per-streamer rate limit, the YouTube quota reserve-confirm-rollback (ADR-0006)
  and error classification. Bot sends belong next to it.
- **Never break an existing command.** A missing grant or bot row must not turn
  a working reply into a failure.
- **No new leader election.** engagement-service runs several replicas and
  coordinates through Redis consumer groups and the database.
- **No quota burn while offline.** A timer must not spend YouTube Data API
  units discovering that a channel is not live.

## Considered Options

### Sender

1. **One All-Chat Bot by default, the streamer's own account as an opt-out
   (chosen).** The brand shows up everywhere commands run.
2. **The streamer's own account by default, the bot as an opt-in.** No change
   for existing users, but almost nobody would see the brand.

### How the bot posts

- **Twitch.** Helix Send Chat Message with an **app access token**. The bot
  account authorizes our app once with `user:bot` and `user:write:chat`, and the
  broadcaster grants `channel:bot`. Twitch then shows the message from the bot
  with the Chat Bot badge. The same holds for a custom bot, so a Twitch bot row
  needs only the bot's user id at send time.
- **Kick.** `POST /public/v1/chat` with `type: "bot"` and the **streamer's**
  token posts as the bot configured on our Kick app. No second account and no
  database row. A custom Kick bot posts with its own token as `type: "user"`
  into the streamer's `broadcaster_user_id`.
- **YouTube.** There is no bot concept. The bot is an ordinary Google account
  whose token inserts into the streamer's live chat. Quota cost and the premium
  rule for YouTube sends are unchanged.

### Timer coordination

1. **Leader election per timer.** Rejected: engagement-service has none, and
   adding one for a 15-second tick is out of proportion.
2. **A conditional `UPDATE` claim (chosen).** Every replica ticks, decides
   which timers are due, and claims each one with
   `UPDATE … WHERE id = $1 AND last_fired_at IS NOT DISTINCT FROM $seen`. Only
   one replica's update matches; the others get zero rows and skip.

## Decision Outcome

### Data

- `chat_bot_accounts` (migration 115, owned by auth-service): one row per bot
  account. `owner_user_id` NULL is the All-Chat Bot, otherwise a streamer's
  custom bot. Unique per owner and platform. Tokens are encrypted like every
  other OAuth token.
- `chat_bot_settings` (migration 115, written by engagement-service, read by
  auth-service): `sender` is `bot` or `self`. No row means `bot`.
- `chat_timers` (migration 116, owned by engagement-service): name, message
  template, optional `interval_minutes` (5 to 1440), optional `min_messages`
  (1 to 10000), at least one of them required, plus `last_fired_at`,
  `lines_at_last_fire` and `fire_count`. At most 20 per streamer.
- Feature gate `custom_chat_bot`, seeded `is_premium=TRUE, early_access=TRUE`.
  Timer and sender-setting writes use the existing `chat_commands` gate.

### Sending

engagement-service sends command replies and timer posts through a new
auth-service endpoint, `POST /internal/chat/bot-send`, with a `purpose` of
`command` or `timer`. Poll and prediction round announcements stay on
`/internal/chat/announce` as the streamer.

auth-service picks the sender per platform:

1. The streamer chose `self`: the existing streamer send.
2. The streamer is premium and has a custom bot on that platform: the custom bot.
3. Otherwise the All-Chat Bot.

A Twitch bot send needs the broadcaster's `channel:bot` grant, a new `bot`
action in the existing re-consent flow (ADR-0017). Without it, or when the
All-Chat Bot row for Twitch or YouTube is missing, the send **falls back to the
streamer's own account** and reports `sender: "self"`. Existing commands keep
working while the operator connects the bot accounts and streamers grant
`channel:bot`.

For `purpose: "timer"`, YouTube reads the live chat id only from the Redis
stream state the YouTube listeners keep. It never falls back to the Data API
lookup, so an offline channel costs nothing.

### Connecting bot accounts

`GET /auth/{twitch,kick,youtube}/bot-connect?scope=custom|allchat` returns an
authorize URL. The flow mirrors the delegated-moderator consent (ADR-0048): it
reuses each platform's existing callback, stores the bot row, and never signs
the bot account in. `custom` needs the `custom_chat_bot` gate; `allchat` needs
an admin and exists for Twitch and YouTube only. The admin page is
`/admin/chat-bots`; streamers manage custom bots on `/commands`.

### Timers

- **Counting.** The chatcmd consumer increments `chattimer:lines:{owner}` once
  per unique chat message on the streamer's own channels. Kick's per-overlay
  copies are deduplicated by message id, and the echo of a reply or timer post
  is skipped. Streamers with a message-counting timer join `chatcmd:channels`
  even without commands.
- **Firing.** Every replica ticks every 15 seconds. A timer is due when its
  interval has elapsed since `last_fired_at`, when the counter has grown by
  `min_messages` since `lines_at_last_fire`, or when both hold if both are set.
  A messages-only timer also waits at least 60 seconds between posts.
- **Posting.** The claim winner renders the message with the ADR-0079 template
  engine without chatter context (`{count}` is the fire count), registers the
  echo so a keyword command cannot fire on it, and posts to Twitch and Kick,
  plus YouTube when the owner is premium.

## Consequences

- **Operator setup.** Someone must create the bot accounts, connect the Twitch
  and YouTube All-Chat Bot on `/admin/chat-bots`, and configure the bot on the
  Kick developer app. Until then, Twitch and YouTube replies fall back to the
  streamer.
- **Streamer grant.** Twitch bot replies need each streamer to grant
  `channel:bot` once, from `/commands`.
- **Offline timers.** There is no cross-platform live signal. A time-only timer
  posts into Twitch and Kick chat while the stream is offline. Setting a
  minimum message count avoids that, and the editor says so.
- **Staging.** auth-service and engagement-service run from `:main` only, so the
  feature reaches beta.allch.at once its backend is on main. It stays behind the
  `chat_commands` early-access gate and the beta-only `/commands` page.
- **Supersedes in part.** ADR-0079's "a reply is posted from the streamer's own
  account" and "timers are left out" no longer hold.
