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

// Package moderation ports the cross-channel-spam detector: it fingerprints a message
// by its normalized text plus the sorted byte sizes of its attachments (so a re-posted
// scam image is matched even though Discord re-issues a distinct CDN URL each upload),
// and reports when a user posts the same fingerprint across enough distinct channels
// within a window — a strong signal of a compromised account.
package moderation

import (
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Options configures the detector. Zero values fall back to the defaults (3 channels,
// 10-minute window, wall clock).
type Options struct {
	ChannelThreshold int
	Window           time.Duration
	Now              func() time.Time
}

type entry struct {
	channelID string
	ts        time.Time
}

// SpamDetector tracks per-user message fingerprints across channels. Safe for
// concurrent use.
type SpamDetector struct {
	channelThreshold int
	window           time.Duration
	now              func() time.Time

	mu           sync.Mutex
	userMessages map[string]map[string][]entry
	actioned     map[string]struct{}
}

// NewSpamDetector builds a detector.
func NewSpamDetector(opts Options) *SpamDetector {
	threshold := opts.ChannelThreshold
	if threshold <= 0 {
		threshold = 3
	}
	window := opts.Window
	if window <= 0 {
		window = 10 * time.Minute
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	return &SpamDetector{
		channelThreshold: threshold,
		window:           window,
		now:              now,
		userMessages:     make(map[string]map[string][]entry),
		actioned:         make(map[string]struct{}),
	}
}

// Record notes a message and returns true iff this call pushes the user past the
// cross-channel threshold (and the user has not already been actioned).
func (d *SpamDetector) Record(userID, channelID, content string, attachmentSizes []int) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	if _, done := d.actioned[userID]; done {
		return false
	}

	text := strings.ToLower(strings.TrimSpace(content))
	sizesKey := sizeKey(attachmentSizes)
	if text == "" && sizesKey == "" {
		return false
	}
	fingerprint := text + " " + sizesKey
	cutoff := d.now().Add(-d.window)

	userMap := d.userMessages[userID]
	if userMap == nil {
		userMap = make(map[string][]entry)
		d.userMessages[userID] = userMap
	}

	fresh := userMap[fingerprint][:0:0]
	for _, e := range userMap[fingerprint] {
		if !e.ts.Before(cutoff) {
			fresh = append(fresh, e)
		}
	}
	fresh = append(fresh, entry{channelID: channelID, ts: d.now()})
	userMap[fingerprint] = fresh

	distinct := make(map[string]struct{}, len(fresh))
	for _, e := range fresh {
		distinct[e.channelID] = struct{}{}
	}
	if len(distinct) >= d.channelThreshold {
		d.actioned[userID] = struct{}{}
		delete(d.userMessages, userID)
		return true
	}
	return false
}

// HasActioned reports whether the user has already been actioned.
func (d *SpamDetector) HasActioned(userID string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	_, ok := d.actioned[userID]
	return ok
}

func sizeKey(sizes []int) string {
	if len(sizes) == 0 {
		return ""
	}
	cp := append([]int(nil), sizes...)
	sort.Ints(cp)
	parts := make([]string, len(cp))
	for i, s := range cp {
		parts[i] = strconv.Itoa(s)
	}
	return strings.Join(parts, ",")
}
