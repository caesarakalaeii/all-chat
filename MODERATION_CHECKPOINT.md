# Overlay Moderation — Implementation Checkpoint

Working handoff doc so a fresh session can continue. Delete before final PR.

- **Worktree**: `/home/caesar/git/all-chat/.claude/worktrees/overlay-moderation`
- **Branch**: `worktree-overlay-moderation` (off `origin/main` @ `bbf8b76f`)
- **Plan**: `/home/caesar/.claude/plans/this-is-going-to-deep-noodle.md`
- **ADR**: `docs/adr/0017-chat-moderation-write-path.md`
- **Deployment repo changes (separate, NOT this git branch)**: `/home/caesar/git/caesar-deployment` — `apps/workloads/all-chat/moderation-service-deployment.yaml` (new), `configmap.yaml` (+`MODERATION_SERVICE_URL`), `kustomization.yaml` (+manifest). Commit/push that repo separately for ArgoCD.

## Decisions locked (from the user)
- Platforms: **all feasible incl. YouTube** — Twitch full, Kick timeout/ban, Discord delete-only, YouTube ban-only (gated on Google re-verify), TikTok unsupported.
- **Owner-only** moderation; `shared_overlay` sources blocked.
- Toggles **view-local** (localStorage), never write `visual_settings`.
- Impersonated moderation **allowed but audited as the admin**.
- **Least-privilege scopes** (non-negotiable): opt-in only, per platform/account, minimised to enabled actions; login/add-source flows untouched.

## DONE + tested (green: `go vet`, `go build`, `gofmt`, full test suite)

### Phase 0 backend — complete
- `services/moderation-service/` new service:
  - `publisher/deletion.go` — emits `RawChatMessage{message_deletion}` to `chat:raw` (reflect-back). Shapes match `twitch-eventsub-listener/webhooks/handler.go` + `message-processor` consumer/normalizer. Unit-tested.
  - `repository/repository.go` — `VerifyOverlayOwnership`, `IsModeratableSource`, `ListModeratableSources` (excludes `shared_overlay`). testcontainers-tested.
  - `audit/store.go` + migration `migrations/060_moderation_actions.sql` (+down) — records every command incl. `impersonated_by`. testcontainers-tested.
  - `handler/moderation.go` — owner-only authz + source-membership + platform-support gating; delete/timeout/ban/unban (**dry-run**: emits reflect-back, no platform call yet); capabilities endpoint. Unit-tested with fakes.
  - `handler/idempotency.go` — `Idempotency-Key` dedup (miniredis-tested). Rate-limit via `shared/ratelimit` wired in `cmd/main.go`.
  - `models/moderation.go` — request DTOs + `Capabilities`/`SourceCapability` + platform support matrix.
  - `cmd/main.go`, `Dockerfile`.
- `shared/middleware/auth.go` — threads `impersonated_by`/`impersonated_user` into gin context (additive). Tested.
- api-gateway: registry entry + protected routes (`services/api-gateway/models/service_config.go`, `cmd/main.go`). Builds.
- ADR-0017 + index; READMEs (CLAUDE.md, root README, `services/moderation-service/README.md`).
- k8s manifests (deployment repo, see above).

### Phase 1 backend — Twitch client only
- `clients/twitch.go` — Helix `DeleteMessage`/`TimeoutUser`/`BanUser`/`UnbanUser` (broadcaster==moderator); 401→`ErrUnauthorized`, 403→`ErrForbidden`. httptest-tested.

### Phase 1 backend — Twitch token resolution + dispatcher WIRED (NEXT STEPS 1+2 DONE, all tests green)
- `tokens/source.go` — `TwitchSource.Resolve(userID, channelID)` selects the requesting user's OWN broadcaster credential (users row preferred over `twitch_oauth_tokens`, ADR-0016), decrypts via `MultiKeyEncryptor`. **broadcaster_id comes straight from `users.twitch_id` / `twitch_oauth_tokens.twitch_user_id`** — no Helix login→id lookup needed. `Refresh()` does the id.twitch.tv refresh-grant + re-encrypt + write-back to the origin row. `ErrNoCredential`→422. testcontainers-tested (users path, linked path, users-preference, scoping, not-found; httptest refresh incl. refresh-token-rotation).
- `tokens/scopes.go` — real `TwitchScopeChecker` (replaces `NoScopeChecker`): resolves the owner cred, maps `granted_scopes`→actions via `models.ActionsForTwitchScopes`.
- `models/moderation.go` — scope consts, `ActionsForTwitchScopes`, `RequiredTwitchScope`, `DispatchRequest`/`DispatchResult`/`DispatchOutcome`. unit-tested.
- `dispatch/twitch.go` — `Dispatcher` impl: scope pre-check (fail fast, no API call), proactive refresh (<5min to expiry), real Helix call, **401→refresh→retry once**, 403→reauth. Non-twitch platforms → `DispatchDryRun`. fakes-tested (all branches).
- `handler/moderation.go` — `finish`→`execute`: dispatches before reflect-back; maps `Performed`→success+emit, `DryRun`→dry_run+emit, `ReauthRequired`→403 `{requires_reauth, missing_scopes}` (no emit), `NoCredential`→422 (no emit), dispatcher error→502. `ScopeChecker` now takes `userID`. New `Dispatcher`/`DryRunDispatcher` seam. handler tests cover all paths.
- `audit/store.go` — `OutcomeReauthRequired`, `OutcomeNoCredential` added.
- `cmd/main.go` — builds cipher (`NewMultiKeyEncryptorFromEnv`) + `TwitchSource` + `clients.TwitchClient` + `dispatch.Twitch` + `TwitchScopeChecker` when `TOKEN_ENCRYPTION_KEY_V1` **and** `TWITCH_CLIENT_ID`/`TWITCH_CLIENT_SECRET` present; otherwise **graceful dry-run** (`DryRunDispatcher`+`NoScopeChecker`) with a warning. **⚠️ deployment manifest must add `TWITCH_CLIENT_ID`, `TWITCH_CLIENT_SECRET` (TOKEN_ENCRYPTION_KEY* already there).**
- **Frontend fix**: `buildDeleteRequest` now sends `native_message_id = metadata.twitch_message_id` (the Twitch normalizer stamps the native `Tags["id"]` there; the prior `target_msg_id` key only exists on deletion-echo events). Test updated + green.
- **SAFE-BUT-INERT until Step 3**: with no user holding mod scopes, capabilities report `missing_scope` and the dispatcher's scope pre-check returns reauth — so no real Helix call fires until the opt-in re-consent ships.

### Phase 1 frontend — ✅ VERIFIED (tsc clean, eslint clean on all mod files, 480 vitest unit tests / 49 files green)
Auth-gate `/overlay/[id]/view` + WS token, view-local toggles, moderation UI (optimistic/rollback, per-platform affordances). Verified this session alongside Step 4.
**Frontend verify commands (the obvious ones are broken — use these):** a fresh worktree has no `frontend/node_modules` → run `npm ci` in `frontend/` first. `npm run lint` is broken (Next 16 removed `next lint`) → lint via `npx eslint '<files>'` (quote paths; zsh glob-expands `[id]`). `npx vitest run` also runs the storybook browser project (needs playwright) → use `npx vitest run --project unit`.

## NEXT STEPS (in order)

### 1. Twitch token resolution + real ScopeChecker  ✅ DONE (see Phase 1 backend section above)
### 2. Wire the Twitch client into the handler  ✅ DONE (dispatcher; dry-run dropped for Twitch)

### 3. auth-service opt-in re-consent flow  ✅ DONE (all auth-service tests green, no regression)
- `oauth/twitch.go`: `GetAuthURLWithScopes(state, extra)` (`force_verify=true`, deduped) + `ModerationScopesForActions` + `twitchModerationScopeByAction`. unit-tested.
- `handlers/platform_auth_v2.go`: new **separate** `HandleEnableModeration` endpoint → **R4 moot** (never hits the add-source short-circuit). Builds consent URL with `extra = existing_granted ∪ ModerationScopesForActions(actions)` (always a superset). Reuses `NewAddSourceState` → existing callback stores token + granted_scopes (incl. linked-token path) unchanged.
- **R5 fixed by GENERALIZING the guard**: `wouldDowngradeScopes(existing,new)` over `preservableScopes = {user:read:chat, moderator:manage:chat_messages, moderator:manage:banned_users}`, applied at the login path (was :857) and `linkMayReplacePrimaryCredentials` (was :922). Strengthens (protects mod scopes too); chat behavior unchanged. Table-tested + a test proving `union(existing, modScopes)` is never a downgrade. **Full handlers+oauth suites pass (incl. existing downgrade/link testcontainer tests) — no regression.**
- Routes: `auth-service/cmd/main.go` `protected` (`/twitch/moderation/:overlay_id`) + `api-gateway/cmd/main.go` `publicAPI` (`/auth/twitch/moderation/:overlay_id`).
- **Frontend wired**: `moderationApi.getTwitchConsentUrl(overlayId, actions)` + the missing-scope banner now renders an **Enable moderation** button (Twitch) → fetches the consent URL → `window.location.href`. Non-Twitch sources show "coming soon". eslint/tsc/tests green.
- token-refresh-service preserves `granted_scopes` (and moderation-service `Refresh` leaves it untouched).

### ⚠️ DEPLOYMENT (separate repo `caesar-deployment`, NOT this git branch)
moderation-service env (configmap/secret) — each platform independently falls back to dry-run if absent:
- **Twitch**: `TWITCH_CLIENT_ID` + `TWITCH_CLIENT_SECRET` (+ `TOKEN_ENCRYPTION_KEY_V*`, already present).
- **Kick**: `KICK_CLIENT_ID` + `KICK_CLIENT_SECRET` (+ `TOKEN_ENCRYPTION_KEY_V*`). Same Kick app as the listeners/token-refresh.
- **YouTube**: `YOUTUBE_CLIENT_ID` + `YOUTUBE_CLIENT_SECRET` (+ `TOKEN_ENCRYPTION_KEY_V*`). Optional `YOUTUBE_QUOTA_LIMIT_DAILY` (default 1009000 — keep in sync with the listener's `QUOTA_LIMIT_DAILY`). Uses the shared Redis (liveChatId cache) + DB (quota functions, migration 008) already present.
- **Discord**: `DISCORD_BOT_TOKEN` (the same token discord-listener uses). **Operational: re-invite the bot with the `MANAGE_MESSAGES` permission** (current invite `permissions=68608` lacks it → delete 403s until fixed).
- Without a platform's creds the service starts that platform in **dry-run** (safe).

### 4. Feature gate (task 13 remainder)  ✅ DONE (all tests green: mod-svc full suite, shared featuregates+middleware, frontend tsc+eslint+480 vitest)
- `shared/featuregates/cache.go` — `GateModeration = "moderation"` const.
- `migrations/061_moderation_feature_gate.sql` (+down) — seeds `moderation` gate `is_premium=TRUE` (premium cohort first), `ON CONFLICT DO NOTHING` (idempotent; admin toggle owns it after).
- `repository/repository.go` — `IsUserPremium(ctx,userID)` (+testcontainer test; added `users` table to test schema).
- `handler/moderation.go` — `FeatureGate` interface + `OpenGate{}` default + `Handler.SetFeatureGate`; `HandleCapabilities` now returns `Enabled` (gate ∧ premium, **fail-closed** on error). Unit-tested (disabled→false, error→false, default OpenGate→true).
- `models/moderation.go` — `Capabilities.Enabled bool` (`json:"enabled"`).
- `cmd/main.go` — boots `FeatureGateCache`; `RequirePremium("moderation")` on the **4 action routes only** (capabilities stays ungated so non-cohort owners still get `enabled:false`); `moderationGate` adapter (`gateCache.IsPremium ∨ repo.IsUserPremium`) wired via `SetFeatureGate`. Defense in depth: capabilities hides controls, routes still 403.
- **Frontend**: `lib/types/moderation.ts` `enabled` field; `view/page.tsx` `moderationEnabled`/`featureGated` derived, controls wired only when enabled, missing-scope banners gated on enabled, new "Chat moderation is a premium feature → Upgrade" banner; `.catch` fallback includes `enabled:false`.
- **ADR-0017** + `services/moderation-service/README.md` updated (Rollout section, capabilities shape, related ADR-0008).
- **No new env**: uses existing dbPool+redisClient. Migration 061 runs every pod start (idempotent). Graduate to all users by flipping the gate `is_premium=FALSE` via the feature-gate admin endpoint — no redeploy. **Local non-premium dev**: set `users.is_premium=true` or flip the gate to test moderation.

### 5. Phase 2 — Kick + Discord  ✅ DONE (all green: mod-svc full suite incl. testcontainers, message-processor consumer, auth-service oauth+handlers, api-gateway, frontend tsc+eslint+ModerationControls)

**Cross-platform dispatch routing (new seam):**
- `dispatch/multi.go` — `Multi` routes `Dispatch` by `req.Platform` to a per-platform `PlatformDispatcher`; unconfigured platform → `DispatchDryRun`. Keeps the handler platform-agnostic.
- `handler/scopecheck.go` — `MultiScopeChecker` (routes capability scope-checks per platform) + `StaticScopeChecker` (fixed action set, used for Discord's bot-credential model). Both implement `handler.ScopeChecker`.
- `cmd/main.go` builds `dispatchers`/`scopeCheckers` maps; registers a platform only when its creds are present, else dry-run.

**Reflect-back fix (REQUIRED for non-Twitch — only twitch-listener populates the msgid registry):**
- `publisher/deletion.go` `BuildSingleDeletion(platform, channelID, nativeMsgID, targetUUID)` now carries `target_uuid`.
- `services/message-processor/consumer/stream_consumer.go` `processDeletionEvent` (single): if `target_uuid` is present+non-empty, trust it and SKIP the registry lookup (additive — native deletions still use the registry). Tested (`stream_consumer_test.go`: fast-path / registry-fallback / buffer-when-unknown). Without this, Discord/Kick deletes would buffer-and-drop and never strike through on the OBS overlay (frontend matches `single` only by `item.id === target_uuid`).
- handler `HandleDelete` threads `req.TargetUUID`.

**Kick (timeout/ban/unban; NO single-message delete):**
- `clients/kick.go` — `POST/DELETE /public/v1/moderation/bans`, Bearer; ids sent as JSON ints; timeout seconds→minutes (ceil, min 1); 401→`ErrKickUnauthorized`, 403→`ErrKickForbidden`. ⚠️ **staging-validate the id JSON type (int per OpenAPI vs chat_send's string) + the bans body** — defensive body logging on non-2xx. httptest-tested.
- `tokens/kick.go` — `KickSource` mirrors Twitch's **users-row** path (`auth_provider='kick'`, `LOWER(username)=slug`, `kick_id`=broadcaster_user_id, `granted_scopes`, refresh via id.kick.com). **v1 scope: Kick-login broadcasters only** (linked `kick_oauth_tokens` lacks the numeric id + per-link scopes → reported no-credential). testcontainer-tested.
- `tokens/scopes.go` `KickScopeChecker`; `dispatch/kick.go` (mirrors Twitch dispatcher); `models` `ScopeKickModeration="moderation:ban"` + `ActionsForKickScopes` + `RequiredKickScope`.
- auth-service re-consent: `oauth/kick.go` `KickModerationScopesForActions` + `GetAuthURLWithScopesPKCE` (PKCE, base `user:read` ∪ extra); `HandleEnableModeration` **generalized** to branch Twitch/Kick (Kick stores the PKCE verifier in Redis like add-source, reuses the shared callback); `preservableScopes += moderation:ban`; routes (auth-service `protected /kick/moderation/:overlay_id`, gateway `publicAPI /auth/kick/moderation/:overlay_id`). token-refresh-service `UpdateUserTokens` never touches `granted_scopes` → grant survives refresh (verified, no change needed).
- Frontend: `moderationApi.getKickConsentUrl`; `enableModeration(platform)` generalized; banner Enable button now fires for `twitch||kick`.

**Discord (delete-only; shared bot token, NO per-user OAuth, NO re-consent):**
- `clients/discord.go` — `DELETE /api/v10/channels/{c}/messages/{m}`, `Authorization: Bot <token>`, User-Agent; 404→idempotent success; 401→`ErrDiscordUnauthorized`, 403→`ErrDiscordForbidden`. httptest-tested.
- `dispatch/discord.go` — delete-only; success→Performed; any failure→dispatch error (NO false reflect-back). 403 (bot lacks Manage Messages) is a re-invite, not OAuth re-consent.
- Capability via `StaticScopeChecker{[delete]}` registered only when `DISCORD_BOT_TOKEN` set.
- Frontend: capability-driven, NO change needed (already in `MODERATABLE_PLATFORMS`). `ModerationControls` refactored to per-action affordances (hide delete on Kick/YouTube, hide per-user menu on Discord) — tests added.
- ⚠️ **Discord bot must be re-invited with the `MANAGE_MESSAGES` (8192) permission** — current invite (`permissions=68608`) lacks it; until then delete 403s (fails loudly, no false reflect-back).

**api-gateway:** fixed `service_config_test.go` registry count 17→18 (Phase 0 added moderation-service to the registry but didn't bump the test — the Phase-0 verify ran `build` not `test` for api-gateway).

### 5b. Phase 3 — YouTube  ✅ DONE, ban-only (force-ssl already verified with Google from earlier official-API use → NOT externally blocked; all green)
**User correction:** the `youtube.force-ssl` scope is already registered/verified with Google (earlier official-API attempts), so the multi-week re-verification blocker does NOT apply — only the code change (re-add force-ssl to a re-consent flow) was needed.

- `clients/youtube.go` — `BanUser` → `POST /liveChat/bans?part=snippet` `{snippet:{liveChatId, type:"permanent", bannedUserDetails:{channelId}}}`, Bearer; 401→`ErrYouTubeUnauthorized`, 403→`ErrYouTubeForbidden`. Plus `YouTubeLiveChatResolver` reading Redis `youtube:stream:state:{ch}` ({live_chat_id,is_live}) — **Redis cache ONLY, no search.list** (avoids 100-unit cost); not live/cached → `ErrYouTubeNotLive`. httptest+miniredis-tested.
- `tokens/youtube.go` — `YouTubeSource` mirrors the Kick/Twitch **users-row** path (`auth_provider='youtube'`, `users.access_token/refresh_token/token_expires_at/granted_scopes`); **channel id NOT matched** (the user's YT login token; channel ownership enforced by source-membership + YouTube's own 403). Refresh via Google token endpoint (no refresh-token rotation). **v1: YT-login broadcasters only** (linked `youtube_oauth_tokens` has no granted_scopes column → deferred). testcontainer-tested.
- `quota/quota.go` — `Reserver` calls the shared SQL functions `reserve/confirm/rollback_youtube_quota` (migration 008) directly on the same `youtube_quota_usage` counter (NOT the listener's in-memory Tracker), Pacific date, ban=50 units, `YOUTUBE_QUOTA_LIMIT_DAILY` default 1009000. fake-Querier-tested.
- `dispatch/youtube.go` — resolve cred → force-ssl precheck → resolve liveChatId (Redis) → **reserve 50 units** → ban (401→refresh→retry once) → **confirm on success / rollback on failure**. fakes-tested (all branches incl. quota confirm/rollback).
- `models` — matrix `youtube: [ban]` (was `[ban,unban]`; **unban dropped** — `liveChatBans.delete` keys on the ban resource id, not persisted); `ScopeYouTubeModeration` (force-ssl) + `ActionsForYouTubeScopes` + `RequiredYouTubeScope`.
- auth-service re-consent: `oauth/youtube.go` `YouTubeModerationScopesForActions` + `GetAuthURLWithScopes(prompt=consent, access_type=offline)`; `HandleEnableModeration` YouTube branch (non-PKCE); `preservableScopes += force-ssl`; routes (auth-service `protected /youtube/moderation/:overlay_id`, gateway `publicAPI /auth/youtube/moderation/:overlay_id`).
- Frontend: `moderationApi.getYouTubeConsentUrl`; `enableModeration` youtube branch (`['ban']`); banner Enable button fires for `twitch||kick||youtube`; ModerationControls YouTube test → ban-only (no delete/timeout/unban).
- **Deferred (documented):** unban (needs ban-id persistence); linked-YT (`youtube_oauth_tokens`) accounts; emergency-threshold soft gate (the DB function still enforces the hard daily limit).

---

## NEXT — continuation (CORE, requested by user 2026-06-19)

### 6. Connected (non-primary / linked) account moderation  — **CORE, not a nice-to-have**
v1 only resolves PRIMARY-login broadcasters (Twitch handles linked already; Kick/YouTube are users-row only). A streamer whose All-Chat login is platform A but who also linked platform B must be able to moderate B. **Root blocker: the per-platform token tables lack the columns moderation needs.**

- **Twitch**: `tokens/source.go` ALREADY UNIONs `users` + `twitch_oauth_tokens` (has `granted_scopes` + `twitch_user_id`, ADR-0016). ✅ linked likely works — **verify** with a testcontainer case (linked twitch mod scope) and the re-consent linked path.
- **Kick** (`tokens/kick.go`, users-row only): linked tokens are in `kick_oauth_tokens` which has **NO numeric broadcaster id and NO `granted_scopes`** (schema migration 005/050). Needed:
  1. Migration: add `kick_user_id VARCHAR` (numeric broadcaster id) + `granted_scopes TEXT[]` to `kick_oauth_tokens`.
  2. Populate `kick_user_id` at link time (`platform_auth_v2.go` link callback → `kickProvider.GetUserInfoKick` returns numeric `UserID`) and on re-consent.
  3. Populate `granted_scopes` on the linked re-consent callback (the moderation flow stores to users today; must also write the per-link table for linked accounts).
  4. `KickSource.Resolve` → UNION users-row (auth_provider=kick) + `kick_oauth_tokens` (by user_id + LOWER(channel_id slug)); decide precedence (users-row first, like Twitch).
- **YouTube** (`tokens/youtube.go`, users-row only): linked tokens are in `youtube_oauth_tokens` keyed by `channel_id` (UC...) — which actually **matches the overlay source channel_id directly** (cleaner than the users-row match). But it has **NO `granted_scopes`** (migration 003/006). Needed:
  1. Migration: add `granted_scopes TEXT[]` to `youtube_oauth_tokens`.
  2. Populate on the linked re-consent callback + token-refresh preserve.
  3. `YouTubeSource.Resolve` → prefer `youtube_oauth_tokens` WHERE user_id + channel_id=UC when present (exact per-channel token), else users-row.
- **Cross-cutting**: the re-consent callback (`platform_auth_v2.go` `linkPlatformToUser` / the linked-credential store path) must persist moderation `granted_scopes` to the PER-PLATFORM tables for non-primary accounts (today it only writes `users.granted_scopes`). token-refresh-service must preserve those per-link `granted_scopes` (it currently touches neither — add UPDATEs that leave granted_scopes alone, same as users). Capability scope-checkers already call `Resolve`, so they inherit the linked path once the sources UNION.
- E2E: extend `tokens/{kick,youtube}_test.go` with a linked-row case; verify capabilities report moderatable for a linked-but-opted-in account.

### 7. Discord ban + timeout, opt-in invite toggle, clear reinvite path  — **expand beyond delete-only**
Discord moderation is GUILD-level bot permissions (NOT per-user OAuth), so "opt-in" happens at **invite time**.
- **Matrix**: `discord: [delete, timeout, ban, unban]` (was `[delete]`). Update `models.PlatformActions`.
- **Client (`clients/discord.go`)** add:
  - timeout: `PATCH /guilds/{guild_id}/members/{user_id}` `{communication_disabled_until: <ISO8601 now+duration>}` — perm `MODERATE_MEMBERS` (1<<40). Clear = `null`.
  - ban: `PUT /guilds/{guild_id}/bans/{user_id}` (opt. `delete_message_seconds`) — perm `BAN_MEMBERS` (1<<2). unban: `DELETE /guilds/{guild_id}/bans/{user_id}`.
- **guild_id GAP**: ban/timeout are guild-scoped but `guild_id` is **NOT** in the normalized message (only channel_id + author id). It IS in the source config (`overlay_chat_sources.config.guild_id`). → moderation-service must resolve guild_id from the source config: extend `repository` (`IsModeratableSource`/`ListModeratableSources` or a new `SourceConfig` lookup) to return `guild_id`, and thread it into the Discord dispatcher (the handler passes it on the DispatchRequest, or the dispatcher looks it up). target = message `User.ID` (Discord member snowflake).
- **Invite toggle (opt-in)**: at Discord add-source, a frontend toggle "Enable moderation". `oauth/discord.go` builds the invite URL (currently `permissions=68608` = VIEW_CHANNEL+SEND_MESSAGES+READ_MESSAGE_HISTORY). Add a moderation variant: `68608 | MANAGE_MESSAGES(8192) | MODERATE_MEMBERS(1<<40) | BAN_MEMBERS(4)`. Store the choice on the discord source config (e.g. `config.moderation_enabled`).
- **Reinvite path (existing guilds)**: bots already in guilds were invited WITHOUT mod perms. Provide a clear CTA (the elevated invite URL) when the owner opts into Discord moderation on an existing source OR a dispatch 403 surfaces. The frontend missing-scope banner already has a Discord branch ("coming soon") — replace with a **"Re-invite the bot with moderation permissions"** link (NOT an OAuth re-consent — Discord uses a bot invite URL). Add `moderationApi.getDiscordReinviteUrl(...)` or surface the URL from capabilities.
- **Capability**: Discord changes from static `[delete]`. Options: (a) compute the bot's effective permissions in the channel/guild via Discord API (accurate, 1 call/source) → report exactly what the bot can do; (b) report from the stored opt-in flag; (c) report optimistically and let 403→reinvite CTA. **Recommend (a)** (most honest; the bot REST token can `GET /guilds/{g}/members/@me` + role perms) with the reinvite CTA as the actionable fallback. The current `StaticScopeChecker` is replaced by a Discord-specific checker.
- **Dispatcher (`dispatch/discord.go`)**: add timeout/ban/unban (currently delete-only); reflect-back ban/timeout → batch deletion (`target_user_id` = Discord author id; frontend matches batch by user id). 403 on a member op → surface "re-invite with moderation perms" (distinct from the delete-403 message).
- **Frontend**: ModerationControls already renders timeout/ban/unban from `actions` — once Discord reports them, they work. Add the reinvite CTA + the add-source moderation toggle.

(Both 6 and 7 keep the established seams: per-platform token `Resolve`/scope-checker for #6; the dispatch + capability routers for #7. No handler changes needed beyond threading guild_id for Discord.)

## Build / test / verify
```
WT=/home/caesar/git/all-chat/.claude/worktrees/overlay-moderation
go -C $WT/services/moderation-service test ./... -count=1      # needs Docker (testcontainers)
go -C $WT/services/api-gateway build ./...
go -C $WT/shared test ./middleware/ ./featuregates/ -count=1
# frontend (fresh worktree needs deps; npm run lint + bare vitest are both broken — see Phase 1 frontend note):
( cd $WT/frontend && npm ci && npx tsc --noEmit && npx eslint 'src/app/overlay/[id]/view/page.tsx' && npx vitest run --project unit )
```
E2E reflect-back (no platform needed): POST a dry-run delete → `message_deletion` on `chat:raw` → message-processor → `overlay:{id}` Pub/Sub → frontend `onDeletion`. Live Twitch needs a real channel + mod-scoped token (manual/staging).

## Gotchas
- **Reflect-back contract** (`overlayViewModel.ts`): `single` is matched by `item.id === target_uuid` (NOT target_msg_id — that's debug-only); `batch`+`ban_duration>0`→timeout, no duration→ban. Moderation single-deletes now carry `target_uuid` directly (frontend sends it as `target_uuid`; consumer trusts it, skips the registry). Native platform deletes (Twitch EventSub) still resolve `target_msg_id`→`target_uuid` via the twitch-listener-populated registry. `ChannelID` must match the ingest/registry key (Twitch = lowercased login; Discord = channel snowflake; Kick = slug).
- **Twitch IRC listener (`services/twitch-listener`) is being DEPRECATED — do not touch it** (user, 2026-06-19); build on `twitch-eventsub-listener`. Moderation is unaffected (Helix write path + `target_uuid` reflect-back, no IRC dependency). Caveat: the msgid registry that backs *native* single-deletion reflect-back is populated in the IRC listener today; once IRC is gone, that population must move to the EventSub listener — but moderation deletions already carry `target_uuid` and don't need it.
- **Migrations re-run every pod start** (untracked) → must stay idempotent; a migration-only merge needs `--admin`.
- **Dry-run is live**: dry-run actions make messages vanish from overlays but perform NO real platform action. Now **double-gated**: cohort feature gate (Step 4, premium-only) + per-streamer opt-in scopes. Real Twitch calls fire only when a cohort user has opted into mod scopes.
- Least-privilege: never add moderation scopes to login/add-source; only the opt-in flow.
