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

package testgen

import (
	"math/rand"
	"testing"
)

const testOverlayID = "00000000-0000-4000-8000-000000000a11"

func newTestGen() *Generator {
	// build* methods don't touch the publisher/enrichers, so nil is fine here.
	return NewGenerator(testOverlayID, nil, nil, nil, nil)
}

func TestBuildChatVotesAreBareNumbers(t *testing.T) {
	g := newTestGen()
	rng := rand.New(rand.NewSource(1))

	votes := 0
	for i := 0; i < 500; i++ {
		msg := g.buildChat(rng, 1.0) // 100% votes
		if msg.OverlayID != testOverlayID {
			t.Fatalf("overlay id = %q, want %q", msg.OverlayID, testOverlayID)
		}
		if msg.Event != nil {
			t.Fatalf("vote chat message should have no event")
		}
		switch msg.Message.Text {
		case "1", "2", "3", "4":
			votes++
		default:
			t.Fatalf("vote text = %q, want one of 1/2/3/4", msg.Message.Text)
		}
		if msg.Metadata["vote"] != true {
			t.Fatalf("vote message missing vote metadata")
		}
	}
	if votes != 500 {
		t.Fatalf("expected all 500 messages to be votes, got %d", votes)
	}
}

func TestBuildChatZeroVoteRatioNeverVotes(t *testing.T) {
	g := newTestGen()
	rng := rand.New(rand.NewSource(2))
	for i := 0; i < 500; i++ {
		msg := g.buildChat(rng, 0.0)
		switch msg.Message.Text {
		case "1", "2", "3", "4":
			t.Fatalf("got bare vote %q with voteRatio=0", msg.Message.Text)
		}
	}
}

func TestBuildEventHasValidEventInfo(t *testing.T) {
	g := newTestGen()
	rng := rand.New(rand.NewSource(3))
	for i := 0; i < 500; i++ {
		msg := g.buildEvent(rng)
		if msg.Event == nil {
			t.Fatalf("event message has nil Event")
		}
		if msg.Event.Type == "" {
			t.Fatalf("event message has empty Type")
		}
		if msg.Platform == "" {
			t.Fatalf("event message has empty Platform")
		}
		if msg.OverlayID != testOverlayID {
			t.Fatalf("overlay id = %q, want %q", msg.OverlayID, testOverlayID)
		}
	}
}

func TestApplyDefaults(t *testing.T) {
	var c Config // empty body
	c.applyDefaults()
	if c.DurationSeconds != defaultDurationSeconds {
		t.Fatalf("duration default = %d, want %d", c.DurationSeconds, defaultDurationSeconds)
	}
	if c.RatePerSecond != defaultRatePerSecond {
		t.Fatalf("rate default = %v, want %v", c.RatePerSecond, defaultRatePerSecond)
	}
	if c.VoteRatio != defaultVoteRatio || c.EventEveryN != defaultEventEveryN {
		t.Fatalf("empty body should fall back to vote+event mix, got vote=%v event=%d", c.VoteRatio, c.EventEveryN)
	}

	// Clamping
	clamp := Config{DurationSeconds: 999999, RatePerSecond: 999, VoteRatio: 5}
	clamp.applyDefaults()
	if clamp.DurationSeconds != maxDurationSeconds {
		t.Fatalf("duration not clamped: %d", clamp.DurationSeconds)
	}
	if clamp.RatePerSecond != maxRatePerSecond {
		t.Fatalf("rate not clamped: %v", clamp.RatePerSecond)
	}
	if clamp.VoteRatio != 1 {
		t.Fatalf("vote ratio not clamped: %v", clamp.VoteRatio)
	}
}
