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

// Package orphans removes uploaded media whose registry row is gone (ADR-0088).
//
// The media_objects row is the record that a file exists for someone. It goes away
// when the user deletes the file, and with the user's account (ON DELETE CASCADE),
// but the bytes sit in a public-read bucket: a deleted account's files stayed
// downloadable at their URL. The sweep makes the bucket follow the registry.
//
// Deleting an object whose row exists can not happen by race: media-service writes
// the row before it hands out the upload URL, so any object the listing sees had its
// row committed before the upload began, and therefore before the lookup.
package orphans

import (
	"context"
	"fmt"
	"iter"
	"regexp"
	"time"

	"go.uber.org/zap"
)

// Store lists and removes bucket objects.
type Store interface {
	Keys(ctx context.Context) iter.Seq2[string, error]
	Remove(ctx context.Context, key string) error
}

// Registry reports which object keys have a media_objects row.
type Registry interface {
	ExistingKeys(ctx context.Context, keys []string) (map[string]bool, error)
}

// batchSize bounds one registry lookup (an = ANY($1) over the unique object_key index).
const batchSize = 500

// mediaKeyPattern matches the keys media-service issues: {user_id}/{uuid}/{filename}. Anything
// else in the bucket was not put there by this service and is left alone.
var mediaKeyPattern = regexp.MustCompile(`^[0-9a-f]{8}(-[0-9a-f]{4}){3}-[0-9a-f]{12}/[0-9a-f]{8}(-[0-9a-f]{4}){3}-[0-9a-f]{12}/.`)

// Sweep removes every media object without a registry row and returns how many it
// removed. A failed listing or registry lookup stops the sweep with nothing more
// removed (a lookup error is never read as "no row"); a failed removal is logged and
// retried by the next sweep.
func Sweep(ctx context.Context, store Store, registry Registry, log *zap.Logger) (int, error) {
	removed := 0
	batch := make([]string, 0, batchSize)
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		existing, err := registry.ExistingKeys(ctx, batch)
		if err != nil {
			return fmt.Errorf("look up media registry: %w", err)
		}
		for _, key := range batch {
			if existing[key] {
				continue
			}
			if err := store.Remove(ctx, key); err != nil {
				log.Warn("Failed to remove orphaned media object", zap.String("object_key", key), zap.Error(err))
				continue
			}
			removed++
		}
		batch = batch[:0]
		return nil
	}

	for key, err := range store.Keys(ctx) {
		if err != nil {
			return removed, fmt.Errorf("list media bucket: %w", err)
		}
		if !mediaKeyPattern.MatchString(key) {
			continue
		}
		batch = append(batch, key)
		if len(batch) == batchSize {
			if err := flush(); err != nil {
				return removed, err
			}
		}
	}
	return removed, flush()
}

// Run sweeps once right away, then every interval until ctx is cancelled.
//
// ponytail: every replica sweeps; removals are idempotent, so the only cost is a
// duplicate bucket listing per interval. Take a Postgres advisory lock if listing
// the bucket ever gets expensive.
func Run(ctx context.Context, interval time.Duration, store Store, registry Registry, log *zap.Logger) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		removed, err := Sweep(ctx, store, registry, log)
		switch {
		case err != nil:
			log.Error("Orphaned media sweep failed", zap.Int("removed", removed), zap.Error(err))
		case removed > 0:
			log.Info("Removed orphaned media objects", zap.Int("removed", removed))
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
