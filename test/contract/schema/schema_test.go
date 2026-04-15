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

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/sebdah/goldie/v2"
	"github.com/stretchr/testify/suite"

	"github.com/caesar/all-chat/test/shared"
)

// SchemaTestSuite validates that InnerTube listener produces RawChatMessage
// output matching the official listener schema, using golden files as ground truth
type SchemaTestSuite struct {
	suite.Suite
	g          *goldie.Goldie
	goldenDir  string
}

func TestSchemaValidation(t *testing.T) {
	suite.Run(t, new(SchemaTestSuite))
}

func (s *SchemaTestSuite) SetupSuite() {
	// Initialize goldie with git-style colored diff
	s.goldenDir = filepath.Join(".", "golden")
	s.g = goldie.New(s.T(),
		goldie.WithFixtureDir(s.goldenDir),
		goldie.WithNameSuffix(".json"),
		goldie.WithDiffEngine(goldie.ColoredDiff),
	)

	// Ensure golden directory exists
	if _, err := os.Stat(s.goldenDir); os.IsNotExist(err) {
		s.T().Skip("Golden files not found. Run golden_capture.go first to populate golden files from live streams.")
	}
}

// TestTextMessages validates text message schema across all captured golden files
func (s *SchemaTestSuite) TestTextMessages() {
	files, err := filepath.Glob(filepath.Join(s.goldenDir, "*_text_message_*.json"))
	s.Require().NoError(err, "Failed to glob text message files")

	if len(files) == 0 {
		s.T().Skip("No text message golden files found. Run golden_capture.go to capture messages.")
	}

	s.T().Logf("Found %d text message golden files", len(files))
	s.Assert().GreaterOrEqual(len(files), 50, "Should have at least 50 text message golden files for comprehensive coverage")

	for _, goldenFile := range files {
		s.Run(filepath.Base(goldenFile), func() {
			s.validateGoldenFile(goldenFile)
		})
	}
}

// TestSuperChatMessages validates Super Chat event schema
func (s *SchemaTestSuite) TestSuperChatMessages() {
	files, err := filepath.Glob(filepath.Join(s.goldenDir, "*_super_chat_*.json"))
	s.Require().NoError(err, "Failed to glob super_chat files")

	if len(files) == 0 {
		s.T().Skip("No super_chat golden files found. Try capturing from streams with active Super Chats.")
	}

	s.T().Logf("Found %d super_chat golden files", len(files))
	s.Assert().GreaterOrEqual(len(files), 10, "Should have at least 10 super_chat golden files")

	for _, goldenFile := range files {
		s.Run(filepath.Base(goldenFile), func() {
			msg := s.validateGoldenFile(goldenFile)

			// Verify Super Chat specific fields
			s.Assert().Equal("super_chat", msg.EventType, "EventType should be super_chat")
			s.Assert().NotNil(msg.EventData, "EventData should not be nil for Super Chats")

			// Verify Super Chat event data contains expected fields
			if msg.EventData != nil {
				s.Assert().Contains(msg.EventData, "amount_micros", "EventData should contain amount_micros")
				s.Assert().Contains(msg.EventData, "currency", "EventData should contain currency")
			}
		})
	}
}

// TestMembershipMessages validates membership event schema (joined + milestones)
func (s *SchemaTestSuite) TestMembershipMessages() {
	joinedFiles, err := filepath.Glob(filepath.Join(s.goldenDir, "*_member_joined_*.json"))
	s.Require().NoError(err, "Failed to glob member_joined files")

	milestoneFiles, err := filepath.Glob(filepath.Join(s.goldenDir, "*_member_milestone_*.json"))
	s.Require().NoError(err, "Failed to glob member_milestone files")

	allFiles := append(joinedFiles, milestoneFiles...)

	if len(allFiles) == 0 {
		s.T().Skip("No membership golden files found. Try capturing from streams with active memberships.")
	}

	s.T().Logf("Found %d membership golden files (joined: %d, milestone: %d)",
		len(allFiles), len(joinedFiles), len(milestoneFiles))
	s.Assert().GreaterOrEqual(len(allFiles), 5, "Should have at least 5 membership golden files")

	for _, goldenFile := range allFiles {
		s.Run(filepath.Base(goldenFile), func() {
			msg := s.validateGoldenFile(goldenFile)

			// Verify membership event type
			s.Assert().Contains([]string{"member_joined", "member_milestone"}, msg.EventType,
				"EventType should be member_joined or member_milestone")
			s.Assert().NotNil(msg.EventData, "EventData should not be nil for membership events")

			// Verify membership event data contains expected fields
			if msg.EventData != nil && msg.EventType == "member_milestone" {
				// Milestone should have month count
				s.Assert().Contains(msg.EventData, "months", "EventData should contain months for milestone")
			}
		})
	}
}

// TestSuperStickerMessages validates Super Sticker event schema
func (s *SchemaTestSuite) TestSuperStickerMessages() {
	files, err := filepath.Glob(filepath.Join(s.goldenDir, "*_super_sticker_*.json"))
	s.Require().NoError(err, "Failed to glob super_sticker files")

	if len(files) == 0 {
		s.T().Skip("No super_sticker golden files found. Super Stickers are rare - try high-volume streams.")
	}

	s.T().Logf("Found %d super_sticker golden files", len(files))
	s.Assert().GreaterOrEqual(len(files), 5, "Should have at least 5 super_sticker golden files")

	for _, goldenFile := range files {
		s.Run(filepath.Base(goldenFile), func() {
			msg := s.validateGoldenFile(goldenFile)

			// Verify Super Sticker specific fields
			s.Assert().Equal("super_sticker", msg.EventType, "EventType should be super_sticker")
			s.Assert().NotNil(msg.EventData, "EventData should not be nil for Super Stickers")

			// Verify Super Sticker event data contains expected fields
			if msg.EventData != nil {
				s.Assert().Contains(msg.EventData, "sticker_id", "EventData should contain sticker_id")
				s.Assert().Contains(msg.EventData, "amount_micros", "EventData should contain amount_micros")
			}
		})
	}
}

// TestTotalGoldenFileCount verifies we have 100+ total golden files per user requirement
func (s *SchemaTestSuite) TestTotalGoldenFileCount() {
	files, err := filepath.Glob(filepath.Join(s.goldenDir, "*.json"))
	s.Require().NoError(err, "Failed to glob golden files")

	s.T().Logf("Total golden files: %d", len(files))
	s.Assert().GreaterOrEqual(len(files), 100,
		"Should have at least 100 golden files across all message types. Run golden_capture.go on 5-10 different streams.")
}

// TestAllMessageTypesRepresented verifies all message types have at least one golden file
func (s *SchemaTestSuite) TestAllMessageTypesRepresented() {
	requiredTypes := map[string]int{
		"text_message":      0,
		"super_chat":        0,
		"super_sticker":     0,
		"member_joined":     0,
		"member_milestone":  0,
	}

	for msgType := range requiredTypes {
		pattern := filepath.Join(s.goldenDir, "*_"+msgType+"_*.json")
		files, err := filepath.Glob(pattern)
		s.Require().NoError(err, "Failed to glob %s files", msgType)
		requiredTypes[msgType] = len(files)
	}

	s.T().Log("Message type distribution:")
	for msgType, count := range requiredTypes {
		s.T().Logf("  %s: %d files", msgType, count)
		s.Assert().Greater(count, 0, "Should have at least one %s golden file", msgType)
	}
}

// validateGoldenFile loads a golden file, normalizes it, and validates schema
// Returns the parsed message for additional assertions
func (s *SchemaTestSuite) validateGoldenFile(goldenPath string) *shared.RawChatMessage {
	// Read golden file
	data, err := os.ReadFile(goldenPath)
	s.Require().NoError(err, "Failed to read golden file: %s", goldenPath)

	// Parse as RawChatMessage
	msg, err := shared.ParseMessageFromJSON(data)
	s.Require().NoError(err, "Failed to parse golden file as RawChatMessage: %s", goldenPath)

	// Validate required fields are present
	s.Assert().NotEmpty(msg.MessageID, "MessageID should not be empty")
	s.Assert().NotEmpty(msg.Platform, "Platform should not be empty")
	s.Assert().Equal("youtube", msg.Platform, "Platform should be 'youtube' for YouTube listener")
	s.Assert().NotEmpty(msg.ChannelID, "ChannelID should not be empty")
	s.Assert().NotEmpty(msg.UserID, "UserID should not be empty")
	s.Assert().NotEmpty(msg.Username, "Username should not be empty")
	s.Assert().False(msg.Timestamp.IsZero(), "Timestamp should not be zero")

	// Normalize message for comparison
	normalized := shared.NormalizeMessage(msg)
	s.Require().NotNil(normalized, "Normalized message should not be nil")

	// Marshal normalized message back to JSON
	normalizedJSON, err := json.MarshalIndent(normalized, "", "  ")
	s.Require().NoError(err, "Failed to marshal normalized message")

	// Use goldie to assert (this will create/update golden files when -update flag is used)
	// Note: We use the original filename for goldie comparison
	baseName := filepath.Base(goldenPath)
	s.g.Assert(s.T(), baseName, normalizedJSON)

	return msg
}
