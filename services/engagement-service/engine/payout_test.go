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
