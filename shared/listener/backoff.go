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

package listener

import (
	"math/rand"
	"time"
)

// JitteredBackoff returns a randomized duration using the full jitter algorithm.
// Formula: random_between(0, min(cap, base * 2^attempt))
// Base: 1s, Cap: 30s.
//
// This prevents thundering herd when many goroutines retry simultaneously.
// Use attempt=0 for the first retry, incrementing on each subsequent failure.
func JitteredBackoff(attempt int) time.Duration {
	const base = time.Second
	const cap = 30 * time.Second

	exp := base
	for i := 0; i < attempt; i++ {
		exp *= 2
		if exp > cap {
			exp = cap
			break
		}
	}

	if exp <= 0 {
		return 0
	}

	return time.Duration(rand.Int63n(int64(exp)))
}
