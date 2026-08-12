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

The rule is applied to **owner** actions as well, not only delegated ones. That is close to a no-op in practice — on Twitch and Kick a missing owner credential for a channel already yields 422 at credential resolution, and admin-added YouTube/Kick channels pass because the add path writes a real per-channel credential row — but it closes the YouTube case, where the channel-agnostic `users`-row fallback would otherwise match any channel id. The remaining refusals are hand-entered admin channels, where refusing is the correct answer.

The anchor proves **control only**. It must never require the owner to hold a moderation scope, a live token, or premium: requiring the owner's moderation scope would deny delegation to precisely the streamer who delegates *because* they do not moderate themselves.

| Platform | What proves owner control | When it cannot be proven |
|---|---|---|
| **Twitch** | The owner holds a Twitch credential row whose login equals the source's `channel_id` (a Twitch source's `channel_id` *is* the login). The row also yields the numeric `broadcaster_id` the write needs. `users` branch (`auth_provider='twitch'`) preferred over `twitch_oauth_tokens` (ADR-0016), **no scope predicate**. | 403 `owner_channel_unverified`; the Twitch leg of every grant on that overlay is unusable. Remediation targets the **owner** ("reconnect your Twitch account"), surfaced in the owner's UI. |
| **Kick** | Same shape: `users.kick_id` (Kick-login) or `kick_oauth_tokens.kick_user_id`, yielding the numeric `broadcaster_user_id`. `kick_user_id IS NOT NULL` is mandatory — it is NULL on legacy listener-only rows (migration 062). | 403 `owner_channel_unverified`. Load-bearing beyond authorization: this is the **only** legitimate source of `broadcaster_user_id`. |
| **YouTube** | A `youtube_oauth_tokens` row for the owner and **exactly** the source's `channel_id` — that column is only ever written from `channels?mine=true` with the owner's own token. The `users`-row fallback used elsewhere is **forbidden as an anchor**: it is channel-agnostic and would match any channel id. | 403 `owner_channel_unverified`. A non-trivial existing population may fail this; measure before opening the gate. |
| **Discord** | Both required: a `discord_guilds` row for (owner, guild) — migration 035, `UNIQUE(user_id, guild_id)`, written by the bot-invite callback — **and** a live bot-token read showing the owner's own Discord member permissions in that guild satisfy `owner_id ∥ ADMINISTRATOR ∥ MANAGE_GUILD`. Plus the source's `channel_id` must actually resolve to that guild. **Amended 2026-08-10 — see "Discord anchor strength" below: the live read is required on delegated actions only.** | Owner not Discord-linked → 403 `discord_link_required`. Row missing → 403 `owner_guild_unverified`. Live-check error → **fail closed**; no stamp-based bypass. |
| **TikTok** | n/a — no moderation API on any TikTok product. Absent from `PlatformActions` by design; reported `unsupported_platform`, never a fake button. | — |
| **shared_overlay** | n/a — stays non-moderatable. Owner-only authorization made "a recipient must not moderate the original streamer's channel" true by construction; role-based authorization does not, so the existing `platform <> 'shared_overlay'` predicates become security-critical and get an explicit regression test. | — |

#### Discord anchor strength (amendment, 2026-08-10)

The anchor's two halves apply asymmetrically on Discord, decided when the leg was implemented:

| Path | What must hold |
|---|---|
| **Owner** action | The `discord_guilds` row for (owner, guild). |
| **Delegated** action | The row **and** a live `DiscordOwnerControlsGuild` read of the owner's own member permissions. |

The reason is that the row is not a weak substitute for the live read — it is itself platform-attested. Discord only lets someone add a bot to a guild where they hold **Manage Server**, so a `discord_guilds` row *is* Discord's own record that the owner controlled that guild at invite time. What the row cannot see is a later loss of that standing, which is what the live read adds.

That staleness matters on the delegated path, where a third party acts on the strength of the owner's reach, and matters far less on the owner path, where a stale row grants the owner authority over a guild they themselves connected. Set against that: the live read needs the owner's Discord **user id**, which no existing streamer has — the bot-invite flow never captured one — so requiring it on the owner path would switch Discord moderation off for **every** current Discord streamer until each completed a new account link. Disabling a working feature to re-prove something Discord already attested is the worse trade.

So the anchor still applies to owner actions, as the original rule requires; only its strength differs by path. The unlinked-owner case does surface on the delegated path, where the moderator is told the streamer must link their Discord account — remediation aimed at the owner, per the anchor-failure UX decision below.

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

#### Discord reason codes as built (amendment, 2026-08-11)

Two departures from the table above, both settled while implementing the leg. They change which name a refusal carries, never which refusals happen.

**The three owner-side Discord failures share one code.** The row above names `discord_link_required` for an unlinked owner and `owner_guild_unverified` for a missing `discord_guilds` row; as built, an unlinked owner, a missing row, and an owner who has lost `MANAGE_GUILD` all report **`owner_channel_unverified`** — the platform-agnostic code the Twitch and Kick anchors already use. The reason vocabulary is sorted by *who can clear a state*, and all three are the streamer's alone; three codes for one remediation would have the frontend branch on a distinction it cannot act on differently. `discord_link_required` is therefore reserved for the **moderator**, the one person it names something actionable for.

**Capabilities makes no live Discord read.** The pre-check column above describes the *action* path, which is where it is load-bearing. `GET /capabilities` checks only the two account links and the bot's cached guild permissions, and deliberately does not read the moderator's live standing. Capabilities is advisory everywhere else too — on Twitch it checks the scope and lets Helix decide whether they moderate the channel — and reading Discord per source per dashboard load would put platform traffic behind a caller-supplied overlay id to produce an answer that can go stale before the button is pressed. A capability the moderator turns out not to hold fails at action time with `mod_lacks_permission`, which names the fix.

#### Kick single-message delete, as built (amendment, 2026-08-12)

The decision table below makes this its own PR "once the staging matrix confirms the endpoint and the 401-vs-403 mapping". It shipped on a **narrower** basis than that, deliberately, and the two halves of that condition turn out to be independent.

**The endpoint is confirmed from Kick's published API reference**, not a maintainer thread: `DELETE /public/v1/chat/{message_id}`, gated behind a second scope, `moderation:chat_message:manage` ("Execute moderation actions on chat messages"), which is listed in Kick's own scope table. That is the whole of what the owner-facing correction needs.

**The 401-vs-403 question does not gate it.** That mapping only matters where the two statuses lead to *different remediations*, which is the delegated path: 403 means "you are not a moderator here" and 401 means "your token is bad". On the owner path both already resolve to the same answer — re-consent, naming the missing scope — so the correction is safe to ship while the mapping is still unverified. It remains a blocker for the Kick delegation leg.

Two consequences worth stating, because they are what makes this cheap:

- **Kick's moderation scopes are independent grants**, so the scope→action mapping had to become per-scope rather than per-platform. Every streamer who consented before delete existed holds `moderation:ban` alone; reporting delete for them would light a button whose call Kick refuses. They keep timeout/ban/unban and gain delete only by re-consenting.
- **No pipeline work was needed.** Kick's own message UUID *is* All-Chat's message id (the normalizer carries `raw.MessageID` straight through), exactly as with Discord's snowflake, so the frontend's existing `item.id` branch already produces the native id. The id is nonetheless escaped into the path server-side: it is attacker-influenced input, and unescaped it could address a different endpoint.

#### Every proxied route needs an api-gateway entry (2026-08-11)

`services/api-gateway/cmd/main.go` enumerates each proxied path explicitly; there is no prefix forward for `/api/v1/auth/*`. The Discord account-link routes shipped without their gateway entries and were therefore unreachable from the internet — invisible at the time because nothing called them yet. Adding a route to a service is not finished until the gateway forwards it, and "the service registers it" is not testable from outside without that.

### Credential storage prohibition

A delegated moderator's credential is stored **only** in a dedicated store keyed on the moderator's own identity, never in the existing per-channel credential tables, and never in any row a listener selects from.

This is not fastidiousness. `twitch-eventsub-listener/channels/manager.go` selects a chat-reading credential by `LOWER(twitch_login) = LOWER(ocs.channel_id)` with **no user scoping**, merely *preferring* rows that hold chat scope; `kick-listener/channels/manager.go` selects `FROM kick_oauth_tokens WHERE channel_id = $1 … ORDER BY expiry DESC LIMIT 1` with no user scoping and no scope preference at all. A moderator-scoped credential written into either table becomes a candidate **ingest** credential and can sort first, silently breaking chat ingest on a real channel. The same unscoped predicate drives overlay-manager's `chat_via_eventsub` badge and the scope exporter.

Corollary: **never** key a moderator's credential to the streamer's channel identifier.

### Consent flow

Consent is **deferred to first use**. Accepting an invite costs a moderator nothing; a new `/moderate` area lists every channel delegated to them across all streamers, and opening one shows each platform source as ready or as "Connect to moderate" until they consent for that platform. Because Twitch's and Kick's moderation scopes are **role-based rather than channel-scoped**, one consent per platform serves every streamer who delegates that platform — a moderator working for three streamers on Twitch consents once.

This ordering matters beyond convenience: it means a volunteer never faces a stack of OAuth screens before they have done anything, and never grants a scope for a platform they turn out not to moderate.

The `/moderate` list is also not optional. `GET /api/v1/overlays` is owner-filtered and there is no shared-with-me listing, so without it an accepted moderator has no way to reach the overlay at all.

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
- YouTube asks a volunteer for `youtube.force-ssl` over **their own** channel to do unpaid work on someone else's, with a consent screen that reads badly ("See, edit, and permanently delete your YouTube videos, ratings, comments and captions"). Mitigated by deferring that consent until they actually try to moderate a YouTube source, and by an All-Chat-owned explainer before the redirect. The unverified-app 100-user cap does **not** apply — the project is published, not in Testing.
- The moderation write path grows a role dimension, and several currently-owner-only behaviours must stay owner-only by explicit assertion rather than by construction (`shared_overlay` exclusion, YouTube rediscovery, engagement controls, chat send).
- Admin impersonation of a delegated moderator would act with a **third party's** credential and put their name in the streamer's native log for an action they never performed. ADR-0017's "impersonated moderation is allowed but always attributed" was scoped to the owner's own token; extending it to third-party credentials is a new decision that must be settled explicitly, not inherited.
- N moderators multiply per-channel action volume, which platform rate limits bucket per **broadcaster** (Twitch bans) or per **project** (YouTube quota) — spreading across moderator tokens relieves neither.

### Neutral
- No new credential table for platform tokens beyond the dedicated moderator store; the existing owner-path resolvers are untouched.
- The dispatcher signature is unchanged: its `userID` parameter simply becomes "the actor", which is exactly the desired semantics.

## Implementation

**Prerequisite, ships independently:** a Discord source-add entitlement check. Adding a Discord source today performs **no** guild validation — the handler validates `shared_overlay` against `share_requests` and then falls straight through to `Create` + `setDiscordChannelRegistry` for Discord. Combined with owner-only moderation authorization and the shared bot's guild permissions, that is a live cross-tenant escalation path independent of this ADR. Measured on production: 12 Discord sources, all 12 carrying `guild_id`, **0** lacking a matching `discord_guilds` row for their owner — so the check can hard-fail with no grandfathering.

**Phase 1 — foundation (blocks all delegation)**: migrations (grants, moderator credentials, audit columns, DSGVO export/delete wiring, the `delegated_moderation` gate); the `ActionModConsent` state kind and forked callback; a refresh loop for moderator credentials (without it, grants die long before the 90-day dormancy rule can fire); role resolution and the rewritten `authorize()` with owner-keyed premium; the unified outcome family and frontend reason union; both idempotency guards; a moderator-scoped capabilities shape and a "channels I moderate" entry point; owner UI for invite/revoke/per-platform toggles and leg health; `/upgrade` + `OnboardingChecklist` entries.

**Phase 2 — per platform, each independently gated**: **Twitch** first and cheapest (`moderator_id`/`broadcaster_id` split, the anchor, the advisory `moderation:read` pre-check). **Kick** next (anchor, and removal of the moderator's own id from `broadcaster_user_id`); single-message delete lands as its **own earlier PR**, since it is an owner-facing capability correction rather than a delegation requirement. **Discord** (the prerequisite fix, an `identify` link for owner and moderator, generalized member-permission computation, intersection and hierarchy gates); the `GUILD_MEMBERS` intent stays off. **YouTube** last but still in v1 — its write path must first be completed, since it is currently **permanent-ban-only** (`clients/youtube.go` hardcodes `"type":"permanent"`, unban needs a ban resource id the client discards, and single-message delete does not exist). Handing a volunteer permanent-ban-only is a moderation-safety problem, so completing timeout + delete + unban is a hard prerequisite for the YouTube leg.

**Phase 3**: the dormancy job, soft-cap enforcement, this ADR plus the ADR-0017 amendment and README/service-README updates in the same PR, and the Patreon post (CLAUDE.md release step 3).

**Tests first (TDD)**: member-permission table tests (owner short-circuit, ADMINISTRATOR, `@everyone`-only, multi-role OR, position ties, 404 ⇒ deny); bot ∩ moderator intersection; hierarchy denies timeout/ban but allows delete; role resolution returns an identical 403 for existing and non-existent overlays; the escalation matrix for idempotency (collapse, escalate, unban-then-reban); the Kick anchor supplies the owner's id and a moderator's own id can never reach `broadcaster_user_id`; cross-tenant Discord source-add rejected; `shared_overlay` unreachable via a grant; every new migration added to the migration-rerun test.

### Decisions taken

All settled 2026-08-06:

| # | Decision |
|---|---|
| Discord authority | **Amend the own-credential invariant** for Discord's platform-attested model. The residual risk (no platform backstop) is accepted explicitly; Discord ships in v1 with the live ≤60 s permission check as a blocker, not a follow-up. |
| YouTube rollout | **Ships in v1**, not dark. The Google Cloud project is **not** in Testing (quota 1,009,000 units/day), so delegated moderators do **not** consume a 100-authorizing-user cap. |
| Moderator consent | **Deferred to first use, not to invite-accept.** A moderator accepts an invite for free, then consents per platform the first time they try to moderate on it. Twitch/Kick moderation scopes are role-based rather than channel-scoped, so **one consent per platform serves every streamer** who delegates that platform. |
| YouTube auto-provisioning | `liveChatModerators.insert` with the owner's token at stream start is **opt-in per overlay, off by default**. The durable path is a one-time manual add in YouTube Studio; the API grant is per-`liveChatId` and would need re-applying every broadcast. |
| Twitch `channel:manage:moderators` | **Not in v1.** `moderation:read` stays read-only and advisory; remediation copy tells the streamer to mod the person on Twitch. All-Chat does not gain the ability to change who moderates a channel. |
| Kick single-message delete | **Its own PR, before** the delegation Kick leg, once the staging matrix confirms the endpoint and the 401-vs-403 mapping. It is an owner-facing capability correction, so it should not be coupled to a feature that might be rolled back. *(Shipped 2026-08-12 — see the amendment above: the endpoint is confirmed from Kick's published reference, and the 401-vs-403 mapping turned out to gate only the delegated path.)* |
| Anchor scope | **Retrofitted to the owner path too**, hard-fail. Verified low-risk: on Twitch and Kick the anchor is already implied by credential resolution (no owner credential for a channel already yields 422), and admin-added YouTube/Kick channels pass because `copyYouTubeTokenForChannel`/`copyKickTokenForChannel` write a real per-channel row. Hand-entered channels are admin-only (plus TikTok, which is unsupported for moderation), where a refusal is expected. Count affected owner sources before enabling, as a sanity check only. |
| Denial auditing | **No-role denials are counted, not audited** — a Warn log with the caller id plus `allchat_moderation_unauthorized_denials_total{reason}`. Overlay ids are caller-supplied and `moderation_actions` has no FK on them, so auditing probes would pad the log with rows for overlays that never existed. Denials of a legitimate owner or moderator are still audited per ADR-0017. |
| Anchor-failure UX | **Owner banner + neutral moderator copy.** The owner gets an actionable banner ("reconnect your Twitch account — your moderators cannot act"); the moderator sees "ask &lt;streamer&gt; to reconnect", never a re-consent button that would connect the wrong account. |
| Grant acceptance | **Explicit accept retained.** The grant starts `pending`; the moderator sees who is delegating what and accepts. Acceptance is the record they agreed to act on someone's behalf, and it binds a pre-bound invite to the right account. |
| Soft cap | **10 per overlay, enforced at invite time** (409 on the 11th). No retroactive effect, so changing the cap can never cut off a working mod team mid-stream. Admin override via the existing admin surface. |
| Dormancy | 90 days idle → suspend; **the owner re-activates.** A moderator re-consenting must not lift a suspension, or the suspension is a speed bump rather than a control. |

Empirical verification is required before the corresponding leg opens, and none of it is documented by the platforms: whether Twitch issues a token for moderation scopes alone; Kick's 401-vs-403 mapping and whether a moderator's own token reaches a channel they do not own; whether YouTube's `channels.list?mine=true` works with force-ssl alone, whether InnerTube message ids are accepted by `liveChatMessages.delete`, the real per-call quota cost of every `liveChat*` write (the existing "official" cost constant's provenance is wrong), and whether unban survives the broadcast that created the ban.

**Discord — resolved 2026-08-10**, measured against the production application (`All-Chat-Bridge`, intents `33281`): `GET /guilds/{guild_id}/members/{user_id}` **does** return an arbitrary member with the `GUILD_MEMBERS` intent off (200, with the member's `roles`). Only the *list* endpoint is gated (`GET /guilds/{g}/members` → 403 `50001 Missing Access`), which is what the intent requirement in Discord's docs refers to. So the platform-attested model is feasible as designed and the intent stays off.

The same measurement surfaced a live defect in the **existing** owner-side code, fixed separately because it is an owner-facing capability correction rather than a delegation requirement: `@me` is accepted only on routes Discord documents as current-user routes (`/users/@me`). As the `{user_id}` path parameter of the guild-member route it is coerced to a snowflake and rejected with `400 NUMBER_TYPE_COERCE`, and `/users/@me/guilds/{g}/member` is closed to bots outright (403, code `20001`). Both `moderation-service`'s `GuildBotPermissions` and `auth-service`'s `CheckBotPermissions` called it that way, so the bot's effective guild permissions could never be computed: every Discord source reported `missing_scope` and told the streamer to re-invite a bot that re-inviting could never fix. The bot's own member record must be fetched by its explicit snowflake, resolved once from `/users/@me`.

## Related Decisions

- [ADR-0017](./0017-chat-moderation-write-path.md) — **amended** here: owner-only authorization, `broadcaster_id == moderator_id`, the add-source-state reuse for moderation consent, `RequirePremium` on the action routes, and the Discord "no credential resolution or scope pre-check" statement all change.
- [ADR-0012](./0012-oauth-scope-minimisation.md) — the minimisation stance a moderator's consent screen must honour; the base-login-scope leak is a regression against it.
- [ADR-0016](./0016-linked-twitch-credentials.md) — the credential model the moderator store deliberately does *not* extend, and the unscoped listener selectors that make the storage prohibition necessary.
- [ADR-0008](./0008-feature-gate-infrastructure.md) — the gate infrastructure, and its deliberate rejection of per-user targeting.
- [ADR-0019](./0019-split-streamer-viewer-premium.md), [ADR-0027](./0027-time-limited-admin-premium-overrides.md) — the entitlement semantics inherited by owner-keyed gating.
- [ADR-0034](./0034-admin-viewer-identity-model.md) — why a moderator needs a streamer-side account: viewer sessions carry no user id and cannot represent a grant.
- [ADR-0006](./0006-youtube-quota-tracking.md) — quota accounting that bounds YouTube moderation, per project rather than per user.
- [ADR-0013](./0013-overlay-observability-view.md) — the dashboard that hosts the moderation UI a moderator must be able to reach.
