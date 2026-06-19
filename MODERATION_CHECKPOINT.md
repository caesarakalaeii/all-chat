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
- **Discord**: `DISCORD_BOT_TOKEN` (the same token discord-listener uses). The moderation-service needs only the bot token. **Operational: re-invite is now self-serve** — the dashboard's "Re-invite the bot" CTA opens the elevated invite (`MANAGE_MESSAGES | MODERATE_MEMBERS | BAN_MEMBERS` on top of the base `68608`). Until an owner re-invites, capability correctly reports no Discord actions (no 403 spam). The auth-service still needs its existing `DISCORD_CLIENT_ID`/`DISCORD_CLIENT_SECRET`/`DISCORD_BOT_TOKEN` (unchanged) to serve the invite URL.
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

### 6. Connected (non-primary / linked) account moderation  ✅ DONE (all green: mod-svc tokens testcontainers, auth-service handlers+repository testcontainers incl. migrations_rerun)
A streamer whose All-Chat login is platform A but who linked platform B can now moderate B. Capability scope-checkers + dispatchers already call `Resolve`/`GetPlatformGrantedScopes`, so they inherited the linked path once the sources/queries were widened — no handler/dispatch changes needed.

- **Migration `062_linked_moderation_scopes.sql` (+down)** — `kick_oauth_tokens += kick_user_id VARCHAR(255), granted_scopes TEXT[]`; `youtube_oauth_tokens += granted_scopes TEXT[]`. Idempotent (`ADD COLUMN IF NOT EXISTS`), GRANT guarded. (twitch_oauth_tokens already had twitch_user_id + granted_scopes from 056 — Twitch linked already worked.)
- **moderation-service**:
  - `tokens/kick.go` — `KickSource.Resolve` now UNIONs users-row (auth_provider=kick, by username slug) + `kick_oauth_tokens` (by user_id + LOWER(channel_id), `kick_user_id IS NOT NULL` so legacy listener rows are skipped); users-row preferred. `KickCredential` carries `origin`; `Refresh` writes back to the origin row (users `token_expires_at` vs kick_oauth_tokens `expiry`, keeps encryption_version=1), granted_scopes untouched.
  - `tokens/youtube.go` — `YouTubeSource.Resolve` now prefers the EXACT per-channel `youtube_oauth_tokens` row (by user_id + channel_id=UC…) over the channel-agnostic users row; `origin`-aware `Refresh` write-back.
  - testcontainer tests extended (`kick_test.go`, `youtube_test.go`): linked credential, per-channel preference, scoping to requesting user, legacy-row skip, linked-refresh-persists-to-linked-row + scopes preserved. All green.
- **auth-service**:
  - `repository/user_repository.go` — `StoreYouTubeToken` gained a `grantedScopes` param and now MERGES (union) granted_scopes on conflict (a plain add-source can't clobber a prior force-ssl grant); new `StoreKickToken` (writes kick_oauth_tokens with kick_user_id + merged granted_scopes, encrypted); new `GetPlatformGrantedScopes(userID, platform)` reads the AUTHORITATIVE source per platform (users row if login provider, else the per-link table).
  - `handlers/platform_auth_v2.go` — `HandleEnableModeration` now unions with `GetPlatformGrantedScopes` (was `GetGrantedScopes`). **This fixes a latent cross-platform bug**: `getOrCreateUser` writes `users.granted_scopes` for ALL providers, so the old code injected e.g. a YouTube login's scopes into a Twitch consent URL for a linked account (provider rejects). Callback persists linked Kick grant via new `shouldStoreLinkedKickCredentials` (non-kick-login + add-source, mirrors the Twitch sibling) and passes scopes to `StoreYouTubeToken`. Legacy `StoreYouTubeToken` callers (auth_handler.go, platform_auth.go) updated.
  - testcontainer tests added (`platform_auth_v2_link_test.go`, schema extended with kick/youtube oauth tables): `TestShouldStoreLinkedKickCredentials`, `TestStoreKickToken_PersistsLinkedCredentials` (incl. union-on-upsert), `TestStoreYouTubeToken_MergesModerationScope`, `TestGetPlatformGrantedScopes` (primary vs linked vs cross-platform-isolation vs empty).
- **token-refresh-service**: NO change needed (verified). YouTube linked refresh (`youtube_oauth_tokens`) already leaves granted_scopes untouched; linked Kick uses the moderation dispatcher's on-demand refresh (proactive <5min + 401-retry, writes back to kick_oauth_tokens). Kick-login still refreshed via the users row.
- **Deferred (documented)**: proactive token-refresh of linked `kick_oauth_tokens` rows (on-demand refresh covers active moderation use; a long-idle linked Kick credential refreshes on next action or prompts re-consent if its refresh token lapsed).

### 7. Discord ban + timeout, opt-in invite toggle, clear reinvite path  — **expand beyond delete-only**
### 7. Discord ban + timeout, permission-aware capability, re-invite path  ✅ DONE (all green: mod-svc full suite incl. testcontainers; auth-service oauth+handlers; frontend tsc+eslint+484 vitest)
Discord moderation is GUILD-level bot permissions (NOT per-user OAuth), so the "opt-in" is the bot RE-INVITE. The handler stayed platform-agnostic: the Discord dispatcher resolves the guild from the channel itself, so no DispatchRequest/repository changes were needed.

- **Matrix** (`models/moderation.go`): `discord: [delete, timeout, ban, unban]` (was `[delete]`). Added Discord permission bits + `ActionsForDiscordPermissions(perms)` (ADMINISTRATOR ⇒ all; MANAGE_MESSAGES⇒delete, MODERATE_MEMBERS⇒timeout, BAN_MEMBERS⇒ban+unban) + `RequiredDiscordPermission` + `ModerationBotPermissions`. Unit-tested.
- **guild_id GAP solved without schema/threading** (the source config does NOT reliably carry guild_id): `clients/discord.go` `GuildIDForChannel` (`GET /channels/{id}`) + `DiscordGuildResolver` (Redis-cached `discord:channel:guild:{id}`, immutable). The dispatcher resolves the guild from `req.ChannelID` internally — handler untouched. Mirrors the `YouTubeLiveChatResolver` pattern.
- **Client (`clients/discord.go`)**: `TimeoutMember` (PATCH member `communication_disabled_until`), `BanMember` (PUT ban), `UnbanMember` (DELETE ban; 404⇒idempotent success), `GuildBotPermissions` (OR of the bot's role perms incl. @everyone). 401⇒`ErrDiscordUnauthorized`, 403⇒`ErrDiscordForbidden`. httptest-tested.
- **Capability = option (a), honest**: `handler.DiscordScopeChecker` resolves guild → reads effective bot perms → `ActionsForDiscordPermissions`. The cached resolver also caches per-guild perms (`discord:guild:perms:{g}`, 5min TTL) so capabilities stay cheap on dashboard load. Any resolution/permission error degrades to no actions (re-invite prompt), never errors the whole response. Replaced the Phase-2 `StaticScopeChecker{[delete]}`. Unit-tested. (Note: a bot invited with the legacy 68608 now correctly reports NO actions — strictly more honest than the old always-delete static checker, since delete was already 403ing for those bots.)
- **Dispatcher (`dispatch/discord.go`)**: delete (channel-scoped) + timeout/ban/unban (resolve guild → member op). 403 on any op ⇒ dispatch error with a "re-invite the bot with moderation permissions" hint (never a false reflect-back). Reflect-back is unchanged: timeout/ban reuse `BuildBatchDeletion` (frontend matches batch by `target_user_id` = Discord member snowflake = message `User.ID`); unban emits none. fakes-tested (all branches).
- **Re-invite path** (`auth-service`): `oauth/discord.go` `GetModerationAuthURL` (base 68608 | MANAGE_MESSAGES | MODERATE_MEMBERS | BAN_MEMBERS); `handlers/discord.go` `HandleConnect?moderation=true` returns the elevated invite URL (no new route — the existing `/discord/connect` carries the flag through the gateway). Re-authorizing on an existing guild upgrades the bot's perms in place. Unit-tested.
- **Frontend**: `lib/api/discord.ts` `startDiscordModerationReinvite()` (`GET /auth/discord/connect?moderation=true` → redirect); `view/page.tsx` `enableModeration` branches Discord → re-invite; the missing-scope banner's Discord branch is now an actionable **"Re-invite the bot"** CTA (was "coming soon"). `ModerationControls` is already capability-driven, so it renders delete + per-user ban/timeout/unban once Discord reports them — comment fixes + a fully-permissioned-bot test added.
- **Deferred (documented)**: an add-source-time "enable moderation" toggle on the Discord source config (the re-invite CTA covers both new and existing sources, so it is not required); channel-level MANAGE_MESSAGES overwrites (capability uses guild-level perms — a rare channel-overwrite denial surfaces as a dispatch 403 → re-invite hint).
- ⚠️ **Operational** (unchanged from earlier note): existing bots were invited with `permissions=68608` and report no moderation actions until the owner uses the re-invite CTA.

(Both 6 and 7 kept the established seams: per-platform token `Resolve`/scope-checker for #6; the dispatch + capability routers for #7. The Discord guild is resolved inside the dispatcher/scope-checker, so the handler stayed platform-agnostic — no guild_id threading needed.)

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
- **⚠️ STAGING-VALIDATE (Step 7 Discord capability)**: `clients/discord.go` `GuildBotPermissions` reads the bot's roles from `GET /guilds/{g}/members/@me` (matches the existing `oauth/discord.go CheckBotPermissions` usage) then ORs the role perms from `GET /guilds/{g}/roles`. Confirm on staging that `/members/@me` with the BOT token returns the member object WITH `roles` — if Discord only honours it for OAuth bearer tokens, switch to `GET /users/@me` (bot id, cacheable) + `GET /guilds/{g}/members/{botId}`. A failure degrades SAFELY (no actions reported → re-invite CTA; the dispatcher still enforces via a 403), so this is a correctness/UX check, not a safety risk.
- **Discord reflect-back (timeout/ban)** reuses `BuildBatchDeletion` (channel snowflake + `target_user_id` = member snowflake = normalized `User.ID`) — the same platform-agnostic batch path Kick/Twitch bans use; no message-processor change was needed. unban emits no reflect-back.
