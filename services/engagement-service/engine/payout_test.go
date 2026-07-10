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

package engine

import (
	"math"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func mkEntry(viewer, outcome uuid.UUID, amount int64) WagerEntry {
	return WagerEntry{ViewerID: viewer, OutcomeID: outcome, Amount: amount}
}

// sumStakes / sumCredits help assert the core invariant: points are conserved.
func sumStakes(entries []WagerEntry) int64 {
	var s int64
	for _, e := range entries {
		s += e.Amount
	}
	return s
}

func sumCredits(c map[uuid.UUID]int64) int64 {
	var s int64
	for _, v := range c {
		s += v
	}
	return s
}

func TestComputePayouts_SimpleWinnerTakesLosersPool(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	x, y := uuid.New(), uuid.New()
	entries := []WagerEntry{mkEntry(a, x, 100), mkEntry(b, y, 100)}

	res := ComputePayouts(entries, x)

	assert.False(t, res.Refund)
	assert.Equal(t, int64(200), res.Credits[a], "winner gets stake + entire losers' pool")
	assert.Equal(t, int64(0), res.Credits[b], "loser gets nothing")
	assert.Equal(t, sumStakes(entries), sumCredits(res.Credits), "points conserved")
}

func TestComputePayouts_ProportionalSplit(t *testing.T) {
	a, b, c := uuid.New(), uuid.New(), uuid.New()
	x, y := uuid.New(), uuid.New()
	// Winners A(100) + B(50) share losers' pool of 30 proportionally: A+20, B+10.
	entries := []WagerEntry{mkEntry(a, x, 100), mkEntry(b, x, 50), mkEntry(c, y, 30)}

	res := ComputePayouts(entries, x)

	assert.Equal(t, int64(120), res.Credits[a])
	assert.Equal(t, int64(60), res.Credits[b])
	assert.Equal(t, sumStakes(entries), sumCredits(res.Credits), "points conserved")
}

func TestComputePayouts_RemainderToLargestStake(t *testing.T) {
	// Two equal-stake winners split an odd losers' pool → 1 leftover point.
	// Tie-break: the lower-UUID winner receives the remainder.
	a := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	b := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	c := uuid.New()
	x, y := uuid.New(), uuid.New()
	entries := []WagerEntry{mkEntry(a, x, 1), mkEntry(b, x, 1), mkEntry(c, y, 1)}

	res := ComputePayouts(entries, x)

	assert.Equal(t, sumStakes(entries), sumCredits(res.Credits), "no points minted or burned")
	// Both winners get their stake back (1) and one of them the +1 remainder.
	assert.Equal(t, int64(2), res.Credits[a], "lowest-UUID winner takes the remainder")
	assert.Equal(t, int64(1), res.Credits[b])
}

func TestComputePayouts_RemainderToLargestStakeNotLowestUUID(t *testing.T) {
	// Unequal winners: A stakes 3 (HIGHER UUID), B stakes 1 (lower UUID); losers' pool 3.
	// Proportional floors are A=floor(3*3/4)=2, B=floor(3*1/4)=0 → distributed 2, remainder 1.
	// The remainder must go to the LARGEST-STAKE winner (A) even though B has the lower UUID —
	// proving the largest-stake rule dominates the UUID tie-break. (The existing equal-stake
	// remainder test only exercised the tie-break, never "remainder to largest stake".)
	a := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	b := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	c := uuid.New()
	x, y := uuid.New(), uuid.New()
	entries := []WagerEntry{mkEntry(a, x, 3), mkEntry(b, x, 1), mkEntry(c, y, 3)}

	res := ComputePayouts(entries, x)

	assert.Equal(t, sumStakes(entries), sumCredits(res.Credits), "points conserved")
	assert.Equal(t, int64(6), res.Credits[a], "largest-stake winner: stake 3 + share 2 + remainder 1")
	assert.Equal(t, int64(1), res.Credits[b], "smaller-stake winner: stake 1 + floored share 0, no remainder")
}

func TestComputePayouts_NoWinnersRefundsAll(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	x, y, z := uuid.New(), uuid.New(), uuid.New()
	entries := []WagerEntry{mkEntry(a, x, 100), mkEntry(b, y, 50)}

	res := ComputePayouts(entries, z) // nobody wagered outcome z

	assert.True(t, res.Refund)
	assert.Equal(t, int64(100), res.Credits[a])
	assert.Equal(t, int64(50), res.Credits[b])
	assert.Equal(t, sumStakes(entries), sumCredits(res.Credits))
}

func TestComputePayouts_OneSidedReturnsStakes(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	x := uuid.New()
	entries := []WagerEntry{mkEntry(a, x, 100), mkEntry(b, x, 50)} // everyone on the winner

	res := ComputePayouts(entries, x)

	assert.False(t, res.Refund)
	assert.Equal(t, int64(100), res.Credits[a], "no losers' pool → stake back only")
	assert.Equal(t, int64(50), res.Credits[b])
	assert.Equal(t, sumStakes(entries), sumCredits(res.Credits))
}

func TestComputePayouts_Empty(t *testing.T) {
	res := ComputePayouts(nil, uuid.New())
	assert.True(t, res.Refund)
	assert.Empty(t, res.Credits)
}

func TestComputePayouts_ManyWinnersOddLosersPoolConservesAndReclaims(t *testing.T) {
	// 101 equal-stake winners split a single large ODD losers' pool. The floored shares
	// leave a remainder < number-of-winners; it must land on the lowest-UUID winner
	// (equal stakes → the sort's UUID tie-break decides). Expected numbers are derived
	// from the code's floor+remainder rule, not hardcoded.
	x, y := uuid.New(), uuid.New()
	const n = 101
	entries := make([]WagerEntry, 0, n+1)
	winnerIDs := make([]uuid.UUID, 0, n)
	for i := 0; i < n; i++ {
		w := uuid.New()
		winnerIDs = append(winnerIDs, w)
		entries = append(entries, mkEntry(w, x, 1))
	}
	loser := uuid.New()
	const losersStake int64 = 10_007
	entries = append(entries, mkEntry(loser, y, losersStake))

	res := ComputePayouts(entries, x)

	assert.False(t, res.Refund)
	assert.Equal(t, sumStakes(entries), sumCredits(res.Credits), "points conserved")
	assert.Equal(t, int64(0), res.Credits[loser], "loser gets nothing")

	perWinnerFloor := losersStake / int64(n)
	remainder := losersStake - perWinnerFloor*int64(n)
	assert.Less(t, remainder, int64(n), "remainder is strictly less than the winner count")

	// The lowest-UUID winner receives the remainder (equal stakes → UUID tie-break).
	lowest := winnerIDs[0]
	for _, w := range winnerIDs[1:] {
		if w.String() < lowest.String() {
			lowest = w
		}
	}
	for _, w := range winnerIDs {
		want := int64(1) + perWinnerFloor
		if w == lowest {
			want += remainder
		}
		assert.Equal(t, want, res.Credits[w], "winner credit = stake + floored share (+ remainder for lowest UUID)")
	}
}

func TestComputePayouts_NearMaxInt64PoolNoMintOrBurn(t *testing.T) {
	// Two enormous stakes whose raw sum overflows int64. The accumulation clamps at
	// maxPool so nothing wraps: no negative credits, the winner keeps positive points,
	// and only the winner holds points. Guards the addClamp overflow clamp.
	x, y := uuid.New(), uuid.New()
	winner, loser := uuid.New(), uuid.New()
	stake := int64(math.MaxInt64/2 - 1)
	entries := []WagerEntry{mkEntry(winner, x, stake), mkEntry(loser, y, stake)}

	res := ComputePayouts(entries, x)

	assert.False(t, res.Refund)
	for id, c := range res.Credits {
		assert.GreaterOrEqual(t, c, int64(0), "no credit wrapped negative: %s", id)
	}
	assert.Greater(t, res.Credits[winner], int64(0), "winner holds positive points")
	assert.Equal(t, int64(0), res.Credits[loser], "loser gets nothing")
	assert.Equal(t, res.Credits[winner], sumCredits(res.Credits), "only the winner holds points")
}
