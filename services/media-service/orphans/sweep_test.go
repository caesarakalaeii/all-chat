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

package orphans

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type fakeStore struct {
	keys      []string
	listErr   error
	removeErr map[string]error
	removed   []string
}

func (s *fakeStore) Keys(context.Context) iter.Seq2[string, error] {
	return func(yield func(string, error) bool) {
		for _, k := range s.keys {
			if !yield(k, nil) {
				return
			}
		}
		if s.listErr != nil {
			yield("", s.listErr)
		}
	}
}

func (s *fakeStore) Remove(_ context.Context, key string) error {
	if err := s.removeErr[key]; err != nil {
		return err
	}
	s.removed = append(s.removed, key)
	return nil
}

type fakeRegistry struct {
	rows    map[string]bool
	err     error
	batches []int
}

func (r *fakeRegistry) ExistingKeys(_ context.Context, keys []string) (map[string]bool, error) {
	r.batches = append(r.batches, len(keys))
	if r.err != nil {
		return nil, r.err
	}
	out := map[string]bool{}
	for _, k := range keys {
		if r.rows[k] {
			out[k] = true
		}
	}
	return out, nil
}

func mediaKey(name string) string { return uuid.NewString() + "/" + uuid.NewString() + "/" + name }

func TestSweep_RemovesObjectsWithoutRegistryRow(t *testing.T) {
	kept, orphan := mediaKey("horn.mp3"), mediaKey("deleted user.png")
	store := &fakeStore{keys: []string{kept, orphan}}
	reg := &fakeRegistry{rows: map[string]bool{kept: true}}

	removed, err := Sweep(context.Background(), store, reg, zap.NewNop())

	require.NoError(t, err)
	assert.Equal(t, 1, removed)
	assert.Equal(t, []string{orphan}, store.removed)
}

// Only keys media-service issues ({user_id}/{uuid}/{filename}) are its to judge.
func TestSweep_LeavesForeignKeysAlone(t *testing.T) {
	store := &fakeStore{keys: []string{"README.txt", "backups/2026/db.tar", "not-a-uuid/" + uuid.NewString() + "/x.png", uuid.NewString() + "/x.png"}}
	reg := &fakeRegistry{}

	removed, err := Sweep(context.Background(), store, reg, zap.NewNop())

	require.NoError(t, err)
	assert.Zero(t, removed)
	assert.Empty(t, store.removed)
}

// A failed registry lookup must never be read as "no row": that would delete live media.
func TestSweep_RegistryErrorRemovesNothing(t *testing.T) {
	store := &fakeStore{keys: []string{mediaKey("a.png"), mediaKey("b.png")}}
	reg := &fakeRegistry{err: errors.New("db down")}

	_, err := Sweep(context.Background(), store, reg, zap.NewNop())

	require.Error(t, err)
	assert.Empty(t, store.removed)
}

func TestSweep_ListingErrorIsReported(t *testing.T) {
	store := &fakeStore{listErr: errors.New("minio down")}

	_, err := Sweep(context.Background(), store, &fakeRegistry{}, zap.NewNop())

	require.Error(t, err)
}

func TestSweep_RemoveFailureDoesNotStopTheSweep(t *testing.T) {
	a, b := mediaKey("a.png"), mediaKey("b.png")
	store := &fakeStore{keys: []string{a, b}, removeErr: map[string]error{a: errors.New("timeout")}}

	removed, err := Sweep(context.Background(), store, &fakeRegistry{}, zap.NewNop())

	require.NoError(t, err)
	assert.Equal(t, 1, removed)
	assert.Equal(t, []string{b}, store.removed)
}

func TestSweep_LooksUpTheRegistryInBoundedBatches(t *testing.T) {
	var keys []string
	for i := range 2*batchSize + 1 {
		keys = append(keys, mediaKey(fmt.Sprintf("%d.png", i)))
	}
	reg := &fakeRegistry{rows: map[string]bool{}}
	for _, k := range keys {
		reg.rows[k] = true
	}

	removed, err := Sweep(context.Background(), &fakeStore{keys: keys}, reg, zap.NewNop())

	require.NoError(t, err)
	assert.Zero(t, removed)
	assert.Equal(t, []int{batchSize, batchSize, 1}, reg.batches)
}
