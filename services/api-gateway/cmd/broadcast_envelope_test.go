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
	"encoding/json"
	"testing"
	"time"
)

// The processor writes ingested_at and the gateway reads it back; nothing else
// checks that the two agree on the key. A rename on either side silently empties
// pipeline_end_to_end_latency_seconds, which is the defect this metric exists to
// avoid, so the contract is pinned here against a payload the processor produced.
func TestDecodeBroadcastEnvelope_ReadsProducerIngestedAt(t *testing.T) {
	ingestedAt := time.UnixMilli(1700000000123).UTC()
	// Literal rather than a models.UnifiedChatMessage: api-gateway does not
	// import the processor's module, so the key is restated as it goes on the
	// wire. models.TestUnifiedChatMessage_IngestedAtWireFormat pins the
	// producer to this same spelling and encoding.
	payload := []byte(`{"id":"m-1","platform":"twitch","ingested_at":"2023-11-14T22:13:20.123Z"}`)

	envelope, err := decodeBroadcastEnvelope(payload)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if envelope.IngestedAt == nil {
		t.Fatal("IngestedAt is nil; the gateway is not reading the producer's ingested_at key")
	}
	if !envelope.IngestedAt.Equal(ingestedAt) {
		t.Errorf("IngestedAt = %v, want %v", envelope.IngestedAt, ingestedAt)
	}
	if envelope.Platform != "twitch" {
		t.Errorf("Platform = %q, want twitch", envelope.Platform)
	}
	if envelope.ID != "m-1" {
		t.Errorf("ID = %q, want m-1", envelope.ID)
	}
}

// A message published before the processor stamped ingest times, and a buffered
// deletion replayed from the registry, both arrive without the key. That must
// decode cleanly with a nil time rather than erroring or yielding the zero
// instant, which would be recorded as a ~55-year end-to-end latency.
func TestDecodeBroadcastEnvelope_LeavesIngestedAtNilWhenAbsent(t *testing.T) {
	envelope, err := decodeBroadcastEnvelope([]byte(`{"id":"m-1","platform":"twitch"}`))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if envelope.IngestedAt != nil {
		t.Errorf("IngestedAt = %v, want nil when the key is absent", envelope.IngestedAt)
	}
}

// The deletion branch of the handler keys off the nested event type, so the
// envelope must keep carrying it alongside the new ingest time.
func TestDecodeBroadcastEnvelope_ReadsDeletionEvent(t *testing.T) {
	payload, err := json.Marshal(map[string]any{
		"id":       "m-1",
		"platform": "twitch",
		"event": map[string]any{
			"type":     "message_deletion",
			"metadata": map[string]any{"target_message_id": "m-0"},
		},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	envelope, err := decodeBroadcastEnvelope(payload)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if envelope.Event == nil {
		t.Fatal("Event is nil; the deletion branch would never fire")
	}
	if envelope.Event.Type != "message_deletion" {
		t.Errorf("Event.Type = %q, want message_deletion", envelope.Event.Type)
	}
	if got := envelope.Event.Metadata["target_message_id"]; got != "m-0" {
		t.Errorf("target_message_id = %v, want m-0", got)
	}
}
