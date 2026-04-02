package sessions

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/caesar/all-chat/services/message-processor/models"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func setupTestCapture(t *testing.T) (*EventCapture, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rdb.Close() })
	logger := zap.NewNop()
	return NewEventCapture(rdb, logger), mr
}

func makeActiveSession(t *testing.T, mr *miniredis.Miniredis, overlayID, sessionID string) {
	t.Helper()
	key := SessionKeyPrefix + overlayID
	mr.HSet(key, "session_id", sessionID)
	mr.HSet(key, "state", "ACTIVE")
	mr.HSet(key, "started_at", time.Now().UTC().Format(time.RFC3339))
	mr.HSet(key, "event_count", "0")
}

func makeSubEvent(overlayID, userID, displayName, avatarURL string, amount float64) *models.UnifiedChatMessage {
	return &models.UnifiedChatMessage{
		ID:        fmt.Sprintf("msg-%s-%d", userID, time.Now().UnixNano()),
		OverlayID: overlayID,
		Platform:  "twitch",
		User: models.UserInfo{
			ID:          userID,
			DisplayName: displayName,
			AvatarURL:   avatarURL,
		},
		Timestamp: time.Now(),
		Event: &models.EventInfo{
			Type: "subscription",
			Value: &models.EventValue{
				Amount:      amount,
				Currency:    "USD",
				DisplayText: fmt.Sprintf("$%.2f", amount),
			},
		},
	}
}

func TestCaptureIfActive_NoEvent(t *testing.T) {
	ec, _ := setupTestCapture(t)
	msg := &models.UnifiedChatMessage{
		OverlayID: "overlay-1",
		Event:     nil,
	}
	err := ec.CaptureIfActive(context.Background(), msg)
	assert.NoError(t, err)
}

func TestCaptureIfActive_NoSession(t *testing.T) {
	ec, _ := setupTestCapture(t)
	msg := makeSubEvent("overlay-1", "user-1", "Alice", "https://avatar/1", 4.99)
	err := ec.CaptureIfActive(context.Background(), msg)
	assert.NoError(t, err)
}

func TestStoreEvent_DeduplicatesSameUser(t *testing.T) {
	ec, mr := setupTestCapture(t)
	ctx := context.Background()
	overlayID := "overlay-dedup"
	sessionID := "session-dedup"
	makeActiveSession(t, mr, overlayID, sessionID)

	// Same user subscribes twice with different avatar URLs
	msg1 := makeSubEvent(overlayID, "user-42", "Alice", "https://avatar/old.png", 4.99)
	msg2 := makeSubEvent(overlayID, "user-42", "Alice_Updated", "https://avatar/new.png", 9.99)

	require.NoError(t, ec.CaptureIfActive(ctx, msg1))
	require.NoError(t, ec.CaptureIfActive(ctx, msg2))

	// Should have exactly 1 entry in the subs leaderboard (not 2)
	leaderboardKey := fmt.Sprintf("%s%s:subs", LeaderboardKeyPrefix, sessionID)
	members, err := mr.ZMembers(leaderboardKey)
	require.NoError(t, err)
	assert.Len(t, members, 1, "same user should have exactly one entry")

	// Score should be aggregated (4.99 + 9.99 = 14.98)
	score, err := mr.ZScore(leaderboardKey, members[0])
	require.NoError(t, err)
	assert.InDelta(t, 14.98, score, 0.01, "scores should be aggregated")

	// Metadata should reflect latest event (updated display name and avatar)
	metaKey := fmt.Sprintf("%s%s:subs", MetadataKeyPrefix, sessionID)
	metaJSON := mr.HGet(metaKey, members[0])
	require.NotEmpty(t, metaJSON)

	var meta map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(metaJSON), &meta))
	assert.Equal(t, "Alice_Updated", meta["display_name"], "metadata should reflect latest event")
	assert.Equal(t, "https://avatar/new.png", meta["avatar_url"], "avatar should be updated")
}

func TestStoreEvent_DifferentUsersSeparateEntries(t *testing.T) {
	ec, mr := setupTestCapture(t)
	ctx := context.Background()
	overlayID := "overlay-multi"
	sessionID := "session-multi"
	makeActiveSession(t, mr, overlayID, sessionID)

	msg1 := makeSubEvent(overlayID, "user-1", "Alice", "https://avatar/alice.png", 4.99)
	msg2 := makeSubEvent(overlayID, "user-2", "Bob", "https://avatar/bob.png", 9.99)

	require.NoError(t, ec.CaptureIfActive(ctx, msg1))
	require.NoError(t, ec.CaptureIfActive(ctx, msg2))

	leaderboardKey := fmt.Sprintf("%s%s:subs", LeaderboardKeyPrefix, sessionID)
	members, err := mr.ZMembers(leaderboardKey)
	require.NoError(t, err)
	assert.Len(t, members, 2, "different users should have separate entries")
}

func TestStoreEvent_MemberKeyFormat(t *testing.T) {
	ec, mr := setupTestCapture(t)
	ctx := context.Background()
	overlayID := "overlay-key"
	sessionID := "session-key"
	makeActiveSession(t, mr, overlayID, sessionID)

	msg := makeSubEvent(overlayID, "user-99", "TestUser", "https://avatar/test.png", 4.99)
	require.NoError(t, ec.CaptureIfActive(ctx, msg))

	leaderboardKey := fmt.Sprintf("%s%s:subs", LeaderboardKeyPrefix, sessionID)
	members, err := mr.ZMembers(leaderboardKey)
	require.NoError(t, err)
	require.Len(t, members, 1)

	// Member key should be platform:user_id (not JSON)
	assert.Equal(t, "twitch:user-99", members[0], "member key should be platform:user_id")
}

func TestStoreEvent_CountBasedEvents(t *testing.T) {
	ec, mr := setupTestCapture(t)
	ctx := context.Background()
	overlayID := "overlay-follow"
	sessionID := "session-follow"
	makeActiveSession(t, mr, overlayID, sessionID)

	msg := &models.UnifiedChatMessage{
		ID:        "msg-follow-1",
		OverlayID: overlayID,
		Platform:  "twitch",
		User: models.UserInfo{
			ID:          "user-follower",
			DisplayName: "Follower",
		},
		Timestamp: time.Now(),
		Event: &models.EventInfo{
			Type: "follow",
		},
	}

	require.NoError(t, ec.CaptureIfActive(ctx, msg))

	leaderboardKey := fmt.Sprintf("%s%s:follows", LeaderboardKeyPrefix, sessionID)
	members, err := mr.ZMembers(leaderboardKey)
	require.NoError(t, err)
	require.Len(t, members, 1)

	score, err := mr.ZScore(leaderboardKey, members[0])
	require.NoError(t, err)
	assert.Equal(t, 1.0, score, "follow event should have score 1.0")
}

func TestStoreEvent_SessionCounterIncremented(t *testing.T) {
	ec, mr := setupTestCapture(t)
	ctx := context.Background()
	overlayID := "overlay-counter"
	sessionID := "session-counter"
	makeActiveSession(t, mr, overlayID, sessionID)

	msg := makeSubEvent(overlayID, "user-1", "Alice", "", 4.99)
	require.NoError(t, ec.CaptureIfActive(ctx, msg))

	sessionKey := SessionKeyPrefix + overlayID
	count := mr.HGet(sessionKey, "event_count")
	assert.Equal(t, "1", count)

	// Second event
	msg2 := makeSubEvent(overlayID, "user-2", "Bob", "", 9.99)
	require.NoError(t, ec.CaptureIfActive(ctx, msg2))

	count = mr.HGet(sessionKey, "event_count")
	assert.Equal(t, "2", count)
}
