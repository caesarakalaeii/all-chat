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

import "time"

// TTSConfig represents a row of the overlay_tts_configs table.
//
// EncryptedAPIKey is base64([kid(1B)][nonce(12B)][ciphertext][tag(16B)]) of the
// user-supplied ElevenLabs API key, encrypted via shared/encryption.MultiKeyEncryptor
// using the service-wide TOKEN_ENCRYPTION_KEY_V1 master key (Phase 14 versioned format).
// It is stored as BYTEA in Postgres and tagged json:"-" here to guarantee the encrypted
// blob is never serialised to an HTTP response, even by accident.
//
// SigningSecret is the 32 random bytes used as the HMAC secret for tts_token
// JWTs (Phase 13 D-08). Also json:"-" — must never leak out over the wire.
type TTSConfig struct {
	ID              string    `json:"id"`
	OverlayID       string    `json:"overlay_id"`
	EncryptedAPIKey []byte    `json:"-"`
	VoiceID         string    `json:"voice_id"`
	SigningSecret   []byte    `json:"-"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}
