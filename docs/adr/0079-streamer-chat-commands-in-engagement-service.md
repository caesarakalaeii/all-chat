# ADR-0079: Streamer Chat Commands in engagement-service, With a Sandboxed JS Template Engine

**Date**: 2026-10-21
**Status**: Accepted
**Deciders**: caesarakalaeii

## Context and Problem Statement

Streamers want what Nightbot, StreamElements and Fossabot give them in one chat,
across every chat All-Chat merges:

- **Commands**: `!discord` answers with a link.
- **Keyword triggers**: "clip that" anywhere in a message makes a Twitch clip.
- **Templated replies**: `{user}` becomes the chatter. JavaScript is allowed for
  anything the variables cannot express (Nightbot's `$(eval)`).
- **Role limits**: a command may be limited to subscribers, VIPs, moderators or
  the broadcaster.

A reply is posted from the streamer's own account, not from a bot account.
Anything that spends YouTube Data API quota must be premium.

The questions are where commands live, how a chat message reaches them without
taxing the message-processor hot path, how replies avoid answering themselves,
and how to run streamer-written JavaScript on a shared service.

## Decision Drivers

- **Hot-path safety.** This is ADR-0028's driver again. message-processor
  must not take on database work or a second firehose consumer.
- **Reuse the send path.** auth-service already posts as the streamer
  (`/internal/chat/announce`, ADR-0028 H4-2). That path includes the scope
  check, the YouTube quota reserve-confirm-rollback (ADR-0006) and echo dedup.
- **Multi-tenant JS.** Any premium-or-not account can write the code, so CPU,
  memory and I/O must be bounded. Without a memory cap, `'x'.repeat(2**30)`
  takes the pod down.
- **No self-triggering.** A reply arrives back in chat as a message from the
  broadcaster. A keyword its reply contains would fire again forever.

## Considered Options

1. **Extend the ADR-0028 path.** message-processor forwards messages from
   channels that have commands to a new durable stream, and engagement-service
   runs them.
   - ✅ No new service. engagement-service already owns chat commands (votes,
     wagers), viewer identity and the auth-service client.
   - ✅ Native-id dedup (ADR-0015) has already run, so an IRC+EventSub duplicate
     cannot fire a command twice.
   - ❌ A second hook in the message-processor forwarder.
2. **A new bot service with its own `chat:raw` consumer group**, like
   alert-processor (ADR-0068).
   - ✅ No message-processor change.
   - ❌ Deserialises the whole firehose to act on a handful of channels.
   - ❌ Needs its own native dedup and its own role parsing.
   - ❌ One more deployment.
3. **JavaScript engine: goja** (pure Go).
   - ❌ No memory limit; a single allocation can OOM the pod.
4. **JavaScript engine: QuickJS via `modernc.org/quickjs`** (pure Go, no cgo).
   - ✅ Has `SetMemoryLimit`, `SetEvalTimeout` and `SetMaxStackSize`.
   - ✅ No host functions unless registered: no `require`, `fetch`, timers,
     file or network access.

## Decision Outcome

**Chosen**: options 1 and 4.

### Transport

engagement-service keeps the Redis SET `chatcmd:channels`. It holds
`platform:channel`, lowercased, for every active chat source on an overlay whose
owner has an enabled command. Kick sources also add their slug, which is what
the Kick listener stamps. The set is rebuilt atomically (SADD to a temp key,
then RENAME) every 30s and right after any command write.

message-processor holds an in-process copy of the set, refreshed every 10s.
For a chat message on a listed channel it XADDs a `CommandJob` to
`chatcmd:jobs`. The job carries the chatter's roles (`ChatRoles`, read from the
same listener tags the normalizers use) and display name. Ordinary chat on
other channels pays only a map lookup.

The forward runs in the existing off-hot-path engagement forwarder after
native dedup. A keyword can match any text, so every message of a command
channel is forwarded, not only `!`-prefixed ones.

### Execution

engagement-service consumes `chatcmd:jobs` (group `chatcmd`) with NOACK reads.
Delivery is at most once on purpose: a lost command is a missed reply, while a
redelivered one would answer twice.

For each streamer whose overlays carry the channel, one query returns that
streamer's enabled commands and whether the channel is their own. The result is
cached for 5s per channel. Ownership uses the predicates moderation-service and
the send path use:

- Twitch: the users row of a Twitch login.
- YouTube: `youtube_oauth_tokens`.
- Kick: a Kick login or `kick_oauth_tokens`.

The first match fires: a command whose trigger equals the first word wins over
a keyword the text contains. Its `min_role` is then checked against the
hierarchy everyone < subscriber < vip < moderator < broadcaster. Three Redis
keys decide, the same way on every replica, whether the match may run:

- `chatcmd:echo:{owner}:{hash}`: the text is a reply this streamer just posted.
- `chatcmd:seen:{owner}:{platform}:{msgid}`: this message was already handled.
  Kick publishes one copy per overlay with the same id.
- `chatcmd:cd:{command}`: the command is in its cooldown.

If the match passes, the command bumps `use_count`, optionally clips, renders
its reply and posts it.

### Replies and clips

- **Replies** go through `/internal/chat/announce` to the platform the message
  came from, and only when the channel is the streamer's own. A reply to a
  co-streamer's chat would otherwise land in the streamer's chat.
- **TikTok, Discord and the beta platforms** have no send path. Commands there
  can still clip.
- **YouTube replies require the owner's `users.is_premium`**, checked per send,
  because a send spends 5 quota units. This is independent of the feature gate.
- **Clips** use a new `/internal/twitch/clip` in auth-service (Helix Create
  Clip, `clips:edit`). A clip trigger fires from any chat in the streamer's
  overlays, which is the "clip when it is said in any chat" case.
- **The `clips:edit` grant** is a new `clip` action in the opt-in Twitch
  re-consent (ADR-0017). It folds in `user:write:chat` and is preserved against
  downgrades. A `return=commands` parameter (allowlisted `ReturnPath`) brings
  the user back to `/commands`.

### Template language

Single-brace tokens. An unknown or malformed token is left as written:

- `{user}` `{touser}` `{query}` `{1}`…`{9}` `{count}`
- `{random}` / `{random 1 6}`, `{pick a | b}`, `{time Europe/Berlin}`
- `{platform}` `{channel}` `{clip}`
- `{js …}`

The `{js}` block has these properties:

- **Scanning.** The block is scanned with brace depth and string/template-literal
  awareness.
- **Evaluation.** It is evaluated by indirect `eval`, so its completion value is
  inserted and `var`s carry over to later blocks of the same reply.
- **Inputs.** The chat inputs enter the VM as a JSON object assigned to
  `globalThis`, never concatenated into code.
- **Limits.** All `{js}` blocks of one render share 100 ms. The VM has 16 MB of
  memory and a 256 KB stack.

A failed block renders `[js error]`. The editor's preview endpoint returns the
message text so streamers can debug without firing in chat.

### Gating and release

- **Feature gate.** `chat_commands` (ADR-0008) gates create/update, seeded
  **open** (migration 113): the one paid part is the YouTube send check above.
  Delete is never gated.
- **Editor.** The editor is `/commands`. It is hidden outside beta.allch.at via
  `BETA_ONLY_FEATURES` in `platform-availability.ts`.

### Compared with the bots

Nightbot's core variables have equivalents:

| Nightbot | All-Chat |
|---|---|
| `$(user)`, `$(touser)` | `{user}`, `{touser}` |
| `$(query)`, arguments | `{query}`, `{1}`…`{9}` |
| `$(count)` | `{count}` |
| `$(time)` | `{time}` |
| `$(channel)` | `{channel}` |
| `$(eval)` | `{js}` |

StreamElements' and Fossabot's random pick maps to `{pick}`, and their user
levels and cooldowns to `min_role` and `cooldown_seconds`.

These are deliberately left out:

- `$(urlfetch)` / `$(customapi)`: server-side fetches of user-chosen URLs are an
  SSRF and latency surface.
- `$(userlevel)` and `$(userid)`: the `role` is available to `{js}`.
- Countdowns.
- Platform lookups (uptime, game, followage): one of these on YouTube would
  spend quota per use.
- Timers (scheduled messages).
- Per-platform reply targeting.

## Consequences

- **New stream and keys.** One new stream (`chatcmd:jobs`), one SET
  (`chatcmd:channels`) and three short-lived key families. The forwarder now
  sends every message of a command channel, not just command-shaped ones.
  Throughput scales with the chat volume of channels that have commands.
- **New dependency.** engagement-service gains `modernc.org/quickjs` (pure Go,
  no cgo, larger binary).
- **New database objects.** `chat_commands` (one row per command, unique
  case-insensitive trigger per user and kind) and the `chat_commands` gate row.
- **Platform reach.** Roles exist only where listeners send role tags: Twitch,
  Kick and YouTube. TikTok and Discord chatters count as everyone, so a
  role-limited command never fires there.
- **Twitch login only.** auth-service sends and clips with the users-row Twitch
  token, so Twitch replies and clips need a Twitch login. A Twitch channel only
  linked via `twitch_oauth_tokens` is not served yet. That is the same limit as
  the existing streamer send.
- **Staging.** Only the frontend and api-gateway have a beta deployment.
  engagement-service, message-processor and auth-service run from `:main`, so
  the feature works on beta once its backend reaches main.
