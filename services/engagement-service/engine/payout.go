// This file is part of All-Chat.
// Copyright (C) 2026 caesarakalaeii
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

// Package engine holds the engagement service's pure business rules: the
// prediction payout split and the event→points earning map. Keeping them free of
// I/O makes the tricky arithmetic (conservation, rounding) unit-testable.
package engine

import (
	"math/big"
	"sort"

	"github.com/google/uuid"
)

// maxPool caps the accumulated stake pool so a pathological set of wagers can't overflow
// int64 before the big.Int proportional split. Mirrors engine/earn.go's maxEventPoints:
// far above any realistic pool yet well clear of math.MaxInt64, preserving the
// sum(Credits)==sum(stakes) conservation guarantee.
const maxPool int64 = 1_000_000_000_000_000 // 1e15

// WagerEntry is one viewer's stake on one outcome. At most one per viewer per
// prediction (enforced by the DB primary key), so ViewerID is unique across the set.
type WagerEntry struct {
	ViewerID  uuid.UUID
	OutcomeID uuid.UUID
	Amount    int64
}

// PayoutResult maps each viewer to the total points they should be CREDITED on
// resolution. Refund is true when nobody wagered the winning outcome and every
// entry is simply refunded its stake.
type PayoutResult struct {
	Credits map[uuid.UUID]int64
	Refund  bool
}

// ComputePayouts resolves a prediction: winners get their stake back plus a
// proportional share of the losers' pool. Shares are floored; the integer
// remainder is assigned to the largest-stake winner (tie-break: lowest viewer
// UUID) so the result is deterministic and conserves points exactly —
// sum(Credits) == sum(all stakes). big.Int is used for the proportional multiply
// so a large pool can't overflow int64.
//
// Edge cases:
//   - winnersPool == 0 (nobody picked the winner): refund every stake (Refund=true).
//   - losersPool == 0 (one-sided / everyone won): each winner just gets their stake back.
func ComputePayouts(entries []WagerEntry, winning uuid.UUID) PayoutResult {
	credits := make(map[uuid.UUID]int64, len(entries))
	var total, winnersPool int64
	winners := make([]WagerEntry, 0, len(entries))
	for _, e := range entries {
		total = addClamp(total, e.Amount)
		if e.OutcomeID == winning {
			winnersPool = addClamp(winnersPool, e.Amount)
			winners = append(winners, e)
		}
	}

	// No winners: refund all stakes.
	if winnersPool == 0 {
		for _, e := range entries {
			credits[e.ViewerID] = e.Amount
		}
		return PayoutResult{Credits: credits, Refund: true}
	}

	losersPool := total - winnersPool

	// Deterministic winner order for remainder assignment.
	sort.Slice(winners, func(i, j int) bool {
		if winners[i].Amount != winners[j].Amount {
			return winners[i].Amount > winners[j].Amount // largest stake first
		}
		return winners[i].ViewerID.String() < winners[j].ViewerID.String()
	})

	var sumFloor int64
	lp := big.NewInt(losersPool)
	wp := big.NewInt(winnersPool)
	for _, e := range winners {
		// share = floor(losersPool * stake / winnersPool)
		share := new(big.Int).Mul(lp, big.NewInt(e.Amount))
		share.Div(share, wp)
		s := share.Int64()
		credits[e.ViewerID] = e.Amount + s
		sumFloor += s
	}

	// Assign the rounding remainder to the largest-stake winner (winners[0]).
	if remainder := losersPool - sumFloor; remainder > 0 {
		credits[winners[0].ViewerID] += remainder
	}
	return PayoutResult{Credits: credits, Refund: false}
}

// addClamp adds a non-negative stake to a running pool, clamping at maxPool so int64 can't
// wrap. Amounts are non-negative (the write path rejects amount<=0). Clamping (not erroring)
// keeps ComputePayouts' signature; the big.Int split still conserves the clamped pool.
func addClamp(pool, amount int64) int64 {
	if amount <= 0 {
		return pool
	}
	if pool > maxPool-amount {
		return maxPool
	}
	return pool + amount
}
