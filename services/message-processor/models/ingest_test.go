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

package models

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// The gateway decodes this key to compute end-to-end pipeline latency, so the
// JSON name and the RFC3339Nano UTC encoding are part of the wire contract.
func TestUnifiedChatMessage_IngestedAtWireFormat(t *testing.T) {
	ingestedAt := time.UnixMilli(1700000000123).UTC()
	payload, err := json.Marshal(&UnifiedChatMessage{IngestedAt: &ingestedAt})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded struct {
		IngestedAt string `json:"ingested_at"`
	}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.IngestedAt != "2023-11-14T22:13:20.123Z" {
		t.Errorf("ingested_at = %q, want RFC3339Nano UTC 2023-11-14T22:13:20.123Z", decoded.IngestedAt)
	}
}

// Overlays and other consumers that predate the field must see no change, so an
// unset ingest time is omitted rather than sent as a zero timestamp or null.
func TestUnifiedChatMessage_IngestedAtOmittedWhenUnset(t *testing.T) {
	payload, err := json.Marshal(&UnifiedChatMessage{})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(payload), "ingested_at") {
		t.Errorf("ingested_at present on a message with no ingest time: %s", payload)
	}
}
