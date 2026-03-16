---
phase: 28-inbound-listener-core
verified: 2026-03-15T00:00:00Z
status: passed
score: 11/11 must-haves verified
re_verification: false
---

# Phase 28: Inbound Listener Core Verification Report

**Phase Goal:** Implement the inbound message pipeline for Discord: discord-listener dispatches MESSAGE_CREATE events to chat:raw Redis Stream, message-processor can normalize Discord messages into the unified format.
**Verified:** 2026-03-15
**Status:** passed
**Re-verification:** No — initial verification

---

## Goal Achievement

### Observable Truths

| #  | Truth | Status | Evidence |
|----|-------|--------|---------|
| 1  | A message from a configured Discord channel is published to the chat:raw Redis Stream | VERIFIED | `HandleMessageCreate` in client.go calls `c.publisher.Publish`; publisher XAdds to `chat:raw`; TestHandleMessageCreate_HappyPath and TestPublish_HappyPath both PASS |
| 2  | Bot messages (author.bot == true) are silently dropped before reaching Redis | VERIFIED | client.go L219-227: `if msg.Author.Bot` returns nil with no publish; TestHandleMessageCreate_BotFiltered PASS |
| 3  | Messages from unconfigured channels are dropped at DEBUG with no publish | VERIFIED | client.go L241-248: `if !found` logs Debug and returns nil; TestHandleMessageCreate_UnknownChannel PASS |
| 4  | First MESSAGE_CREATE with empty content causes service to halt with ERROR log | VERIFIED | client.go L255-262: `if !firstSeen && msg.Content == ""` logs Error and returns error; TestHandleMessageCreate_EmptyContent PASS |
| 5  | overlay-manager writes the channel registry Redis key before publishing the invalidation Pub/Sub event | VERIFIED | sources.go L68-80: `setDiscordChannelRegistry` calls `h.redis.Set` then `h.redis.Publish` in order |
| 6  | A RawChatMessage with platform=discord is normalized to UnifiedChatMessage with correct Username, DisplayName, Color, and Badges | VERIFIED | discord_normalizer.go implements full mapping; TestDiscordNormalizer_HappyPath PASS |
| 7  | member_nick tag used as DisplayName; falls back to Username when empty | VERIFIED | discord_normalizer.go L24-27; TestDiscordNormalizer_NickFallback PASS |
| 8  | role_color tag #000000 or empty results in User.Color == empty string | VERIFIED | discord_normalizer.go L29-32; TestDiscordNormalizer_BlackColor and TestDiscordNormalizer_EmptyColor PASS |
| 9  | badges tag comma-separated moderator/admin/vip values produce Badge slice with Version=1 | VERIFIED | `extractDiscordBadges` function L62-74; TestDiscordNormalizer_Badges and TestDiscordNormalizer_NoBadges PASS |
| 10 | DiscordNormalizer rejects platform != discord with a descriptive error | VERIFIED | discord_normalizer.go L20-22: `if raw.Platform != "discord" { return nil, fmt.Errorf("unsupported platform: %s", ...)}`; TestDiscordNormalizer_WrongPlatform PASS |
| 11 | DiscordNormalizer is registered in message-processor normalizers map under key discord | VERIFIED | message-processor/cmd/main.go L134: `discordNormalizer := normalizer.NewDiscordNormalizer()`, L143: `"discord": discordNormalizer` |

**Score:** 11/11 truths verified

---

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `services/discord-listener/gateway/types.go` | MessageCreateData, DiscordUser, DiscordMember structs | VERIFIED | All three types present at L72-94; properly tagged JSON fields |
| `services/discord-listener/gateway/client.go` | MESSAGE_CREATE dispatch branch, HandleMessageCreate method, ChannelRegistry interface | VERIFIED | ChannelRegistry interface L25-31; MessagePublisher interface L36-38; HandleMessageCreate L217; MESSAGE_CREATE dispatch L176-185 |
| `services/discord-listener/publisher/stream_publisher.go` | StreamPublisher wrapping redis.Cmdable, Publish method | VERIFIED | StreamPublisher struct L40-43; Publish L63-98; XAdds to `chat:raw` with single `data` field |
| `services/discord-listener/gateway/message_create_test.go` | Unit tests: bot filter, unknown channel drop, empty content halt, happy path | VERIFIED | 4 tests present; all PASS |
| `services/discord-listener/publisher/stream_publisher_test.go` | Unit test: happy path publish | VERIFIED | TestPublish_HappyPath present; PASS |
| `services/overlay-manager/handlers/sources.go` | Redis channel registry key write + Pub/Sub invalidation on Discord source create/delete | VERIFIED | `setDiscordChannelRegistry` and `delDiscordChannelRegistry` helpers present; wired into HandleAddSource (L420-427) and HandleDeleteSource (L506-512) |
| `services/message-processor/normalizer/discord_normalizer.go` | DiscordNormalizer implementing Normalizer interface | VERIFIED | `DiscordNormalizer` struct with `Normalize` method; platform guard; all tag mappings implemented |
| `services/message-processor/normalizer/discord_normalizer_test.go` | Unit tests: happy path, nick fallback, black color, wrong platform | VERIFIED | 7 tests present (HappyPath, NickFallback, BlackColor, EmptyColor, WrongPlatform, Badges, NoBadges); all PASS |
| `services/message-processor/cmd/main.go` | DiscordNormalizer registered under 'discord' key | VERIFIED | L134 and L143 confirmed |

---

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `services/discord-listener/gateway/client.go` | `services/discord-listener/publisher/stream_publisher.go` | `HandleMessageCreate` calls `publisher.Publish` | WIRED | L301: `c.publisher.Publish(ctx, rawMsg)`; publisherAdapter in cmd/main.go bridges gateway.MessagePublisher to StreamPublisher.Publish |
| `services/overlay-manager/handlers/sources.go` | Redis key `discord:channels:{channel_id}` | `redisClient.Set` before `redisClient.Publish` | WIRED | L68-80: SET then PUBLISH; order verified in `setDiscordChannelRegistry` |
| `services/message-processor/cmd/main.go` | `services/message-processor/normalizer/discord_normalizer.go` | `normalizers["discord"] = discordNormalizer` | WIRED | L134: `discordNormalizer := normalizer.NewDiscordNormalizer()`, L143: `"discord": discordNormalizer` |
| `services/discord-listener/cmd/main.go` | `services/discord-listener/gateway/client.go` | `NewGatewayClient` wired with registry and pubAdapter | WIRED | L97-101: registry and pubAdapter constructed and passed to NewGatewayClient |
| `services/overlay-manager/cmd/main.go` | `services/overlay-manager/handlers/sources.go` | `NewSourcesHandler` receives `redisClient` | WIRED | L138: `handlers.NewSourcesHandler(sourceRepo, overlayRepo, dbPool, log, redisClient)` |

---

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|---------|
| INBD-01 | 28-01-PLAN.md | Discord channel messages appear in overlays as a first-class chat source | SATISFIED | End-to-end pipeline: MESSAGE_CREATE → bot/channel filter → tags → XAdd to chat:raw; all unit tests pass; discord-listener builds cleanly |
| INBD-02 | 28-02-PLAN.md | Discord messages are normalized to the unified RawChatMessage schema via a discord normalizer in message-processor | SATISFIED | DiscordNormalizer implements Normalizer interface, maps all Tags fields, registered under "discord" key; 7 tests pass; message-processor builds cleanly |

Both requirements confirmed marked complete in REQUIREMENTS.md (L74-75).

---

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `services/discord-listener/cmd/main.go` | 107 | `TODO(Phase 31): gate on shard ownership via source-manager leader election` | Info | Forward-looking placeholder for future phase; does not affect Phase 28 correctness |

No blockers or warnings found. The single TODO is correctly scoped to Phase 31.

---

### Build and Test Results

All services build and pass vet with no errors:

- `discord-listener`: `go build ./...` clean, `go vet ./...` clean
- `overlay-manager`: `go build ./...` clean
- `message-processor`: `go build ./...` clean, `go vet ./...` clean

Test results:
- `TestHandleMessageCreate_BotFiltered` — PASS
- `TestHandleMessageCreate_UnknownChannel` — PASS
- `TestHandleMessageCreate_EmptyContent` — PASS
- `TestHandleMessageCreate_HappyPath` — PASS
- `TestPublish_HappyPath` — PASS
- `TestDiscordNormalizer_HappyPath` — PASS
- `TestDiscordNormalizer_NickFallback` — PASS
- `TestDiscordNormalizer_BlackColor` — PASS
- `TestDiscordNormalizer_EmptyColor` — PASS
- `TestDiscordNormalizer_WrongPlatform` — PASS
- `TestDiscordNormalizer_Badges` — PASS
- `TestDiscordNormalizer_NoBadges` — PASS

---

### Human Verification Required

None. All observable behaviors are covered by unit tests. No visual, real-time, or external service behavior requires human testing for this phase.

---

### Notable Implementation Decisions

**Circular import avoidance:** `gateway.MessagePublisher` uses `interface{}` payload instead of `*publisher.RawMessage` to prevent a circular import between the gateway and publisher packages. `cmd/main.go` bridges this via `publisherAdapter` with JSON re-marshal. This is a necessary architectural decision, not a code smell.

**Pure Redis GET for ChannelRegistry:** `redisChannelRegistry.GetOverlayForChannel` performs a direct Redis GET per message rather than maintaining an in-memory set with Subscribe. Per CONTEXT.md this is acceptable latency at v1.5 scale.

**redis.Cmdable interface in overlay-manager:** `SourcesHandler.redis` accepts `redis.Cmdable` (not `*redis.Client`) for testability, consistent with the plan specification.

---

_Verified: 2026-03-15_
_Verifier: Claude (gsd-verifier)_
