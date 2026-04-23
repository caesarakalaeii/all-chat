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

package tts

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustRandomSecret(t *testing.T) []byte {
	t.Helper()
	b := make([]byte, 32)
	_, err := rand.Read(b)
	require.NoError(t, err)
	return b
}

// TestSignVerifyJWT covers the happy path: sign produces a verifiable token.
func TestSignVerifyJWT(t *testing.T) {
	secret := mustRandomSecret(t)
	overlayID := "4e7b9e3a-1c9a-4c6a-9a7e-aabbccddeeff"

	token, err := SignOverlayToken(overlayID, secret)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	err = VerifyOverlayToken(token, overlayID, secret)
	assert.NoError(t, err, "verify should succeed for valid token+secret+overlayID")
}

// TestVerifyRejectsDifferentSecret — tampering with the signing secret must
// invalidate the token.
func TestVerifyRejectsDifferentSecret(t *testing.T) {
	secretA := mustRandomSecret(t)
	secretB := mustRandomSecret(t)
	overlayID := "overlay-123"

	token, err := SignOverlayToken(overlayID, secretA)
	require.NoError(t, err)

	err = VerifyOverlayToken(token, overlayID, secretB)
	assert.Error(t, err)
}

// TestVerifyRejectsWrongSubject — the sub claim must match the caller-supplied
// overlayID; otherwise the token is invalid.
func TestVerifyRejectsWrongSubject(t *testing.T) {
	secret := mustRandomSecret(t)
	token, err := SignOverlayToken("overlay-A", secret)
	require.NoError(t, err)

	err = VerifyOverlayToken(token, "overlay-B", secret)
	assert.Error(t, err)
}

// TestVerifyRejectsWrongScope — scope claim must be exactly "tts:use".
func TestVerifyRejectsWrongScope(t *testing.T) {
	secret := mustRandomSecret(t)
	overlayID := "overlay-scope"

	// Forge a token manually with a wrong scope claim.
	claims := TTSClaims{
		Scope: "admin",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:  overlayID,
			Issuer:   "all-chat",
			IssuedAt: jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(secret)
	require.NoError(t, err)

	err = VerifyOverlayToken(signed, overlayID, secret)
	assert.Error(t, err)
}

// TestVerifyRejectsTamperedSignature — flipping one character in the
// signature segment must fail verification.
func TestVerifyRejectsTamperedSignature(t *testing.T) {
	secret := mustRandomSecret(t)
	token, err := SignOverlayToken("overlay-tamper", secret)
	require.NoError(t, err)

	// Replace the entire signature segment with garbage. A single-char flip
	// can occasionally decode to the same raw bytes (base64 aliasing), so
	// we substitute the whole segment to guarantee an HMAC mismatch.
	parts := strings.Split(token, ".")
	require.Len(t, parts, 3)
	parts[2] = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	tampered := strings.Join(parts, ".")

	err = VerifyOverlayToken(tampered, "overlay-tamper", secret)
	assert.Error(t, err)
}

// TestVerifyRejectsRS256 — algorithm-confusion mitigation. A token with an
// RS256 header (asymmetric) must be rejected even if the HS256 secret happens
// to match the public key bytes.
func TestVerifyRejectsRS256(t *testing.T) {
	secret := mustRandomSecret(t)
	overlayID := "overlay-rs256"

	// Build a forged RS256 token.
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	claims := TTSClaims{
		Scope: "tts:use",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:  overlayID,
			Issuer:   "all-chat",
			IssuedAt: jwt.NewNumericDate(time.Now()),
		},
	}
	rsToken := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signed, err := rsToken.SignedString(privKey)
	require.NoError(t, err)

	// Verification must reject because the parser callback insists on HMAC.
	err = VerifyOverlayToken(signed, overlayID, secret)
	assert.Error(t, err, "RS256-signed tokens must be rejected (algorithm confusion defence)")
}

// TestRotationInvalidatesOldTokens — sign with secret A, rotate to B,
// verify-against-B of a freshly signed token succeeds, verify-against-B of the
// pre-rotation token fails.
func TestRotationInvalidatesOldTokens(t *testing.T) {
	overlayID := "overlay-rotate"
	secretA := mustRandomSecret(t)
	secretB := mustRandomSecret(t)

	oldToken, err := SignOverlayToken(overlayID, secretA)
	require.NoError(t, err)

	// After rotation: old token fails, new token succeeds.
	err = VerifyOverlayToken(oldToken, overlayID, secretB)
	assert.Error(t, err, "pre-rotation token must fail against new secret")

	newToken, err := SignOverlayToken(overlayID, secretB)
	require.NoError(t, err)
	err = VerifyOverlayToken(newToken, overlayID, secretB)
	assert.NoError(t, err, "post-rotation token must succeed against new secret")
}

// TestVerifyAcceptsOldIssuedAt — D-08 no-expiry design. A token whose IssuedAt
// is 100 years in the past must still verify (no exp claim is emitted).
func TestVerifyAcceptsOldIssuedAt(t *testing.T) {
	secret := mustRandomSecret(t)
	overlayID := "overlay-old"

	claims := TTSClaims{
		Scope: "tts:use",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:  overlayID,
			Issuer:   "all-chat",
			IssuedAt: jwt.NewNumericDate(time.Now().Add(-100 * 365 * 24 * time.Hour)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString(secret)
	require.NoError(t, err)

	err = VerifyOverlayToken(signed, overlayID, secret)
	assert.NoError(t, err, "ancient IssuedAt must still verify (no-expiry design)")
}

// TestVerifyEmptyTokenReturnsError — graceful rejection (no panic) on empty
// input.
func TestVerifyEmptyTokenReturnsError(t *testing.T) {
	secret := mustRandomSecret(t)
	err := VerifyOverlayToken("", "overlay-any", secret)
	assert.Error(t, err)
}

// TestSignDoesNotEmitExpClaim — D-08. Decoded payload of a freshly signed
// token must not contain an exp field.
func TestSignDoesNotEmitExpClaim(t *testing.T) {
	secret := mustRandomSecret(t)
	overlayID := "overlay-noexp"

	signed, err := SignOverlayToken(overlayID, secret)
	require.NoError(t, err)

	// JWT has three dot-separated segments: header.payload.signature.
	parts := strings.Split(signed, ".")
	require.Len(t, parts, 3)

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	require.NoError(t, err)

	var rawClaims map[string]interface{}
	require.NoError(t, json.Unmarshal(payloadBytes, &rawClaims))

	_, expPresent := rawClaims["exp"]
	assert.False(t, expPresent, "SignOverlayToken must not emit an exp claim (D-08)")

	// And the issued token verifies.
	err = VerifyOverlayToken(signed, overlayID, secret)
	assert.NoError(t, err)
}

// TestSignEmitsExpectedClaims — sanity check that the issued token carries the
// documented claim shape (sub, scope, iss, iat).
func TestSignEmitsExpectedClaims(t *testing.T) {
	secret := mustRandomSecret(t)
	overlayID := "overlay-claims"

	signed, err := SignOverlayToken(overlayID, secret)
	require.NoError(t, err)

	parts := strings.Split(signed, ".")
	require.Len(t, parts, 3)

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	require.NoError(t, err)

	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(payloadBytes, &m))

	assert.Equal(t, overlayID, m["sub"])
	assert.Equal(t, "tts:use", m["scope"])
	assert.Equal(t, "all-chat", m["iss"])
	assert.NotNil(t, m["iat"])
}
