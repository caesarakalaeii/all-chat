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

package channels

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// fakeStreamAPI emulates GET /api/4/stream/<key>: "miker" -> 5, "revolly1" -> 13427.
func fakeStreamAPI(t *testing.T, hits *atomic.Int64) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/stream/", func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		key := r.URL.Path[len("/stream/"):]
		w.Header().Set("Content-Type", "application/json")
		switch key {
		case "miker":
			_ = json.NewEncoder(w).Encode(StreamResponse{ID: 5, ChannelKey: "Miker", Online: true})
		case "revolly1":
			_ = json.NewEncoder(w).Encode(StreamResponse{ID: 13427, ChannelKey: "Revolly1", Online: false})
		case "broken":
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("boom"))
		case "emptyid":
			_, _ = w.Write([]byte(`{"id":0,"channelkey":"emptyid"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestResolver_ResolvesSlugToNumericID(t *testing.T) {
	var hits atomic.Int64
	srv := fakeStreamAPI(t, &hits)
	r := NewResolverWithBase(srv.URL+"/stream/", srv.Client(), zap.NewNop())

	id, err := r.Resolve(context.Background(), "miker")
	require.NoError(t, err)
	assert.Equal(t, int64(5), id)

	id, err = r.Resolve(context.Background(), "revolly1")
	require.NoError(t, err)
	assert.Equal(t, int64(13427), id)
}

func TestResolver_CachesWithTTL(t *testing.T) {
	var hits atomic.Int64
	srv := fakeStreamAPI(t, &hits)
	r := NewResolverWithBase(srv.URL+"/stream/", srv.Client(), zap.NewNop())
	ctx := context.Background()

	require.Equal(t, int64(5), mustResolve(t, r, ctx, "miker"))
	require.Equal(t, int64(5), mustResolve(t, r, ctx, "miker"))
	require.Equal(t, int64(5), mustResolve(t, r, ctx, "miker"))
	assert.Equal(t, int64(1), hits.Load(), "repeat resolves must hit the cache, not the API")

	// Expire the entry manually: the next resolve refetches.
	r.mu.Lock()
	r.entries["miker"] = resolveEntry{id: 5, expiresAt: time.Now().Add(-time.Minute)}
	r.mu.Unlock()

	require.Equal(t, int64(5), mustResolve(t, r, ctx, "miker"))
	assert.Equal(t, int64(2), hits.Load(), "expired entry must trigger a refetch")
}

func TestResolver_ResolveFreshBypassesCache(t *testing.T) {
	var hits atomic.Int64
	srv := fakeStreamAPI(t, &hits)
	r := NewResolverWithBase(srv.URL+"/stream/", srv.Client(), zap.NewNop())
	ctx := context.Background()

	require.Equal(t, int64(5), mustResolve(t, r, ctx, "miker"))
	assert.Equal(t, int64(1), hits.Load())

	require.Equal(t, int64(5), mustResolveFresh(t, r, ctx, "miker"))
	assert.Equal(t, int64(2), hits.Load(), "ResolveFresh must bypass the cache")
}

func TestResolver_NotFound(t *testing.T) {
	var hits atomic.Int64
	srv := fakeStreamAPI(t, &hits)
	r := NewResolverWithBase(srv.URL+"/stream/", srv.Client(), zap.NewNop())

	_, err := r.Resolve(context.Background(), "ghost")
	var nf *NotFoundError
	require.ErrorAs(t, err, &nf)
	assert.Equal(t, "ghost", nf.Key)
}

func TestResolver_TransientErrorIsNotNotFound(t *testing.T) {
	var hits atomic.Int64
	srv := fakeStreamAPI(t, &hits)
	r := NewResolverWithBase(srv.URL+"/stream/", srv.Client(), zap.NewNop())

	_, err := r.Resolve(context.Background(), "broken")
	require.Error(t, err)
	var nf *NotFoundError
	assert.NotErrorAs(t, err, &nf, "500 must not be classified as NotFound")
}

func TestResolver_ZeroIDRejected(t *testing.T) {
	var hits atomic.Int64
	srv := fakeStreamAPI(t, &hits)
	r := NewResolverWithBase(srv.URL+"/stream/", srv.Client(), zap.NewNop())

	_, err := r.Resolve(context.Background(), "emptyid")
	require.Error(t, err)
}

func mustResolve(t *testing.T, r *Resolver, ctx context.Context, slug string) int64 {
	t.Helper()
	id, err := r.Resolve(ctx, slug)
	require.NoError(t, err)
	return id
}

func mustResolveFresh(t *testing.T, r *Resolver, ctx context.Context, slug string) int64 {
	t.Helper()
	id, err := r.ResolveFresh(ctx, slug)
	require.NoError(t, err)
	return id
}
