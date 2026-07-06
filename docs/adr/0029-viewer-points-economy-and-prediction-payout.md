# ADR-0029: Viewer Points Economy & Prediction Payout Model

**Date**: 2026-07-01
**Status**: Accepted
**Deciders**: caesarakalaeii

## Context and Problem Statement

Issue #523 introduces an All-Chat **viewer points** currency and **predictions** where viewers wager those points. This is All-Chat's first stateful virtual economy, so its integrity model must be pinned down: what the currency is scoped to, how balances stay correct under concurrency/retries/multi-replica, how prediction payouts are computed, and how All-Chat points relate to Twitch's own Channel Points on mirrored Twitch-native predictions.

## Decision Drivers

- **No minting or burning.** A payout must conserve points exactly; a retry or double-click must never credit twice or debit twice.
- **Concurrency & multi-replica.** engagement-service runs ≥2 replicas; earning arrives via Pub/Sub fan-out (every replica sees it); wagers race the lock/resolve transitions.
- **Auditability.** Every balance change must be reconstructable.
- **Don't fight platform currencies.** All-Chat cannot move a viewer's Twitch Channel Points; a mirrored Twitch prediction must not create a second, conflicting economy.
- **Reuse existing identity/premium machinery** (`viewers`, `viewer_platform_identities`, ADR-0019).

## Decision Outcome

### Points are a per-overlay (per-channel) economy
Balance is keyed `(viewer_id, overlay_id)`. Rationale: earning rules, polls, and predictions are all owned by one streamer's overlay; a global wallet would let one streamer's payouts inflate another's economy and complicate multipliers. A global view, if ever wanted, is a cheap aggregation over the per-overlay ledger — the reverse is not.

### Ledger of record + materialized balance
`points_transactions` is an append-only ledger; `viewer_points.balance` is a materialized cache updated in the **same transaction** as each ledger row.
- **Idempotency** is a `UNIQUE` `dedup_key` on every transaction. Earning uses `earn:{overlay}:{message_id}:{reason}` (stable across the Pub/Sub fan-out, so N replicas credit once); wager uses `wager:{prediction}:{viewer}`; payout/refund use `payout|refund:{prediction}:{viewer}`. Insert is `ON CONFLICT (dedup_key) DO NOTHING`; the balance moves only when a row was actually inserted.
- **Debits are guarded**: `UPDATE viewer_points SET balance = balance - amt WHERE balance >= amt` inside the wager transaction; 0 rows ⇒ insufficient ⇒ rollback (which also undoes the ledger insert). `CHECK (balance >= 0)` is the backstop.

### Prediction integrity
State machine `CREATED → ACTIVE → LOCKED → (RESOLVED | CANCELED)`, all transitions guarded (`UPDATE ... WHERE state = ?`, 0 rows ⇒ lost the race, no-op). A wager `SELECT ... FOR UPDATE`s the prediction row, so a wager in flight as the streamer/auto-lock locks either commits before the lock (included) or sees `LOCKED` and rejects with the balance untouched. Auto-lock is a restart-safe periodic sweep (`WHERE auto_lock_at <= NOW()`), never an in-memory timer.

### Payout = stake back + proportional split of the losers' pool
Winners receive their stake plus `floor(losersPool * stake / winnersPool)`; the integer remainder goes to the largest-stake winner (tie-break: lowest viewer UUID). This conserves points exactly (`sum(credits) == sum(stakes)`), is deterministic, and is computed with `math/big` so a large pool can't overflow. Edge cases: **no winners** ⇒ refund every stake; **one-sided** ⇒ each winner just gets their stake back. Resolve is idempotent via the guarded transition + per-viewer `payout:` dedup keys. (Unit-tested in `engine/payout_test.go`.)

### Twitch-native predictions mirror state only — they do NOT use All-Chat points
A `source = 'twitch_native'` prediction (mirrored from EventSub, ADR-0028 companion work) uses **Twitch Channel Points**, which All-Chat cannot debit or credit. engagement-service therefore records its state/tallies for unified display but **never** touches `viewer_points` for it. All-Chat points wagering applies only to `source = 'allchat'` predictions. Twitch-native *polls* may still award All-Chat participation points (no wager is involved). This is the only coherent semantic — anything else would either double-charge viewers or claim to move a currency we don't control.

## Considered Alternatives

- **Balance as the source of truth (no ledger):** rejected — no audit trail, and idempotency/rebuild become impossible.
- **Global (cross-overlay) wallet:** rejected — cross-economy inflation and multiplier ambiguity; recoverable later as an aggregation if desired.
- **All-Chat points wagered on Twitch-native predictions:** rejected — creates a second economy competing with Twitch Channel Points on the same prediction; confusing and unfair (native voters wager Channel Points, all-chat voters wager points).
- **Per-viewer WebSocket push of balance:** deferred. The gateway is broadcast-per-overlay; pushing balance there would leak every viewer's balance. v1 delivers private balance **pull-first** (`GET /viewers/me/engagement`). A future `viewer:{viewer_id}` channel + per-viewer delivery would warrant its own ADR.

## Consequences

- New tables (migrations 067–069): `viewer_points`, `points_transactions`, `points_earn_config`, `polls`/`poll_options`/`poll_votes`, `predictions`/`prediction_outcomes`/`prediction_entries`.
- The economy is safe under retries, concurrency, and multi-replica fan-out by construction (dedup keys + guarded transitions + conditional debits).
- Points name is streamer-configurable (`points_earn_config.points_name`); no brand currency name is hard-coded.
- Chat/watch passive earning is partial in v1 (watch via authenticated heartbeat; event-driven earning for Twitch/YouTube; Kick/TikTok event earning is limited by their listeners' current event support).
