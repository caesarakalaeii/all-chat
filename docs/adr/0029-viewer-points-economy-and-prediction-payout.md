# ADR-0029: Viewer Points Economy & Prediction Payout Model

**Date**: 2026-07-01
**Status**: Accepted
**Revised**: 2026-11-03 (brought in line with the current implementation)
**Deciders**: caesarakalaeii

## Context

Issue #523 introduces viewer points and predictions where viewers wager them. This is All-Chat's first stateful virtual economy, so the integrity model had to be pinned down up front: scoping of balances, correctness under concurrency/retries/multi-replica fan-out (engagement-service runs ≥2 replicas), payout math that conserves points exactly, and the relationship to Twitch Channel Points on mirrored native predictions (ADR-0030).

## Decision

**Points are a per-overlay economy.** Balance is keyed `(viewer_id, overlay_id)`: earning rules, polls and predictions are all owned by one streamer's overlay, and a global wallet would let one streamer's payouts inflate another's economy. A global view, if ever wanted, is an aggregation over the per-overlay ledger.

**Ledger of record + materialized balance.** `points_transactions` is append-only; `viewer_points.balance` is updated in the same transaction as each ledger row. Every transaction carries a `UNIQUE` `dedup_key` with `ON CONFLICT DO NOTHING`, and the balance moves only when a row is actually inserted:

- earn: `earn:{overlay}:{message_id}:{reason}` — stable across Pub/Sub fan-out, so N replicas credit once;
- chat wager: `wager:overlay:{overlay}:{stream_entry_id}` — the Redis stream entry id, stable across redelivery/reclaim, so a redelivered message cannot debit twice even if a new round has since opened; the overlay in the key lets one message fan out to N overlays;
- web/extension wager: `wager:{prediction}:{viewer}`; payouts/refunds: `payout|refund:{prediction}:{viewer}`.

**Debits are guarded**: `UPDATE viewer_points SET balance = balance - amt WHERE balance >= amt` inside the wager transaction; 0 rows means insufficient and rolls back (also undoing the ledger insert). `CHECK (balance >= 0)` is the backstop.

**Prediction integrity.** State machine `CREATED → ACTIVE → LOCKED → (RESOLVED | CANCELED)` with guarded transitions (`UPDATE ... WHERE state = ?`; 0 rows means the race was lost and the op is a no-op). A wager `SELECT ... FOR UPDATE`s the prediction row, so a wager in flight at lock either commits before the lock or sees `LOCKED` and rejects untouched. Auto-lock is a restart-safe periodic sweep (`WHERE auto_lock_at <= NOW()`), never an in-memory timer.

**Payout** = stake back + `floor(losersPool * stake / winnersPool)`, remainder to the largest-stake winner (tie-break: lowest viewer UUID). This conserves points exactly, is deterministic, and is computed with `math/big` against overflow. No winners ⇒ refund every stake; one-sided ⇒ winners get their stake back. Resolve is idempotent via the guarded transition plus per-viewer payout dedup keys.

**Twitch-native predictions use Twitch Channel Points, never All-Chat points.** A `source = 'twitch_native'` prediction is mirrored for display only (ADR-0030); engagement-service never touches `viewer_points` for it. All-Chat wagering applies only to `source = 'allchat'` predictions. Poll votes carry no stake, so no points ever move on a poll.

Alternatives considered: balance as the source of truth without a ledger (no audit trail, no idempotency); a global cross-overlay wallet (inflation, multiplier ambiguity); wagering All-Chat points on native predictions (a second economy on the same round); per-viewer WebSocket balance push (would leak balances over a broadcast-per-overlay channel — v1 is pull-first via `GET /viewers/me/engagement`).

## Consequences

- New tables (migrations 068–070): `viewer_points`, `points_transactions`, `points_earn_config`, `polls`/`poll_options`/`poll_votes`, `predictions`/`prediction_outcomes`/`prediction_entries`.
- The economy is safe under retries, concurrency and multi-replica fan-out by construction.
- The points name is streamer-configurable (`points_earn_config.points_name`); no brand currency name is hard-coded.
- Watch earning flows through the two viewer-heartbeat endpoints (`POST /viewers/me/heartbeat`, ADR-0031) bounded by `maxWatchCreditsPerDay`. Event-driven earning is best-effort Pub/Sub on `engagement:events`, currently from Twitch, YouTube and TikTok: Kick's listener publishes only chat and deletions, and chat-participation earning (`engagement:chat`) is not published anywhere yet — the UI flags `chat_per_minute` as coming soon.

## Related

- [ADR-0028](./0028-engagement-chat-command-write-path.md) — the write-path feeding these tables.
- [ADR-0030](./0030-twitch-native-engagement-mirroring.md) — enforces native-point isolation end-to-end.
