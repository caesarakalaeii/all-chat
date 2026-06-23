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

package auth

import (
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helper: produce a fixed-length secret string of repeated byte b
func repeatSecret(b byte, n int) string {
	s := make([]byte, n)
	for i := range s {
		s[i] = b
	}
	return string(s)
}

// ─────────────────────────────────────────────────────────────────────────────
// NewKeyChainFromEnv tests
// ─────────────────────────────────────────────────────────────────────────────

func TestKeyChain_NewFromEnv_LoadsAllVersions(t *testing.T) {
	secretV1 := repeatSecret('a', 32)
	secretV2 := repeatSecret('b', 32)
	legacy := repeatSecret('c', 32)

	t.Setenv("JWT_SECRET_V1", secretV1)
	t.Setenv("JWT_SECRET_V2", secretV2)
	t.Setenv("JWT_SECRET", legacy)

	kc, err := NewKeyChainFromEnv("JWT_SECRET")
	require.NoError(t, err)
	require.NotNil(t, kc)

	assert.Equal(t, "v2", kc.LatestKid())
	assert.Equal(t, []byte(secretV1), kc.byKid["v1"])
	assert.Equal(t, []byte(secretV2), kc.byKid["v2"])
	assert.Equal(t, []byte(legacy), kc.legacy)
}

func TestKeyChain_NewFromEnv_RequiresAtLeastOneVersioned(t *testing.T) {
	t.Setenv("JWT_SECRET", repeatSecret('c', 32))
	// do NOT set JWT_SECRET_V1

	_, err := NewKeyChainFromEnv("JWT_SECRET")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNoVersionedJWTSecrets), "error should wrap ErrNoVersionedJWTSecrets, got: %v", err)
}

func TestKeyChain_NewFromEnv_StopsAtFirstGap(t *testing.T) {
	secretV1 := repeatSecret('a', 32)
	secretV3 := repeatSecret('z', 32)

	t.Setenv("JWT_SECRET_V1", secretV1)
	// V2 intentionally not set
	t.Setenv("JWT_SECRET_V3", secretV3)

	kc, err := NewKeyChainFromEnv("JWT_SECRET")
	require.NoError(t, err)

	// Only V1 should be loaded; V3 should be silently ignored
	assert.Equal(t, "v1", kc.LatestKid())
	assert.Equal(t, []byte(secretV1), kc.byKid["v1"])
	_, hasV3 := kc.byKid["v3"]
	assert.False(t, hasV3, "V3 should not be loaded because V2 is missing")
}

func TestKeyChain_NewFromEnv_PrefixIsolation(t *testing.T) {
	userSecret := repeatSecret('a', 32)
	svcSecret := repeatSecret('z', 32)

	t.Setenv("JWT_SECRET_V1", userSecret)
	t.Setenv("SERVICE_JWT_SECRET_V1", svcSecret)

	userChain, err := NewKeyChainFromEnv("JWT_SECRET")
	require.NoError(t, err)

	// User chain must only see userSecret — NOT svcSecret
	assert.Equal(t, []byte(userSecret), userChain.byKid["v1"])
	assert.NotEqual(t, []byte(svcSecret), userChain.byKid["v1"])

	// Service chain must only see svcSecret
	svcChain, err := NewKeyChainFromEnv("SERVICE_JWT_SECRET")
	require.NoError(t, err)
	assert.Equal(t, []byte(svcSecret), svcChain.byKid["v1"])
}

// ─────────────────────────────────────────────────────────────────────────────
// KeyFunc dispatch tests
// ─────────────────────────────────────────────────────────────────────────────

func TestKeyChain_KeyFunc_MatchesByKid(t *testing.T) {
	secretA := repeatSecret('a', 32)
	kc := NewKeyChain(map[string][]byte{"v1": []byte(secretA)}, nil, "v1")

	// sign a token with kid "v1" and secretA
	tok, err := GenerateJWTWithKid("v1", "u1", "tw1", "alice", secretA, false)
	require.NoError(t, err)

	// validate with the chain — must succeed
	claims, err := ValidateJWTWithKeyChain(tok, kc)
	require.NoError(t, err)
	assert.Equal(t, "u1", claims.UserID)
}

func TestKeyChain_KeyFunc_NoKidUsesLegacy(t *testing.T) {
	legacySecret := repeatSecret('c', 32)
	versioned := repeatSecret('a', 32)
	kc := NewKeyChain(map[string][]byte{"v1": []byte(versioned)}, []byte(legacySecret), "v1")

	// generate a legacy (no-kid) token using the old GenerateJWT
	tok, err := GenerateJWT("u2", "tw2", "bob", legacySecret, false)
	require.NoError(t, err)

	// chain must fall back to legacy and validate successfully
	claims, err := ValidateJWTWithKeyChain(tok, kc)
	require.NoError(t, err)
	assert.Equal(t, "u2", claims.UserID)
}

func TestKeyChain_KeyFunc_UnknownKidFallsBackToLegacy(t *testing.T) {
	legacySecret := repeatSecret('c', 32)
	kc := NewKeyChain(map[string][]byte{"v1": []byte(repeatSecret('a', 32))}, []byte(legacySecret), "v1")

	// sign a token with unknown kid "v99" using the legacy secret
	claims := Claims{
		UserID: "u3",
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			Issuer:    "all-chat",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token.Header["kid"] = "v99"
	tok, err := token.SignedString([]byte(legacySecret))
	require.NoError(t, err)

	// chain has v99 unknown → fall back to legacy → validates IFF legacy secret matches
	parsed, err := ValidateJWTWithKeyChain(tok, kc)
	require.NoError(t, err)
	assert.Equal(t, "u3", parsed.UserID)
}

func TestKeyChain_KeyFunc_NoLegacyAndUnknownKid(t *testing.T) {
	kc := NewKeyChain(map[string][]byte{"v1": []byte(repeatSecret('a', 32))}, nil, "v1")

	// token with unknown kid "v99"
	claims := Claims{
		UserID: "u4",
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			Issuer:    "all-chat",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token.Header["kid"] = "v99"
	tok, err := token.SignedString([]byte(repeatSecret('z', 32)))
	require.NoError(t, err)

	_, err = ValidateJWTWithKeyChain(tok, kc)
	require.Error(t, err)
	assert.Equal(t, ErrInvalidToken, err)
}

func TestKeyChain_KeyFunc_RejectsNonHMAC(t *testing.T) {
	kc := NewKeyChain(map[string][]byte{"v1": []byte(repeatSecret('a', 32))}, nil, "v1")

	// craft a token using SigningMethodNone (alg-confusion attack)
	claims := Claims{
		UserID: "attacker",
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			Issuer:    "all-chat",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
	tok, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	// KeyChain.KeyFunc must reject this
	_, err = ValidateJWTWithKeyChain(tok, kc)
	require.Error(t, err)
	assert.Equal(t, ErrInvalidToken, err)
}

// ─────────────────────────────────────────────────────────────────────────────
// GenerateJWTWithKid — kid header presence
// ─────────────────────────────────────────────────────────────────────────────

func TestGenerateJWTWithKid_HeaderPresent(t *testing.T) {
	tok, err := GenerateJWTWithKid("v1", "user-1", "twitch-1", "alice", repeatSecret('s', 32), false)
	require.NoError(t, err)

	// parse without verifying signature to inspect header
	p := jwt.NewParser()
	token, _, err := p.ParseUnverified(tok, &Claims{})
	require.NoError(t, err)
	assert.Equal(t, "v1", token.Header["kid"])
}

// ─────────────────────────────────────────────────────────────────────────────
// ValidateJWTWithKeyChain
// ─────────────────────────────────────────────────────────────────────────────

func TestValidateJWTWithKeyChain_RoundTrip(t *testing.T) {
	secret := repeatSecret('a', 32)
	kc := NewKeyChain(map[string][]byte{"v1": []byte(secret)}, nil, "v1")

	tok, err := GenerateJWTWithKid("v1", "user-1", "twitch-1", "alice", secret, false)
	require.NoError(t, err)

	claims, err := ValidateJWTWithKeyChain(tok, kc)
	require.NoError(t, err)
	assert.Equal(t, "user-1", claims.UserID)
}

func TestValidateJWTWithKeyChain_LegacyToken(t *testing.T) {
	legacySecret := repeatSecret('l', 32)
	kc := NewKeyChain(map[string][]byte{"v1": []byte(repeatSecret('a', 32))}, []byte(legacySecret), "v1")

	// existing GenerateJWT produces a token without kid
	tok, err := GenerateJWT("user-legacy", "twitch-legacy", "carol", legacySecret, false)
	require.NoError(t, err)

	claims, err := ValidateJWTWithKeyChain(tok, kc)
	require.NoError(t, err)
	assert.Equal(t, "user-legacy", claims.UserID)
}

func TestValidateJWTWithKeyChain_ExpiredKidStillRejects(t *testing.T) {
	secret := repeatSecret('a', 32)
	kc := NewKeyChain(map[string][]byte{"v1": []byte(secret)}, nil, "v1")

	// build an already-expired token manually
	claims := Claims{
		UserID: "user-expired",
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
			Issuer:    "all-chat",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token.Header["kid"] = "v1"
	tok, err := token.SignedString([]byte(secret))
	require.NoError(t, err)

	_, err = ValidateJWTWithKeyChain(tok, kc)
	require.Error(t, err)
	assert.Equal(t, ErrExpiredToken, err, "expired token must return ErrExpiredToken even when kid is valid")
}

// ─────────────────────────────────────────────────────────────────────────────
// ValidateServiceJWTWithKeyChain — chain isolation (D-10)
// ─────────────────────────────────────────────────────────────────────────────

func TestValidateServiceJWTWithKeyChain_ChainIsolation(t *testing.T) {
	userSecret := repeatSecret('u', 32)
	svcSecret := repeatSecret('s', 32)

	userChain := NewKeyChain(map[string][]byte{"v1": []byte(userSecret)}, nil, "v1")
	svcChain := NewKeyChain(map[string][]byte{"v1": []byte(svcSecret)}, nil, "v1")

	// sign a user JWT with the user chain secret
	userTok, err := GenerateJWTWithKid("v1", "user-1", "tw1", "alice", userSecret, false)
	require.NoError(t, err)

	// attempt to validate the user token against the service chain — MUST fail (D-10)
	// We re-parse with serviceChain: the HMAC will be wrong → should return ErrInvalidToken.
	// ValidateServiceJWTWithKeyChain expects ServiceClaims; a Claims token won't parse cleanly,
	// but more importantly the signing secret is different so HMAC fails first.
	_, err = ValidateServiceJWTWithKeyChain(userTok, svcChain)
	require.Error(t, err, "user-chain token must not validate under service chain")
	assert.Equal(t, ErrInvalidToken, err)

	// confirm userChain itself validates the same token correctly
	userClaims, err := ValidateJWTWithKeyChain(userTok, userChain)
	require.NoError(t, err)
	assert.Equal(t, "user-1", userClaims.UserID)
}

// ─────────────────────────────────────────────────────────────────────────────
// ValidateViewerJWTWithKeyChain
// ─────────────────────────────────────────────────────────────────────────────

func TestValidateViewerJWTWithKeyChain_RoundTrip(t *testing.T) {
	secret := repeatSecret('v', 32)
	kc := NewKeyChain(map[string][]byte{"v1": []byte(secret)}, nil, "v1")

	viewerClaims := ViewerClaims{
		ViewerID:  "viewer-1",
		SessionID: "sess-abc",
		Platform:  "twitch",
		IsViewer:  true,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			Issuer:    "all-chat",
		},
	}
	tok, err := GenerateViewerJWTWithKid("v1", viewerClaims, secret)
	require.NoError(t, err)
	require.NotEmpty(t, tok)

	// kid header must be present
	p := jwt.NewParser()
	parsed, _, err := p.ParseUnverified(tok, &ViewerClaims{})
	require.NoError(t, err)
	assert.Equal(t, "v1", parsed.Header["kid"])

	// round-trip validation
	vc, err := ValidateViewerJWTWithKeyChain(tok, kc)
	require.NoError(t, err)
	assert.Equal(t, "viewer-1", vc.ViewerID)
	assert.True(t, vc.IsViewer)
}

// ─────────────────────────────────────────────────────────────────────────────
// GenerateServiceJWTWithKid round-trip
// ─────────────────────────────────────────────────────────────────────────────

func TestGenerateServiceJWTWithKid_RoundTrip(t *testing.T) {
	secret := repeatSecret('s', 32)
	kc := NewKeyChain(map[string][]byte{"v1": []byte(secret)}, nil, "v1")

	tok, err := GenerateServiceJWTWithKid("v1", "kick-listener", secret, 15*time.Minute)
	require.NoError(t, err)
	require.NotEmpty(t, tok)

	sc, err := ValidateServiceJWTWithKeyChain(tok, kc)
	require.NoError(t, err)
	assert.Equal(t, "kick-listener", sc.ServiceName)
}

// ─────────────────────────────────────────────────────────────────────────────
// GenerateImpersonationJWTWithKid kid header presence
// ─────────────────────────────────────────────────────────────────────────────

func TestGenerateImpersonationJWTWithKid_HeaderPresent(t *testing.T) {
	secret := repeatSecret('i', 32)
	tok, err := GenerateImpersonationJWTWithKid("v1", "admin-1", "admin-alice", "target-2", "target-bob", "target-twitch", secret)
	require.NoError(t, err)

	p := jwt.NewParser()
	token, _, err := p.ParseUnverified(tok, &Claims{})
	require.NoError(t, err)
	assert.Equal(t, "v1", token.Header["kid"])
}

// ─────────────────────────────────────────────────────────────────────────────
// GenerateTokenWithKid kid header presence
// ─────────────────────────────────────────────────────────────────────────────

func TestGenerateTokenWithKid_HeaderPresent(t *testing.T) {
	secret := repeatSecret('t', 32)
	tok, err := GenerateTokenWithKid("v1", "user-x", "alice", secret, 1*time.Hour, false)
	require.NoError(t, err)

	p := jwt.NewParser()
	token, _, err := p.ParseUnverified(tok, &Claims{})
	require.NoError(t, err)
	assert.Equal(t, "v1", token.Header["kid"])
}

// ─────────────────────────────────────────────────────────────────────────────
// LatestKid / LatestSecret accessors
// ─────────────────────────────────────────────────────────────────────────────

func TestKeyChain_LatestKidAndSecret(t *testing.T) {
	v1 := []byte(repeatSecret('a', 32))
	v2 := []byte(repeatSecret('b', 32))
	kc := NewKeyChain(map[string][]byte{"v1": v1, "v2": v2}, nil, "v2")

	assert.Equal(t, "v2", kc.LatestKid())
	assert.Equal(t, v2, kc.LatestSecret())
}

// ─────────────────────────────────────────────────────────────────────────────
// M3: minimum secret length enforcement
// ─────────────────────────────────────────────────────────────────────────────

func TestKeyChain_NewFromEnv_RejectsShortVersionedSecret(t *testing.T) {
	t.Setenv("JWT_SECRET_V1", "tooshort")
	_, err := NewKeyChainFromEnv("JWT_SECRET")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrSecretTooShort)
}

func TestKeyChain_NewFromEnv_RejectsShortLegacySecret(t *testing.T) {
	t.Setenv("JWT_SECRET_V1", repeatSecret('a', 32))
	t.Setenv("JWT_SECRET", "tooshort")
	_, err := NewKeyChainFromEnv("JWT_SECRET")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrSecretTooShort)
}

// ─────────────────────────────────────────────────────────────────────────────
// L3: issuer validation
// ─────────────────────────────────────────────────────────────────────────────

func TestValidateJWTWithKeyChain_RejectsWrongIssuer(t *testing.T) {
	secret := repeatSecret('a', 32)
	kc := NewKeyChain(map[string][]byte{"v1": []byte(secret)}, nil, "v1")

	claims := Claims{
		UserID: "u1",
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			Issuer:    "evil-issuer",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token.Header["kid"] = "v1"
	tok, err := token.SignedString([]byte(secret))
	require.NoError(t, err)

	_, err = ValidateJWTWithKeyChain(tok, kc)
	require.Error(t, err)
}

func TestValidateJWTWithKeyChain_AcceptsImpersonationIssuer(t *testing.T) {
	secret := repeatSecret('i', 32)
	kc := NewKeyChain(map[string][]byte{"v1": []byte(secret)}, nil, "v1")

	tok, err := GenerateImpersonationJWTWithKid("v1", "admin-1", "admin-alice", "target-2", "target-bob", "target-twitch", secret)
	require.NoError(t, err)

	claims, err := ValidateJWTWithKeyChain(tok, kc)
	require.NoError(t, err)
	assert.Equal(t, "target-2", claims.UserID)
}
