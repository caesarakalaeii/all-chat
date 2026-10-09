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
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Env var names, documented in services/media-service/README.md.
const (
	envEndpoint    = "MINIO_ENDPOINT"
	envAccessKey   = "MINIO_MEDIA_USER"
	envSecretKey   = "MINIO_MEDIA_PASSWORD"
	envUseSSL      = "MINIO_USE_SSL"
	envBucket      = "MINIO_BUCKET"
	envPublicBase  = "MEDIA_PUBLIC_URL"
	envPresignTTL  = "MEDIA_PRESIGN_EXPIRY"
	envMaxObjects  = "MEDIA_MAX_OBJECTS_PER_USER"
	defaultBucket  = "allchat-media"
	defaultBaseURL = "https://media.allch.at"
)

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

// LoadConfig reads the storage settings from the environment. An empty
// Endpoint is NOT an error: it means MinIO is not deployed (or not wired to
// this service yet) and the media routes answer 503 — same env-gating pattern
// as youtube-listener-innertube's optional subscribers. Everything else
// defaults to the ADR-0064 values; unparsable overrides are errors, because a
// silently-ignored MEDIA_MAX_OBJECTS_PER_USER is a quota that isn't enforced
// and a silently-ignored MINIO_USE_SSL is TLS that isn't on.
func LoadConfig() (Config, error) {
	useSSL, err := strconv.ParseBool(envDefault(envUseSSL, "true"))
	if err != nil {
		return Config{}, fmt.Errorf("%s: %w", envUseSSL, err)
	}

	cfg := Config{
		Endpoint:      strings.TrimSpace(os.Getenv(envEndpoint)),
		AccessKey:     os.Getenv(envAccessKey),
		SecretKey:     os.Getenv(envSecretKey),
		UseSSL:        useSSL,
		Bucket:        envDefault(envBucket, defaultBucket),
		PublicBaseURL: strings.TrimSuffix(envDefault(envPublicBase, defaultBaseURL), "/"),
	}

	expiry, err := time.ParseDuration(envDefault(envPresignTTL, "5m"))
	if err != nil {
		return Config{}, fmt.Errorf("%s: %w", envPresignTTL, err)
	}
	cfg.PresignExpiry = expiry

	maxObjects, err := strconv.Atoi(envDefault(envMaxObjects, "100"))
	if err != nil {
		return Config{}, fmt.Errorf("%s: %w", envMaxObjects, err)
	}
	cfg.MaxObjectsPerUser = maxObjects

	return cfg, nil
}

func envDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// ErrDisabled is returned by DisabledStore operations; they are unreachable
// behind the RequireStore gate but must not pretend to succeed if a future
// route forgets the gate.
var ErrDisabled = errors.New("media storage is not configured")

// MinioStore talks to MinIO for the media bucket. Thin on purpose: all policy
// (validation, quota, ownership) lives in the handlers so it can be tested
// without a bucket.
type MinioStore struct {
	client *minio.Client
	bucket string
}

// NewMinioStore connects to MinIO. It does not create the bucket: the bucket
// is provisioned with the MinIO instance itself (issue #948), and a service
// pod must not own cluster state.
func NewMinioStore(cfg Config) (*MinioStore, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("minio client: %w", err)
	}
	return &MinioStore{client: client, bucket: cfg.Bucket}, nil
}

func (s *MinioStore) Available() bool { return s != nil }

// PresignPut returns a presigned PUT URL for objectKey, valid for expiry.
// The URL does not pin the content type or size — those are validated at
// presign time and not enforced by single-node MinIO after the fact.
func (s *MinioStore) PresignPut(ctx context.Context, objectKey string, expiry time.Duration) (string, error) {
	url, err := s.client.PresignedPutObject(ctx, s.bucket, objectKey, expiry)
	if err != nil {
		return "", fmt.Errorf("presigned put %s/%s: %w", s.bucket, objectKey, err)
	}
	return url.String(), nil
}

func (s *MinioStore) Remove(ctx context.Context, objectKey string) error {
	if err := s.client.RemoveObject(ctx, s.bucket, objectKey, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("remove %s/%s: %w", s.bucket, objectKey, err)
	}
	return nil
}

// DisabledStore is the no-MinIO fallback: every media route serves 503
// through it.
type DisabledStore struct{}

func (s DisabledStore) Available() bool { return false }

// PresignPut always fails: there is no bucket to upload to.
func (s DisabledStore) PresignPut(_ context.Context, _ string, _ time.Duration) (string, error) {
	return "", ErrDisabled
}

// Remove always fails: there is no bucket to remove from.
func (s DisabledStore) Remove(_ context.Context, _ string) error { return ErrDisabled }
