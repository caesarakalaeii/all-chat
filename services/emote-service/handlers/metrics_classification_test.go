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

package handlers

import (
	"errors"
	"fmt"
	"testing"

	"github.com/caesar/all-chat/services/emote-service/clients"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// TestRecordAPIResult_Classification verifies the emote_api_calls_total metric separates
// a benign "not_found" miss (channel has no emotes on this provider — the norm for
// BTTV/FFZ and unset 7TV channels) from a real "error". Conflating them inflated the
// error rate and made a healthy service look like it was failing during the 2026-07-17
// investigation.
func TestRecordAPIResult_Classification(t *testing.T) {
	vec := prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "test_emote_api_calls_total"},
		[]string{"service", "provider", "result"},
	)
	h := &EmoteHandler{apiCalls: vec}

	h.recordAPIResult("bttv", fmt.Errorf("bttv: no emotes: %w", clients.ErrNotFound))
	h.recordAPIResult("7tv", errors.New("7tv api returned status 503"))
	h.recordAPIResult("twitch", nil)

	cases := []struct {
		provider, result string
		want             float64
	}{
		{"bttv", "not_found", 1}, // 404 classified as a benign miss
		{"bttv", "error", 0},     // and NOT as an error
		{"7tv", "error", 1},      // a real 5xx stays an error
		{"7tv", "not_found", 0},  //
		{"twitch", "success", 1}, // nil error is success
	}
	for _, tc := range cases {
		got := testutil.ToFloat64(vec.WithLabelValues("emote-service", tc.provider, tc.result))
		if got != tc.want {
			t.Errorf("%s/%s = %v, want %v", tc.provider, tc.result, got, tc.want)
		}
	}
}
