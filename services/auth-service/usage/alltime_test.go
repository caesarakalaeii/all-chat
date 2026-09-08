// This file is part of All-Chat.
// Copyright (C) 2026 caesarakalaeii
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published
// by the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package usage

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type fakeAllTimeStore struct {
	getErr error
	setErr error

	stored map[string]int64
	setTo  map[string]int64
}

func newFakeAllTimeStore() *fakeAllTimeStore {
	return &fakeAllTimeStore{stored: map[string]int64{}, setTo: map[string]int64{}}
}

func (f *fakeAllTimeStore) GetLendingStat(_ context.Context, key string) (int64, error) {
	if f.getErr != nil {
		return 0, f.getErr
	}
	return f.stored[key], nil
}

func (f *fakeAllTimeStore) SetLandingStat(_ context.Context, key string, value int64) error {
	if f.setErr != nil {
		return f.setErr
	}
	f.setTo[key] = value
	return nil
}

// redisKey reads a miniredis key as int64, failing the test on absence or
// parse errors — the tests never read a key they did not just set.
func redisKey(t *testing.T, mr *miniredis.Miniredis, key string) int64 {
	t.Helper()
	v, err := mr.Get(key)
	require.NoError(t, err)
	n, err := strconv.ParseInt(v, 10, 64)
	require.NoError(t, err)
	return n
}

func newAllTimeKeeper(t *testing.T, store *fakeAllTimeStore) (*AllTimeStatsKeeper, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	redis := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return NewAllTimeStatsKeeper(redis, store, zap.NewNop(), 0), mr
}

func TestReconcile_RedisAheadPersistsLarger(t *testing.T) {
	store := newFakeAllTimeStore()
	store.stored["all_time_messages"] = 500
	k, mr := newAllTimeKeeper(t, store)
	mr.Set(AllTimeMessagesKey, "900")

	require.NoError(t, k.Reconcile(context.Background()))

	assert.EqualValues(t, 900, store.setTo["all_time_messages"])
	assert.EqualValues(t, 900, redisKey(t, mr, AllTimeMessagesKey), "the counter is not touched")
}

func TestReconcile_RedisBehindRestoresFromStore(t *testing.T) {
	store := newFakeAllTimeStore()
	store.stored["all_time_messages"] = 900
	k, mr := newAllTimeKeeper(t, store)
	mr.Set(AllTimeMessagesKey, "100")

	require.NoError(t, k.Reconcile(context.Background()))

	assert.EqualValues(t, 900, redisKey(t, mr, AllTimeMessagesKey), "a flushed counter is repaired from the durable row")
	assert.Empty(t, store.setTo, "the row already holds the larger truth; no rewrite")
}

func TestReconcile_EqualIsANoOp(t *testing.T) {
	store := newFakeAllTimeStore()
	store.stored["all_time_messages"] = 900
	k, mr := newAllTimeKeeper(t, store)
	mr.Set(AllTimeMessagesKey, "900")

	require.NoError(t, k.Reconcile(context.Background()))

	assert.Empty(t, store.setTo)
	assert.EqualValues(t, 900, redisKey(t, mr, AllTimeMessagesKey))
}

func TestReconcile_MissingRedisKeyCountsAsZero(t *testing.T) {
	store := newFakeAllTimeStore()
	store.stored["all_time_messages"] = 900
	k, mr := newAllTimeKeeper(t, store)
	require.False(t, mr.Exists(AllTimeMessagesKey))

	require.NoError(t, k.Reconcile(context.Background()))

	assert.True(t, mr.Exists(AllTimeMessagesKey), "a cold Redis is seeded with the durable total")
	assert.EqualValues(t, 900, redisKey(t, mr, AllTimeMessagesKey))
}

func TestReconcile_StoreErrorIsReturned(t *testing.T) {
	store := newFakeAllTimeStore()
	store.getErr = errors.New("db down")
	k, _ := newAllTimeKeeper(t, store)

	assert.Error(t, k.Reconcile(context.Background()))
}

func TestReconcile_SetErrorIsReturned(t *testing.T) {
	store := newFakeAllTimeStore()
	store.setErr = errors.New("db down")
	k, mr := newAllTimeKeeper(t, store)
	mr.Set(AllTimeMessagesKey, "900")

	// The keeper must not lose the larger Redis value on a transient store
	// error: the error is surfaced and the next tick retries.
	assert.Error(t, k.Reconcile(context.Background()))
}
