package enricher

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/caesar/all-chat/services/message-processor/models"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// pronounsMapJSON is the mock /v1/pronouns response.
var pronounsMapJSON = `{
	"hehim":   {"name":"hehim",   "subject":"He",   "object":"Him",   "singular":false},
	"sheher":  {"name":"sheher",  "subject":"She",  "object":"Her",   "singular":false},
	"theythem":{"name":"theythem","subject":"They", "object":"Them",  "singular":false},
	"any":     {"name":"any",     "subject":"Any",  "object":"Any",   "singular":true},
	"other":   {"name":"other",   "subject":"Other","object":"Other", "singular":true}
}`

// newTestPronounServer creates a mock Alejo API server.
// The server URL is used directly as the baseURL (which replaces alejoAPIBaseURL).
// Paths are registered WITHOUT the /v1 prefix since the enricher builds:
//
//	fmt.Sprintf("%s/users/%s", baseURL, login)
//	fmt.Sprintf("%s/pronouns", baseURL)
func newTestPronounServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/pronouns", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(pronounsMapJSON))
	})
	mux.HandleFunc("/users/pajlada", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"channel_id":"11148817","channel_login":"pajlada","pronoun_id":"hehim","alt_pronoun_id":null}`))
	})
	mux.HandleFunc("/users/shetheyperson", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"channel_id":"2","channel_login":"shetheyperson","pronoun_id":"sheher","alt_pronoun_id":"theythem"}`))
	})
	mux.HandleFunc("/users/anyperson", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"channel_id":"3","channel_login":"anyperson","pronoun_id":"any","alt_pronoun_id":null}`))
	})
	mux.HandleFunc("/users/unknown", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	return httptest.NewServer(mux)
}

// newTestPronounEnricher creates a PronounEnricher pointed at the mock server.
func newTestPronounEnricher(t *testing.T, mr *miniredis.Miniredis, serverURL string) *PronounEnricher {
	t.Helper()
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rc.Close() })

	// Parse pronouns map from mock server
	pronounsMap := map[string]alejoPronounDef{}
	if err := json.Unmarshal([]byte(pronounsMapJSON), &pronounsMap); err != nil {
		t.Fatalf("failed to parse test pronouns map: %v", err)
	}

	return newPronounEnricherWithURL(rc, zap.NewNop(), serverURL, pronounsMap)
}

// makeTwitchMsg creates a Twitch UnifiedChatMessage for testing.
func makeTwitchMsg(username string) *models.UnifiedChatMessage {
	return &models.UnifiedChatMessage{
		Platform: "twitch",
		User: models.UserInfo{
			ID:       "123456",
			Username: username,
		},
	}
}

// makeKickMsg creates a Kick UnifiedChatMessage with a linked TwitchUsername.
func makeKickMsg(kickUserID, twitchUsername string) *models.UnifiedChatMessage {
	return &models.UnifiedChatMessage{
		Platform: "kick",
		User: models.UserInfo{
			ID:             kickUserID,
			Username:       "kickuser",
			TwitchUsername: twitchUsername,
		},
	}
}

func TestPronounEnricher_TwitchMessage_HehimPronouns(t *testing.T) {
	mr := miniredis.RunT(t)
	srv := newTestPronounServer(t)
	defer srv.Close()
	e := newTestPronounEnricher(t, mr, srv.URL)

	msg := makeTwitchMsg("pajlada")
	if err := e.Enrich(context.Background(), msg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.User.Pronouns != "he/him" {
		t.Errorf("expected pronouns %q, got %q", "he/him", msg.User.Pronouns)
	}
}

func TestPronounEnricher_NonTwitch_WithTwitchUsername(t *testing.T) {
	mr := miniredis.RunT(t)
	srv := newTestPronounServer(t)
	defer srv.Close()
	e := newTestPronounEnricher(t, mr, srv.URL)

	msg := makeKickMsg("kick-987", "pajlada")
	if err := e.Enrich(context.Background(), msg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.User.Pronouns != "he/him" {
		t.Errorf("expected pronouns %q for non-Twitch with linked username, got %q", "he/him", msg.User.Pronouns)
	}
}

func TestPronounEnricher_NonTwitch_NoTwitchUsername_Skips(t *testing.T) {
	mr := miniredis.RunT(t)
	srv := newTestPronounServer(t)
	defer srv.Close()
	e := newTestPronounEnricher(t, mr, srv.URL)

	msg := makeKickMsg("kick-111", "") // no linked Twitch account
	if err := e.Enrich(context.Background(), msg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.User.Pronouns != "" {
		t.Errorf("expected empty pronouns when no TwitchUsername, got %q", msg.User.Pronouns)
	}
}

func TestPronounEnricher_404_CachesEmptySentinel(t *testing.T) {
	mr := miniredis.RunT(t)
	srv := newTestPronounServer(t)
	defer srv.Close()
	e := newTestPronounEnricher(t, mr, srv.URL)

	msg := makeTwitchMsg("unknown")
	if err := e.Enrich(context.Background(), msg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.User.Pronouns != "" {
		t.Errorf("expected empty pronouns for 404 user, got %q", msg.User.Pronouns)
	}

	// Verify empty sentinel was cached
	cacheKey := PronounCacheKeyPrefix + "unknown"
	cached, err := mr.Get(cacheKey)
	if err != nil {
		t.Fatalf("expected empty sentinel in cache, got error: %v", err)
	}
	if cached != "" {
		t.Errorf("expected empty string sentinel in cache, got %q", cached)
	}
}

func TestPronounEnricher_NetworkError_SilentSkip(t *testing.T) {
	mr := miniredis.RunT(t)
	// Point to a URL that will immediately fail
	e := newTestPronounEnricher(t, mr, "http://127.0.0.1:1") // nothing listening

	msg := makeTwitchMsg("pajlada")
	if err := e.Enrich(context.Background(), msg); err != nil {
		t.Fatalf("expected nil error on network failure (D-05 silent skip), got: %v", err)
	}
	if msg.User.Pronouns != "" {
		t.Errorf("expected empty pronouns on network error, got %q", msg.User.Pronouns)
	}
}

func TestPronounEnricher_CacheHit_ReturnsCachedPronoun(t *testing.T) {
	mr := miniredis.RunT(t)
	srv := newTestPronounServer(t)
	defer srv.Close()
	e := newTestPronounEnricher(t, mr, srv.URL)

	// Pre-populate cache
	cacheKey := PronounCacheKeyPrefix + "pajlada"
	mr.Set(cacheKey, "she/her")
	mr.SetTTL(cacheKey, 24*time.Hour)

	msg := makeTwitchMsg("pajlada")
	if err := e.Enrich(context.Background(), msg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should get "she/her" from cache, not "he/him" from API
	if msg.User.Pronouns != "she/her" {
		t.Errorf("expected cached pronouns %q, got %q", "she/her", msg.User.Pronouns)
	}
}

func TestPronounEnricher_CacheHit_EmptySentinel_NoPronouns(t *testing.T) {
	mr := miniredis.RunT(t)
	srv := newTestPronounServer(t)
	defer srv.Close()
	e := newTestPronounEnricher(t, mr, srv.URL)

	// Pre-populate with empty sentinel (means no pronouns set)
	cacheKey := PronounCacheKeyPrefix + "pajlada"
	mr.Set(cacheKey, pronounEmptySentinel)
	mr.SetTTL(cacheKey, 24*time.Hour)

	msg := makeTwitchMsg("pajlada")
	if err := e.Enrich(context.Background(), msg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.User.Pronouns != "" {
		t.Errorf("expected empty pronouns from empty sentinel cache, got %q", msg.User.Pronouns)
	}
}

func TestPronounEnricher_AltPronounID_SheTheyProduces(t *testing.T) {
	mr := miniredis.RunT(t)
	srv := newTestPronounServer(t)
	defer srv.Close()
	e := newTestPronounEnricher(t, mr, srv.URL)

	msg := makeTwitchMsg("shetheyperson")
	if err := e.Enrich(context.Background(), msg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.User.Pronouns != "she/they" {
		t.Errorf("expected pronouns %q for she/they combo, got %q", "she/they", msg.User.Pronouns)
	}
}

func TestPronounEnricher_SingularPronoun_Any(t *testing.T) {
	mr := miniredis.RunT(t)
	srv := newTestPronounServer(t)
	defer srv.Close()
	e := newTestPronounEnricher(t, mr, srv.URL)

	msg := makeTwitchMsg("anyperson")
	if err := e.Enrich(context.Background(), msg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.User.Pronouns != "any" {
		t.Errorf("expected singular pronoun %q, got %q", "any", msg.User.Pronouns)
	}
}

// TestPronounEnricher_CacheTTL verifies that cached entries have 24h TTL.
func TestPronounEnricher_CacheTTL(t *testing.T) {
	mr := miniredis.RunT(t)
	srv := newTestPronounServer(t)
	defer srv.Close()
	e := newTestPronounEnricher(t, mr, srv.URL)

	msg := makeTwitchMsg("pajlada")
	if err := e.Enrich(context.Background(), msg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cacheKey := PronounCacheKeyPrefix + "pajlada"
	ttl := mr.TTL(cacheKey)
	if ttl <= 0 || ttl > PronounCacheTTL+time.Second {
		t.Errorf("unexpected TTL %v, expected ~%v", ttl, PronounCacheTTL)
	}
}
