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

package altstream

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// fakeAltstream serves the REST bootstrap and an SSE stream whose behaviour
// is scripted per connection (n counts connections from 0).
type fakeAltstream struct {
	history string // body of GET /api/chat; "" answers 500
	stream  func(n int, w http.ResponseWriter, r *http.Request)

	mu       sync.Mutex
	lastIDs  []string
	connects []time.Time
}

func (f *fakeAltstream) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/api/chat":
		if f.history == "" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(f.history))
	case "/api/v1/channels/u/emotes":
		_, _ = w.Write([]byte(`{"data":{"channel":"u","emotes":[]}}`))
	case "/api/chat/stream":
		f.mu.Lock()
		n := len(f.connects)
		f.lastIDs = append(f.lastIDs, r.URL.Query().Get("lastId"))
		f.connects = append(f.connects, time.Now())
		f.mu.Unlock()
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		f.stream(n, w, r)
	default:
		http.NotFound(w, r)
	}
}

func (f *fakeAltstream) snapshot() ([]string, []time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.lastIDs...), append([]time.Time(nil), f.connects...)
}

func send(w http.ResponseWriter, line string) {
	_, _ = fmt.Fprintf(w, "%s\n\n", line)
	w.(http.Flusher).Flush()
}

func msg(id string, sentAt time.Time) string {
	return fmt.Sprintf(`{"type":"message","id":%q,"author":"A","authorUsername":"a","text":"t%s","sentAt":%d,"channel":"u"}`,
		id, id, sentAt.UnixMilli())
}

// recorder collects the ids the client hands to its handler.
type recorder struct {
	mu  sync.Mutex
	ids []string
}

func (r *recorder) handle(_ string, ev *Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ids = append(r.ids, ev.ID)
}

func (r *recorder) got() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.ids...)
}

func startClient(t *testing.T, f *fakeAltstream) (*Client, *recorder) {
	t.Helper()
	srv := httptest.NewServer(f)
	rec := &recorder{}
	c := NewClient(srv.URL, rec.handle, zap.NewNop())
	c.backoff = func(int) time.Duration { return 300 * time.Millisecond }
	t.Cleanup(func() {
		c.DisconnectAll()
		srv.Close()
	})
	return c, rec
}

func TestStream_BootstrapsLastIDAndDropsCatchUp(t *testing.T) {
	now := time.Now()
	f := &fakeAltstream{
		history: fmt.Sprintf(`[%s]`, msg("100", now.Add(-time.Hour))),
		stream: func(_ int, w http.ResponseWriter, r *http.Request) {
			// The stream replays old messages first, then goes live.
			send(w, fmt.Sprintf("data: [%s,%s]", msg("99", now.Add(-time.Hour)), msg("100", now.Add(-time.Minute))))
			send(w, ": ping")
			send(w, fmt.Sprintf("data: [%s]", msg("101", time.Now())))
			<-r.Context().Done()
		},
	}
	c, rec := startClient(t, f)

	require.Error(t, c.Connect("not a channel!"))
	require.NoError(t, c.Connect("U"))
	require.Eventually(t, func() bool { return len(rec.got()) == 1 }, 2*time.Second, 10*time.Millisecond)
	assert.Equal(t, []string{"101"}, rec.got())
	assert.True(t, c.IsConnected("u"))

	lastIDs, _ := f.snapshot()
	assert.Equal(t, []string{"100"}, lastIDs, "lastId bootstraps from the newest REST message")

	c.Disconnect("u")
	assert.False(t, c.IsConnected("u"))
}

func TestStream_EmptyOrFailedHistoryUsesSentinel(t *testing.T) {
	for name, history := range map[string]string{"empty": "[]", "rest error": ""} {
		t.Run(name, func(t *testing.T) {
			f := &fakeAltstream{history: history, stream: func(_ int, _ http.ResponseWriter, r *http.Request) {
				<-r.Context().Done()
			}}
			c, _ := startClient(t, f)
			require.NoError(t, c.Connect("u"))
			require.Eventually(t, func() bool { ids, _ := f.snapshot(); return len(ids) == 1 }, 2*time.Second, 10*time.Millisecond)
			ids, _ := f.snapshot()
			assert.Equal(t, sentinelLastID, ids[0])
			assert.NotEmpty(t, ids[0], "an empty lastId makes the stream send only pings")
		})
	}
}

func TestStream_ReconnectsWithLastSeenIDAndDedupes(t *testing.T) {
	var closedAtMs atomic.Int64
	f := &fakeAltstream{
		history: "[]",
		stream: func(n int, w http.ResponseWriter, r *http.Request) {
			if n == 0 {
				send(w, fmt.Sprintf("data: [%s]", msg("101", time.Now())))
				closedAtMs.Store(time.Now().UnixMilli())
				return // server drops the stream
			}
			// The reconnect replays what it thinks we missed: a duplicate,
			// a message sent in the gap, and one far older than the gap.
			closedAt := time.UnixMilli(closedAtMs.Load())
			send(w, fmt.Sprintf("data: [%s,%s,%s]", msg("101", time.Now()), msg("102", closedAt), msg("103", closedAt.Add(-time.Hour))))
			<-r.Context().Done()
		},
	}
	c, rec := startClient(t, f)
	c.catchUpGrace = 100 * time.Millisecond // shorter than the 300ms backoff gap

	require.NoError(t, c.Connect("u"))
	require.Eventually(t, func() bool { return len(rec.got()) == 2 }, 3*time.Second, 10*time.Millisecond)
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, []string{"101", "102"}, rec.got())

	lastIDs, _ := f.snapshot()
	assert.Equal(t, []string{sentinelLastID, "101"}, lastIDs, "reconnect resumes from the last seen id")
}

func TestStream_SilentStreamIsStaleAndReconnects(t *testing.T) {
	f := &fakeAltstream{
		history: "[]",
		stream: func(n int, w http.ResponseWriter, r *http.Request) {
			if n == 0 {
				// Pings keep the stream alive past the stale window...
				for i := 0; i < 12; i++ {
					send(w, ": ping")
					time.Sleep(50 * time.Millisecond)
				}
			}
			// ...then it goes silent without closing.
			<-r.Context().Done()
		},
	}
	c, _ := startClient(t, f)
	c.staleAfter = 300 * time.Millisecond
	c.backoff = func(int) time.Duration { return 10 * time.Millisecond }

	require.NoError(t, c.Connect("u"))
	require.Eventually(t, func() bool { _, cs := f.snapshot(); return len(cs) >= 2 }, 3*time.Second, 10*time.Millisecond)
	_, connects := f.snapshot()
	assert.GreaterOrEqual(t, connects[1].Sub(connects[0]), 600*time.Millisecond, "pings must reset the stale timer")
}
