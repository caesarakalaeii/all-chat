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

package kickevents

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/caesar/all-chat/services/kick-listener/channels"
	"github.com/caesar/all-chat/services/kick-listener/publisher"
)

// fakeKick is Kick's token, channels, public-key and subscription API.
type fakeKick struct {
	t    *testing.T
	key  *rsa.PrivateKey
	mu   sync.Mutex
	subs map[string]fakeSub // subscription id -> sub
	next int
	// postLimit, when set, is how many subscription POSTs succeed before Kick
	// answers 429, as it does to a burst.
	postLimit, posts int
	log              []string
	srv              *httptest.Server
}

type fakeSub struct {
	Broadcaster int
	Event       string
}

var broadcasterIDs = map[string]int{"alice": 11, "bob-b": 22, "gone": 33}

func newFakeKick(t *testing.T) *fakeKick {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeKick{t: t, key: key, subs: map[string]fakeSub{}}
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.PostForm.Get("grant_type") != "client_credentials" || r.PostForm.Get("client_id") != "cid" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "app-token", "expires_in": 3600})
	})
	mux.HandleFunc("/public/v1/public-key", func(w http.ResponseWriter, _ *http.Request) {
		der, _ := x509.MarshalPKIXPublicKey(&f.key.PublicKey)
		pemKey := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]string{"public_key": string(pemKey)}})
	})
	mux.HandleFunc("/public/v1/channels", func(w http.ResponseWriter, r *http.Request) {
		var data []map[string]any
		for _, slug := range r.URL.Query()["slug"] {
			if id, ok := broadcasterIDs[slug]; ok {
				data = append(data, map[string]any{"slug": slug, "broadcaster_user_id": id})
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
	})
	mux.HandleFunc("/public/v1/events/subscriptions", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer app-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		switch r.Method {
		case http.MethodGet:
			data := []map[string]any{}
			for id, s := range f.subs {
				data = append(data, map[string]any{"id": id, "broadcaster_user_id": s.Broadcaster, "event": s.Event})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
		case http.MethodPost:
			f.posts++
			if f.postLimit > 0 && f.posts > f.postLimit {
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}
			var body struct {
				BroadcasterUserID int    `json:"broadcaster_user_id"`
				Method            string `json:"method"`
				Events            []struct {
					Name    string `json:"name"`
					Version int    `json:"version"`
				} `json:"events"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			for _, e := range body.Events {
				f.next++
				f.subs["sub"+string(rune('0'+f.next))] = fakeSub{body.BroadcasterUserID, e.Name}
				f.log = append(f.log, "POST "+e.Name+" "+string(rune('0'+body.BroadcasterUserID/11)))
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
		case http.MethodDelete:
			for _, id := range r.URL.Query()["id"] {
				f.log = append(f.log, "DELETE "+f.subs[id].Event)
				delete(f.subs, id)
			}
			w.WriteHeader(http.StatusNoContent)
		}
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeKick) sign(id, ts string, body []byte) string {
	sum := sha256.Sum256([]byte(id + "." + ts + "." + string(body)))
	sig, err := rsa.SignPKCS1v15(rand.Reader, f.key, crypto.SHA256, sum[:])
	if err != nil {
		f.t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(sig)
}

type fakeStore struct {
	mu   sync.Mutex
	keys map[string]bool
}

func (s *fakeStore) SetNX(_ context.Context, key string, _ time.Duration) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.keys[key] {
		return false, nil
	}
	s.keys[key] = true
	return true, nil
}

type fakeSources struct{ channels []*channels.ActiveChannel }

func (s fakeSources) GetActiveChannels(context.Context) ([]*channels.ActiveChannel, error) {
	return s.channels, nil
}

type capture struct {
	mu  sync.Mutex
	got []*publisher.RawMessage
}

func (c *capture) Publish(_ context.Context, m *publisher.RawMessage) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.got = append(c.got, m)
	return nil
}

func newService(t *testing.T, f *fakeKick, pub *capture) *Service {
	t.Helper()
	gin.SetMode(gin.TestMode)
	return &Service{
		ClientID: "cid", ClientSecret: "secret",
		TokenURL: f.srv.URL + "/oauth/token", APIURL: f.srv.URL,
		Sources: fakeSources{[]*channels.ActiveChannel{
			{OverlayID: "overlay-1", ChannelSlug: "Alice"},
			{OverlayID: "overlay-2", ChannelSlug: "alice"},
			{OverlayID: "overlay-3", ChannelSlug: "bob_b"},
		}},
		Store: &fakeStore{keys: map[string]bool{}}, Publisher: pub,
		HTTP: f.srv.Client(), Logger: zap.NewNop(),
	}
}

func deliver(s *Service, f *fakeKick, eventType, id string, body []byte, signature string) int {
	ts := time.Now().UTC().Format(time.RFC3339)
	if signature == "" {
		signature = f.sign(id, ts, body)
	}
	req := httptest.NewRequest(http.MethodPost, "/webhooks/kick", bytes.NewReader(body))
	req.Header.Set("Kick-Event-Message-Id", id)
	req.Header.Set("Kick-Event-Message-Timestamp", ts)
	req.Header.Set("Kick-Event-Signature", signature)
	req.Header.Set("Kick-Event-Type", eventType)
	rec := httptest.NewRecorder()
	router := gin.New()
	router.POST("/webhooks/kick", s.Webhook)
	router.ServeHTTP(rec, req)
	return rec.Code
}

const followBody = `{"broadcaster":{"user_id":11,"username":"Alice","channel_slug":"alice"},"follower":{"user_id":77,"username":"newfan","channel_slug":"newfan"}}`
const kicksBody = `{"broadcaster":{"user_id":22,"username":"Bob","channel_slug":"bob-b"},"sender":{"user_id":88,"username":"whale","channel_slug":"whale"},"gift":{"amount":500,"name":"Rage Quit","type":"LEVEL_UP","tier":"MID","message":"gg","pinned_time_seconds":600},"created_at":"2025-10-20T04:00:08.634Z"}`

func TestWebhook_FollowReachesEveryOverlayOfTheChannel(t *testing.T) {
	f, pub := newFakeKick(t), &capture{}
	s := newService(t, f, pub)

	if code := deliver(s, f, "channel.followed", "msg-1", []byte(followBody), ""); code != http.StatusOK {
		t.Fatalf("status %d, want 200", code)
	}
	if len(pub.got) != 2 {
		t.Fatalf("published %d, want one per overlay of alice (2)", len(pub.got))
	}
	overlays := []string{pub.got[0].OverlayID, pub.got[1].OverlayID}
	sort.Strings(overlays)
	m := pub.got[0]
	if strings.Join(overlays, ",") != "overlay-1,overlay-2" || m.EventType != "kick_follow" ||
		m.Username != "newfan" || m.UserID != "77" || m.Platform != "kick" || m.MessageID != "kick-webhook-msg-1" {
		t.Errorf("message = %+v, overlays = %v", m, overlays)
	}

	// Kick retries a delivery under the same message id.
	if code := deliver(s, f, "channel.followed", "msg-1", []byte(followBody), ""); code != http.StatusOK {
		t.Fatalf("retry status %d, want 200", code)
	}
	if len(pub.got) != 2 {
		t.Errorf("a retried delivery published again: %d messages", len(pub.got))
	}
}

func TestWebhook_KicksGift(t *testing.T) {
	f, pub := newFakeKick(t), &capture{}
	s := newService(t, f, pub)
	if code := deliver(s, f, "kicks.gifted", "msg-2", []byte(kicksBody), ""); code != http.StatusOK {
		t.Fatalf("status %d", code)
	}
	if len(pub.got) != 1 {
		t.Fatalf("published %d, want 1", len(pub.got))
	}
	m := pub.got[0]
	if m.EventType != "kick_kicks" || m.Username != "whale" || m.Text != "gg" || m.OverlayID != "overlay-3" ||
		m.ChannelID != "bob_b" || m.EventData["kicks"] != 500 || m.EventData["gift_name"] != "Rage Quit" {
		t.Errorf("message = %+v", m)
	}
}

func TestWebhook_RejectsWhatKickDidNotSign(t *testing.T) {
	f, pub := newFakeKick(t), &capture{}
	s := newService(t, f, pub)

	other := newFakeKick(t)
	forged := other.sign("msg-3", time.Now().UTC().Format(time.RFC3339), []byte(followBody))
	if code := deliver(s, f, "channel.followed", "msg-3", []byte(followBody), forged); code != http.StatusUnauthorized {
		t.Errorf("forged signature: status %d, want 401", code)
	}
	if code := deliver(s, f, "channel.followed", "msg-4", []byte(followBody), "not-base64!"); code != http.StatusUnauthorized {
		t.Errorf("garbage signature: status %d, want 401", code)
	}
	if len(pub.got) != 0 {
		t.Errorf("unsigned delivery was published: %+v", pub.got)
	}
}

// A signed event of a type the listener does not use is acknowledged, so Kick
// does not count it as a failed delivery and drop the subscription.
func TestWebhook_AcknowledgesOtherEventTypes(t *testing.T) {
	f, pub := newFakeKick(t), &capture{}
	s := newService(t, f, pub)
	if code := deliver(s, f, "livestream.status.updated", "msg-5", []byte(`{"is_live":true}`), ""); code != http.StatusOK {
		t.Errorf("status %d, want 200", code)
	}
	if len(pub.got) != 0 {
		t.Errorf("published %+v", pub.got)
	}
}

func TestReconcile_SubscribesMissingAndDropsStale(t *testing.T) {
	f := newFakeKick(t)
	// alice already has follows; "gone" (33) is no longer a source.
	f.subs["old1"] = fakeSub{11, "channel.followed"}
	f.subs["old2"] = fakeSub{33, "channel.followed"}
	s := newService(t, f, &capture{})

	if err := s.Reconcile(context.Background()); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	want := map[fakeSub]bool{
		{11, "channel.followed"}: true, {11, "kicks.gifted"}: true,
		{22, "channel.followed"}: true, {22, "kicks.gifted"}: true,
	}
	if len(f.subs) != len(want) {
		t.Fatalf("subscriptions = %v (calls %v)", f.subs, f.log)
	}
	for _, sub := range f.subs {
		if !want[sub] {
			t.Errorf("unexpected subscription %+v (calls %v)", sub, f.log)
		}
	}

	// A second pass inside the lock window does nothing: one pod reconciles.
	before := len(f.log)
	if err := s.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(f.log) != before {
		t.Errorf("second pass called Kick again: %v", f.log[before:])
	}
}

// Kick rate-limits subscription requests. A pass that runs into the limit must
// stop there instead of spending the rest of its requests on more 429s; the
// next pass picks up what is still missing.
func TestReconcile_StopsAtTheRateLimitAndResumesNextPass(t *testing.T) {
	f := newFakeKick(t)
	f.postLimit = 1
	s := newService(t, f, &capture{})
	s.Sources = fakeSources{[]*channels.ActiveChannel{
		{OverlayID: "o1", ChannelSlug: "alice"}, {OverlayID: "o2", ChannelSlug: "bob-b"}, {OverlayID: "o3", ChannelSlug: "gone"},
	}}

	if err := s.Reconcile(context.Background()); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if f.posts != 2 {
		t.Fatalf("POSTs = %d, want 2 (one accepted, one refused, then stop)", f.posts)
	}

	f.postLimit = 0
	s.Store = &fakeStore{keys: map[string]bool{}} // the next interval
	if err := s.Reconcile(context.Background()); err != nil {
		t.Fatalf("second Reconcile: %v", err)
	}
	if len(f.subs) != 6 {
		t.Errorf("subscriptions after the second pass = %v, want both events for all three channels", f.subs)
	}
}
