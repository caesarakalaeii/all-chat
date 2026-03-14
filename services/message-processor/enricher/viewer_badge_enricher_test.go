package enricher

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/caesar/all-chat/services/message-processor/models"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// fakeViewerDB is a test double for the DB pool, executing a callback per query.
type fakeViewerDB struct {
	queryFn func(platform, userID string) (viewerID string, nameColor *string, err error)
}

func (f *fakeViewerDB) QueryRow(ctx context.Context, sql string, args ...interface{}) pgxRowScanner {
	platform := fmt.Sprint(args[0])
	userID := fmt.Sprint(args[1])
	vid, nc, err := f.queryFn(platform, userID)
	return &fakeRow{queryResult{viewerID: vid, nameColor: nc, err: err}}
}

type queryResult struct {
	viewerID  string
	nameColor *string
	err       error
}

type fakeRow struct {
	result queryResult
}

func (r *fakeRow) Scan(dest ...interface{}) error {
	if r.result.err != nil {
		return r.result.err
	}
	if len(dest) >= 1 {
		if s, ok := dest[0].(*string); ok {
			*s = r.result.viewerID
		}
	}
	if len(dest) >= 2 {
		if sp, ok := dest[1].(**string); ok {
			*sp = r.result.nameColor
		}
	}
	return nil
}

func newTestEnricher(t *testing.T, mr *miniredis.Miniredis, db viewerDB) *ViewerBadgeEnricher {
	t.Helper()
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rc.Close() })
	return &ViewerBadgeEnricher{
		redis:  rc,
		db:     db,
		logger: zap.NewNop(),
	}
}

func ptr(s string) *string { return &s }

func makeMsg(platform, userID, color string) *models.UnifiedChatMessage {
	return &models.UnifiedChatMessage{
		Platform: platform,
		User: models.UserInfo{
			ID:    userID,
			Color: color,
		},
	}
}

func TestViewerBadgeEnricher_CacheHit_WithColor(t *testing.T) {
	mr := miniredis.RunT(t)
	db := &fakeViewerDB{queryFn: func(_, _ string) (string, *string, error) {
		t.Error("DB should not be called on cache hit")
		return "", nil, nil
	}}
	e := newTestEnricher(t, mr, db)

	cacheKey := "viewer:identity:twitch:user123"
	val, _ := json.Marshal(viewerIdentityCache{ViewerID: "uuid-1", NameColor: ptr("#ff6600")})
	mr.Set(cacheKey, string(val))

	msg := makeMsg("twitch", "user123", "")
	if err := e.Enrich(context.Background(), msg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.User.Color != "#ff6600" {
		t.Errorf("expected color #ff6600, got %q", msg.User.Color)
	}
}

func TestViewerBadgeEnricher_CacheHit_NoColor(t *testing.T) {
	mr := miniredis.RunT(t)
	db := &fakeViewerDB{queryFn: func(_, _ string) (string, *string, error) {
		t.Error("DB should not be called on cache hit")
		return "", nil, nil
	}}
	e := newTestEnricher(t, mr, db)

	cacheKey := "viewer:identity:twitch:user456"
	val, _ := json.Marshal(viewerIdentityCache{ViewerID: "uuid-2", NameColor: nil})
	mr.Set(cacheKey, string(val))

	msg := makeMsg("twitch", "user456", "original")
	if err := e.Enrich(context.Background(), msg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.User.Color != "original" {
		t.Errorf("color should be unchanged, got %q", msg.User.Color)
	}
}

func TestViewerBadgeEnricher_NullSentinel(t *testing.T) {
	mr := miniredis.RunT(t)
	dbCalled := false
	db := &fakeViewerDB{queryFn: func(_, _ string) (string, *string, error) {
		dbCalled = true
		return "", nil, nil
	}}
	e := newTestEnricher(t, mr, db)

	cacheKey := "viewer:identity:kick:user789"
	mr.Set(cacheKey, viewerNullSentinel)

	msg := makeMsg("kick", "user789", "blue")
	if err := e.Enrich(context.Background(), msg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dbCalled {
		t.Error("DB should NOT be called when null sentinel is cached")
	}
	if msg.User.Color != "blue" {
		t.Errorf("color should be unchanged, got %q", msg.User.Color)
	}
}

func TestViewerBadgeEnricher_CacheMiss_ViewerFound(t *testing.T) {
	mr := miniredis.RunT(t)
	db := &fakeViewerDB{queryFn: func(platform, userID string) (string, *string, error) {
		return "viewer-uuid", ptr("#aabbcc"), nil
	}}
	e := newTestEnricher(t, mr, db)

	msg := makeMsg("youtube", "yt-user1", "")
	if err := e.Enrich(context.Background(), msg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.User.Color != "#aabbcc" {
		t.Errorf("expected color #aabbcc, got %q", msg.User.Color)
	}

	// Verify cache was populated
	cacheKey := "viewer:identity:youtube:yt-user1"
	cached, err := mr.Get(cacheKey)
	if err != nil {
		t.Fatalf("expected cache entry, got error: %v", err)
	}
	var identity viewerIdentityCache
	if jsonErr := json.Unmarshal([]byte(cached), &identity); jsonErr != nil {
		t.Fatalf("failed to parse cached value: %v", jsonErr)
	}
	if identity.ViewerID != "viewer-uuid" {
		t.Errorf("expected viewer_id viewer-uuid, got %q", identity.ViewerID)
	}

	// Verify TTL was set
	ttl := mr.TTL(cacheKey)
	if ttl <= 0 || ttl > ViewerIdentityCacheTTL+time.Second {
		t.Errorf("unexpected TTL %v, expected ~%v", ttl, ViewerIdentityCacheTTL)
	}
}

func TestViewerBadgeEnricher_CacheMiss_ViewerNotFound(t *testing.T) {
	mr := miniredis.RunT(t)
	db := &fakeViewerDB{queryFn: func(platform, userID string) (string, *string, error) {
		return "", nil, pgxErrNoRows
	}}
	e := newTestEnricher(t, mr, db)

	msg := makeMsg("tiktok", "tt-user1", "green")
	if err := e.Enrich(context.Background(), msg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.User.Color != "green" {
		t.Errorf("color should be unchanged, got %q", msg.User.Color)
	}

	// Verify null sentinel was cached
	cacheKey := "viewer:identity:tiktok:tt-user1"
	cached, err := mr.Get(cacheKey)
	if err != nil {
		t.Fatalf("expected null sentinel in cache, got error: %v", err)
	}
	if cached != viewerNullSentinel {
		t.Errorf("expected %q sentinel, got %q", viewerNullSentinel, cached)
	}
}

func TestViewerBadgeEnricher_EmptyUserID(t *testing.T) {
	mr := miniredis.RunT(t)
	dbCalled := false
	db := &fakeViewerDB{queryFn: func(_, _ string) (string, *string, error) {
		dbCalled = true
		return "", nil, nil
	}}
	e := newTestEnricher(t, mr, db)
	// Connect a real redis client but ensure no keys are touched
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rc.Close()
	keysBefore, _ := rc.Keys(context.Background(), "*").Result()

	msg := makeMsg("twitch", "", "red")
	if err := e.Enrich(context.Background(), msg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dbCalled {
		t.Error("DB should NOT be called for empty user ID")
	}
	keysAfter, _ := rc.Keys(context.Background(), "*").Result()
	if len(keysAfter) != len(keysBefore) {
		t.Errorf("no Redis keys should be written for empty user ID")
	}
	if msg.User.Color != "red" {
		t.Errorf("color should be unchanged, got %q", msg.User.Color)
	}
}

func TestViewerBadgeEnricher_PlatformPreservesColor(t *testing.T) {
	mr := miniredis.RunT(t)
	storedColor := "#deadbe"
	db := &fakeViewerDB{queryFn: func(platform, userID string) (string, *string, error) {
		return "viewer-uuid", ptr(storedColor), nil
	}}
	e := newTestEnricher(t, mr, db)

	msg := makeMsg("twitch", "user-color-test", "original")
	if err := e.Enrich(context.Background(), msg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.User.Color != storedColor {
		t.Errorf("expected exact stored color %q, got %q", storedColor, msg.User.Color)
	}
}
