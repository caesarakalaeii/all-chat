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

package shared

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/nsf/jsondiff"
)

// Note: RawChatMessage type is already defined in message_matcher.go
// We use the existing type definition to avoid conflicts

// NormalizeMessage creates a normalized copy of a RawChatMessage
// by applying user-defined normalization rules:
// - MessageID set to "<normalized>" (allow different ID schemes)
// - Timestamp truncated to 1-second precision (remove microseconds)
// - All other fields preserved unchanged
func NormalizeMessage(msg *RawChatMessage) *RawChatMessage {
	if msg == nil {
		return nil
	}

	// Deep copy to avoid mutating original
	normalized := &RawChatMessage{
		MessageID:   "<normalized>",                    // Normalize ID field
		Platform:    msg.Platform,
		OverlayID:   msg.OverlayID,
		ChannelID:   msg.ChannelID,
		ChannelName: msg.ChannelName,
		UserID:      msg.UserID,
		Username:    msg.Username,
		Text:        msg.Text,
		Timestamp:   msg.Timestamp.Truncate(time.Second), // Truncate to 1-second precision
		RawMessage:  msg.RawMessage,
		EventType:   msg.EventType,
	}

	// Deep copy Tags map
	if msg.Tags != nil {
		normalized.Tags = make(map[string]string, len(msg.Tags))
		for k, v := range msg.Tags {
			normalized.Tags[k] = v
		}
	}

	// Deep copy EventData map
	if msg.EventData != nil {
		normalized.EventData = make(map[string]interface{}, len(msg.EventData))
		for k, v := range msg.EventData {
			normalized.EventData[k] = v
		}
	}

	return normalized
}

// CompareMessages performs semantic comparison of two RawChatMessage instances
// after normalization. Returns (true, "") if messages match, or (false, diff_string)
// if they differ. The diff string is in git-style unified format.
func CompareMessages(official, innertube *RawChatMessage) (bool, string) {
	// Normalize both messages
	normalizedOfficial := NormalizeMessage(official)
	normalizedInnerTube := NormalizeMessage(innertube)

	// Marshal to JSON with sorted keys for comparison
	officialJSON, err := json.MarshalIndent(normalizedOfficial, "", "  ")
	if err != nil {
		return false, fmt.Sprintf("Failed to marshal official message: %v", err)
	}

	innertubeJSON, err := json.MarshalIndent(normalizedInnerTube, "", "  ")
	if err != nil {
		return false, fmt.Sprintf("Failed to marshal innertube message: %v", err)
	}

	// Use jsondiff for semantic comparison
	opts := jsondiff.DefaultConsoleOptions()
	diff, explanation := jsondiff.Compare(officialJSON, innertubeJSON, &opts)

	if diff == jsondiff.FullMatch {
		return true, ""
	}

	// Generate human-readable diff
	diffString := explanation
	return false, diffString
}

// AllowedFieldDifferences returns the list of fields that are allowed to differ
// between official and InnerTube listeners. This is primarily for documentation
// and test assertions.
func AllowedFieldDifferences() []string {
	return []string{
		"message_id", // InnerTube and official use different ID generation schemes
		"timestamp",  // Allowed to differ within 1-second precision
	}
}

// ParseMessageFromJSON is a helper to parse JSON bytes into RawChatMessage
func ParseMessageFromJSON(data []byte) (*RawChatMessage, error) {
	var msg RawChatMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("parse message JSON: %w", err)
	}
	return &msg, nil
}
