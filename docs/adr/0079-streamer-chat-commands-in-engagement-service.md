# ADR-0079: Streamer Chat Commands in engagement-service, With a Sandboxed JS Template Engine

**Date**: 2026-10-21
**Status**: Accepted
**Revised**: 2026-11-15 (released to everyone: the `chat_commands` gate is open and `/commands` is offered on every host)
**Deciders**: caesarakalaeii

## Context

Streamers want what Nightbot, StreamElements and Fossabot give them across every chat
All-Chat merges: commands (`!discord` answers with a link), keyword triggers ("clip that"
anywhere in a message makes a Twitch clip), templated replies where `{user}` becomes the
chatter and JavaScript covers what variables cannot express, and role limits (subscriber,
VIP, moderator, broadcaster). Replies are posted into the streamer's chat, sent by the
default (ADR-0081) All-Chat Bot account; anything that spends YouTube Data API quota must
be premium.

Constraints: the message-processor hot path stays clear (no database work there, no
second firehose consumer, this is ADR-0028's driver again); the existing send path is
reused (auth-service posts as the streamer via `/internal/chat/announce`, ADR-0028 H4-2,
with the scope check, the YouTube quota reserve-confirm-rollback, ADR-0006, and echo
dedup); streamer JavaScript must be bounded (any account can write it, and without a
memory cap `'x'.repeat(2**30)` takes the pod down); and replies must not self-trigger (a
reply arrives back in chat as a message from the sender, so a keyword its reply contains
would fire again forever).

## Considered options

- Extend the ADR-0028 path: message-processor forwards messages of channels that have
  commands to a durable stream, engagement-service runs them. No new service, reuses its
  ownership of chat commands and the native-id dedup (ADR-0015) that already ran. Cost:
  a second hook in the forwarder. Chosen.
- A new bot service with its own `chat:raw` consumer group: no message-processor change,
  but it deserialises the whole firehose for a handful of channels and needs its own
  dedup, role parsing and deployment.
- JS engine: goja (no memory limit; one allocation can OOM the pod) rejected.
- JS engine: QuickJS via `modernc.org/quickjs` (pure Go, no cgo): `SetMemoryLimit`,
  `SetEvalTimeout`, `SetMaxStackSize`, no host functions unless registered. Chosen.

## Decision

The ADR-0028 path extended, QuickJS as the engine.

**Transport.** engagement-service keeps the Redis SET `chatcmd:channels` of
`platform:channel` (lowercased) for every active chat source whose owner has an enabled
command or message-counting timer (Kick sources add their slug, which the Kick listener
stamps), rebuilt atomically (SADD to a temp key, RENAME) every 30s and after any command
or timer write. message-processor holds a copy refreshed every 10s and XADDs every message
of a listed channel (not only `!`-shaped ones, so keywords work) as a `CommandJob` to
`chatcmd:jobs`, carrying the chatter's `ChatRoles` and display name; the forward runs in
the existing off-hot-path forwarder after native dedup.

**Execution.** engagement-service consumes `chatcmd:jobs` (group `chatcmd`) with NOACK
reads: at-most-once on purpose, a lost command is a missed reply while a redelivered one
would answer twice. One query per streamer whose overlays carry the channel returns the
enabled commands and whether the channel is their own (Twitch: a users row of a Twitch
login; YouTube: `youtube_oauth_tokens`; Kick: a Kick login or `kick_oauth_tokens`), cached
5s. First match wins (trigger equals first word beats a keyword in the text), then
`min_role` against everyone < subscriber < vip < moderator < broadcaster, then three Redis
guards the same way on every replica: `chatcmd:echo:{owner}:{hash}` (reply just posted),
`chatcmd:seen:{owner}:{platform}:{msgid}` (already handled), `chatcmd:cd:{command}`
(cooldown). The command bumps `use_count`, optionally clips, renders its reply and posts
it through `/internal/chat/bot-send` (ADR-0081).

**Replies and clips.** Replies go to the platform the message came from and only when the
channel is the streamer's own (Twitch, Kick and YouTube only have a send path; TikTok and
Discord have none). YouTube replies require the owner's `users.is_premium`, checked per
send because each spends 5 quota units. Clips use `/internal/twitch/clip` in auth-service
(Helix Create Clip, `clips:edit`, the `clip` action in the opt-in re-consent, ADR-0017);
a clip trigger fires from any chat in the streamer's overlays. `return=commands` brings
the user back to `/commands` after the consent (allowlisted `ReturnPath`).

**Template language.** Single-brace tokens; an unknown or malformed token is left as
written: `{user}` `{touser}` `{query}` `{1}`...`{9}` `{count}` `{random}`/`{random 1 6}`,
`{pick a | b}`, `{time Europe/Berlin}`, `{platform}` `{channel}` `{clip}`, `{js ...}`.
`{js}` blocks are scanned with brace depth and string-literal awareness, evaluated by
indirect `eval` (completion value inserted, `var`s carried across blocks), get the chat
inputs as a JSON object on `globalThis` (never concatenated into code), and share a 100 ms
budget with 16 MB memory and a 256 KB stack per VM; a failed block renders `[js error]`,
and the editor preview returns the render without firing in chat. Nightbot's
`$(user)`, `$(query)`, `$(count)`, `$(time)`, `$(channel)`, `$(eval)` map to the tokens
above; StreamElements'/Fossabot's random pick and user levels map to `{pick}` and
`min_role`/`cooldown_seconds`. Left out: `$(customapi)`'s arbitrary-HTTP form
(`{urlfetch}`, ADR-0080, is the guarded subset), countdowns, platform lookups (each on
YouTube would spend quota per use), and per-platform reply targeting.

**Gates.** `chat_commands` (ADR-0008) gates create and update (`services/engagement-service/cmd/main.go`); it is open to every account, free since 2026-11-15. Delete is
never gated. The editor is `/commands`, linked from the top bar on every host.

## Implementation

`services/engagement-service/` (consumer `consumer/chatcmd.go`, template engine, clip and
send flows); QuickJS dependency `modernc.org/quickjs`. Migrations: 113 (`chat_commands`
gate). ADR-0080 adds `image_url` (114), `sound_url`/`alert_text` (123) and `shared/safehttp`; ADR-0081 adds `chat_bot_accounts`/`chat_bot_settings` (115), `chat_timers`
(116) and the `custom_chat_bot` gate.

## Consequences

- `chatcmd:jobs`, `chatcmd:channels` and three short-lived key families; `chat_commands`
  and the QuickJS dependency (pure Go, larger binary).
- Roles exist only where listeners send tags (Twitch, Kick, YouTube); TikTok and Discord
  chatters count as everyone, so a role-limited command never fires there.
- Twitch sending and clipping use the users-row Twitch token; a channel linked only via
  `twitch_oauth_tokens` is not served (same limit as the streamer send).
- engagement-service, message-processor and auth-service run from `:main` only.
