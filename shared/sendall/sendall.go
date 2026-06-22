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

// Package sendall defines the shared Redis contract for the streamer "send to all"
// feature: when a streamer sends one message to every connected platform, each
// platform echoes that message back through chat:raw, so the message-processor would
// otherwise emit N copies. To collapse them into ONE message with a combined platform
// pill, auth-service PRE-REGISTERS the outgoing message (one key per target platform
// identity) just before fanning out; message-processor then recognises each echo and
// publishes the group exactly once per overlay.
//
// Both writer (auth-service) and reader (message-processor) import this package so the
// key derivation and value schema can never drift apart.
package sendall

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
)

// TTL is how long a registration lives. It only needs to outlive the platform
// round-trip plus echo latency for the streamer's own message, so it is deliberately
// short — long enough to catch the echoes, short enough that an unrelated later message
// with identical text never collides with a stale group.
const TTL = 15 * time.Second

// Registration is the value stored under each per-platform key. GroupID is reused as
// the unified message id (so every echo collapses to one id) and Platforms is the full
// set the message was sent to (rendered as the combined pill, known up front so the
// processor never has to wait for all echoes).
type Registration struct {
	GroupID   string   `json:"group_id"`
	Platforms []string `json:"platforms"`
}

// NormalizeText canonicalises message text for fingerprinting: collapse internal
// whitespace runs to single spaces (this also trims) and lowercase. Writer and reader
// MUST use this identical normalization or the echoes will not match.
func NormalizeText(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

// Key is the Redis key for a (platform, sender platform-id, message text) tuple. The
// sender id is the platform-native id of the streamer on that platform (Twitch user id,
// YouTube channel id, Kick user id) — the same id that appears as the author on the
// echoed-back message, which is how the processor matches it.
func Key(platform, senderID, text string) string {
	sum := sha256.Sum256([]byte(NormalizeText(text)))
	return "sendall:" + platform + ":" + senderID + ":" + hex.EncodeToString(sum[:])
}

// PublishedKey marks that the combined message for a group has already been published
// to a given overlay, so later echoes of the same group are dropped for that overlay
// (the message-processor SETNX-claims this key; the winner publishes once).
func PublishedKey(overlayID, groupID string) string {
	return "sendall:pub:" + overlayID + ":" + groupID
}
