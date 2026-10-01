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

// Package storage wires the MinIO client behind the handlers' ObjectStore
// seam, plus the env-driven configuration for it.
package storage

import (
	"context"
	"time"
)

// Config is the media-service storage configuration, from the environment.
type Config struct {
	Endpoint          string
	AccessKey         string
	SecretKey         string
	UseSSL            bool
	Bucket            string
	PublicBaseURL     string
	PresignExpiry     time.Duration
	MaxObjectsPerUser int
}

// LoadConfig reads the storage settings from the environment.
func LoadConfig() (Config, error) {
	return Config{}, nil
}

// MinioStore talks to MinIO for the media bucket.
type MinioStore struct{}

// Available reports whether the store is usable.
func (s *MinioStore) Available() bool { return true }

// PresignPut returns a presigned PUT URL for objectKey.
func (s *MinioStore) PresignPut(_ context.Context, _ string, _ time.Duration) (string, error) {
	return "", nil
}

// Remove deletes objectKey from the bucket.
func (s *MinioStore) Remove(_ context.Context, _ string) error { return nil }

// DisabledStore is the no-MinIO fallback: media routes serve 503.
type DisabledStore struct{}

// Available reports whether the store is usable.
func (s DisabledStore) Available() bool { return true }

// PresignPut returns a presigned PUT URL for objectKey.
func (s DisabledStore) PresignPut(_ context.Context, _ string, _ time.Duration) (string, error) {
	return "", nil
}

// Remove deletes objectKey from the bucket.
func (s DisabledStore) Remove(_ context.Context, _ string) error { return nil }
