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

package scopeexport

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

const testService = "twitch-eventsub-listener"

func get(t *testing.T, state string) float64 {
	t.Helper()
	return testutil.ToFloat64(backlog.WithLabelValues(testService, state))
}

// setBacklog must publish every known state from the query result.
func TestSetBacklog_PublishesAllStates(t *testing.T) {
	setBacklog(testService, map[string]float64{
		stateScoped:          51,
		stateUnscopedHasCred: 27,
		stateUnscopedNoCred:  6,
	})

	if got := get(t, stateScoped); got != 51 {
		t.Errorf("%s = %v, want 51", stateScoped, got)
	}
	if got := get(t, stateUnscopedHasCred); got != 27 {
		t.Errorf("%s = %v, want 27", stateUnscopedHasCred, got)
	}
	if got := get(t, stateUnscopedNoCred); got != 6 {
		t.Errorf("%s = %v, want 6", stateUnscopedNoCred, got)
	}
}

// A state absent from a later cycle must report 0, not its stale prior value — otherwise the
// backlog would never visibly drain to zero, which is the whole point of the gauge.
func TestSetBacklog_ZeroFillsAbsentStates(t *testing.T) {
	setBacklog(testService, map[string]float64{
		stateScoped:          40,
		stateUnscopedHasCred: 10,
		stateUnscopedNoCred:  3,
	})
	// Next cycle: only scoped channels remain; the unscoped states are gone from the result.
	setBacklog(testService, map[string]float64{stateScoped: 53})

	if got := get(t, stateScoped); got != 53 {
		t.Errorf("%s = %v, want 53", stateScoped, got)
	}
	if got := get(t, stateUnscopedHasCred); got != 0 {
		t.Errorf("%s = %v, want 0 (must zero-fill, not go stale)", stateUnscopedHasCred, got)
	}
	if got := get(t, stateUnscopedNoCred); got != 0 {
		t.Errorf("%s = %v, want 0 (must zero-fill, not go stale)", stateUnscopedNoCred, got)
	}
}
