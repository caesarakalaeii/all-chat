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

package main

import (
	"testing"
	"time"

	"github.com/caesar/all-chat/services/message-processor/models"
)

// This carry is the entire producer half of the end-to-end metric: drop it and
// the ingest time never reaches the wire, leaving
// pipeline_end_to_end_latency_seconds empty with a green build.
func TestCarryRawFields_CopiesIngestedAt(t *testing.T) {
	ingestedAt := time.UnixMilli(1700000000123).UTC()
	raw := &models.RawChatMessage{IngestedAt: &ingestedAt}
	unified := &models.UnifiedChatMessage{}

	carryRawFields(unified, raw)

	if unified.IngestedAt == nil {
		t.Fatal("IngestedAt is nil; the ingest time never reaches the published message")
	}
	if !unified.IngestedAt.Equal(ingestedAt) {
		t.Errorf("IngestedAt = %v, want %v", unified.IngestedAt, ingestedAt)
	}
}

// An unparseable stream ID leaves the raw ingest time nil; the published message
// must stay nil too, so the gateway skips the observation rather than measuring
// from the zero instant.
func TestCarryRawFields_LeavesIngestedAtNilWhenRawHasNone(t *testing.T) {
	unified := &models.UnifiedChatMessage{}

	carryRawFields(unified, &models.RawChatMessage{})

	if unified.IngestedAt != nil {
		t.Errorf("IngestedAt = %v, want nil when the raw message has no ingest time", unified.IngestedAt)
	}
}

// IsUpdate rides the same convergence point; the gateway routes late-arriving
// media and TikTok like aggregates on it, so it must survive the carry.
func TestCarryRawFields_CopiesIsUpdate(t *testing.T) {
	unified := &models.UnifiedChatMessage{}

	carryRawFields(unified, &models.RawChatMessage{IsUpdate: true})

	if !unified.IsUpdate {
		t.Error("IsUpdate = false, want true")
	}
}
