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

// Package invites mints and hashes the single-use secrets that redeem a delegated-moderation
// grant (ADR-0048).
//
// The secret is a bearer credential for a moderation grant, so it is treated like one: it is
// generated from crypto/rand, shown to the streamer exactly once, and persisted only as a
// SHA-256 digest. Nothing in All-Chat can re-display or re-derive it — losing it means minting a
// new invite, which is the correct trade for a token that can hand someone the moderation
// write-path on a live channel.
//
// A plain digest (no salt, no KDF) is deliberate and sufficient here: the input is 256 bits of
// uniform randomness, so there is no dictionary to attack and nothing for a work factor to slow
// down. The digest exists to keep a database read from yielding usable tokens, and to give the
// lookup an exact-match key.
package invites

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"time"
)

const (
	// TTL is how long a fresh invite stays redeemable. Long enough to survive "I'll set this up
	// after stream", short enough that a forgotten invite in a Discord DM stops being a key to
	// the channel.
	TTL = 7 * 24 * time.Hour

	// secretBytes is the entropy behind an invite. 256 bits makes guessing irrelevant next to
	// every other way an invite can leak.
	secretBytes = 32
)

// NewSecret returns a fresh invite secret, base64url-encoded without padding so it can be pasted
// into a URL, a chat message, or a form field without escaping.
func NewSecret() (string, error) {
	buf := make([]byte, secretBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate invite secret: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// Hash returns the SHA-256 digest of secret — the only form ever written to the database, and the
// lookup key for redeeming it. It hashes the exact string given: callers trim user input first,
// so that a stray newline from a copy-paste is not silently accepted as a different secret.
func Hash(secret string) []byte {
	sum := sha256.Sum256([]byte(secret))
	return sum[:]
}
