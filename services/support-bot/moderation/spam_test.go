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

package moderation

import (
	"testing"
	"time"
)

func TestCrossChannelTriggers(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	d := NewSpamDetector(Options{Now: func() time.Time { return now }})

	if d.Record("u1", "c1", "buy now", nil) {
		t.Fatal("1 channel should not trigger")
	}
	if d.Record("u1", "c2", "buy now", nil) {
		t.Fatal("2 channels should not trigger")
	}
	if !d.Record("u1", "c3", "buy now", nil) {
		t.Fatal("3rd distinct channel should trigger")
	}
	if !d.HasActioned("u1") {
		t.Fatal("user should be marked actioned")
	}
	// Already actioned -> no re-trigger.
	if d.Record("u1", "c4", "buy now", nil) {
		t.Fatal("actioned user should not re-trigger")
	}
}

func TestSameChannelDoesNotTrigger(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	d := NewSpamDetector(Options{Now: func() time.Time { return now }})
	for i := 0; i < 5; i++ {
		if d.Record("u1", "c1", "hello", nil) {
			t.Fatal("repeated posts in the SAME channel must not trigger")
		}
	}
}

func TestAttachmentFingerprint(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	d := NewSpamDetector(Options{Now: func() time.Time { return now }})
	// Same short text + same attachment sizes across 3 channels -> trigger.
	if d.Record("u2", "c1", "bro", []int{12345}) {
		t.Fatal("1 channel")
	}
	if d.Record("u2", "c2", "bro", []int{12345}) {
		t.Fatal("2 channels")
	}
	if !d.Record("u2", "c3", "bro", []int{12345}) {
		t.Fatal("3 channels with identical attachment fingerprint should trigger")
	}
}

func TestWindowExpiry(t *testing.T) {
	cur := time.Unix(1_000_000, 0)
	d := NewSpamDetector(Options{Window: 10 * time.Minute, Now: func() time.Time { return cur }})
	d.Record("u3", "c1", "spam", nil)
	d.Record("u3", "c2", "spam", nil)
	// Advance beyond the window; the earlier entries expire.
	cur = cur.Add(11 * time.Minute)
	if d.Record("u3", "c3", "spam", nil) {
		t.Fatal("entries older than the window should have expired, so 3rd channel alone must not trigger")
	}
}

func TestEmptyMessageIgnored(t *testing.T) {
	d := NewSpamDetector(Options{})
	for i := 0; i < 5; i++ {
		if d.Record("u4", "c1", "   ", nil) {
			t.Fatal("empty message with no attachments must never trigger")
		}
	}
}
