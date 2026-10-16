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

package goals

import (
	"context"
	"net/http"
	"testing"

	"github.com/caesar/all-chat/services/alert-processor/repository"
	"github.com/caesar/all-chat/shared/youtubetoken"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeYouTubeTokens struct {
	cred      *youtubetoken.YouTubeCredential
	err       error
	refreshed int
}

func (f *fakeYouTubeTokens) Resolve(context.Context, string, string) (*youtubetoken.YouTubeCredential, error) {
	return f.cred, f.err
}

func (f *fakeYouTubeTokens) Refresh(_ context.Context, cred *youtubetoken.YouTubeCredential) error {
	f.refreshed++
	cred.AccessToken = "fresh"
	return nil
}

// fakeQuota records the reserve/confirm/rollback ledger the shared
// youtube_quota_usage table would see (ADR-0006).
type fakeQuota struct {
	deny                            bool
	reserved, confirmed, rolledBack int
}

func (q *fakeQuota) Reserve(_ context.Context, units int) (bool, error) {
	if q.deny {
		return false, nil
	}
	q.reserved += units
	return true, nil
}
func (q *fakeQuota) Confirm(_ context.Context, units int) error  { q.confirmed += units; return nil }
func (q *fakeQuota) Rollback(_ context.Context, units int) error { q.rolledBack += units; return nil }

var ytGoal = repository.PolledGoal{OverlayID: "ov", OwnerUserID: "user-1", Metric: "subscribers", Platform: "youtube", ChannelID: "UCabc"}

func TestYouTubeCounter_SpendsOneConfirmedUnit(t *testing.T) {
	q := &fakeQuota{}
	url := helix(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/channels", r.URL.Path)
		assert.Equal(t, "statistics", r.URL.Query().Get("part"))
		assert.Equal(t, "UCabc", r.URL.Query().Get("id"))
		assert.Equal(t, "Bearer tok", r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`{"items":[{"statistics":{"subscriberCount":"4560","hiddenSubscriberCount":false}}]}`))
	})
	c := &YouTubeCounter{Tokens: &fakeYouTubeTokens{cred: &youtubetoken.YouTubeCredential{AccessToken: "tok"}}, Quota: q, BaseURL: url}
	n, err := c.Count(context.Background(), ytGoal)
	require.NoError(t, err)
	assert.EqualValues(t, 4560, n)
	assert.Equal(t, fakeQuota{reserved: 1, confirmed: 1}, *q)
}

func TestYouTubeCounter_QuotaRefusalMakesNoCall(t *testing.T) {
	called := false
	url := helix(t, func(w http.ResponseWriter, r *http.Request) { called = true })
	c := &YouTubeCounter{Tokens: &fakeYouTubeTokens{cred: &youtubetoken.YouTubeCredential{AccessToken: "tok"}}, Quota: &fakeQuota{deny: true}, BaseURL: url}
	_, err := c.Count(context.Background(), ytGoal)
	assert.ErrorIs(t, err, ErrQuota)
	assert.False(t, called, "no reservation, no Data API call")
}

func TestYouTubeCounter_401RollsBackRefreshesAndRetries(t *testing.T) {
	q := &fakeQuota{}
	url := helix(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fresh" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"items":[{"statistics":{"subscriberCount":"10"}}]}`))
	})
	src := &fakeYouTubeTokens{cred: &youtubetoken.YouTubeCredential{AccessToken: "stale"}}
	n, err := (&YouTubeCounter{Tokens: src, Quota: q, BaseURL: url}).Count(context.Background(), ytGoal)
	require.NoError(t, err)
	assert.EqualValues(t, 10, n)
	assert.Equal(t, 1, src.refreshed)
	assert.Equal(t, fakeQuota{reserved: 2, confirmed: 1, rolledBack: 1}, *q)
}

func TestYouTubeCounter_HiddenAndErrors(t *testing.T) {
	q := &fakeQuota{}
	url := helix(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"items":[{"statistics":{"hiddenSubscriberCount":true}}]}`))
	})
	tok := &fakeYouTubeTokens{cred: &youtubetoken.YouTubeCredential{AccessToken: "tok"}}
	_, err := (&YouTubeCounter{Tokens: tok, Quota: q, BaseURL: url}).Count(context.Background(), ytGoal)
	assert.ErrorIs(t, err, ErrHidden)
	assert.Equal(t, 1, q.confirmed, "the call happened, so its unit is spent")

	q = &fakeQuota{}
	url = helix(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) })
	_, err = (&YouTubeCounter{Tokens: tok, Quota: q, BaseURL: url}).Count(context.Background(), ytGoal)
	require.Error(t, err)
	assert.Equal(t, fakeQuota{reserved: 1, rolledBack: 1}, *q, "a failed call rolls its unit back")

	_, err = (&YouTubeCounter{Tokens: &fakeYouTubeTokens{err: youtubetoken.ErrNoCredential}, Quota: &fakeQuota{}, BaseURL: url}).Count(context.Background(), ytGoal)
	assert.ErrorIs(t, err, ErrReconnect)
}
