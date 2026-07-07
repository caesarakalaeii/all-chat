# PR #524 — engagement review fixes: handoff & follow-ups

**State (2026-07-06):** The full adversarial-review fix plan (`pr-524-engagement-review-fixes.md`, all P0→P3) is implemented and pushed to `feature/523-engagement-polls-predictions-points` (6 commits). Backend builds/vets/tests clean; frontend `tsc` + touched-file ESLint clean. A 4th adversarial review (13-agent) found 5 follow-ups (1 medium, 4 low) — **all fixed** (see bottom). Merge gates (B1 cross-tenant IDOR, H1 cross-economy wager) are closed with regression tests.

> The commits are the implementation; more checks were deferred to the next session ("we'll run more checks later"). This doc is what's left.

---

## A. Deploy / infra — REQUIRED before the announce (H4-2) works in prod

1. **Run migration 072** (`migrations/072_engagement_dedup_scope_and_announce.sql`) via the normal runner (globs `migrations/`, in-pod path). It adds a `LOWER(channel_id)` functional index and `points_earn_config.announce_on_start`, and drops the retired global replay-dedup index names as a no-op safety net. The `source_message_id` and `(source, external_id)` uniqueness are now created **per-round / per-overlay directly in 069/070** — a migration RE-RUN would otherwise rebuild a global unique over legit multi-overlay data and abort every deploy (see P0-1 in the round-6 review). Migration **074** adds `predictions.sweep_canceled` (P2-4). **Not** mirrored into `migrations/init/` (frontend-dev doesn't run engagement) — intentional.
2. **caesar-deployment env** for the announce (absent ⇒ announce silently no-ops, never crashes):
   - engagement-service: `SERVICE_JWT_SECRET_V1` (mint the service JWT), plus optional `AUTH_SERVICE_URL` (default `http://auth-service:8081`) and `ENGAGEMENT_PUBLIC_BASE_URL` (default `https://allch.at`).
   - auth-service: `SERVICE_JWT_SECRET_V1` — without it the `/internal/chat/announce` route isn't registered.
   - `SERVICE_JWT_SECRET_V1` is already used by gateway/source-manager, so it's likely in the shared secret; just ensure both Deployments mount it.
3. **caesar-deployment branch** `feature/523-engagement-deployment` (Deployment/Service/HPA + `ENGAGEMENT_SERVICE_URL`) — push + PR **after** #524 merges so the `:main` image exists. Add the new envs from (2) there.

## B. Verification still to run ("more checks")

- **Integration regression tests are local-only** (the nightly matrix runs `go test ./...` without `-tags=integration` and has no Postgres; the PR `test-engagement-integration` job runs them with a Postgres service). Run against a DB with engagement migrations 068–074 applied:
  `cd services/engagement-service && go test -tags=integration ./repository/...`
  Covers `idor_test.go` (B1 cross-tenant lock/resolve/cancel/close → 404 + unchanged; H1 wager binds debit to the prediction's overlay) and `native_mirror_test.go` (L-C1 terminal-flip blocked).
- **Migrations 069–074 against a real DB** — confirm the per-scope dedup/mirror indexes, the `LOWER(channel_id)` functional index, and the `announce_on_start`/`sweep_canceled` columns apply cleanly, and that **re-applying the full batch over multi-overlay data succeeds** (`TestP0_1_MigrationRerunWithMultiOverlayData`, the P0-1 regression).
- **E2E worth exercising:** cross-tenant lifecycle attempts 404; a channel feeding two overlays lands the vote/wager on **both** (M-C1); announce posts to chat once the send scope is granted (and no-ops without it); PEL drain reclaims votes after a pod kill (H3).
- **Frontend manual/Playwright:** participate-page a11y (live regions announce balance/notice/settled; dark tokens render on OS-light), monitor two-step payout confirm, WS live refresh, config announce toggle round-trips.

## C. Known gaps / deferred (need a decision or are by-design)

- **M2 (exact payout):** the participate page shows a neutral "your prediction settled — check your balance" banner, **not** "+N / −N / refunded". The public `active-prediction` endpoint 404s on resolve and there's no payout field, so an exact figure needs a small server change — add a settlement/payout field to the wager/resolve response, or a short recently-resolved grace window on the public read.
- **L-U9 (QR):** **not** added — needs either an npm dependency (`qrcode.react`) or reusing `services/share-service` to mint a short code (a build-risk call). Only the copy-link + "share with mobile viewers" + OBS browser-source setup notes were added.
- **M-C1 test:** the migration is the fix; there's no dedicated 2-overlays-one-channel integration test yet. Worth adding.
- **M4/M5 mirror consent return:** the Twitch-mirror control now lives on both the monitor and the config page, but the consent flow still returns to `/overlay/[id]/view` (the monitor). Making a config-initiated consent return to the config page needs a backend redirect change to the shared moderation-consent flow — deferred.
- **By-design (plan says "won't fix"):** L-U4 (transposed `!predict 500 1` → dropped, intrinsic to positional chat grammar), L-U5 (a bare "2" means different options across multi-overlay), L-U6 (chat-vote acceptance is silent — acceptable now that options are numbered on-screen).

## D. Adversarial-review fixes already applied (context only, no action)

1. **[medium]** `useEngagementLive`'s WS on the monitor view shared the `ws_last_seen` chat watermark with the chat pane → possible dropped chat messages. Fixed: `WebSocketClient` `engagementOnly` mode (ignores chat frames, no `?since=`, no watermark writes).
2. **[low]** `announcer.buildMessage` byte-sliced the title → could emit invalid UTF-8. Fixed: rune-boundary backoff (`utf8.ValidString`).
3. **[low]** migration down-path recreated a *global unique* `source_message_id` index that would abort on legit multi-overlay rows. Fixed: down path recreates them **non-unique**. **Superseded by round-6 P0-1:** the per-scope indexes now live in 069/070 (created in their final scope), so 071/072 no longer drop+recreate anything and their down paths no longer recreate a global unique at all.
4. **[low]** `periodicDrain` could self-reclaim a >60s in-flight message (double-broadcast, non-corrupting). Fixed: `pelReclaimMinIdle` → 5m (matches message-processor).
5. **[low]** M2 settled banner could fire stale if the private fetch failed at a fast round handoff. Fixed: clear the wager ref on any observed round-id change regardless of the private-fetch result.
