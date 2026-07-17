<!-- Compiled by an audit workflow (7 parallel area audits -> synthesis -> adversarial critique -> verified final plan). 54 problems found across the admin area. -->

> **Verification note.** Verification confirms the critique's load-bearing claims: `youtube_id` is populated from `user.GoogleID` (admin.go:131, a Google account id, not a `UC…` channel id); the Users "Connected Platforms" block renders `twitch_id`/`youtube_id`/`kick_id` as raw mono strings (users/page.tsx:558-585); `profile_image_url` is in the payload (admin.go:128) and interface (users/page.tsx:38) but never rendered; `platform-colors.ts` defines `discord` (line 30) but has no `shared_overlay` key, and badge.tsx:65 dereferences `.text` unguarded (so only `shared_overlay` crashes); and `filter` is React state with no query-param reader (users/page.tsx:68). Final plan below.

# Admin Dashboard Usability Revamp - Plan

## Problem

The admin dashboard is eight flat, independently-built pages that never connect the entities they display. This maps directly to the four complaints:

1. **"Overlays are not linked to their users."** An overlay shows its owner only as a raw `user_id` UUID with no username and nothing to click. The list rows omit the owner entirely (`overlays/page.tsx:302-337`) and the detail panel shows a bare mono UUID (`overlays/page.tsx:407-412`).
2. **"Sources should be clickable and resolve to the actual channel."** Channel names/ids on the Sources page, the Overlays sub-panel, and the Users "Connected Platforms" block are all plain text that never resolve to the real Twitch/YouTube/Kick/TikTok page. There is no channel-URL helper anywhere in the frontend today.
3. **"Generally the usability is sub par."** The pages disagree on nearly every interaction pattern: how you reach a detail view, whether a selection is URL-addressable, whether search/empty-states exist, and which confirmation primitive destructive actions use.
4. **"The viewer view is also basically useless."** A truncated 100-row dump with no search, filters, pagination, detail, avatars, or streamer/channel context, whose stat cards silently miscount because they are computed from the loaded page (`viewers/page.tsx:177-179,243`).

The backend-data audit confirms **every requested fix is feasible against the current shared PostgreSQL DB**. The owner JOIN is trivial (`source_repo.go` already joins `users`), most channel URLs are buildable from data already in the payload, and the viewer identity fields already ride in the response. The gaps are in what the SELECTs and response structs choose to return, plus missing frontend wiring. The one genuinely new backend capability is the viewer-to-streamer activity aggregate.

One latent crash must be fixed regardless of scope (see Phase 0).

## Current state (evidence-cited)

- **Owner linkage absent end to end.** `GetAllOverlaysWithSourceCount` selects `o.user_id` but never joins `users` (`overlay_repo.go:288-317`). Usernames are in **no** admin payload. `user_id` itself is present in the sources payload and even declared in the TS interface but never rendered (`sources/page.tsx:37`).
- **No addressable user target.** The Users page keeps `selectedUser` in React state only (`users/page.tsx:62`), `filter` is likewise state-only with no query-param reader (`users/page.tsx:68`), and there is no `[id]` route. The Overlays page already has a working `?overlay=` deep-link to copy from (`overlays/page.tsx:92-102`).
- **No channel-URL helper.** Even the main overlay editor renders the channel as plain text (`app/overlays/[id]/page.tsx:311`). `/admin/sources` already SELECTs `s.channel_handle` (`source_repo.go:329,346`) but `SourceResponse` drops the field (`admin.go:93-118`).
- **YouTube identity ambiguity.** `youtube_id` on the Users payload is populated from `user.GoogleID` (`auth-service/handlers/admin.go:131`), a Google account id, **not** a `UC…` channel id. There is no write-time YouTube validation on add-source (`sources.go:698+` guards only twitch/kick), so stored source `channel_id` for legacy YouTube rows is also not guaranteed to be a `UC…` id.
- **Viewer view is data-starved.** `viewer_sessions` carries no overlay/streamer/channel association; the only linking data lives in `viewer_message_history`, which is write-only today and surfaced only by the DSGVO export (`viewer_repository.go:269-301` writes it, `data_export.go:234` is the only reader). The "activity" counters shown are rate-limit windows (`models/viewer.go:37-40`), not real usage. `avatar_url` and `platform_user_id` are returned by the API (`admin_viewers.go:76-79`, `viewer_repository.go:322,353`) but dropped by the TS type (`viewers/page.tsx:49-65`).
- **Latent crash.** `PlatformBadge` does `PLATFORM_COLORS[platform].text` unguarded (`components/ui/badge.tsx:65`). `platform-colors.ts:25-32` defines `discord` but has **no** `shared_overlay` key, so rendering a `shared_overlay` source throws a `TypeError` and breaks the whole page. (Discord does not crash; the crash scope is `shared_overlay` only.)

## Themes & changes

### Theme 1 - Cross-linking (overlays and sources to their owner)

Goal: every entity is addressable and every owner reference is a resolved, clickable username. This is complaint #1 and the structural prerequisite for most other links.

**Backend changes**
- Add owner JOIN to the overlay and source list queries. `GetAllOverlaysWithSourceCount` (`overlay_repo.go:288-317`) needs `LEFT JOIN users u ON u.id = o.user_id`, selecting `u.username, u.display_name`. `GetAllSourcesWithOverlay` (`source_repo.go:327-357`) already joins `overlays`; add the `users` join there too. Add `OwnerUsername`/`OwnerDisplayName` to `OverlayResponse` and `SourceResponse` (`overlay-manager/handlers/admin.go:55-77, 93-118`). Also apply to `ListByUserIDWithSourceCount`. Same-DB JOIN, already idiomatic. Effort: M. Data available: yes.
- Optional, cheap once editing the query: surface overlay-level state (`is_active`, `is_public_for_viewers`, `description`, `updated_at`) which the model has but the admin list query omits (`overlay_repo.go:288-296`). Effort: S.

**Frontend changes**
- Add a `?user=<id>` deep-link handler to the Users page, mirroring the existing `?overlay=` pattern (read param on load, `setSelectedUser`, seed search). Effort: M. **Hard dependency for every owner link below.** Note that `user_id` is already present in payloads, so this deep-linking works with no backend change.
- Render owner as `@username` in overlay list rows and detail panel, falling back to the UUID only when the join is null (orphaned overlay). Effort: M.
- Add an Owner column to the Sources table linking the resolved username to `/admin/users?user=<id>`. Effort: M.
- Add `?user=<id>` client-side filtering to the Sources page (mirrors `?overlay=`, `sources/page.tsx:82-109`) plus a "View this user's sources" link in the user detail panel. Effort: M.
- Repoint Users-page overlay links from the live render `/overlay/{id}` (`users/page.tsx:971`) to the admin detail `/admin/overlays?overlay={id}`, matching the Sources page (`sources/page.tsx:331`); keep the live overlay as a small secondary external-link icon. Effort: S.
- Once usernames land, add them to the overlay search predicate (`overlays/page.tsx:193-198`) and relabel from "user ID" (`overlays/page.tsx:253`) to "…or owner"; add `u.id` to the Users search predicate (`users/page.tsx:343-349`) and a "Beta" filter tab (`users/page.tsx:68`). Effort: S each.

**Evidence:** overlay_repo.go:288-317; source_repo.go:327-357; admin.go:55-118; overlays/page.tsx:92-102,302-337,407-412; users/page.tsx:62,971; sources/page.tsx:37,331.

**Effort:** M backend + M/S frontend.

**Feasibility notes:** every "resolved username" item is gated on the owner-JOIN backend change and is listed as such. Bare `user_id` links and `?user=` deep-linking need no backend change.

### Theme 2 - Clickable channel resolution

Goal: channel names resolve to the real platform page (complaint #2), across **all** the surfaces that show a channel, including the Users "Connected Platforms" block the draft originally missed.

**Backend changes**
- Stop dropping `channel_handle` on `/admin/sources`. The query already SELECTs it (`source_repo.go:329,346`); add `ChannelHandle *string json:"channel_handle,omitempty"` to `SourceResponse` and populate it (`admin.go:93-118`). No query change. This lets YouTube/TikTok links use the nicer `@handle`. Effort: S. Data available: yes. (`GetOverlaySources` already returns the full model including `channel_handle`; only `ListAllSources` needs the struct field.)

**Frontend changes**
- Create `frontend/src/lib/platform-channel-url.ts` exporting `channelUrl(platform, channelId, channelHandle?)`:
  - twitch -> `https://twitch.tv/{channelId}` (channelId = lowercased login, `sources.go:380-393`)
  - kick -> `https://kick.com/{channelId}` (slug, validated non-numeric, `sources.go:397-415`)
  - tiktok -> `https://www.tiktok.com/@{channelId without leading @}` (channelId = username)
  - youtube -> prefer `https://www.youtube.com/{channelHandle}` when present, else `https://www.youtube.com/channel/{channelId}` **only when `channelId` starts with `UC`**; otherwise return null (see unknowns)
  - discord / shared_overlay / unknown -> return `null` (discord channelId is a numeric snowflake; shared_overlay channelId is an overlay UUID)
  - Effort: M. Data available: yes for twitch/kick/tiktok/youtube links; the nicer YouTube handle needs the S backend change above.
- Wire the helper into an external `<a>` (`target=_blank rel="noopener noreferrer"`, external-link icon, aria-label), rendering plain text when the helper returns null, at **four** sites:
  - Sources table channel cell, desktop + mobile (`sources/page.tsx:325-328, 376-381`)
  - Overlays Sources sub-panel (`overlays/page.tsx:480-485`); add `channel_handle` to the `OverlaySource` interface (`overlays/page.tsx:50-57`), which currently drops it
  - Main overlay editor `SourceCard` (`app/overlays/[id]/page.tsx:311`), adopting the same helper (Pathfinder)
  - **Users "Connected Platforms" block** (`users/page.tsx:558-585`): linkify twitch/kick using the account handle where available, **explicitly excluding YouTube** because `youtube_id` is a `GoogleID`, not a channel id (`admin.go:131`), and would produce a `/channel/{GoogleID}` link that may not resolve. Effort: S per site.
- A11y/security guardrails (repo has strict jsx-a11y ratchets): retrofit `rel="noopener noreferrer"` onto the two existing bare `target=_blank` links (`overlays/page.tsx:340-346`, `users/page.tsx:971-973`); inside the stretched-button overlay row (`overlays/page.tsx:300`, `after:inset-0`) any new link needs `relative` + `stopPropagation` like the existing open-overlay link (`overlays/page.tsx:344-345`). Effort: S.

**Evidence:** source_repo.go:329,346; admin.go:93-118; sources.go:380-415; users/page.tsx:558-585; admin.go:131; app/overlays/[id]/page.tsx:311.

**Effort:** S backend + M/S frontend.

**Feasibility notes:** twitch/kick/tiktok links are buildable from data already in the payload (frontend-only). The one backend dependency (`channel_handle`) is S and only improves the YouTube link; YouTube degrades to the `/channel/UC…` form or to plain text without it. The **Users-block YouTube link is deliberately not built** because the stored value is a Google account id.

### Theme 3 - Viewer view overhaul

Goal: make the Viewers page usable for its one real job (find a reported viewer and act on them) and give it identity and streamer context (complaint #4). Split the work so the client-only identity wins ship **early** (Phase 1) and the search/aggregate backend follows.

**Backend changes**
- Server-side search + filters + total count on `ListAll` (`viewer_repository.go:320-335`; `HandleListViewers` `admin_viewers.go:50-86` currently takes only limit/offset). Add `q` (ILIKE on username/display_name/platform_user_id), `is_banned`, `is_premium`, `platform` params + WHERE clauses, and a `SELECT COUNT(*)` so the stat cards and "showing X of Y" are correct across the whole dataset (today capped at 100, `viewers/page.tsx:177-179,243`). Effort: M. Data available: yes.
- Surface `viewer_sessions.user_id` (added by migration 040 to link a viewer who is also a streamer). `ListAll` selects only `vs.viewer_id` (`viewer_repository.go:328`); the model has no field (`models/viewer.go:43`). Add to SELECT + model, optionally LEFT JOIN `users` for that streamer's username. Effort: S.
- Add a per-viewer activity aggregate from `viewer_message_history` (`streamer_user_id`, `overlay_id`, `channel_id`, `channel_name`, `sent_at`): `COUNT(*)`, `MAX(sent_at)`, distinct recent streamers/overlays joined to `users.username`. Prefer a separate `GET /admin/viewers/:session_id/activity` endpoint over folding into the hot list query, pending the index/cost check (see unknowns). This is the only way to answer "whose chat does this viewer participate in." Effort: L. Data available: yes, joinable in the shared DB, but query cost is unvalidated.
- Replace the misleading activity counters. `message_count_1min`/`1hour` are rate-limiter windows (`models/viewer.go:37-40`) that reset constantly; stop presenting them as engagement and use the real totals from the aggregate + `last_message_at` as "last seen." Effort: M (rides on the aggregate).

**Frontend changes**
- Client-only identity wins (ship early, all data already in the payload once the type is widened): render `platform_user_id` (declared but never shown, `viewers/page.tsx:52`); add `avatar_url` to the interface and render a thumbnail; render platform via the shared `PlatformBadge` instead of `capitalize` text (`viewers/page.tsx:318,383`); explain the premium `—` for session-only viewers (viewer_id null) with a tooltip (`viewers/page.tsx:184-187`; premium keys on a linked `viewer_id`, `viewer_repository.go:437-443`). Effort: S.
- Profile links via `buildViewerProfileUrl(platform, username)` for twitch/kick/tiktok only (`twitch.tv/{username}`, `kick.com/{username}`, `tiktok.com/@{username}`). **Do not** build a YouTube viewer link: by analogy with the streamer `youtube_id`, the viewer's stored id is not a confirmed `UC…` channel id, so YouTube stays plain text until a real row is checked. This also assumes `viewer_sessions.username` equals the platform login/slug; verify before promising the links (see unknowns). Effort: S.
- Add a search box (client-side over the page as an interim, wired to the backend `q` param for full-dataset search). No search input exists in the file today. Effort: M.
- Add pagination controls. The page hardcodes `?limit=100` with no offset UI (`viewers/page.tsx:98-100`) though the backend already accepts+echoes offset; add offset state + Prev/Next now, "page X of Y" once COUNT lands. Effort: S.
- Add status/premium/platform filters, matching sibling pages (`sources/page.tsx:237-274`, `users/page.tsx:402-447`). Effort: M.
- Add a "chats in" column/detail linking `overlay_id` -> `/admin/overlays?overlay=…` once the activity aggregate exists. Effort: rides on backend L.

**Evidence:** viewer_repository.go:320-335,437-443; admin_viewers.go:50-86,76-79; models/viewer.go:37-43; viewers/page.tsx:49-65,98-100,177-187,243,318,383; data_export.go:234.

**Effort:** M+S+L backend + S/M frontend.

**Feasibility notes:** avatar, `platform_user_id`, twitch/kick/tiktok profile links, and pagination all use data already in the response (frontend-only). Full-dataset search/count is the M backend change. The streamer/overlay context is the single new backend capability (L), ADR-gated on the viewer-identity decision, and its "fully joinable" framing stays subordinate to the unresolved query-cost and identity questions below.

### Theme 4 - General usability & consistency

Goal: stop each page teaching a different interaction model (complaint #3).

**Backend changes (both optional/lower priority)**
- Server-side pagination on the currently-unbounded lists. `ListUsers`, `ListOverlays`, and `ListAllSources` each SELECT every row with no LIMIT (`user_repository.go:636-656`, `overlay_repo.go:288-317`, `source_repo.go:327-357`); filtering is client-side after downloading everything. Add optional `q`/`platform`/`limit`/`offset` + count. Effort: M. Only if dataset size is hurting.
- Admin source mutations for real row actions: `admin.go` is read-only and there is no `SetActive` repo method (`Delete` exists at `source_repo.go:264`). Add `PATCH/DELETE /api/v1/admin/sources/:id` only if the empty "Actions" column should carry real operations. Effort: M.

**Frontend changes**
- **Harden `PlatformBadge` (crash fix, do first).** Add a `system` fallback for unknown platforms inside `components/ui/badge.tsx:65` (the shares-domain `PlatformBadge.tsx:36-41` already does this; promote it), and widen the admin `Source`/`OverlaySource` platform unions to include `discord`/`shared_overlay` (`sources/page.tsx:32`, `overlays/page.tsx:52`), removing the unsafe `as Platform` casts. Only `shared_overlay` crashes today, but widening the union to include both is still correct. Effort: S.
- Render the **user avatar** (`profile_image_url`, already in payload at `admin.go:128` and typed at `users/page.tsx:38`, currently never rendered) in the Users list rows and detail header. Frontend-only. Effort: S.
- Make the home dashboard the nav surface it claims to be: wrap the non-clickable stat cards (`page.tsx:41-63,102-145`) in Links to the matching filtered list; generate the home nav grid from the same `ADMIN_LINKS` the sidebar uses (`AdminSidebar.tsx:51-60`), since the home grid has only 4 of 8 entries (`page.tsx:149-197`) and drifts from the sidebar; render the per-platform source breakdown the payload already carries instead of collapsing to a sum (`page.tsx:93-95`). Effort: S. **Dependency:** the filtered stat-card links require a destination-side query-param reader that does not exist today. `overlays/page.tsx:92-102` reads only `?overlay=` (not `?connected=`) and `users/page.tsx:68` keeps `filter` in state with no reader at all. Adding `?filter=` to Users and `?connected=` to Overlays is in scope for this item; the nav-grid-from-single-source and per-platform breakdown parts have no such dependency and can ship first.
- Add "no results" empty states to Users/Overlays/Sources lists, which render a blank card body on zero matches (`users/page.tsx:449`, `overlays/page.tsx:277-381`, `sources/page.tsx:320`); Viewers/Features/Maintenance already do this. Effort: S.
- Fix the silent active-overlays poll failure: a failed `/admin/overlays/active` fetch only console.errors, leaving every overlay reading "Not connected," indistinguishable from a genuinely idle system (`overlays/page.tsx:112-135`). Track a `connectionStatusUnavailable` flag and show "unknown" + a note. Effort: S.
- Consistency of list/detail: cap the Users list height and make its detail panel sticky (`users/page.tsx:449,529`), matching the Overlays page (`overlays/page.tsx:277,387`); regroup the always-expanded user detail panel (Impersonate/Premium/Beta/Ban all stacked inline, `users/page.tsx:588-960`) into compact state chips + a separated "Danger zone." Effort: M.
- Extract shared primitives (Pathfinder, larger): an `AdminPage`/`PageHeader` (widths drift `max-w-7xl` vs `max-w-3xl` vs `max-w-4xl`, and cosmetics double-applies `min-h-screen bg-bg` over the layout, `cosmetics/page.tsx:163-168` vs `layout.tsx:25`); a shared `AdminTable`; route all destructive actions through the shared `Dialog` (cosmetics deletes with **no confirm** `cosmetics/page.tsx:108-122`, maintenance uses native `window.confirm` `maintenance/page.tsx:128-129`, others use the styled Dialog). Replace the redundant Sources "View" action that duplicates the Overlay link (`sources/page.tsx:351-358`). Effort: M-L.
- Optional later: column sorting on Sources and Viewers (surface high-volume/recently-active viewers, group inactive sources). None exists today; if added, it needs `aria-sort` to stay inside the a11y ratchet. Effort: M.

**Evidence:** badge.tsx:65; platform-colors.ts:25-32; PlatformBadge.tsx:36-41; sources/page.tsx:32,320,351-358; overlays/page.tsx:52,112-135,277-381; users/page.tsx:38,68,449,529,588-960; page.tsx:41-197; AdminSidebar.tsx:51-60; cosmetics/page.tsx:108-122,163-168; maintenance/page.tsx:128-129; layout.tsx:25.

**Effort:** S crash fix; S-M frontend polish; M-L shared primitives; both backend items optional.

**Feasibility notes:** all Theme-4 frontend items except the filtered stat-card links use data already present (frontend-only). The filtered links depend on new destination param readers, flagged above. The two backend items are optional and clearly lower priority.

### Cross-cutting: the a11y gate risk the avatar work creates

Both the viewer avatar (Theme 3) and the user avatar (Theme 4) introduce new `<img>` elements into a repo with shrink-only, commit-blocking jsx-a11y/lint ratchets. Every new image must carry meaningful `alt` text (for example `alt={`${username} avatar`}`), and bare `<img>` will trip `@next/next/no-img-element`; use the project's existing image component or an explicitly justified exception. This guardrail must be applied wherever an avatar is added, not only to anchors. Effort: folded into each avatar item.

## Open questions / unknowns to resolve first

- **YouTube `channel_id` semantics for both sources and viewers.** Sources: no write-time YouTube validation (`sources.go:698+` guards only twitch/kick), so a legacy source row may hold a handle rather than a `UC…` id; the helper's `UC`-prefix branch handles this but returns null (plain text) otherwise. Viewers/users: the account-linked id is a `GoogleID` (`admin.go:131`), not a channel id, so no YouTube profile/channel link is built for the Users block or the viewer view until a real row confirms the stored value. Confirm against prod rows before trusting any YouTube link.
- **`channel_id`/`channel_name` semantics for the other platforms are confirmed:** twitch (login), kick (slug), tiktok (username/@handle). discord (snowflake) and shared_overlay (overlay UUID) are confirmed non-linkable.
- **Viewer `username` equals the platform login/slug/@handle?** The twitch/kick/tiktok viewer profile links assume this. Verify a real `viewer_sessions.username` holds the login before shipping the links.
- **Viewer-to-streamer linkage quality and cost.** The association exists only via `viewer_message_history`, which appears unindexed for this access pattern. Decide the endpoint shape (inline vs separate `/activity`) after checking indexes; prefer the separate endpoint if the JOIN is expensive.
- **Sessions vs durable viewer identity.** One human maps to many sessions/re-auths. Whether admin operates on raw sessions or a deduped durable identity changes what a "row" means and what ban/premium act on; must be settled before the viewer aggregate work.

## ADR impact

Flag as "needs ADR + cross-repo number check." The latest ADR is 0032 and numbering spans **both** all-chat and caesar-deployment (they previously collided at 0021), so verify both repos before assigning a number.

1. **Global entity search / resolver** (`/api/v1/admin/search` across users+overlays+sources+viewers). A new cross-service admin surface and query pattern establishing how admin resolves an arbitrary id/name/channel to the right detail page. Architectural -> ADR.
2. **Standardized admin master-detail + URL-addressable-selection pattern** (optionally real `/admin/[section]/[id]` routes + breadcrumbs). Committing every admin page to one navigation contract is an IA decision worth recording. Architectural -> ADR.
3. **Viewer identity model decision** - whether the admin viewer view operates on raw `viewer_sessions` or a deduped durable identity, and whether `viewer_message_history` becomes an admin-queryable surface (currently write-only + DSGVO-export-only). Surfacing it for admin aggregates changes its role. Architectural -> ADR.

The routine per-endpoint changes (owner JOIN, `channel_handle` passthrough, viewer search/filter params, `viewer_sessions.user_id` surfacing, list pagination) are additive and do **not** need an ADR.

## Phased sequencing

**Phase 0 - Crash, safety, and free client-only polish (ship immediately, all frontend, ~S total)**
- Harden `PlatformBadge` with a `system` fallback + widen admin platform unions (fixes the live `shared_overlay` TypeError). No dependency.
- Render the **user avatar** (`profile_image_url`, already in payload). No dependency.
- Home nav grid generated from `ADMIN_LINKS` + per-platform source breakdown (the non-filtered parts). No dependency.
- Add `rel="noopener noreferrer"` to existing external links; add empty-states; fix the silent active-overlays poll. No dependency.

**Phase 1 - Cross-linking foundation + early viewer identity wins**
- Backend: owner JOIN into overlay + source list responses (M).
- Frontend: `?user=` deep-link handler on Users page (M) - **gating dependency for all owner links**; then resolved-owner links on Overlays + Sources, repoint Users->overlay link, search-by-owner (S-M).
- Frontend (parallel, no backend): the viewer client-only identity wins - `platform_user_id`, `avatar_url` thumbnail (+ alt text), `PlatformBadge`, premium tooltip, and twitch/kick/tiktok profile links - so the Viewers page stops feeling useless before the search/aggregate backend lands.
- Depends on Phase 0 badge fix.

**Phase 2 - Channel resolution (all four surfaces)**
- Backend: `channel_handle` on `/admin/sources` (S).
- Frontend: `platform-channel-url.ts` helper + wire into Sources, Overlays sub-panel, main `SourceCard`, **and the Users "Connected Platforms" block** (twitch/kick only, YouTube excluded) (M).
- Independent of Phase 1; can run in parallel. Best YouTube links wait on the S backend change but degrade gracefully.

**Phase 3 - Viewer backend overhaul**
- Backend first: viewer search/filter/count params (M) + `viewer_sessions.user_id` surfacing (S).
- Frontend: wire search/pagination/filters to the backend params.
- Then the `viewer_message_history` activity aggregate (L, ADR-gated on the identity decision) and the "chats in" streamer-context column. The aggregate is the long pole.

**Phase 4 - IA + shared primitives + global search (tail)**
- Home clickable stat cards + the `?filter=`/`?connected=` destination readers they require (S).
- Users list/detail parity + danger-zone grouping (M).
- Shared `AdminPage`/`AdminTable`/confirm-Dialog primitives (M-L, Pathfinder).
- Optional column sorting with `aria-sort`.
- Global search resolver + standardized addressable master-detail (L, both ADR-gated) - last, after the `?user=`/`?overlay=` patterns from Phases 1-3 have proven the shape.

**Optional / defer:** server-side pagination on users/overlays/sources lists (only if dataset size is hurting), admin source mutations (only if the Sources "Actions" column should carry real operations), viewer session-to-identity dedup.
