package shared

import (
	"testing"
	"time"
)

func TestMessageFingerprint_Hash(t *testing.T) {
	// Deterministic hash from username+text+timestamp
	timestamp := time.Date(2026, 2, 21, 15, 30, 0, 0, time.UTC)

	fp1 := MessageFingerprint{
		Username:  "testuser",
		Text:      "Hello world",
		Timestamp: timestamp,
	}

	fp2 := MessageFingerprint{
		Username:  "testuser",
		Text:      "Hello world",
		Timestamp: timestamp,
	}

	hash1 := fp1.Hash()
	hash2 := fp2.Hash()

	if hash1 != hash2 {
		t.Errorf("Expected identical hashes for same fingerprint, got %s != %s", hash1, hash2)
	}

	// Different content produces different hash
	fp3 := MessageFingerprint{
		Username:  "testuser",
		Text:      "Different message",
		Timestamp: timestamp,
	}

	hash3 := fp3.Hash()
	if hash1 == hash3 {
		t.Errorf("Expected different hashes for different content, got %s == %s", hash1, hash3)
	}

	// Hash length should be 16 characters (8 bytes hex)
	if len(hash1) != 16 {
		t.Errorf("Expected hash length 16, got %d", len(hash1))
	}
}

func TestMatchMessages_Identical(t *testing.T) {
	// 100% match rate when messages identical
	timestamp := time.Date(2026, 2, 21, 15, 30, 0, 0, time.UTC)

	official := []*RawChatMessage{
		{
			MessageID: "msg-1",
			Username:  "user1",
			Text:      "Hello",
			Timestamp: timestamp,
		},
		{
			MessageID: "msg-2",
			Username:  "user2",
			Text:      "World",
			Timestamp: timestamp.Add(1 * time.Second),
		},
	}

	innertube := []*RawChatMessage{
		{
			MessageID: "different-id-1", // IDs differ (allowlisted)
			Username:  "user1",
			Text:      "Hello",
			Timestamp: timestamp,
		},
		{
			MessageID: "different-id-2",
			Username:  "user2",
			Text:      "World",
			Timestamp: timestamp.Add(1 * time.Second),
		},
	}

	result := MatchMessages(official, innertube, time.Second)

	if result.Matched != 2 {
		t.Errorf("Expected 2 matches, got %d", result.Matched)
	}
	if result.MissingInInnerTube != 0 {
		t.Errorf("Expected 0 missing in innertube, got %d", result.MissingInInnerTube)
	}
	if result.MissingInOfficial != 0 {
		t.Errorf("Expected 0 missing in official, got %d", result.MissingInOfficial)
	}
	if result.ContentMismatches != 0 {
		t.Errorf("Expected 0 content mismatches, got %d", result.ContentMismatches)
	}
	if result.TotalMessages != 2 {
		t.Errorf("Expected 2 total messages, got %d", result.TotalMessages)
	}
	if result.MismatchRate != 0.0 {
		t.Errorf("Expected 0.0 mismatch rate, got %f", result.MismatchRate)
	}
}

func TestMatchMessages_MissingInInnerTube(t *testing.T) {
	// Detects messages in official but not innertube
	timestamp := time.Date(2026, 2, 21, 15, 30, 0, 0, time.UTC)

	official := []*RawChatMessage{
		{
			MessageID: "msg-1",
			Username:  "user1",
			Text:      "Message 1",
			Timestamp: timestamp,
		},
		{
			MessageID: "msg-2",
			Username:  "user2",
			Text:      "Message 2",
			Timestamp: timestamp.Add(1 * time.Second),
		},
		{
			MessageID: "msg-3",
			Username:  "user3",
			Text:      "Message 3",
			Timestamp: timestamp.Add(2 * time.Second),
		},
	}

	innertube := []*RawChatMessage{
		{
			MessageID: "msg-1",
			Username:  "user1",
			Text:      "Message 1",
			Timestamp: timestamp,
		},
		// Missing msg-2
		{
			MessageID: "msg-3",
			Username:  "user3",
			Text:      "Message 3",
			Timestamp: timestamp.Add(2 * time.Second),
		},
	}

	result := MatchMessages(official, innertube, time.Second)

	if result.Matched != 2 {
		t.Errorf("Expected 2 matches, got %d", result.Matched)
	}
	if result.MissingInInnerTube != 1 {
		t.Errorf("Expected 1 missing in innertube, got %d", result.MissingInInnerTube)
	}
	if result.MissingInOfficial != 0 {
		t.Errorf("Expected 0 missing in official, got %d", result.MissingInOfficial)
	}
	if result.TotalMessages != 3 {
		t.Errorf("Expected 3 total messages, got %d", result.TotalMessages)
	}

	// Mismatch rate = 1/3 = 0.333...
	expectedRate := 1.0 / 3.0
	if result.MismatchRate < expectedRate-0.01 || result.MismatchRate > expectedRate+0.01 {
		t.Errorf("Expected mismatch rate ~%f, got %f", expectedRate, result.MismatchRate)
	}

	// Check mismatch details
	if len(result.Mismatches) != 1 {
		t.Fatalf("Expected 1 mismatch detail, got %d", len(result.Mismatches))
	}
	mismatch := result.Mismatches[0]
	if mismatch.Type != "missing_innertube" {
		t.Errorf("Expected type 'missing_innertube', got '%s'", mismatch.Type)
	}
	if mismatch.OfficialMessage == nil {
		t.Error("Expected official message in mismatch detail")
	}
	if mismatch.InnertubeMessage != nil {
		t.Error("Expected nil innertube message in missing_innertube mismatch")
	}
}

func TestMatchMessages_MissingInOfficial(t *testing.T) {
	// Detects messages in innertube but not official
	timestamp := time.Date(2026, 2, 21, 15, 30, 0, 0, time.UTC)

	official := []*RawChatMessage{
		{
			MessageID: "msg-1",
			Username:  "user1",
			Text:      "Message 1",
			Timestamp: timestamp,
		},
	}

	innertube := []*RawChatMessage{
		{
			MessageID: "msg-1",
			Username:  "user1",
			Text:      "Message 1",
			Timestamp: timestamp,
		},
		{
			MessageID: "msg-2",
			Username:  "user2",
			Text:      "Extra message",
			Timestamp: timestamp.Add(1 * time.Second),
		},
	}

	result := MatchMessages(official, innertube, time.Second)

	if result.Matched != 1 {
		t.Errorf("Expected 1 match, got %d", result.Matched)
	}
	if result.MissingInOfficial != 1 {
		t.Errorf("Expected 1 missing in official, got %d", result.MissingInOfficial)
	}
	if result.MissingInInnerTube != 0 {
		t.Errorf("Expected 0 missing in innertube, got %d", result.MissingInInnerTube)
	}
	if result.TotalMessages != 2 {
		t.Errorf("Expected 2 total messages, got %d", result.TotalMessages)
	}

	// Check mismatch details
	if len(result.Mismatches) != 1 {
		t.Fatalf("Expected 1 mismatch detail, got %d", len(result.Mismatches))
	}
	mismatch := result.Mismatches[0]
	if mismatch.Type != "missing_official" {
		t.Errorf("Expected type 'missing_official', got '%s'", mismatch.Type)
	}
	if mismatch.OfficialMessage != nil {
		t.Error("Expected nil official message in missing_official mismatch")
	}
	if mismatch.InnertubeMessage == nil {
		t.Error("Expected innertube message in mismatch detail")
	}
}

func TestMatchMessages_ContentMismatch(t *testing.T) {
	// Detects field differences (same fingerprint, different fields)
	timestamp := time.Date(2026, 2, 21, 15, 30, 0, 0, time.UTC)

	official := []*RawChatMessage{
		{
			MessageID: "msg-1",
			Username:  "user1",
			Text:      "Hello",
			Timestamp: timestamp,
			ChannelID: "official-channel-123",
			UserID:    "official-user-456",
		},
	}

	innertube := []*RawChatMessage{
		{
			MessageID: "different-id", // Allowlisted - won't cause mismatch
			Username:  "user1",
			Text:      "Hello",
			Timestamp: timestamp,
			ChannelID: "innertube-channel-789", // Different!
			UserID:    "official-user-456",     // Same
		},
	}

	result := MatchMessages(official, innertube, time.Second)

	if result.Matched != 0 {
		t.Errorf("Expected 0 matches, got %d", result.Matched)
	}
	if result.ContentMismatches != 1 {
		t.Errorf("Expected 1 content mismatch, got %d", result.ContentMismatches)
	}

	// Check mismatch details
	if len(result.Mismatches) != 1 {
		t.Fatalf("Expected 1 mismatch detail, got %d", len(result.Mismatches))
	}
	mismatch := result.Mismatches[0]
	if mismatch.Type != "content_diff" {
		t.Errorf("Expected type 'content_diff', got '%s'", mismatch.Type)
	}
	if mismatch.OfficialMessage == nil || mismatch.InnertubeMessage == nil {
		t.Error("Expected both messages in content_diff mismatch")
	}
	if len(mismatch.FieldDifferences) == 0 {
		t.Error("Expected field differences in content_diff mismatch")
	}

	// Check specific field difference
	if diff, ok := mismatch.FieldDifferences["ChannelID"]; ok {
		if diff.OfficialValue != "official-channel-123" {
			t.Errorf("Expected official ChannelID 'official-channel-123', got '%v'", diff.OfficialValue)
		}
		if diff.InnertubeValue != "innertube-channel-789" {
			t.Errorf("Expected innertube ChannelID 'innertube-channel-789', got '%v'", diff.InnertubeValue)
		}
	} else {
		t.Error("Expected ChannelID in field differences")
	}
}

func TestMatchMessages_TimeWindowTolerance(t *testing.T) {
	// Messages within 1s window should match
	baseTime := time.Date(2026, 2, 21, 15, 30, 0, 0, time.UTC)

	official := []*RawChatMessage{
		{
			MessageID: "msg-1",
			Username:  "user1",
			Text:      "Hello",
			Timestamp: baseTime,
		},
	}

	innertube := []*RawChatMessage{
		{
			MessageID: "msg-1",
			Username:  "user1",
			Text:      "Hello",
			Timestamp: baseTime.Add(500 * time.Millisecond), // Within 1s window
		},
	}

	result := MatchMessages(official, innertube, time.Second)

	if result.Matched != 1 {
		t.Errorf("Expected 1 match (within time window), got %d", result.Matched)
	}
	if result.MismatchRate != 0.0 {
		t.Errorf("Expected 0.0 mismatch rate, got %f", result.MismatchRate)
	}
}

func TestMatchMessages_MismatchRateCalculation(t *testing.T) {
	// Verify (missing + diff) / total calculation
	timestamp := time.Date(2026, 2, 21, 15, 30, 0, 0, time.UTC)

	official := []*RawChatMessage{
		{MessageID: "1", Username: "u1", Text: "m1", Timestamp: timestamp},
		{MessageID: "2", Username: "u2", Text: "m2", Timestamp: timestamp.Add(1 * time.Second)},
		{MessageID: "3", Username: "u3", Text: "m3", Timestamp: timestamp.Add(2 * time.Second)},
		{MessageID: "4", Username: "u4", Text: "m4", Timestamp: timestamp.Add(3 * time.Second), ChannelID: "official"},
		{MessageID: "5", Username: "u5", Text: "m5", Timestamp: timestamp.Add(4 * time.Second)},
	}

	innertube := []*RawChatMessage{
		{MessageID: "1", Username: "u1", Text: "m1", Timestamp: timestamp},
		{MessageID: "2", Username: "u2", Text: "m2", Timestamp: timestamp.Add(1 * time.Second)},
		// Missing m3
		{MessageID: "4", Username: "u4", Text: "m4", Timestamp: timestamp.Add(3 * time.Second), ChannelID: "innertube"}, // Content diff
		{MessageID: "5", Username: "u5", Text: "m5", Timestamp: timestamp.Add(4 * time.Second)},
		{MessageID: "6", Username: "u6", Text: "m6", Timestamp: timestamp.Add(5 * time.Second)}, // Extra in innertube
	}

	result := MatchMessages(official, innertube, time.Second)

	// Total unique messages: 6 (m1, m2, m3, m4, m5, m6)
	if result.TotalMessages != 6 {
		t.Errorf("Expected 6 total messages, got %d", result.TotalMessages)
	}

	// Matched: m1, m2, m5 = 3
	if result.Matched != 3 {
		t.Errorf("Expected 3 matches, got %d", result.Matched)
	}

	// Missing in innertube: m3 = 1
	if result.MissingInInnerTube != 1 {
		t.Errorf("Expected 1 missing in innertube, got %d", result.MissingInInnerTube)
	}

	// Missing in official: m6 = 1
	if result.MissingInOfficial != 1 {
		t.Errorf("Expected 1 missing in official, got %d", result.MissingInOfficial)
	}

	// Content mismatches: m4 = 1
	if result.ContentMismatches != 1 {
		t.Errorf("Expected 1 content mismatch, got %d", result.ContentMismatches)
	}

	// Mismatch rate = (1 + 1 + 1) / 6 = 0.5
	expectedRate := 3.0 / 6.0
	if result.MismatchRate < expectedRate-0.01 || result.MismatchRate > expectedRate+0.01 {
		t.Errorf("Expected mismatch rate %f, got %f", expectedRate, result.MismatchRate)
	}
}

func TestCompareFields_AllowlistFields(t *testing.T) {
	// MessageID and RawMessage should be allowlisted (no diff reported)
	timestamp := time.Date(2026, 2, 21, 15, 30, 0, 0, time.UTC)

	official := &RawChatMessage{
		MessageID: "official-uuid-123",
		Username:  "user1",
		Text:      "Hello",
		Timestamp: timestamp,
		RawMessage: []byte(`{"official": true}`),
	}

	innertube := &RawChatMessage{
		MessageID: "innertube-uuid-456",
		Username:  "user1",
		Text:      "Hello",
		Timestamp: timestamp,
		RawMessage: []byte(`{"innertube": true}`),
	}

	diffs := CompareFields(official, innertube)

	if len(diffs) != 0 {
		t.Errorf("Expected no diffs for allowlisted fields, got %d diffs: %+v", len(diffs), diffs)
	}
}

func TestCompareFields_TimestampTolerance(t *testing.T) {
	// Timestamps within 1s should not be reported as diff
	baseTime := time.Date(2026, 2, 21, 15, 30, 0, 0, time.UTC)

	official := &RawChatMessage{
		Username:  "user1",
		Text:      "Hello",
		Timestamp: baseTime,
	}

	innertube := &RawChatMessage{
		Username:  "user1",
		Text:      "Hello",
		Timestamp: baseTime.Add(500 * time.Millisecond),
	}

	diffs := CompareFields(official, innertube)

	if _, hasTimestampDiff := diffs["Timestamp"]; hasTimestampDiff {
		t.Error("Expected no timestamp diff for timestamps within 1s tolerance")
	}

	// Timestamp difference > 1s should be reported
	innertube2 := &RawChatMessage{
		Username:  "user1",
		Text:      "Hello",
		Timestamp: baseTime.Add(2 * time.Second),
	}

	diffs2 := CompareFields(official, innertube2)

	if _, hasTimestampDiff := diffs2["Timestamp"]; !hasTimestampDiff {
		t.Error("Expected timestamp diff for timestamps > 1s apart")
	}
}
