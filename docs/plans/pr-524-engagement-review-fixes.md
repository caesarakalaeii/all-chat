# PR #524 — Engagement (polls/predictions/points) review fix plan

**Source:** adversarial review of PR #524 (`feature/523-engagement-polls-predictions-points`), 2026-07-06.
**Scope:** issue #523 — cross-platform polls, predictions, per-overlay viewer points. ADRs 0027 (chat write-path), 0028 (points economy), 0029 (Twitch-native mirroring).
**Status of PR:** feature-complete, builds/tests/lints clean, but **not merge-ready** — one blocker + one high-sev security bug move real points across tenants.

This plan is ordered by priority. **P0 + P1 gate the merge. P2/P3 are follow-ups.** Every item carries the exact location, root cause, and fix. Severities reflect the skeptic-verified ratings (several finder "high"s were corrected down to medium — noted inline).

> **Design decision for the author before coding P2 (not a code task):** the review confirmed the reviewer's hypothesis that **chat wagering is a poor experience** (blind to balance, silent failures, one-shot/irreversible, undiscoverable syntax). But the no-install participate page **already is** the UI that fixes this — so the right framing is **UI-first for wagering, keep `!predict` as a de-emphasized power-user shortcut**, *not* "extension-only" (which would wrongly imply an install barrier and strand TikTok/Discord chatters who have no viewer login). Chat *poll voting* is fine and worth keeping. Several P2 items (M1, M2, M3) implement this "steer wagering to the UI" posture. **Do not remove `cmdWager`.**

---

## P0 — BLOCKER (must fix before merge)

### B1. Cross-tenant IDOR: any streamer can lock/resolve/cancel/close another streamer's round
- **Category:** security · **Confidence:** high (verified firsthand)
- **Where:**
  - `services/engagement-service/handler/prediction.go:86` (LockPrediction), `:115` (ResolvePrediction), `:150` (CancelPrediction)
  - `services/engagement-service/handler/poll.go` (ClosePoll)
  - `services/engagement-service/repository/predictions.go:167` (LockPrediction UPDATE), `:279` (ResolvePrediction UPDATE), `:329` (CancelPrediction UPDATE)
  - `services/engagement-service/repository/polls.go:164-166` (ClosePoll UPDATE)
- **Root cause:** the owner handlers call `requireOwnedOverlay(:id)` and get an authorized `overlayID`, but then resolve the target round from an **independent** path segment (`:pid` / `pollId`) and pass **only that id** to the repo. The repo UPDATEs are keyed `WHERE id = $1 AND state = … AND source = 'allchat'` with **no `overlay_id` predicate**. The authorized `overlayID` is used only for `clearActive` flag housekeeping, never to authorize the target. So ownership of overlay A passes while the mutation lands on a round belonging to overlay B. Target UUIDs are non-secret (returned by public `active-poll`/`active-prediction` and broadcast over WS).
- **Impact:** any authenticated streamer (trivially self-provisioned via `POST /overlays`) can force-resolve a stranger's live prediction on an attacker-chosen winner (moving real wagered viewer points), mass-refund via cancel, or grief with an early lock/close.
- **Fix:** thread the owned `overlayID` into every guarded UPDATE and add `AND overlay_id = $N`. A mismatched target then affects 0 rows → repo returns `ErrNotFound` → `statusForRepoErr` maps to 404. Both tables have `overlay_id UUID NOT NULL` (migrations 068/069), so this is one predicate per query plus threading the arg through the four repo methods (`LockPrediction`, `ResolvePrediction`, `CancelPrediction`, `ClosePoll`).
- **Test:** regression test — a second owner cannot lock/resolve/cancel/close the first owner's round (expect 404, and the round unchanged).

---

## P1 — HIGH (fix before merge; H1 shares B1's root cause)

### H1. Cross-economy points inflation: `WebWager` debits the path overlay but pays out the prediction's real overlay
- **Category:** security · **Confidence:** high (verified firsthand)
- **Where:** `services/engagement-service/handler/prediction.go:219` (Wager call); `services/engagement-service/repository/predictions.go:188` (SELECT source,state FOR UPDATE), `:222` (applyLedger debit against caller-supplied `overlayID`), `:289`/`:306` (payout credits the prediction's real overlay). Farmable balance via `services/engagement-service/handler/points.go:151` (Heartbeat accepts an arbitrary body `overlay_id`).
- **Root cause:** `WebWager` takes both `overlayID` (from `:id`) and `pid` (from path) as independent client-controlled values. `Wager` debits the caller-supplied `overlayID` but only `SELECT … WHERE id=$1 FOR UPDATE`s the prediction — it never verifies `pid` belongs to `overlayID`. Resolution later credits the prediction's **real** overlay. Debit economy ≠ credit economy → violates ADR-0028's per-`(viewer, overlay)` scoping.
- **Impact:** farm cheap/meaningless points in overlay A (Heartbeat with arbitrary `overlay_id`), then wager on a lucrative prediction in overlay B while naming A in the path → stake drained from A, stake+winnings minted into B's economy from a worthless source.
- **Fix:** derive the debit overlay from the prediction row **inside `Wager`** (`SELECT overlay_id … FOR UPDATE`) instead of trusting the path arg — or reject when `pid.overlay_id != overlayID`. (The finder's suggested WebVote/poll check is **not** needed: polls never touch `viewer_points`, ADR-0028 L36.)
- **Test:** wager on a prediction while naming a different overlay in the path → rejected (or debits/credits the same, correct economy). Covers the same binding gap as B1 — share one regression test module.

### H2. Chat votes/wagers silently dropped: unconditional `XAck` after a swallowed `handle()` error
- **Category:** correctness · **Confidence:** high
- **Where:** `services/engagement-service/consumer/command.go:84-89` (Run acks unconditionally), `:94-155` (handle logs+continues, returns nothing).
- **Root cause:** `handle()` logs-and-`continue`s on every error (`OverlaysForChannel`, `GetOrCreateViewerByPlatform`, `GetActivePoll`, `RecordVote`, `Wager`) and returns nothing; `Run()` then `XAck`s the message regardless of whether the write persisted. A transient Postgres error/timeout removes the entry from the PEL forever — no redelivery. (No balance corruption: `Wager` is one atomic tx with `defer Rollback`, so a failed write persists nothing — the harm is **lost participation**, which is exactly what ADR-0027 L17 chose a durable stream to prevent.)
- **Fix:** give `handle()` an `error` return. `XAck` only on `nil` for transient (DB) failures — leave them pending for redelivery. Ack the permanently-unprocessable cases (bad JSON, non-command) so they don't wedge. Redelivery is safe: `RecordVote`/`Wager` dedupe via `ON CONFLICT` on `(poll_id, viewer_id)` / `(prediction_id, viewer_id)`.
- **Pairs with H3** (reclaimed entries must actually be retried, not re-acked on transient failure).

### H3. No PEL drain / no `XAutoClaim`: entries held by a crashed pod strand forever
- **Category:** correctness · **Confidence:** high
- **Where:** `services/engagement-service/consumer/command.go:57-92` (Run reads only `>`); `services/engagement-service/cmd/main.go:97-104`. Grep for `XAutoClaim|XClaim|XPending|drainPEL` = 0 hits.
- **Root cause:** the consumer never reclaims idle pending entries, and the consumer name is `engagement-{hostname}-{pid}` (unique per pod), so a restarted/rescheduled pod never reclaims the dead pod's PEL. `message-processor/consumer/stream_consumer.go` already solves this with `drainPEL()`; engagement-service copied the ack-after-handle shape but not the reclaim.
- **Impact:** given documented routine pod churn (kured reboot storms) + ADR-0028's ≥2 replicas + rolling deploys, a pod that dies mid-batch (`Count:64`) orphans up to 64 votes/wagers permanently. Recurring silent loss.
- **Fix:** add a startup + periodic `XAutoClaim` (min-idle ~60s) over `StreamEngagementCommands` **and** `StreamEngagementTwitchNative`, reclaiming stale entries into the live consumer, mirroring `stream_consumer.go`'s `drainPEL`.

### H4. Discoverability: the participation loop has no trigger — round start is silent and no viewer surface teaches voting
- **Category:** usability · **Confidence:** high (this is what determines whether the feature works in the field)
- **Where:**
  - Silent start: `services/engagement-service/consumer/command.go` / handlers — no announce path anywhere in the service (grep-confirmed); `frontend/src/components/overlay/EngagementControls.tsx:238` (`startPoll` has no announce option).
  - No numbers on stream: `frontend/src/app/overlay/[id]/poll/page.tsx:92` renders `{o.label}` with no `{o.idx}`; the vote grammar is 1-based (`!vote N` / bare `N`).
- **Root cause:** chat commands are the ADR-0027 universal baseline, but (a) nothing posts to chat when a round opens, (b) the OBS widget never prints option numbers, and (c) the `!vote`/`!predict` syntax lives only on owner-only surfaces. A viewer is told "type 2" and shown "Pizza / Sushi / Tacos" with no way to know which is 2. This is the exact "empty polls, dead feature" outcome ADR-0027 L9 exists to prevent.
- **Fix (two parts):**
  1. **Number the OBS options** — prefix each option in `poll/page.tsx` (and the monitor `TallyBar`) with `{o.idx}.` The field is already in the payload. Cheap, high-leverage. Also apply to the prediction widget for parity.
  2. **Opt-in "announce round start in chat"** — add a toggle to the earn config that, on poll/prediction start, posts the question + numbered options + participate URL to chat, reusing the moderation send-scope ADR-0027 L38 already anticipates ("a future opt-in 'confirm in chat' can reuse the moderation send-scope check").
- **Note:** part 1 is small and should land regardless. Part 2 is the larger piece and unblocks the "steer wagering to the UI" recommendation (the announce carries the participate link).

---

## P2 — MEDIUM (mergeable with follow-up; group into a UX/a11y/correctness polish pass)

### Correctness

- **M-C1. Chat wager dropped on all but the first overlay when a channel feeds multiple overlays.**
  `repository/predictions.go:211-217`, `migrations/069_engagement_predictions.sql:68-69`, `consumer/command.go:121-152` (and the parallel `poll_votes` path: `migrations/068_engagement_polls.sql:50`, `RecordVote`).
  The consumer calls `Wager`/`RecordVote` once per overlay with the **same** `source_message_id`, but the replay-dedup unique index on `source_message_id` is **global** (partial `WHERE source_message_id IS NOT NULL`), while the `ON CONFLICT` only handles `(prediction_id, viewer_id)`. The 2nd overlay's insert hits the global index → `unique_violation` (not translated to a `WagerResult`) → the wager silently rolls back for that overlay. ADR-0027 explicitly supports one channel → many overlays. No minting (clean rollback), so this is a fairness/participation bug.
  **Fix:** scope the replay-dedup index to the round — `UNIQUE(prediction_id, source_message_id)` and `UNIQUE(poll_id, source_message_id)` (new migration) — or include `source_message_id` in the `ON CONFLICT`. Add a test for a channel feeding two overlays with concurrent active rounds.

- **M-C2. Display-precedence invariant not enforced on the WS broadcast path.**
  `consumer/native.go:172,202` + `publisher/publisher.go:53-68` + `services/api-gateway/cmd/main.go:237-240`.
  ADR-0029 L62 says a native round can never shadow a live All-Chat round; but that preference lives only in `GetActiveDisplayPoll/Prediction` (HTTP pull). The native consumer broadcasts the specific native round it upserted directly via `PublishPoll/PublishPrediction` with no source check, and the gateway forwards it verbatim as `poll_update`/`prediction_update`. Both write paths publish to the same `overlay:{id}:poll`/`:prediction` channel → last-writer-wins. **Latent today** (no WS consumer subscribes to these types — see L-D1), but contradicts the ADR and would bite the moment a widget moves to WS.
  **Fix:** after upsert, broadcast `GetActiveDisplay*` (so a live All-Chat round keeps the wire) rather than the specific native round; or drop a native update when an All-Chat round is live for that overlay.

### Usability (UX)

- **M1. Steer wagering to the participate page** (implements the headline recommendation). Chat wagering is blind (balance is JWT-only — `handler/points.go:33-50`), one-shot/irreversible (`predictions.go:211-220`, no cancel/undo, vs. poll votes which honor `allow_change`), and its syntax/indices are undiscoverable (`EngagementControls.tsx:637-639`). **Do not remove `cmdWager`** — instead change the owner hint at `EngagementControls.tsx:637` to lead with the participate link and treat the chat command as secondary. (Findings: chat-predictions-ux #1–#4.)

- **M2. No win/lose payout feedback on the participate page.** `frontend/src/app/overlay/[id]/participate/page.tsx:59-70,241`. A resolved/canceled prediction stops being returned by the public endpoint, so the section unmounts on the next 3s poll and the balance just silently ticks. **Fix:** retain/fetch the last terminal round (with `winning_outcome_id` + the viewer's prior wager) long enough to show "You won +N / You lost N / Refunded", or at minimum a transient notice when the balance changes due to a resolution. The wager/resolution responses already carry the new balance.

- **M3. Irreversible payout has no confirm, while the reversible cancel/refund has a two-step confirm.** `EngagementControls.tsx:296-313, 547-579`. The stronger guardrail is on the *less* destructive action. **Fix:** add an echoing confirm to Pay-out (`Pay out "<outcome>"? This is final.`) matching/exceeding the cancel confirm's friction.

- **M4. Twitch-mirror opt-in is buried and vanishes when a round is live.** `EngagementControls.tsx:470-476`. The only activation entry point is an 11px `text-dim` underlined link rendered solely in the poll column's create-form empty state — absent from the prediction column, hidden whenever a poll is truthy, and absent from the config-page EngagementPanel. No indicator of whether mirroring is ON. **Fix:** move to a stable, labeled control (config-page EngagementPanel is the natural home) with real affordance and a current-state indicator. *(ADR-0029 L71 already lists this as a follow-up.)*

- **M5. No warning that a late scope grant won't mirror until channel resync.** `EngagementControls.tsx:332-338, 470-476`. The consent redirect returns to `/overlay/[id]/view` with zero messaging; per ADR-0029 L70 mirroring only activates on the next channel (re)sync. Streamer opts in, sees nothing, assumes it's broken. **Fix:** post-consent notice ("Twitch mirroring enabled — native rounds will appear after the next channel sync") and/or expose actual mirror state.

- **M6. Earn config advertises earning that never accrues in v1.** `frontend/src/app/overlays/[id]/page.tsx:679-695, 822-827`; `services/engagement-service/engine/earn.go` (nothing publishes `engagement:chat`, so `chat_per_minute` is fully inert). **Fix:** disable/flag `chat_per_minute` with a "coming soon" note, clarify `watch_per_minute` counts participation-page time (not stream-watch time), and align the intro copy that currently promises "chatting" as an earn mechanism.

- **M7. Bare-number reaction collision.** `consumer/command.go:199-205`. Any whole-message integer 1–99 is a silent vote **while an active All-Chat poll exists** (bounded: predictions and mirrored Twitch polls don't collide). Real chat reactions ("2", "10") get counted, and with `allow_change` default-on a re-reaction silently re-votes. **Fix (accept + disclose):** document it in the create form ("bare numbers count as votes while a poll is live") and consider a per-poll `!vote`-only mode for streamers who care about reaction noise.

### Accessibility (WCAG 2.2 AA)

- **M-A1. Participate page `dark:`-gated styles on a fixed near-black body.** `participate/page.tsx:199,224,226,260,262`. Tailwind v4 with no `@custom-variant dark` / no `.dark` class → on OS-light devices the `dark:` branch never activates while `globals.css` forces a near-black background. Sharpest symptom: the number input (`:260`, `dark:bg-transparent` with no light bg) renders as a light box on black; the balance pill (`bg-black/10`, `:199`) disappears. **Fix:** scope a `dark` class/wrapper to this route (like the existing `.overlay-view` / `#legal-wrapper` patterns) or drop the `dark:` pairing and use the unconditionally-dark design tokens (`bg-surface`, `border-border`, `text-text`).

- **M-A2. No live regions anywhere (SC 4.1.3, plus 3.3.1 for errors).** `participate/page.tsx:204` (the `notice` banner is a plain `<p>`); tally/balance/state changes swap silently on the polling loop. (Monitor mutation feedback already goes through react-hot-toast — only its polled tallies are unannounced.) **Fix:** wrap `notice` in `role="alert"`; give the balance chip + tally sections `aria-live="polite"`; announce prediction state transitions.

- **M-A3. Detail numbers on translucent tally bars fall to ~2.2–2.7:1 (SC 1.4.3).** `EngagementControls.tsx:142`, `participate/page.tsx:232,283`. Worst exactly on the leading option (right-aligned number over the widest fill). **Fix:** use `text-text` for the detail number over the fill, or darken/reduce the bar opacity. The participate `text-slate-500` is novel to this PR — switch to design tokens.

- **M-A4. `text-[11px] text-text-dim` instructional captions ~1.76:1 (SC 1.4.3).** `EngagementControls.tsx:387,414-417,463-465,530,595-602,637-639` — these carry the `!vote`/`!predict` syntax and the mirror explanation. Owner-facing + a pre-existing token, so lower priority, but below floor for load-bearing text. **Fix:** promote to `text-text-sub` (≈4.65:1) or raise the size.

---

## P3 — LOW / NIT (backlog; do not gate merge)

- **L-U1.** Chat wager rejection `Reason` discarded even server-side on the `!res.Accepted` path — zero observability into *why* chat wagers are rejected. `consumer/command.go:148`. Add a debug/info log. *(Cheapest genuine gap on the chat path; not a design tradeoff.)*
- **L-U2.** Wager rejection `reason` discarded client-side → all failures collapse to "wager not accepted". `frontend/src/lib/api/viewer.ts:71-77`, `participate/page.tsx:154-155`; server returns it at `handler/prediction.go:226`. Map `reason` → human copy.
- **L-U3.** No client-side balance guard though `engagement.balance` is available; balance far from the wager input on tall layouts. `participate/page.tsx:139-161,199-201,251-263`. Pre-validate `amount<=balance` + show balance near the input / add "max".
- **L-U4.** Transposed `!predict 500 1` parses as idx=500/amount=1 and is silently dropped as `bad_outcome`. `consumer/command.go:187-194`. Intrinsic to positional chat grammar — another reason for M1. A `bad_outcome` chat reply (if the announce/send-scope lands) would catch it.
- **L-U5.** One bare "2" votes on every overlay's active poll simultaneously with different meanings (multi-overlay). `consumer/command.go:110-137`. Per-ADR; document as a known multi-overlay constraint.
- **L-U6.** Vote acceptance/rejection fully silent on chat (low stakes; fine once H4's numbers land). `repository/polls.go:177-222`.
- **L-U7.** Resolve-while-ACTIVE no-op toast ("Lock the prediction before resolving") is effectively dead code — the Resolve button only renders when LOCKED. `EngagementControls.tsx:296-313,547-557`. Soften the toast to acknowledge the race ("Prediction is no longer locked — refresh and try again").
- **L-U8.** A created round can't be edited — a question typo forces close/cancel + re-create (costly for a prediction with wagers). `EngagementControls.tsx:374-419,495-604`. Consider in-place edit of the human-readable title only; at minimum document "proofread before Start".
- **L-U9.** Participate URL reachable only by manual paste of a long UUID URL — no QR, no short/branded link, not auto-surfaced. `overlays/[id]/page.tsx:783-798`. Add a QR + short link so streamers can put it on-screen; include it in H4's announce.
- **L-D1.** Engagement real-time WS path fully wired server-side but unused — all four surfaces HTTP-poll (2–3s lag) while the gateway fans out dropped `poll_update`/`prediction_update` frames. `frontend/src/lib/api/websocket.ts:147-168`, `poll/page.tsx:33,42`, `subscription/subscriber.go:206-208,484-486`. Either consume the WS types (keep HTTP as fallback) or stop publishing them. (Interacts with M-C2.)
- **L-U10.** Expired-token vote/wager shows raw "Unauthorized" instead of bouncing to login. `participate/page.tsx:130-134,154-158`. Detect the Unauthorized error in the catch and `setAuthed(false)`.
- **L-Docs1.** OBS widget links have no browser-source setup guidance (size, transparency, "appears only while a round is live"); no end-user docs/FAQ for the feature. `overlays/[id]/page.tsx:772-788,865-884`.
- **L-A1.** OBS prediction winner marked by ring + `aria-hidden` 👑 only — no text/accessible name (SC 1.1.1/4.1.2, not 1.4.1 since the crown is visible). `prediction/page.tsx:84,88,96`. Add a "Winner" pill or accessible name.
- **L-A2.** Disabled participate buttons drop to `opacity-60` with the reason not programmatically associated (no title/aria); `busy` state unexplained. `participate/page.tsx:221-226,258-262,272-277`. Mirror the monitor Resolve button's `title` pattern.
- **L-A3.** Meaningful/decorative emoji without accessible handling (📊🔮🔒🔥 decorative → `aria-hidden`; monitor 🏆 at `EngagementControls.tsx:523` is the sole winner marker → needs a real accessible name). `participate/page.tsx:200,208,244,245`.
- **L-C1.** Prediction monotonic guard collapses RESOLVED and CANCELED into one rank (`native.go:126-127`, `ELSE 3`), so a redelivered opposite-terminal event can laterally flip a terminal round (display-only; no points move). Make terminal states absorbing (`AND predictions.state NOT IN ('RESOLVED','CANCELED')`) or rank them distinctly.
- **L-C2.** `OverlaysForChannel` uses case-sensitive `channel_id` equality while the codebase elsewhere case-folds Twitch logins (`repository.go:100-103`). A legacy non-lowercase Twitch source row is silently never mirrored. Use `LOWER(channel_id) = $2` (+ functional index). Producer that lowercases: `services/twitch-eventsub-listener/webhooks/engagement.go:63,101`.
- **L-C3.** No panic `recover()` on the consume loop (`consumer/command.go:57-92`, `cmd/main.go:102-104`). A poison message crash-loops the pod (Go propagates the panic to the whole process → k8s restarts it — so it's a visible crash-loop, **not** a silent freeze). Add a per-message `recover()` that logs + skips; optionally expose consumer liveness as a metric.
- **L-C4.** Active-flag refcount can leak a stale SET member if an overlay's source channels change between open and close (`handler/poll.go:181-200`, `publisher/publisher.go:87-108`, `repository.go:121-138`). `ClearActive` re-derives channels at call time, so it can `SREM` the wrong keys. Self-heals within the 24h `activeTTL` except in the multi-overlay/re-add case. Persist the exact channel set flagged at open time.
- **L-C5.** `EngagementControls` leaks a finished round across in-tab overlay switches — state never reset on `overlayId` change, mounted without `key`. `EngagementControls.tsx:199-204,208,217`; `view/page.tsx:603`. Add `key={id}` at the mount site (or reset state in an effect on `overlayId`).
- **L-Sec1.** Viewer rate limiting falls back to IP, not `viewer_id` (`shared/ratelimit/ratelimit.go:131-143`, `engagement-service/cmd/main.go:143,157-161`). Viewers behind one NAT share a bucket; per-viewer flooding only IP-bounded. Integrity is safe (dedup keys); availability/fairness only. Key on `viewer_id` when present.
- **L-Perf1.** `EXISTS`+`XADD` run synchronously on the message-processor consume loop for every command-shaped message during a live poll (`message-processor/consumer/engagement.go:40-78`, `stream_consumer.go:335`). Not on the fan-out path (post-publish) and ordinary chat is gated out, but "rare hit" understates a live poll. Consider fire-and-forget on a bounded worker + a forward-rate/latency metric.
- **N1.** `loadPublic`/`loadPrivate` refresh independently → brief "is this mine?" highlight flicker each tick. `participate/page.tsx:81-95`. Merge both results into a single combined state update (note: `Promise.all` alone does **not** batch the two `setState`s).
- **N2.** No signpost for TikTok/Discord viewers on the login screen (they participate via chat, but see only Twitch/YouTube/Kick buttons). `participate/page.tsx:38-42,167-185`. Add a one-line hint.
- **N3.** LOCKED-prediction winner radios lack an explicit `focus-visible` ring (rely on UA default on a dark surface; borderline SC 2.4.7). `EngagementControls.tsx:514-521`. Add the same focus ring as sibling controls; keep the correct `radiogroup` semantics.

---

## Suggested execution order

1. **B1 + H1 together** (shared root cause: target round never bound to the owning overlay) with one shared regression-test module. — merge gate
2. **H2 + H3 together** (`handle()` error return + `XAutoClaim`/PEL drain; reclaimed entries must be retried, not re-acked). — merge gate
3. **H4 part 1** (number the OBS options) — small, land before GA. **H4 part 2** (opt-in chat announce) — larger, before GA.
4. **P2 pass** — decide M1's "UI-first wagering" posture first (it's a product decision), then the correctness (M-C1, M-C2), UX (M2–M7), and a11y (M-A1–M-A4) items.
5. **P3** — backlog, address opportunistically.

Findings that are **accepted/documented tradeoffs** (not defects) and should be closed as "won't fix / by design" unless the product view changes: chat feedback silence (ADR-0027 L18/L38), one-shot wagers (inherent to a balance debit + payout idempotency), balance not readable over chat (ADR-0028 L43 pull-first). These motivate M1 (steer to UI) rather than code changes on the chat path.
