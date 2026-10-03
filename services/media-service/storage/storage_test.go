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

package storage

import (
	"testing"
	"time"

	"github.com/caesar/all-chat/services/media-service/handlers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig_DefaultsWhenUnset(t *testing.T) {
	cfg, err := LoadConfig()
	require.NoError(t, err)

	assert.Empty(t, cfg.Endpoint, "no endpoint means MinIO is not wired and routes serve 503")
	assert.True(t, cfg.UseSSL, "TLS on by default: the public host media.allch.at is https")
	assert.Equal(t, "allchat-media", cfg.Bucket)
	assert.Equal(t, "https://media.allch.at", cfg.PublicBaseURL)
	assert.Equal(t, 5*time.Minute, cfg.PresignExpiry)
	assert.Equal(t, 100, cfg.MaxObjectsPerUser)
}

func TestLoadConfig_ParsesExplicitValues(t *testing.T) {
	t.Setenv("MINIO_ENDPOINT", "minio.allchat.svc.cluster.local:9000")
	t.Setenv("MINIO_MEDIA_USER", "media-user")
	t.Setenv("MINIO_MEDIA_PASSWORD", "media-pass")
	t.Setenv("MINIO_USE_SSL", "false")
	t.Setenv("MINIO_BUCKET", "other-bucket")
	t.Setenv("MEDIA_PUBLIC_URL", "https://cdn.example.com/allchat-media/")
	t.Setenv("MEDIA_PRESIGN_EXPIRY", "30s")
	t.Setenv("MEDIA_MAX_OBJECTS_PER_USER", "7")

	cfg, err := LoadConfig()
	require.NoError(t, err)

	assert.Equal(t, "minio.allchat.svc.cluster.local:9000", cfg.Endpoint)
	assert.Equal(t, "media-user", cfg.AccessKey)
	assert.Equal(t, "media-pass", cfg.SecretKey)
	assert.False(t, cfg.UseSSL)
	assert.Equal(t, "other-bucket", cfg.Bucket)
	assert.Equal(t, "https://cdn.example.com/allchat-media", cfg.PublicBaseURL, "trailing slash must be trimmed so public_url joins cleanly")
	assert.Equal(t, 30*time.Second, cfg.PresignExpiry)
	assert.Equal(t, 7, cfg.MaxObjectsPerUser)
}

func TestLoadConfig_RejectsInvalidValues(t *testing.T) {
	t.Setenv("MEDIA_PRESIGN_EXPIRY", "not-a-duration")
	_, err := LoadConfig()
	assert.Error(t, err, "an unparsable expiry must not silently fall back to the default")

	t.Setenv("MEDIA_PRESIGN_EXPIRY", "5m")
	t.Setenv("MEDIA_MAX_OBJECTS_PER_USER", "not-a-number")
	_, err = LoadConfig()
	assert.Error(t, err, "an unparsable quota must not silently fall back to the default")
}

func TestLoadConfig_RejectsUnparsableUseSSL(t *testing.T) {
	// MINIO_USE_SSL controls TLS to the object store: silently reading a typo
	// like "flase" as false would downgrade the connection to plaintext. The
	// function's contract is that unparsable overrides are errors.
	t.Setenv("MINIO_USE_SSL", "flase")

	_, err := LoadConfig()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "MINIO_USE_SSL")
}

func TestLoadConfig_ParsesBoolShorthandsForUseSSL(t *testing.T) {
	// strconv.ParseBool semantics, like the bool env parsing in auth-service
	// and api-gateway: "1" is true, not a value quietly read as false.
	t.Setenv("MINIO_USE_SSL", "1")

	cfg, err := LoadConfig()

	require.NoError(t, err)
	assert.True(t, cfg.UseSSL)
}

func TestDisabledStore_IsUnavailableAndFailsLoudly(t *testing.T) {
	s := DisabledStore{}

	assert.False(t, s.Available())

	// The handler layer gates on Available(), so these should never be
	// reached in production — but a miswired route must error, not
	// silently pretend to have presigned.
	_, err := s.PresignPut(t.Context(), "user-1/uuid/a.mp3", time.Minute)
	assert.Error(t, err)
	err = s.Remove(t.Context(), "user-1/uuid/a.mp3")
	assert.Error(t, err)
}

func TestDisabledStore_SatisfiesHandlerObjectStore(t *testing.T) {
	// Compile-time check that the 503 path's store can be injected where
	// NewMediaHandler expects an ObjectStore.
	var _ handlers.ObjectStore = DisabledStore{}
}
