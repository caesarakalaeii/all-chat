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

package invites

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSecret_IsURLSafeAndHighEntropy(t *testing.T) {
	secret, err := NewSecret()
	require.NoError(t, err)

	raw, err := base64.RawURLEncoding.DecodeString(secret)
	require.NoError(t, err, "the secret must survive a round trip through a URL fragment")
	assert.Len(t, raw, 32, "256 bits: the secret is the only thing standing between a stranger and a grant")
	assert.NotContains(t, secret, "=", "padding would be mangled by careless copy-paste")
	assert.False(t, strings.ContainsAny(secret, "+/"), "the secret must be URL- and path-safe")
}

func TestNewSecret_NeverRepeats(t *testing.T) {
	seen := make(map[string]struct{}, 256)
	for i := 0; i < 256; i++ {
		secret, err := NewSecret()
		require.NoError(t, err)
		_, dup := seen[secret]
		require.False(t, dup, "a repeated secret would hand one invite to two people")
		seen[secret] = struct{}{}
	}
}

func TestHash_IsStableAndOneWay(t *testing.T) {
	secret, err := NewSecret()
	require.NoError(t, err)

	first := Hash(secret)
	assert.Len(t, first, 32, "SHA-256 digest")
	assert.Equal(t, first, Hash(secret), "the same secret must always hash to the same lookup key")
	assert.NotContains(t, string(first), secret, "the digest must not embed the secret")

	other, err := NewSecret()
	require.NoError(t, err)
	assert.NotEqual(t, first, Hash(other))
}

// The digest is what a compromised database row exposes, so it must not be reversible to a
// usable token: only an exact-match lookup key.
func TestHash_DiffersForNearlyIdenticalSecrets(t *testing.T) {
	assert.NotEqual(t, Hash("abc"), Hash("abd"))
	assert.NotEqual(t, Hash("abc"), Hash("abc "), "hashing is over the exact string; callers trim first")
}

func TestTTL_IsSevenDays(t *testing.T) {
	assert.Equal(t, 7*24*time.Hour, TTL,
		"an invite that outlives the conversation that created it is an unattended key to a channel")
}
