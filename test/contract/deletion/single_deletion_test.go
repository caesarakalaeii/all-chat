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

package deletion

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/caesar/all-chat/services/youtube-listener-innertube/innertube"
	"github.com/stretchr/testify/suite"
)

// DeletionTestSuite tests deletion event detection from InnerTube API responses
type DeletionTestSuite struct {
	suite.Suite
	fixtures map[string][]byte
}

// SetupSuite loads fixtures before running tests
func (s *DeletionTestSuite) SetupSuite() {
	s.fixtures = make(map[string][]byte)

	fixtureDir := "fixtures"
	fixtures := []string{"deletion_event.json", "mixed_events.json"}

	for _, fixture := range fixtures {
		path := filepath.Join(fixtureDir, fixture)
		data, err := os.ReadFile(path)
		s.Require().NoError(err, "Failed to load fixture: %s", fixture)
		s.fixtures[fixture] = data
	}
}

// TestDetectSingleDeletion tests detection of a single deletion event
func (s *DeletionTestSuite) TestDetectSingleDeletion() {
	// Load and parse fixture
	var response innertube.LiveChatResponse
	err := json.Unmarshal(s.fixtures["deletion_event.json"], &response)
	s.Require().NoError(err, "Failed to unmarshal deletion_event.json")

	// Parse messages
	channelID := "UC_test_channel"
	messages, err := innertube.ParseMessages(
		response.ContinuationContents.LiveChatContinuation.Actions,
		channelID,
	)
	s.Require().NoError(err, "ParseMessages should not error")

	// Verify single deletion event returned
	s.Require().Len(messages, 1, "Should return exactly 1 deletion event")

	msg := messages[0]

	// Verify deletion event properties
	s.Equal("message_deletion", msg.EventType, "EventType should be message_deletion")
	s.Equal("youtube", msg.Platform, "Platform should be youtube")
	s.Equal(channelID, msg.ChannelID, "ChannelID should match")
	s.Empty(msg.Text, "Deletion events should have empty text")
	s.Empty(msg.UserID, "Deletion events should have empty UserID")
	s.Empty(msg.Username, "Deletion events should have empty Username")
	s.NotEmpty(msg.MessageID, "MessageID should be generated")

	// Verify metadata
	s.Require().NotNil(msg.EventData, "EventData should not be nil")
	s.Equal("single", msg.EventData["deletion_type"], "deletion_type should be single")

	targetMsgID, ok := msg.EventData["target_msg_id"].(string)
	s.Require().True(ok, "target_msg_id should be a string")
	s.NotEmpty(targetMsgID, "target_msg_id should not be empty")
	s.Equal("ChwKGkNNT3M4UF9BdTRvRENNeTQ5Z0FkaERaeFhBZw%3D%3D", targetMsgID, "target_msg_id should match fixture")
}

// TestDetectMixedEvents tests detection of mixed regular messages and deletion events
func (s *DeletionTestSuite) TestDetectMixedEvents() {
	// Load and parse fixture
	var response innertube.LiveChatResponse
	err := json.Unmarshal(s.fixtures["mixed_events.json"], &response)
	s.Require().NoError(err, "Failed to unmarshal mixed_events.json")

	// Parse messages
	channelID := "UC_test_channel"
	messages, err := innertube.ParseMessages(
		response.ContinuationContents.LiveChatContinuation.Actions,
		channelID,
	)
	s.Require().NoError(err, "ParseMessages should not error")

	// Verify total count (5 regular + 2 deletions = 7)
	s.Require().Len(messages, 7, "Should return 7 total events")

	// Count event types
	regularCount := 0
	deletionCount := 0

	for _, msg := range messages {
		if msg.EventType == "message_deletion" {
			deletionCount++
			// Verify deletion event properties
			s.Empty(msg.Text, "Deletion events should have empty text")
			s.Empty(msg.UserID, "Deletion events should have empty UserID")
			s.Empty(msg.Username, "Deletion events should have empty Username")
			s.NotNil(msg.EventData, "Deletion events should have EventData")
			s.Equal("single", msg.EventData["deletion_type"], "deletion_type should be single")
			s.NotEmpty(msg.EventData["target_msg_id"], "target_msg_id should be present")
		} else if msg.EventType == "" {
			regularCount++
			// Verify regular message properties
			s.NotEmpty(msg.Text, "Regular messages should have text")
			s.NotEmpty(msg.UserID, "Regular messages should have UserID")
			s.NotEmpty(msg.Username, "Regular messages should have Username")
			// Regular messages should not have deletion metadata
			if msg.EventData != nil {
				s.Nil(msg.EventData["target_msg_id"], "Regular messages should not have target_msg_id")
				s.Nil(msg.EventData["deletion_type"], "Regular messages should not have deletion_type")
			}
		}
	}

	// Verify counts
	s.Equal(5, regularCount, "Should have 5 regular messages")
	s.Equal(2, deletionCount, "Should have 2 deletion events")
}

// TestDeletionSchemaValidation tests that deletion event JSON schema matches expected format
func (s *DeletionTestSuite) TestDeletionSchemaValidation() {
	// Load and parse fixture
	var response innertube.LiveChatResponse
	err := json.Unmarshal(s.fixtures["deletion_event.json"], &response)
	s.Require().NoError(err, "Failed to unmarshal deletion_event.json")

	// Parse messages
	channelID := "UC_test_channel"
	messages, err := innertube.ParseMessages(
		response.ContinuationContents.LiveChatContinuation.Actions,
		channelID,
	)
	s.Require().NoError(err, "ParseMessages should not error")
	s.Require().Len(messages, 1, "Should return exactly 1 deletion event")

	msg := messages[0]

	// Marshal to JSON
	jsonData, err := json.Marshal(msg)
	s.Require().NoError(err, "Failed to marshal deletion event")

	// Unmarshal back to generic map
	var decoded map[string]interface{}
	err = json.Unmarshal(jsonData, &decoded)
	s.Require().NoError(err, "Failed to unmarshal JSON")

	// Verify required fields exist
	requiredFields := []string{
		"message_id",
		"platform",
		"channel_id",
		"stream_id",
		"user_id",
		"username",
		"text",
		"timestamp",
		"tags",
		"event_type",
		"event_data",
	}

	for _, field := range requiredFields {
		s.Contains(decoded, field, "JSON should contain field: %s", field)
	}

	// Verify field values
	s.Equal("youtube", decoded["platform"], "platform should be youtube")
	s.Equal("message_deletion", decoded["event_type"], "event_type should be message_deletion")
	s.Equal("", decoded["text"], "text should be empty string")
	s.Equal("", decoded["user_id"], "user_id should be empty string")
	s.Equal("", decoded["username"], "username should be empty string")

	// Verify event_data structure
	eventData, ok := decoded["event_data"].(map[string]interface{})
	s.Require().True(ok, "event_data should be a map")
	s.Contains(eventData, "target_msg_id", "event_data should contain target_msg_id")
	s.Contains(eventData, "deletion_type", "event_data should contain deletion_type")
	s.Equal("single", eventData["deletion_type"], "deletion_type should be single")

	// Verify no extra fields in event_data (schema compliance)
	allowedEventDataFields := map[string]bool{
		"target_msg_id":  true,
		"deletion_type":  true,
	}
	for field := range eventData {
		s.True(allowedEventDataFields[field], "event_data contains unexpected field: %s", field)
	}
}

// TestSuite runs the deletion test suite
func TestDeletionSuite(t *testing.T) {
	suite.Run(t, new(DeletionTestSuite))
}
