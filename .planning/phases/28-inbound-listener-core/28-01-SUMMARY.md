---
phase: 28-inbound-listener-core
plan: "01"
subsystem: discord-listener, overlay-manager
tags: [discord, gateway, redis-streams, publisher, channel-registry]
dependency_graph:
  requires: []
  provides:
    - discord-listener/publisher.StreamPublisher
    - gateway.ChannelRegistry interface
    - gateway.MessagePublisher interface
    - gateway.GatewayClient.HandleMessageCreate
    - overlay-manager discord:channels: Redis registry
  affects:
    - services/discord-listener/gateway/client.go
    - services/discord-listener/publisher/stream_publisher.go
    - services/overlay-manager/handlers/sources.go
tech_stack:
  added:
    - services/discord-listener/publisher package (new)
  patterns:
    - TDD red-green with stub tests then full implementation
    - Interface-based decoupling (ChannelRegistry, MessagePublisher) to avoid circular imports
    - gateway.MessagePublisher uses interface{} payload bridged via JSON re-marshal in cmd/main.go
    - redis.Cmdable interface in overlay-manager for testability
key_files:
  created:
    - services/discord-listener/publisher/stream_publisher.go
    - services/discord-listener/gateway/message_create_test.go
    - services/discord-listener/publisher/stream_publisher_test.go
  modified:
    - services/discord-listener/gateway/types.go
    - services/discord-listener/gateway/client.go
    - services/discord-listener/cmd/main.go
    - services/overlay-manager/handlers/sources.go
    - services/overlay-manager/cmd/main.go
decisions:
  - "gateway.MessagePublisher uses interface{} payload (not *publisher.RawMessage) to prevent circular import between gateway and publisher packages; cmd/main.go's publisherAdapter bridges via JSON re-marshal"
  - "Pure Redis GET approach for ChannelRegistry (no in-memory set + Subscribe goroutine) — acceptable latency at v1.5 scale per CONTEXT.md"
  - "redis.Cmdable interface in overlay-manager SourcesHandler (not *redis.Client) for testability, consistent with plan spec"
  - "HandleMessageCreate exported (capital H) to enable direct unit testing from gateway_test package without exposing Connect()"
metrics:
  duration: "8 minutes"
  completed_date: "2026-03-15"
  tasks_completed: 3
  files_changed: 8
---

# Phase 28 Plan 01: MESSAGE_CREATE Dispatch and Publisher Summary

MESSAGE_CREATE dispatch wired end-to-end: Discord Gateway messages flow through bot-filter, channel-registry lookup, empty-content halt guard, tag enrichment, and are published to the chat:raw Redis Stream; overlay-manager maintains the discord:channels: Redis keys on source create/delete.

## What Was Built

### gateway/types.go

Added three new types after `ReadyEventData`:

- `MessageCreateData` — full MESSAGE_CREATE payload (id, channel_id, guild_id, content, timestamp, author, member)
- `DiscordUser` — author fields including the `bot` boolean flag
- `DiscordMember` — guild-specific member data (nick, roles)

### gateway/client.go

- Added `ChannelRegistry` interface: `GetOverlayForChannel(ctx, channelID) (overlayID, found, error)` and `Subscribe`
- Added `MessagePublisher` interface: `Publish(ctx, msg interface{}) error` (uses `interface{}` to avoid circular import)
- Extended `GatewayClient` struct with `registry`, `publisher`, and `firstMessageSeen` fields
- Updated `NewGatewayClient` to accept `registry ChannelRegistry` and `pub MessagePublisher` parameters
- Added `MESSAGE_CREATE` dispatch branch in `Connect()` loop
- Implemented `HandleMessageCreate` method with full filter chain:
  1. Bot filter (silent drop, no error)
  2. Channel registry lookup (DEBUG drop on missing, service continues on error)
  3. Empty content halt on first message (returns error that exits Connect loop)
  4. Tag building (author_id, member_nick from Member.Nick pointer)
  5. Timestamp parse with time.RFC3339 fallback to time.Now()
  6. Publisher call

### publisher/stream_publisher.go

New package mirroring kick-listener/publisher/redis.go exactly:
- `RawMessage` struct with same JSON tags
- `StreamPublisher` backed by `redis.Cmdable`
- `NewStreamPublisher(*redis.Client, *zap.Logger)` — production constructor
- `NewStreamPublisherFromCmdable(redis.Cmdable, *zap.Logger)` — test constructor
- `Publish()` calls `XAdd` to `chat:raw` stream with `"data"` field only

### cmd/main.go (discord-listener)

- Added `redisChannelRegistry` concrete type implementing `ChannelRegistry` via Redis GET on `discord:channels:{channelID}`
- Added `publisherAdapter` adapting `*publisher.StreamPublisher` to `gateway.MessagePublisher` via JSON re-marshal
- Wired both into `gateway.NewGatewayClient`

### handlers/sources.go (overlay-manager)

- Added `redis redis.Cmdable` field to `SourcesHandler`
- Updated `NewSourcesHandler` signature to accept `redisClient redis.Cmdable` as last parameter
- Added `setDiscordChannelRegistry` helper: JSON-marshals `discordChannelEntry{OverlayID, SourceID}`, SET `discord:channels:{channelID}` (TTL 0), then PUBLISH `discord:channel:invalidation`
- Added `delDiscordChannelRegistry` helper: DEL key, then PUBLISH invalidation
- Wired into `HandleAddSource` (after successful DB create for platform=="discord")
- Wired into `HandleDeleteSource` (after successful DB delete for platform=="discord")
- `inbound_channel_id` extracted from `source.Config["inbound_channel_id"]` with fallback to `source.ChannelID`

### cmd/main.go (overlay-manager)

Passed `redisClient` as the new final argument to `NewSourcesHandler`.

## Tests

| Test | File | Status |
|------|------|--------|
| TestHandleMessageCreate_BotFiltered | gateway/message_create_test.go | PASS |
| TestHandleMessageCreate_UnknownChannel | gateway/message_create_test.go | PASS |
| TestHandleMessageCreate_EmptyContent | gateway/message_create_test.go | PASS |
| TestHandleMessageCreate_HappyPath | gateway/message_create_test.go | PASS |
| TestPublish_HappyPath | publisher/stream_publisher_test.go | PASS |

## Commits

| Task | Commit | Description |
|------|--------|-------------|
| Task 1 (RED) | d9e54bb | test(28-01): add failing tests for MESSAGE_CREATE handling and publisher |
| Task 2 (GREEN) | e5090d7 | feat(28-01): implement MESSAGE_CREATE dispatch, publisher package, overlay-manager Redis wiring |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed type assertion on `map[string]interface{}`**

- **Found during:** Task 2
- **Issue:** `source.Config.(map[string]interface{})` failed to compile because `source.Config` is already typed as `map[string]interface{}`, not an `interface{}`
- **Fix:** Removed the outer type assertion; used direct map indexing `source.Config["inbound_channel_id"].(string)`
- **Files modified:** `services/overlay-manager/handlers/sources.go`
- **Commit:** e5090d7

**2. [Rule 2 - Missing critical functionality] Added `NewStreamPublisherFromCmdable` constructor**

- **Found during:** Task 2
- **Issue:** Publisher test (`TestPublish_HappyPath`) used a mock `redis.Cmdable` stub, but `NewStreamPublisher` only accepted `*redis.Client`. Tests would fail to build.
- **Fix:** Added `NewStreamPublisherFromCmdable(redis.Cmdable, *zap.Logger)` constructor that accepts the interface directly
- **Files modified:** `services/discord-listener/publisher/stream_publisher.go`
- **Commit:** e5090d7

## Self-Check: PASSED

- services/discord-listener/gateway/types.go — contains MessageCreateData: FOUND
- services/discord-listener/publisher/stream_publisher.go — contains StreamPublisher: FOUND
- services/overlay-manager/handlers/sources.go — contains discord:channels:: FOUND
- Commit d9e54bb: FOUND
- Commit e5090d7: FOUND
- `go test ./...` discord-listener: all pass
- `go build ./...` overlay-manager: clean
- `go vet ./...` discord-listener: no errors
