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

// Package tts implements the per-overlay JWT scheme used by the OBS-URL
// tts_token auth path (Phase 13 D-08). The JWTs are HS256-signed with a
// per-overlay 32-byte secret; revocation is performed exclusively by rotating
// the per-overlay signing secret (D-10). No ExpiresAt claim is emitted: the
// OBS browser source holds the token persistently, so rotation is the only
// supported revocation mechanism.
package tts

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ErrInvalidToken is returned by VerifyOverlayToken for any validation
// failure. The concrete JWT-library error is intentionally not surfaced so
// callers cannot leak why a token failed (e.g. to probe for valid overlay
// IDs).
var ErrInvalidToken = errors.New("invalid tts_token")

// TTSClaims are the claims carried in an OBS-URL tts_token.
//
// Scope is the fixed string "tts:use" — any other value is rejected by
// VerifyOverlayToken. Subject is the overlay UUID.
//
// NOTE: no ExpiresAt field — D-08 mandates rotation-based revocation via
// regenerating the per-overlay tts_signing_secret (see HandleRotateToken).
type TTSClaims struct {
	Scope string `json:"scope"`
	jwt.RegisteredClaims
}

// SignOverlayToken produces a HS256-signed JWT for the given overlayID using
// the per-overlay signing secret. The returned token is suitable for
// embedding in the OBS browser-source URL as ?tts_token=... (D-08).
func SignOverlayToken(overlayID string, signingSecret []byte) (string, error) {
	claims := TTSClaims{
		Scope: "tts:use",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:  overlayID,
			Issuer:   "all-chat",
			IssuedAt: jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(signingSecret)
}

// VerifyOverlayToken validates a tts_token against the expected overlayID and
// per-overlay signing secret. Returns nil on success, non-nil error on any
// validation failure (signature mismatch, scope/subject mismatch, algorithm
// confusion attempt, etc.).
//
// Algorithm-confusion defence (T-13-02): the ParseWithClaims callback asserts
// *jwt.SigningMethodHMAC on token.Method, rejecting any non-HMAC alg (e.g.
// RS256, none, ES256). Only HS256 signatures reach the secret comparison.
func VerifyOverlayToken(tokenString, overlayID string, signingSecret []byte) error {
	if tokenString == "" {
		return ErrInvalidToken
	}

	parsed, err := jwt.ParseWithClaims(tokenString, &TTSClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return signingSecret, nil
	})
	if err != nil || parsed == nil || !parsed.Valid {
		return ErrInvalidToken
	}

	claims, ok := parsed.Claims.(*TTSClaims)
	if !ok {
		return ErrInvalidToken
	}

	if claims.Subject != overlayID {
		return ErrInvalidToken
	}
	if claims.Scope != "tts:use" {
		return ErrInvalidToken
	}

	return nil
}
