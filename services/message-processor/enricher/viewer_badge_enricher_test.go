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
// Phase 31: callback returns isAdmin and isPremium bools.
// Phase 9: callback also returns twitchUsername string.
type fakeViewerDB struct {
	queryFn func(platform, userID string) (viewerID string, nameColor *string, nameGradient []byte, avatarFrameURL string, avatarFlairURL string, isAdmin bool, isPremium bool, twitchUsername string, err error)
}

func (f *fakeViewerDB) QueryRow(ctx context.Context, sql string, args ...interface{}) pgxRowScanner {
	platform := fmt.Sprint(args[0])
	userID := fmt.Sprint(args[1])
	vid, nc, ng, frameURL, flairURL, isAdm, isPrem, twitchUser, err := f.queryFn(platform, userID)
	return &fakeRow{queryResult{viewerID: vid, nameColor: nc, nameGradient: ng, avatarFrameURL: frameURL, avatarFlairURL: flairURL, isAdmin: isAdm, isPremium: isPrem, twitchUsername: twitchUser, err: err}}
}

type queryResult struct {
	viewerID       string
	nameColor      *string
	nameGradient   []byte
	avatarFrameURL string
	avatarFlairURL string
	isAdmin        bool
	isPremium      bool
	twitchUsername string
	err            error
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
	// Phase 29: third dest arg is *[]byte for name_gradient
	if len(dest) >= 3 {
		if bp, ok := dest[2].(*[]byte); ok {
			*bp = r.result.nameGradient
		}
	}
	// Phase 30: fourth dest arg is *string for avatar_frame_url
	if len(dest) >= 4 {
		if sp, ok := dest[3].(*string); ok {
			*sp = r.result.avatarFrameURL
		}
	}
	// Phase 30: fifth dest arg is *string for avatar_flair_url
	if len(dest) >= 5 {
		if sp, ok := dest[4].(*string); ok {
			*sp = r.result.avatarFlairURL
		}
	}
	// Phase 31: sixth dest arg is *bool for is_admin
	if len(dest) >= 6 {
		if bp, ok := dest[5].(*bool); ok {
			*bp = r.result.isAdmin
		}
	}
	// Phase 31: seventh dest arg is *bool for is_premium
	if len(dest) >= 7 {
		if bp, ok := dest[6].(*bool); ok {
			*bp = r.result.isPremium
		}
	}
	// Phase 9: eighth dest arg is *string for twitch_username
	if len(dest) >= 8 {
		if sp, ok := dest[7].(*string); ok {
			*sp = r.result.twitchUsername
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

// noGradientDB is a convenience wrapper to build fakeViewerDB without gradient, frame/flair, badges, or twitchUsername.
func noGradientDB(queryFn func(platform, userID string) (string, *string, error)) *fakeViewerDB {
	return &fakeViewerDB{
		queryFn: func(platform, userID string) (string, *string, []byte, string, string, bool, bool, string, error) {
			vid, nc, err := queryFn(platform, userID)
			return vid, nc, nil, "", "", false, false, "", err
		},
	}
}

func TestViewerBadgeEnricher_CacheHit_WithColor(t *testing.T) {
	mr := miniredis.RunT(t)
	db := noGradientDB(func(_, _ string) (string, *string, error) {
		t.Error("DB should not be called on cache hit")
		return "", nil, nil
	})
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
	db := noGradientDB(func(_, _ string) (string, *string, error) {
		t.Error("DB should not be called on cache hit")
		return "", nil, nil
	})
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
	db := noGradientDB(func(_, _ string) (string, *string, error) {
		dbCalled = true
		return "", nil, nil
	})
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
	db := noGradientDB(func(platform, userID string) (string, *string, error) {
		return "viewer-uuid", ptr("#aabbcc"), nil
	})
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
	db := noGradientDB(func(platform, userID string) (string, *string, error) {
		return "", nil, pgxErrNoRows
	})
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
	db := noGradientDB(func(_, _ string) (string, *string, error) {
		dbCalled = true
		return "", nil, nil
	})
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
	db := noGradientDB(func(platform, userID string) (string, *string, error) {
		return "viewer-uuid", ptr(storedColor), nil
	})
	e := newTestEnricher(t, mr, db)

	msg := makeMsg("twitch", "user-color-test", "original")
	if err := e.Enrich(context.Background(), msg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.User.Color != storedColor {
		t.Errorf("expected exact stored color %q, got %q", storedColor, msg.User.Color)
	}
}

// Phase 29: gradient propagation test

func TestEnrich_PropagatesNameGradient(t *testing.T) {
	mr := miniredis.RunT(t)
	gradientJSON := []byte(`{"type":"linear","colors":["#ff0000","#0000ff"],"angle":90}`)
	db := &fakeViewerDB{
		queryFn: func(platform, userID string) (string, *string, []byte, string, string, bool, bool, string, error) {
			return "viewer-uuid-grad", nil, gradientJSON, "", "", false, false, "", nil
		},
	}
	e := newTestEnricher(t, mr, db)

	msg := makeMsg("twitch", "gradient-user", "")
	if err := e.Enrich(context.Background(), msg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if msg.User.NameGradient != string(gradientJSON) {
		t.Errorf("expected NameGradient %q, got %q", string(gradientJSON), msg.User.NameGradient)
	}
	// Color should remain unset since no nameColor was returned
	if msg.User.Color != "" {
		t.Errorf("expected empty color when only gradient is set, got %q", msg.User.Color)
	}

	// Verify gradient is also cached correctly
	cacheKey := "viewer:identity:twitch:gradient-user"
	cached, err := mr.Get(cacheKey)
	if err != nil {
		t.Fatalf("expected cache entry, got error: %v", err)
	}
	var identity viewerIdentityCache
	if jsonErr := json.Unmarshal([]byte(cached), &identity); jsonErr != nil {
		t.Fatalf("failed to parse cached value: %v", jsonErr)
	}
	if string(identity.NameGradient) != string(gradientJSON) {
		t.Errorf("cached gradient %q != original %q", string(identity.NameGradient), string(gradientJSON))
	}
}

func TestEnrich_PropagatesNameGradient_FromCache(t *testing.T) {
	mr := miniredis.RunT(t)
	gradientJSON := []byte(`{"type":"linear","colors":["#aabbcc","#112233"],"angle":45}`)

	db := noGradientDB(func(_, _ string) (string, *string, error) {
		t.Error("DB should not be called on cache hit")
		return "", nil, nil
	})
	e := newTestEnricher(t, mr, db)

	// Pre-populate cache with gradient
	cacheKey := "viewer:identity:kick:cached-grad-user"
	val, _ := json.Marshal(viewerIdentityCache{
		ViewerID:     "uuid-grad",
		NameColor:    nil,
		NameGradient: gradientJSON,
	})
	mr.Set(cacheKey, string(val))

	msg := makeMsg("kick", "cached-grad-user", "")
	if err := e.Enrich(context.Background(), msg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.User.NameGradient != string(gradientJSON) {
		t.Errorf("expected NameGradient %q from cache, got %q", string(gradientJSON), msg.User.NameGradient)
	}
}

// Phase 30: avatar frame and flair URL injection tests

func TestEnrichWithAvatarFrameURL(t *testing.T) {
	mr := miniredis.RunT(t)
	db := &fakeViewerDB{
		queryFn: func(platform, userID string) (string, *string, []byte, string, string, bool, bool, string, error) {
			return "viewer-uuid-frame", nil, nil, "https://cdn.example.com/frame.png", "", false, false, "", nil
		},
	}
	e := newTestEnricher(t, mr, db)

	msg := makeMsg("twitch", "frame-user", "")
	if err := e.Enrich(context.Background(), msg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.User.AvatarFrameURL != "https://cdn.example.com/frame.png" {
		t.Errorf("expected AvatarFrameURL %q, got %q", "https://cdn.example.com/frame.png", msg.User.AvatarFrameURL)
	}
	if msg.User.AvatarFlairURL != "" {
		t.Errorf("expected empty AvatarFlairURL, got %q", msg.User.AvatarFlairURL)
	}
}

func TestEnrichWithNoFrameOrFlair(t *testing.T) {
	mr := miniredis.RunT(t)
	db := &fakeViewerDB{
		queryFn: func(platform, userID string) (string, *string, []byte, string, string, bool, bool, string, error) {
			// COALESCE returns empty strings when no frame/flair selected
			return "viewer-uuid-no-cosm", ptr("#336699"), nil, "", "", false, false, "", nil
		},
	}
	e := newTestEnricher(t, mr, db)

	msg := makeMsg("kick", "no-cosm-user", "")
	if err := e.Enrich(context.Background(), msg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.User.AvatarFrameURL != "" {
		t.Errorf("expected empty AvatarFrameURL when no frame selected, got %q", msg.User.AvatarFrameURL)
	}
	if msg.User.AvatarFlairURL != "" {
		t.Errorf("expected empty AvatarFlairURL when no flair selected, got %q", msg.User.AvatarFlairURL)
	}
}

func TestEnrichCacheHitWithFrameURL(t *testing.T) {
	mr := miniredis.RunT(t)
	db := noGradientDB(func(_, _ string) (string, *string, error) {
		t.Error("DB should not be called on cache hit")
		return "", nil, nil
	})
	e := newTestEnricher(t, mr, db)

	// Pre-seed cache with frame URL
	cacheKey := "viewer:identity:twitch:frame-cache-user"
	val, _ := json.Marshal(viewerIdentityCache{
		ViewerID:       "uuid-frame-cache",
		NameColor:      nil,
		AvatarFrameURL: "https://cdn.example.com/frame2.png",
	})
	mr.Set(cacheKey, string(val))

	msg := makeMsg("twitch", "frame-cache-user", "")
	if err := e.Enrich(context.Background(), msg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.User.AvatarFrameURL != "https://cdn.example.com/frame2.png" {
		t.Errorf("expected AvatarFrameURL %q from cache, got %q", "https://cdn.example.com/frame2.png", msg.User.AvatarFrameURL)
	}
}

// Phase 31: All-Chat badge injection tests

func TestEnrich_AdminBadge(t *testing.T) {
	mr := miniredis.RunT(t)
	db := &fakeViewerDB{
		queryFn: func(platform, userID string) (string, *string, []byte, string, string, bool, bool, string, error) {
			return "v1", nil, nil, "", "", true, false, "", nil
		},
	}
	e := newTestEnricher(t, mr, db)

	msg := makeMsg("twitch", "admin-user", "")
	if err := e.Enrich(context.Background(), msg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msg.User.Badges) < 1 {
		t.Fatalf("expected at least 1 badge, got 0")
	}
	if msg.User.Badges[0].Name != "allchat" {
		t.Errorf("expected badges[0].Name == \"allchat\", got %q", msg.User.Badges[0].Name)
	}
}

func TestEnrich_PremiumBadge(t *testing.T) {
	mr := miniredis.RunT(t)
	db := &fakeViewerDB{
		queryFn: func(platform, userID string) (string, *string, []byte, string, string, bool, bool, string, error) {
			return "v2", nil, nil, "", "", false, true, "", nil
		},
	}
	e := newTestEnricher(t, mr, db)

	msg := makeMsg("twitch", "premium-user", "")
	if err := e.Enrich(context.Background(), msg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msg.User.Badges) < 1 {
		t.Fatalf("expected at least 1 badge, got 0")
	}
	if msg.User.Badges[0].Name != "allchat-premium" {
		t.Errorf("expected badges[0].Name == \"allchat-premium\", got %q", msg.User.Badges[0].Name)
	}
}

func TestEnrich_AdminAndPremiumBadge(t *testing.T) {
	mr := miniredis.RunT(t)
	db := &fakeViewerDB{
		queryFn: func(platform, userID string) (string, *string, []byte, string, string, bool, bool, string, error) {
			return "v3", nil, nil, "", "", true, true, "", nil
		},
	}
	e := newTestEnricher(t, mr, db)

	msg := makeMsg("twitch", "admin-premium-user", "")
	if err := e.Enrich(context.Background(), msg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msg.User.Badges) < 2 {
		t.Fatalf("expected at least 2 badges, got %d", len(msg.User.Badges))
	}
	if msg.User.Badges[0].Name != "allchat" {
		t.Errorf("expected badges[0].Name == \"allchat\", got %q", msg.User.Badges[0].Name)
	}
	if msg.User.Badges[1].Name != "allchat-premium" {
		t.Errorf("expected badges[1].Name == \"allchat-premium\", got %q", msg.User.Badges[1].Name)
	}
}

func TestEnrich_NoBadgesForNonRegisteredViewer(t *testing.T) {
	mr := miniredis.RunT(t)
	db := noGradientDB(func(platform, userID string) (string, *string, error) {
		return "", nil, pgxErrNoRows
	})
	e := newTestEnricher(t, mr, db)

	msg := makeMsg("twitch", "unknown-user", "")
	if err := e.Enrich(context.Background(), msg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msg.User.Badges) != 0 {
		t.Errorf("expected no badges for non-registered viewer, got %d", len(msg.User.Badges))
	}
}

// Phase 9: TwitchUsername resolution tests

func TestViewerBadgeEnricher_TwitchUsername(t *testing.T) {
	mr := miniredis.RunT(t)
	db := &fakeViewerDB{
		queryFn: func(platform, userID string) (string, *string, []byte, string, string, bool, bool, string, error) {
			return "viewer-uuid", nil, nil, "", "", false, false, "pajlada", nil
		},
	}
	e := newTestEnricher(t, mr, db)

	msg := makeMsg("kick", "kick-user-123", "")
	if err := e.Enrich(context.Background(), msg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.User.TwitchUsername != "pajlada" {
		t.Errorf("expected TwitchUsername %q, got %q", "pajlada", msg.User.TwitchUsername)
	}
}

func TestViewerBadgeEnricher_TwitchUsernameEmpty(t *testing.T) {
	mr := miniredis.RunT(t)
	db := &fakeViewerDB{
		queryFn: func(platform, userID string) (string, *string, []byte, string, string, bool, bool, string, error) {
			return "viewer-uuid", nil, nil, "", "", false, false, "", nil
		},
	}
	e := newTestEnricher(t, mr, db)

	msg := makeMsg("youtube", "yt-user-456", "")
	if err := e.Enrich(context.Background(), msg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.User.TwitchUsername != "" {
		t.Errorf("expected empty TwitchUsername, got %q", msg.User.TwitchUsername)
	}
}

func TestViewerBadgeEnricher_TwitchUsernameCacheHit(t *testing.T) {
	mr := miniredis.RunT(t)
	db := &fakeViewerDB{
		queryFn: func(platform, userID string) (string, *string, []byte, string, string, bool, bool, string, error) {
			t.Error("DB should not be called on cache hit")
			return "", nil, nil, "", "", false, false, "", nil
		},
	}
	e := newTestEnricher(t, mr, db)

	// Pre-populate cache with TwitchUsername
	cacheKey := "viewer:identity:kick:cached-twitch-user"
	val, _ := json.Marshal(viewerIdentityCache{
		ViewerID:      "uuid-cached",
		TwitchUsername: "cached_user",
	})
	mr.Set(cacheKey, string(val))

	msg := makeMsg("kick", "cached-twitch-user", "")
	if err := e.Enrich(context.Background(), msg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.User.TwitchUsername != "cached_user" {
		t.Errorf("expected TwitchUsername %q from cache, got %q", "cached_user", msg.User.TwitchUsername)
	}
}
