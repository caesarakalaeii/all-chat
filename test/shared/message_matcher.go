package shared

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"time"
)

// RawChatMessage represents the unified message format from all listeners
// Duplicated here to avoid circular dependencies with service packages
type RawChatMessage struct {
	MessageID   string            `json:"message_id"`
	Platform    string            `json:"platform"`
	OverlayID   string            `json:"overlay_id,omitempty"`
	ChannelID   string            `json:"channel_id"`
	ChannelName string            `json:"channel_name,omitempty"`
	UserID      string            `json:"user_id"`
	Username    string            `json:"username"`
	Text        string            `json:"text"`
	Timestamp   time.Time         `json:"timestamp"`
	Tags        map[string]string `json:"tags"`
	RawMessage  json.RawMessage   `json:"raw_message,omitempty"`

	// Event support (backwards compatible - omitted for chat messages)
	EventType string                 `json:"event_type,omitempty"`
	EventData map[string]interface{} `json:"event_data,omitempty"`
}

// MessageFingerprint creates a content-based identifier for message correlation
// Uses username + text + timestamp (truncated to timeWindow precision)
type MessageFingerprint struct {
	Username  string
	Text      string
	Timestamp time.Time // Truncated to timeWindow precision
}

// Hash creates a deterministic hash from the fingerprint
// Returns first 16 hex characters (8 bytes) for readability
func (f MessageFingerprint) Hash() string {
	data := fmt.Sprintf("%s|%s|%d", f.Username, f.Text, f.Timestamp.Unix())
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", hash[:8])
}

// CreateFingerprint generates a fingerprint from a RawChatMessage
func CreateFingerprint(msg *RawChatMessage, timeWindow time.Duration) MessageFingerprint {
	// Truncate timestamp to timeWindow precision (default 1 second)
	truncated := msg.Timestamp.Truncate(timeWindow)

	return MessageFingerprint{
		Username:  msg.Username,
		Text:      msg.Text,
		Timestamp: truncated,
	}
}

// MatchResult contains correlation results and mismatch details
type MatchResult struct {
	Matched              int
	MissingInInnerTube   int
	MissingInOfficial    int
	ContentMismatches    int
	TotalMessages        int
	MismatchRate         float64
	Mismatches           []MismatchDetail
}

// MismatchDetail describes a single mismatch between official and InnerTube
type MismatchDetail struct {
	Type              string // "missing_innertube", "missing_official", "content_diff"
	OfficialMessage   *RawChatMessage
	InnertubeMessage  *RawChatMessage
	FieldDifferences  map[string]FieldDiff
	Timestamp         time.Time
	Fingerprint       string // For debugging
}

// FieldDiff represents a difference in a specific field
type FieldDiff struct {
	OfficialValue  interface{}
	InnertubeValue interface{}
}

// MatchMessages correlates messages from official and InnerTube listeners
// Uses content-based fingerprinting (username+text+timestamp) to match messages
// Returns detailed match statistics and mismatch details
func MatchMessages(official, innertube []*RawChatMessage, timeWindow time.Duration) MatchResult {
	result := MatchResult{
		Mismatches: make([]MismatchDetail, 0),
	}

	// Build fingerprint maps for both message sets
	officialMap := make(map[string]*RawChatMessage)
	innertubeMap := make(map[string]*RawChatMessage)

	for _, msg := range official {
		fp := CreateFingerprint(msg, timeWindow)
		hash := fp.Hash()
		officialMap[hash] = msg
	}

	for _, msg := range innertube {
		fp := CreateFingerprint(msg, timeWindow)
		hash := fp.Hash()
		innertubeMap[hash] = msg
	}

	// Calculate total unique messages
	uniqueHashes := make(map[string]bool)
	for hash := range officialMap {
		uniqueHashes[hash] = true
	}
	for hash := range innertubeMap {
		uniqueHashes[hash] = true
	}
	result.TotalMessages = len(uniqueHashes)

	// Compare messages by fingerprint
	for hash, officialMsg := range officialMap {
		innertubeMsg, foundInInnertube := innertubeMap[hash]

		if !foundInInnertube {
			// Message missing in InnerTube
			result.MissingInInnerTube++
			result.Mismatches = append(result.Mismatches, MismatchDetail{
				Type:             "missing_innertube",
				OfficialMessage:  officialMsg,
				InnertubeMessage: nil,
				FieldDifferences: nil,
				Timestamp:        time.Now(),
				Fingerprint:      hash,
			})
		} else {
			// Messages match by fingerprint - check for field differences
			diffs := CompareFields(officialMsg, innertubeMsg)
			if len(diffs) > 0 {
				result.ContentMismatches++
				result.Mismatches = append(result.Mismatches, MismatchDetail{
					Type:             "content_diff",
					OfficialMessage:  officialMsg,
					InnertubeMessage: innertubeMsg,
					FieldDifferences: diffs,
					Timestamp:        time.Now(),
					Fingerprint:      hash,
				})
			} else {
				result.Matched++
			}
		}
	}

	// Find messages that exist in InnerTube but not in official
	for hash, innertubeMsg := range innertubeMap {
		if _, foundInOfficial := officialMap[hash]; !foundInOfficial {
			result.MissingInOfficial++
			result.Mismatches = append(result.Mismatches, MismatchDetail{
				Type:             "missing_official",
				OfficialMessage:  nil,
				InnertubeMessage: innertubeMsg,
				FieldDifferences: nil,
				Timestamp:        time.Now(),
				Fingerprint:      hash,
			})
		}
	}

	// Calculate mismatch rate: (missing + content_diff) / total
	if result.TotalMessages > 0 {
		mismatches := result.MissingInInnerTube + result.MissingInOfficial + result.ContentMismatches
		result.MismatchRate = float64(mismatches) / float64(result.TotalMessages)
	}

	return result
}

// CompareFields compares fields between official and InnerTube messages
// Excludes allowlisted fields that are expected to differ (MessageID, RawMessage)
// Returns map of field differences
func CompareFields(official, innertube *RawChatMessage) map[string]FieldDiff {
	diffs := make(map[string]FieldDiff)

	// Fields expected to differ (allowlist)
	allowlist := map[string]bool{
		"MessageID":  true, // UUIDs will differ
		"RawMessage": true, // Platform-specific raw data
	}

	// Compare string fields
	compareField := func(name, officialVal, innertubeVal string) {
		if allowlist[name] {
			return
		}
		if officialVal != innertubeVal {
			diffs[name] = FieldDiff{
				OfficialValue:  officialVal,
				InnertubeValue: innertubeVal,
			}
		}
	}

	compareField("Platform", official.Platform, innertube.Platform)
	compareField("OverlayID", official.OverlayID, innertube.OverlayID)
	compareField("ChannelID", official.ChannelID, innertube.ChannelID)
	compareField("ChannelName", official.ChannelName, innertube.ChannelName)
	compareField("UserID", official.UserID, innertube.UserID)
	compareField("Username", official.Username, innertube.Username)
	compareField("Text", official.Text, innertube.Text)
	compareField("EventType", official.EventType, innertube.EventType)

	// Compare timestamps (within 1 second tolerance already handled by fingerprint)
	if !official.Timestamp.Equal(innertube.Timestamp) {
		timeDiff := official.Timestamp.Sub(innertube.Timestamp)
		if timeDiff < 0 {
			timeDiff = -timeDiff
		}
		// Only report if difference is > 1 second (fingerprint tolerance)
		if timeDiff > time.Second {
			diffs["Timestamp"] = FieldDiff{
				OfficialValue:  official.Timestamp,
				InnertubeValue: innertube.Timestamp,
			}
		}
	}

	// Compare Tags map
	if !reflect.DeepEqual(official.Tags, innertube.Tags) {
		diffs["Tags"] = FieldDiff{
			OfficialValue:  official.Tags,
			InnertubeValue: innertube.Tags,
		}
	}

	// Compare EventData map
	if !reflect.DeepEqual(official.EventData, innertube.EventData) {
		diffs["EventData"] = FieldDiff{
			OfficialValue:  official.EventData,
			InnertubeValue: innertube.EventData,
		}
	}

	return diffs
}
