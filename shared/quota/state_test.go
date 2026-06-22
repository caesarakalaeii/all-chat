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

package quota

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCalculateState_Boundaries(t *testing.T) {
	th := DefaultThresholds() // 70 / 85 / 95 / 100
	cases := []struct {
		pct  float64
		want QuotaState
	}{
		{0, QuotaStateHealthy},
		{69.9, QuotaStateHealthy},
		{70, QuotaStateDegraded},
		{84.9, QuotaStateDegraded},
		{85, QuotaStateCritical},
		{94.9, QuotaStateCritical},
		{95, QuotaStateExhausted},
		{99.9, QuotaStateExhausted},
		{100, QuotaStateDepleted},
		{150, QuotaStateDepleted},
	}
	for _, c := range cases {
		assert.Equalf(t, c.want, CalculateState(c.pct, th), "pct=%.1f", c.pct)
	}
}

func TestSeverity_Mapping(t *testing.T) {
	cases := map[QuotaState]string{
		QuotaStateDepleted:  "critical",
		QuotaStateExhausted: "error",
		QuotaStateCritical:  "error",
		QuotaStateDegraded:  "warning",
		QuotaStateHealthy:   "info",
		QuotaState("WAT"):   "info", // unknown → info
	}
	for state, want := range cases {
		assert.Equalf(t, want, Severity(state), "state=%s", state)
	}
}
