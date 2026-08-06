# ADR-0048: Delegated Overlay Moderators

**Date**: 2026-08-06
**Status**: Proposed
**Deciders**: caesarakalaeii

## Context and Problem Statement

ADR-0017 gave streamers a moderation write-path from the dashboard, deliberately scoped **owner-only**: authorization is `VerifyOverlayOwnership`, and every platform call is made with the broadcaster's own stored OAuth token so that `broadcaster_id == moderator_id`. Streamers have since asked for the obvious next thing — **let their existing volunteer moderators use it too**. A streamer who multistreams to four platforms currently has no way to hand the merged, single-pane moderation view to the people who actually moderate for them; their mods must fall back to four native tool sets, which is the exact problem All-Chat exists to solve.

This is a **premium** feature, with an unusual gating requirement: the gate must key on the **overlay owner**. If the streamer has premium, their moderators moderate for free. Nothing in the codebase gates on anyone other than the caller, so this is a new enforcement shape as well as a new authorization shape.

Two questions dominate the design, and they turn out to be the same question:

1. **Whose platform credential performs the write** — the streamer's stored token, or the moderator's own?
2. **Must All-Chat verify that a delegated moderator is actually a moderator on that channel**, and is such a check even needed?

The answer to (2) is a *consequence* of (1). Where the acting credential belongs to the moderator, the platform re-checks their moderator role on every call and our check is advisory UX. Where the acting credential is the owner's or a shared bot's, the platform sees a legitimate principal, validates nothing about the delegate, and All-Chat becomes the sole authority.

## Decision Drivers

- **Delegation must not become privilege escalation.** A volunteer moderator must not end up holding the streamer's credential, the streamer's scopes, or broadcaster-only powers.
- **Prefer an external authority over our own.** A check the platform performs on every call cannot be bypassed by a bug in our authorization code, and cannot go stale.
- **Correct attribution.** A moderation action must be attributable to the human who performed it, in the platform's own moderator log as well as in ours. A streamer's native mod log that names the streamer for actions they did not perform is worse than no log.
- **Least privilege (ADR-0012), including for the delegate.** The consent screen a volunteer sees must not ask for the scope set All-Chat asks of a streamer.
- **Premium keyed on the owner, not the caller.** A moderator must never be shown an upgrade CTA for a plan they cannot buy.
- **Per-platform reality, honestly surfaced.** The five platforms differ so much that a uniform model would either overclaim (a moderation button that 403s) or underdeliver.
- **Revocation must be believable.** A streamer's instinct is to remove someone in the platform's own UI; that should be sufficient wherever the platform can make it so.

## Considered Options

1. **Delegated moderators act with the streamer's stored token** (`broadcaster_id == moderator_id` unchanged; a grant table is the only new authorization).
   - ✅ Pros: smallest possible diff — `tokens/source.go` already scopes credential resolution by user id, so resolving with the *owner's* id needs no SQL change; all four platforms light up at once; no consent flow for moderators at all.
   - ❌ Cons: the moderator inherits every scope the streamer ever granted, with no way to hand out "just delete"; they gain broadcaster-only powers including the chat-wipe primitive (`clients/twitch.go` always sets the `message_id` key, and Twitch documents an empty `message_id` as "remove all messages"); N moderators racing the streamer's single refresh-token chain against token-refresh-service can destroy the credential that EventSub ingest, chat send and engagement all depend on; the platform validates nothing about the delegate, so a grant row is the only thing standing between a volunteer and the channel; and every action is attributed to the broadcaster in their own mod log.

2. **Delegated moderators act with their own platform credential.**
   - ✅ Pros: the platform re-checks the moderator role on every call — a free, authoritative, never-stale authorization oracle that our code cannot bypass; unmodding on the platform revokes All-Chat access instantly, whether we noticed or not; actions land in the native mod log under the right name; scopes are per-moderator and minimal; the streamer's credential is never read on a delegated path.
   - ❌ Cons: each moderator completes an OAuth consent per platform; they must actually hold the role; Discord has no per-user moderation API at all, so it cannot be covered this way; and it requires splitting `moderator_id` from `broadcaster_id` throughout the Twitch client.

3. **An All-Chat service account / bot as the actor on every platform.**
   - ✅ Pros: one credential to operate; no per-moderator consent.
   - ❌ Cons: only Discord works this way; Twitch/Kick/YouTube would require the streamer to mod the All-Chat bot, and then *every* action is attributed to All-Chat rather than to a person, destroying accountability. It is Option 1's authority problem with worse attribution.

4. **Do not build it; point moderators at native tools.**
   - ✅ Pros: zero new attack surface on the highest-blast-radius path in the product.
   - ❌ Cons: abandons the core value proposition (one pane for N platforms) for exactly the users who need it most, and multistreaming streamers have repeatedly asked.

## Decision Outcome

**Chosen: Option 2 — a delegated moderator acts with their own platform credential, with no fallback to the owner's, ever.** Discord is the single exception, because no per-user Discord moderation API exists; it is handled under an explicitly narrower authority model rather than by silently borrowing the owner's credential.

**Rationale**: Option 2 converts the hardest part of the problem — "is this person really allowed to moderate here?" — from something All-Chat must implement, cache, and keep fresh into something the platform answers correctly on every single call. That is strictly stronger than any check we could write, and it is free. The costs are real but bounded and local: a consent flow per moderator, and a `moderator_id`/`broadcaster_id` split in one client. Option 1's cost saving is smaller than it appears (the dispatcher signature and the credential tables both stay put) and it is paid for with the streamer's credential, their attribution, and their refresh-token integrity.

### Two authority categories

Because Discord cannot satisfy the invariant, the model has exactly two categories, and the ADR names them rather than blurring them:

- **Platform-executed authority** (Twitch, Kick, YouTube): the moderator's own token performs the write. The platform re-checks their moderator role per call. Our pre-checks are advisory.
- **Platform-attested authority** (Discord): the shared bot performs the write — every Discord write authenticates as the bot and `dispatch/discord.go` discards the actor id entirely. The platform *attests* the moderator's role via a live read of their guild permissions with the bot token; All-Chat enforces it in-process, fail-closed. Because the `GUILD_MEMBERS` privileged intent is off (`RequiredIntents = 33281`), revocations cannot be pushed to us, so the **60-second cache TTL is a security bound, not a tuning knob**.

Two Discord-specific consequences follow and are load-bearing. Discord hierarchy-gates the *actor*, and the bot is always the actor, so All-Chat must itself enforce `modHighestRolePos > targetHighestRolePos` for timeout and ban (not for delete) or a delegated moderator could ban someone they cannot touch natively. And the effective permission set is `ActionsForDiscordPermissions(botBits) ∩ ActionsForDiscordPermissions(modBits)`.

### Owner-reach anchor rule

**Delegation never exceeds what the owner could do themselves.** A moderator may only act on a channel the overlay **owner can prove they control**. Evaluated twice — at grant-leg activation and again at action time — fail-closed both times.

The anchor proves **control only**. It must never require the owner to hold a moderation scope, a live token, or premium: requiring the owner's moderation scope would deny delegation to precisely the streamer who delegates *because* they do not moderate themselves.

| Platform | What proves owner control | When it cannot be proven |
|---|---|---|
| **Twitch** | The owner holds a Twitch credential row whose login equals the source's `channel_id` (a Twitch source's `channel_id` *is* the login). The row also yields the numeric `broadcaster_id` the write needs. `users` branch (`auth_provider='twitch'`) preferred over `twitch_oauth_tokens` (ADR-0016), **no scope predicate**. | 403 `owner_channel_unverified`; the Twitch leg of every grant on that overlay is unusable. Remediation targets the **owner** ("reconnect your Twitch account"), surfaced in the owner's UI. |
| **Kick** | Same shape: `users.kick_id` (Kick-login) or `kick_oauth_tokens.kick_user_id`, yielding the numeric `broadcaster_user_id`. `kick_user_id IS NOT NULL` is mandatory — it is NULL on legacy listener-only rows (migration 062). | 403 `owner_channel_unverified`. Load-bearing beyond authorization: this is the **only** legitimate source of `broadcaster_user_id`. |
| **YouTube** | A `youtube_oauth_tokens` row for the owner and **exactly** the source's `channel_id` — that column is only ever written from `channels?mine=true` with the owner's own token. The `users`-row fallback used elsewhere is **forbidden as an anchor**: it is channel-agnostic and would match any channel id. | 403 `owner_channel_unverified`. A non-trivial existing population may fail this; measure before opening the gate. |
| **Discord** | Both required: a `discord_guilds` row for (owner, guild) — migration 035, `UNIQUE(user_id, guild_id)`, written by the bot-invite callback — **and** a live bot-token read showing the owner's own Discord member permissions in that guild satisfy `owner_id ∥ ADMINISTRATOR ∥ MANAGE_GUILD`. Plus the source's `channel_id` must actually resolve to that guild. | Owner not Discord-linked → 403 `discord_link_required`. Row missing → 403 `owner_guild_unverified`. Live-check error → **fail closed**; no stamp-based bypass. |
| **TikTok** | n/a — no moderation API on any TikTok product. Absent from `PlatformActions` by design; reported `unsupported_platform`, never a fake button. | — |
| **shared_overlay** | n/a — stays non-moderatable. Owner-only authorization made "a recipient must not moderate the original streamer's channel" true by construction; role-based authorization does not, so the existing `platform <> 'shared_overlay'` predicates become security-critical and get an explicit regression test. | — |

### Moderator-status verification model

"Load-bearing" means a failed pre-check blocks the API call. "Advisory" means it informs UI and copy only; the platform is the authority.

| Platform | Who enforces at action time | Our pre-check | Surfaced reason |
|---|---|---|---|
| **Twitch** | **Twitch Helix**, per call: `moderator_id` must equal the token's user id and that user must be a moderator of `broadcaster_id`. | **Load-bearing**: the moderator holds a credential with the required scope. **Advisory**: `GET /helix/moderation/moderators?user_id=…` on the **owner's** token (`moderation:read`), whose repeatable `user_id` filter makes it a one-call membership test for a whole mod team. | `mod_link_required` · `missing_scope` · `not_moderator_on_platform` · `owner_channel_unverified` |
| **Kick** | **Kick API**: missing scope → 401, not-a-moderator-of-that-channel → 403. The current client documents and maps this **inverted**, which must be corrected first. | **Load-bearing**: scope pre-check, anchor-supplied `broadcaster_user_id`, and rejection of synthetic `^kick-` message ids. | `not_moderator_on_platform` · `missing_scope` · `message_not_found` · `owner_channel_unverified` |
| **YouTube** | **YouTube Data API** authorizes "the channel owner or a moderator of the live chat". 403 `insufficientPermissions` = not a moderator; 403 `liveChatBanInsertionNotAllowed` = target is the owner or another moderator. | **Load-bearing**: force-ssl present; `youtube:stream:state:{channel}` present (else `stream_not_live` before any call); for unban, the stored `live_chat_id` matches the current one. | `stream_not_live` · `not_moderator_on_platform` · `target_protected` · `ban_from_previous_stream` · `missing_scope` |
| **Discord** | **Nobody at the platform.** Our in-process check is the only authority and is fully load-bearing. | Live `MemberPermissions(guild, modDiscordID)`; `bot ∩ mod` action intersection; required-permission bit; role hierarchy for timeout/ban only. 60 s TTL. Fail closed on any Discord API error. | `discord_link_required` · `mod_not_in_guild` · `mod_lacks_permission` · `mod_below_target` · `bot_missing_permission` |

Asymmetry worth recording: only YouTube (`liveChatModerators.insert`, per-broadcast) and Twitch (`channel:manage:moderators`, deliberately **not** in scope here) expose an API to *grant* moderator status. On Kick and Discord the owner must add the moderator in the platform's own UI, and our remediation copy says so.

A cached "not a moderator" state is **telemetry, never authorization**. It is surfaced to the streamer and never read in `authorize()`. Reading it would make All-Chat the stale authority the whole design exists to avoid, and one transient 403 would otherwise lock out a legitimate moderator with no self-service recovery.

### Credential storage prohibition

A delegated moderator's credential is stored **only** in a dedicated store keyed on the moderator's own identity, never in the existing per-channel credential tables, and never in any row a listener selects from.

This is not fastidiousness. `twitch-eventsub-listener/channels/manager.go` selects a chat-reading credential by `LOWER(twitch_login) = LOWER(ocs.channel_id)` with **no user scoping**, merely *preferring* rows that hold chat scope; `kick-listener/channels/manager.go` selects `FROM kick_oauth_tokens WHERE channel_id = $1 … ORDER BY expiry DESC LIMIT 1` with no user scoping and no scope preference at all. A moderator-scoped credential written into either table becomes a candidate **ingest** credential and can sort first, silently breaking chat ingest on a real channel. The same unscoped predicate drives overlay-manager's `chat_via_eventsub` badge and the scope exporter.

Corollary: **never** key a moderator's credential to the streamer's channel identifier.

### Consent flow

Moderator-facing consent **cannot** reuse the existing moderation re-consent path. `NewModerationState` builds an *add-source* state — deliberately, per ADR-0017 ("Reuses the add-source state + callback unchanged") — and the shared callback calls `addSourceToOverlay` unconditionally for add-source states, which 404s for a non-owner at overlay-manager's ownership check. A moderator's consent would half-succeed: credential persisted, then an error redirect. `OAuthState.Validate` also rejects any action other than `login`/`add_source`.

So: a third state kind (`ActionModConsent`), a `Validate` change, an explicit fork of every callback branch keyed on `IsAddSource()`, and a redirect to the invite/accept page. A regression test asserts a mod-consent callback creates **zero** `overlay_chat_sources` rows.

Separately, the consent must not leak streamer scopes onto a volunteer's screen: `GetAuthURLWithScopes` unconditionally prepends the base login set (`channel:read:redemptions`, `channel:read:subscriptions`, `bits:read`, `moderator:read:followers`), which would ask a volunteer for channel-point, subscription and bits read **on their own channel** — an ADR-0012 regression. Moderator consent gets its own auth-URL builder requesting only the action scopes.

The invariant governing the scope-downgrade guard is stated positively, resolving an apparent contradiction: **request the minimum for the action, but never *store* a narrower grant than the user already had.** A moderator who is also a streamer keeps their own preservable scopes through the union; what is forbidden is *requesting* streamer scopes on a moderator consent screen, and keying the resulting row to anyone else's channel.

### Owner-keyed premium gating

Two sites are caller-keyed today and produce the exact inverse of the requirement: `middleware.RequirePremium(…, GateModeration, …)` on the action routes (reads `c.GetString("user_id")`, then `SELECT is_premium FROM users WHERE id = $1`), and `ModerationEnabled → IsUserPremium(callerID)` behind the capabilities payload. The 403 body also hardcodes `upgrade_url: "/upgrade"`, aimed at a volunteer who cannot buy the streamer's plan.

Decision: **drop `RequirePremium` from the action group and enforce inside `authorize()`** after role resolution, using the in-memory gate cache.

- `shared/middleware/premium.go` is **not** modified — three other services depend on its contract and tests.
- `feature_gates` gains **no** per-user targeting — ADR-0008 rejected that deliberately, and four services boot that cache.
- Middleware is the wrong place: it runs before the handler, so its denial writes **no audit row**, making "a free-tier moderator tried to act" invisible while every other denial in this service is audited. It also cannot emit caller-aware copy.
- Owner and moderator see different copy. The moderator never sees `/upgrade`, a Patreon link, or the owner's re-consent flow.
- Inherited semantics, accepted: `is_premium` is materialized by `shared/premium.Recompute`, so beta-tester and ambassador are already folded in, and an admin force-deny beats everything. A comped-off streamer's whole mod team loses access at once, so the moderator-facing copy must name the streamer's plan and never blame the moderator.

A second gate key (`delegated_moderation`, `is_premium=TRUE`) is seeded in the same migration so delegation can be rolled back independently of the base write path.

### Grant model

Per-overlay, not per-channel: `overlay_chat_sources` churns, so per-channel grants would go stale on every source edit and silently drop a moderator mid-stream. The overlay is also the object the streamer conceptually hands over.

Narrowed by three fail-closed filters: per-platform enablement on the grant (absent row = disabled, which keeps Discord off by default); the moderator must hold their own credential for that platform; and the owner-reach anchor must hold.

Also decided: grants **dormancy-suspend after 90 days with no actions** (not hard expiry, which would cut off a working moderator mid-stream), a **soft cap of 10 moderators per overlay**, and **no chat-send for moderators in v1** — send is a distinct, higher-trust capability and stays owner-only.

An accepted moderator needs a way to *find* the overlay: `GET /api/v1/overlays` is owner-filtered and there is no shared-with-me listing, so a "channels I moderate" surface is required, not optional. Note the pre-existing corollary that an overlay UUID alone already grants chat **read** to anyone who has it; delegation adds write, not read.

### Audit and idempotency

Five identities stay distinguishable forever, and `impersonated_by` is **not** overloaded: the human who acted, their role, the overlay owner they acted for, **whose credential actually acted** (the machine-checkable proof of the no-fallback invariant), and the platform actor id we sent. Legacy rows need no backfill — `actor_role IS NULL` unambiguously means "actor was the owner" — but migration 060's comments and `audit/store.go`, which both assert `actor_user_id` is the overlay owner, become false and must be rewritten in the same PR.

Streamer visibility is a **security requirement**: revocation is worthless if the streamer cannot see what their moderators did. Denial spikes and `not_moderator` flips are the signals that a moderator went hostile, and nobody will find them in Postgres. Moderators get read access to their own actions, which makes the logging visible rather than surveillance-shaped.

Idempotency uses two guards. The client-supplied `Idempotency-Key` is preserved so that a **legitimate escalation is never swallowed** — a 60-second timeout followed by a 10-minute timeout on the same target must not collapse into `duplicate_ignored` with a success-looking UI — and a separate short-lived per-(overlay, platform, channel, action, target) marker suppresses only the duplicate reflect-back when two moderators click the same message. The existing middleware cannot compute the second key (it runs before JSON binding and sees only the header, `user_id` and the route param), so that guard lives in the handlers.

Denials must keep existing and non-existent overlays **indistinguishable**, or role resolution becomes an overlay-existence oracle for any token holder.

## Consequences

### Positive
- The hardest authorization question is answered by the platform on every call, for free, on three of four platforms. It cannot go stale and cannot be bypassed by a bug in our code.
- A streamer's instinctive revocation (unmod on Twitch) actually works, immediately.
- Actions appear in the platform's native moderator log under the real moderator's name.
- The streamer's credential is never read on a delegated path, so a moderator cannot damage the streamer's refresh-token chain or inherit their scopes.
- A moderator's blast radius after compromise is bounded to: granted actions only, on channels where the platform still recognises them as a moderator, only while the grant is live, with their own name attached.
- The frontend's moderation surface is already purely capability-driven rather than ownership-driven, so mixed per-platform readiness renders correctly with no component changes — only the payload becomes moderator-aware.

### Negative
- **Discord has no platform backstop.** A stale All-Chat grant is a live grant over guild members who never appear in chat. Mitigated by per-guild opt-in, default off, the owner's own `MANAGE_GUILD` proof, and a 60-second live permission check — but the residual risk is real and is the weakest point in the design.
- Every moderator completes an OAuth consent per platform, and volunteers are consent-averse.
- YouTube asks a volunteer for `youtube.force-ssl` over **their own** channel to do unpaid work on someone else's, with a consent screen that reads badly. Delegated moderators also count as new authorizing users against the unverified-app cap.
- The moderation write path grows a role dimension, and several currently-owner-only behaviours must stay owner-only by explicit assertion rather than by construction (`shared_overlay` exclusion, YouTube rediscovery, engagement controls, chat send).
- Admin impersonation of a delegated moderator would act with a **third party's** credential and put their name in the streamer's native log for an action they never performed. ADR-0017's "impersonated moderation is allowed but always attributed" was scoped to the owner's own token; extending it to third-party credentials is a new decision that must be settled explicitly, not inherited.
- N moderators multiply per-channel action volume, which platform rate limits bucket per **broadcaster** (Twitch bans) or per **project** (YouTube quota) — spreading across moderator tokens relieves neither.

### Neutral
- No new credential table for platform tokens beyond the dedicated moderator store; the existing owner-path resolvers are untouched.
- The dispatcher signature is unchanged: its `userID` parameter simply becomes "the actor", which is exactly the desired semantics.

## Implementation

**Prerequisite, ships independently:** a Discord source-add entitlement check. Adding a Discord source today performs **no** guild validation — the handler validates `shared_overlay` against `share_requests` and then falls straight through to `Create` + `setDiscordChannelRegistry` for Discord. Combined with owner-only moderation authorization and the shared bot's guild permissions, that is a live cross-tenant escalation path independent of this ADR. Measured on production: 12 Discord sources, all 12 carrying `guild_id`, **0** lacking a matching `discord_guilds` row for their owner — so the check can hard-fail with no grandfathering.

**Phase 1 — foundation (blocks all delegation)**: migrations (grants, moderator credentials, audit columns, DSGVO export/delete wiring, the `delegated_moderation` gate); the `ActionModConsent` state kind and forked callback; a refresh loop for moderator credentials (without it, grants die long before the 90-day dormancy rule can fire); role resolution and the rewritten `authorize()` with owner-keyed premium; the unified outcome family and frontend reason union; both idempotency guards; a moderator-scoped capabilities shape and a "channels I moderate" entry point; owner UI for invite/revoke/per-platform toggles and leg health; `/upgrade` + `OnboardingChecklist` entries.

**Phase 2 — per platform, each independently gated**: **Twitch** first and cheapest (`moderator_id`/`broadcaster_id` split, the anchor, the advisory `moderation:read` pre-check). **Kick** next (anchor, and removal of the moderator's own id from `broadcaster_user_id`); single-message delete is a separate sub-step gated on staging verification. **Discord** (the prerequisite fix, an `identify` link for owner and moderator, generalized member-permission computation, intersection and hierarchy gates); the `GUILD_MEMBERS` intent stays off. **YouTube** last and dark — its write path must first be completed, since it is currently **permanent-ban-only** (`clients/youtube.go` hardcodes `"type":"permanent"`, unban needs a ban resource id the client discards, and single-message delete does not exist). Handing a volunteer permanent-ban-only is a moderation-safety problem.

**Phase 3**: the dormancy job, soft-cap enforcement, this ADR plus the ADR-0017 amendment and README/service-README updates in the same PR, and the Patreon post (CLAUDE.md release step 3).

**Tests first (TDD)**: member-permission table tests (owner short-circuit, ADMINISTRATOR, `@everyone`-only, multi-role OR, position ties, 404 ⇒ deny); bot ∩ moderator intersection; hierarchy denies timeout/ban but allows delete; role resolution returns an identical 403 for existing and non-existent overlays; the escalation matrix for idempotency (collapse, escalate, unban-then-reban); the Kick anchor supplies the owner's id and a moderator's own id can never reach `broadcaster_user_id`; cross-tenant Discord source-add rejected; `shared_overlay` unreachable via a grant; every new migration added to the migration-rerun test.

### Open decisions

Recorded here rather than settled silently, because each changes behaviour a human should sign off on: amending the own-credential invariant for Discord's attested model (**required** for Discord in v1); YouTube per-broadcast moderator auto-provisioning with the owner's token (recommend opt-in per overlay, off by default); adding `channel:manage:moderators` to the owner's consent so All-Chat can mod someone on Twitch (recommend no for v1); whether Kick single-message delete ships as its own PR first (recommend yes); Google verification exposure for third-party force-ssl consent (recommend shipping YouTube dark); retrofitting the anchor to the **owner** path, which would improve security but regress owners with hand-typed or missing rows (recommend not in v1, measure first); suppressing the audit row for unauthorized-stranger denials to avoid attacker-controlled write amplification (recommend suppress, keep every authorized-actor denial); soft-cap mechanics (recommend reject the 11th invite, no retroactive effect); and whether dormancy re-activation is owner-only (recommend yes — moderator self-re-consent must not lift a suspension).

Empirical verification is required before the corresponding leg opens, and none of it is documented by the platforms: whether Twitch issues a token for moderation scopes alone; Kick's 401-vs-403 mapping and whether a moderator's own token reaches a channel they do not own; whether YouTube's `channels.list?mine=true` works with force-ssl alone, whether InnerTube message ids are accepted by `liveChatMessages.delete`, the real per-call quota cost of every `liveChat*` write (the existing "official" cost constant's provenance is wrong), and whether unban survives the broadcast that created the ban; and whether Discord returns an arbitrary member without the `GUILD_MEMBERS` intent for our production application.

## Related Decisions

- [ADR-0017](./0017-chat-moderation-write-path.md) — **amended** here: owner-only authorization, `broadcaster_id == moderator_id`, the add-source-state reuse for moderation consent, `RequirePremium` on the action routes, and the Discord "no credential resolution or scope pre-check" statement all change.
- [ADR-0012](./0012-oauth-scope-minimisation.md) — the minimisation stance a moderator's consent screen must honour; the base-login-scope leak is a regression against it.
- [ADR-0016](./0016-linked-twitch-credentials.md) — the credential model the moderator store deliberately does *not* extend, and the unscoped listener selectors that make the storage prohibition necessary.
- [ADR-0008](./0008-feature-gate-infrastructure.md) — the gate infrastructure, and its deliberate rejection of per-user targeting.
- [ADR-0019](./0019-split-streamer-viewer-premium.md), [ADR-0027](./0027-time-limited-admin-premium-overrides.md) — the entitlement semantics inherited by owner-keyed gating.
- [ADR-0034](./0034-admin-viewer-identity-model.md) — why a moderator needs a streamer-side account: viewer sessions carry no user id and cannot represent a grant.
- [ADR-0006](./0006-youtube-quota-tracking.md) — quota accounting that bounds YouTube moderation, per project rather than per user.
- [ADR-0013](./0013-overlay-observability-view.md) — the dashboard that hosts the moderation UI a moderator must be able to reach.
