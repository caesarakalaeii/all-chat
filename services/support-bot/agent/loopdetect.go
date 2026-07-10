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

package agent

import "strings"

// loopDetector blocks a tool call that has been repeated identically too many times
// within a sliding window, so a wedged model cannot burn the whole iteration budget
// re-issuing the same call.
type loopDetector struct {
	window    int
	threshold int
	recent    []string
}

func newLoopDetector(window, threshold int) *loopDetector {
	if window <= 0 {
		window = 30
	}
	if threshold <= 0 {
		threshold = 5
	}
	return &loopDetector{window: window, threshold: threshold}
}

// key builds a canonical identity for a call. Argument whitespace is normalized so
// cosmetically different but semantically identical arg strings collide. (Full
// key-order canonicalization is intentionally avoided to steer clear of untyped
// decoding; models emit stable key order in practice.)
func (d *loopDetector) key(name, args string) string {
	return name + "\x00" + strings.Join(strings.Fields(args), " ")
}

// count returns how many times k appears in the current window.
func (d *loopDetector) count(k string) int {
	n := 0
	for _, r := range d.recent {
		if r == k {
			n++
		}
	}
	return n
}

// wouldBlock reports whether recording this call would meet/exceed the repeat
// threshold. It does NOT mutate state — record only after the call is allowed.
func (d *loopDetector) wouldBlock(name, args string) bool {
	return d.count(d.key(name, args)) >= d.threshold
}

// record appends a call to the window, evicting the oldest entry past the cap.
func (d *loopDetector) record(name, args string) {
	d.recent = append(d.recent, d.key(name, args))
	if len(d.recent) > d.window {
		d.recent = d.recent[len(d.recent)-d.window:]
	}
}
