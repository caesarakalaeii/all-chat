package deletion

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/caesar/all-chat/services/youtube-listener-innertube/innertube"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// DeletionEmissionSuite tests end-to-end deletion event emission to Redis
type DeletionEmissionSuite struct {
	suite.Suite

	// Test infrastructure
	redisContainer testcontainers.Container
	redisClient    *redis.Client

	// Test data
	fixtures map[string][]byte
	ctx      context.Context
}

// SetupSuite initializes test infrastructure
func (s *DeletionEmissionSuite) SetupSuite() {
	s.ctx = context.Background()

	// Load fixtures
	s.fixtures = make(map[string][]byte)
	fixtureDir := "fixtures"
	fixtures := []string{"deletion_event.json", "mixed_events.json"}

	for _, fixture := range fixtures {
		path := filepath.Join(fixtureDir, fixture)
		data, err := os.ReadFile(path)
		s.Require().NoError(err, "Failed to load fixture: %s", fixture)
		s.fixtures[fixture] = data
	}

	// Start Redis container
	req := testcontainers.ContainerRequest{
		Image:        "redis:7-alpine",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForLog("Ready to accept connections"),
	}

	redisC, err := testcontainers.GenericContainer(s.ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	s.Require().NoError(err, "Failed to start Redis container")
	s.redisContainer = redisC

	// Get Redis connection info
	host, err := redisC.Host(s.ctx)
	s.Require().NoError(err, "Failed to get Redis host")

	port, err := redisC.MappedPort(s.ctx, "6379")
	s.Require().NoError(err, "Failed to get Redis port")

	// Create Redis client
	s.redisClient = redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%s", host, port.Port()),
	})

	// Verify Redis connection
	err = s.redisClient.Ping(s.ctx).Err()
	s.Require().NoError(err, "Failed to connect to Redis")
}

// TearDownSuite cleans up test infrastructure
func (s *DeletionEmissionSuite) TearDownSuite() {
	if s.redisClient != nil {
		s.redisClient.Close()
	}
	if s.redisContainer != nil {
		s.redisContainer.Terminate(s.ctx)
	}
}

// SetupTest prepares each test
func (s *DeletionEmissionSuite) SetupTest() {
	// Flush Redis before each test
	s.redisClient.FlushAll(s.ctx)
}

// TestEmitDeletionEvent_DirectPublish tests deletion event emission to Redis
// This simulates what the InnerTube listener service does
func (s *DeletionEmissionSuite) TestEmitDeletionEvent_DirectPublish() {
	// Load and parse mixed_events fixture (5 regular + 2 deletions)
	var response innertube.LiveChatResponse
	err := json.Unmarshal(s.fixtures["mixed_events.json"], &response)
	s.Require().NoError(err, "Failed to unmarshal fixture")

	// Parse messages
	channelID := "UC_test_channel"
	messages, err := innertube.ParseMessages(
		response.ContinuationContents.LiveChatContinuation.Actions,
		channelID,
	)
	s.Require().NoError(err, "ParseMessages should not error")
	s.Require().Len(messages, 7, "Should parse 7 messages")

	// Simulate publishing to Redis Stream (what the listener service does)
	streamName := "chat:raw"
	for _, msg := range messages {
		// Marshal message to JSON
		jsonData, err := json.Marshal(msg)
		s.Require().NoError(err, "Failed to marshal message")

		// Publish to Redis Stream
		err = s.redisClient.XAdd(s.ctx, &redis.XAddArgs{
			Stream: streamName,
			Values: map[string]interface{}{
				"data": string(jsonData),
			},
		}).Err()
		s.Require().NoError(err, "Failed to publish to Redis Stream")
	}

	// Read all messages from Redis Stream
	result, err := s.redisClient.XRange(s.ctx, streamName, "-", "+").Result()
	s.Require().NoError(err, "Failed to read from Redis Stream")

	// Verify total count
	s.Len(result, 7, "Should have 7 messages in Redis Stream")

	// Parse and count message types
	regularCount := 0
	deletionCount := 0

	for _, entry := range result {
		dataStr, ok := entry.Values["data"].(string)
		s.Require().True(ok, "Message should have data field")

		// Unmarshal to generic map
		var msgData map[string]interface{}
		err := json.Unmarshal([]byte(dataStr), &msgData)
		s.Require().NoError(err, "Failed to unmarshal message data")

		eventType, _ := msgData["event_type"].(string)
		if eventType == "message_deletion" {
			deletionCount++
		} else if eventType == "" {
			regularCount++
		}
	}

	// Verify counts
	s.Equal(5, regularCount, "Should have 5 regular messages in Redis")
	s.Equal(2, deletionCount, "Should have 2 deletion events in Redis")
}

// TestEmitDeletionEvent_SchemaCompliance tests deletion event schema in Redis
func (s *DeletionEmissionSuite) TestEmitDeletionEvent_SchemaCompliance() {
	// Load and parse single deletion fixture
	var response innertube.LiveChatResponse
	err := json.Unmarshal(s.fixtures["deletion_event.json"], &response)
	s.Require().NoError(err, "Failed to unmarshal fixture")

	// Parse messages
	channelID := "UC_test_channel"
	messages, err := innertube.ParseMessages(
		response.ContinuationContents.LiveChatContinuation.Actions,
		channelID,
	)
	s.Require().NoError(err, "ParseMessages should not error")
	s.Require().Len(messages, 1, "Should parse 1 deletion event")

	msg := messages[0]

	// Marshal and publish to Redis
	jsonData, err := json.Marshal(msg)
	s.Require().NoError(err, "Failed to marshal message")

	streamName := "chat:raw"
	err = s.redisClient.XAdd(s.ctx, &redis.XAddArgs{
		Stream: streamName,
		Values: map[string]interface{}{
			"data": string(jsonData),
		},
	}).Err()
	s.Require().NoError(err, "Failed to publish to Redis")

	// Read back from Redis
	result, err := s.redisClient.XRange(s.ctx, streamName, "-", "+").Result()
	s.Require().NoError(err, "Failed to read from Redis")
	s.Require().Len(result, 1, "Should have 1 message in Redis")

	// Parse the message
	dataStr, ok := result[0].Values["data"].(string)
	s.Require().True(ok, "Message should have data field")

	var decoded map[string]interface{}
	err = json.Unmarshal([]byte(dataStr), &decoded)
	s.Require().NoError(err, "Failed to unmarshal message")

	// Verify RawChatMessage schema
	requiredFields := []string{
		"message_id", "platform", "channel_id", "stream_id",
		"user_id", "username", "text", "timestamp", "tags",
		"event_type", "event_data",
	}
	for _, field := range requiredFields {
		s.Contains(decoded, field, "Missing field: %s", field)
	}

	// Verify deletion event specific fields
	s.Equal("youtube", decoded["platform"])
	s.Equal("message_deletion", decoded["event_type"])
	s.Equal("", decoded["user_id"])
	s.Equal("", decoded["username"])
	s.Equal("", decoded["text"])

	// Verify event_data structure
	eventData, ok := decoded["event_data"].(map[string]interface{})
	s.Require().True(ok, "event_data should be a map")
	s.Contains(eventData, "target_msg_id")
	s.Contains(eventData, "deletion_type")
	s.Equal("single", eventData["deletion_type"])
}

// TestEmitDeletionEvent_PubSubBroadcast tests Redis Pub/Sub emission
// (Message Processor would consume from Stream and publish to Pub/Sub)
func (s *DeletionEmissionSuite) TestEmitDeletionEvent_PubSubBroadcast() {
	// Load and parse deletion event
	var response innertube.LiveChatResponse
	err := json.Unmarshal(s.fixtures["deletion_event.json"], &response)
	s.Require().NoError(err, "Failed to unmarshal fixture")

	channelID := "UC_test_channel"
	messages, err := innertube.ParseMessages(
		response.ContinuationContents.LiveChatContinuation.Actions,
		channelID,
	)
	s.Require().NoError(err, "ParseMessages should not error")
	s.Require().Len(messages, 1, "Should parse 1 deletion event")

	msg := messages[0]

	// Subscribe to Pub/Sub channel
	overlayID := "overlay_123"
	pubsubChannel := fmt.Sprintf("overlay:%s", overlayID)
	pubsub := s.redisClient.Subscribe(s.ctx, pubsubChannel)
	defer pubsub.Close()

	// Wait for subscription to be ready
	_, err = pubsub.Receive(s.ctx)
	s.Require().NoError(err, "Failed to subscribe")

	// Publish deletion event to Pub/Sub (simulating message-processor)
	jsonData, err := json.Marshal(msg)
	s.Require().NoError(err, "Failed to marshal message")

	err = s.redisClient.Publish(s.ctx, pubsubChannel, string(jsonData)).Err()
	s.Require().NoError(err, "Failed to publish to Pub/Sub")

	// Receive message from Pub/Sub
	ctx, cancel := context.WithTimeout(s.ctx, 5*time.Second)
	defer cancel()

	msgReceived, err := pubsub.ReceiveMessage(ctx)
	s.Require().NoError(err, "Failed to receive from Pub/Sub")

	// Verify message content
	var decoded map[string]interface{}
	err = json.Unmarshal([]byte(msgReceived.Payload), &decoded)
	s.Require().NoError(err, "Failed to unmarshal Pub/Sub message")

	s.Equal("message_deletion", decoded["event_type"])
	s.Equal("youtube", decoded["platform"])

	eventData, ok := decoded["event_data"].(map[string]interface{})
	s.Require().True(ok, "event_data should be a map")
	s.Equal("single", eventData["deletion_type"])
	s.NotEmpty(eventData["target_msg_id"])
}

// TestEmitDeletionEvent_MessageOrder tests that deletions preserve order
func (s *DeletionEmissionSuite) TestEmitDeletionEvent_MessageOrder() {
	// Load and parse mixed events
	var response innertube.LiveChatResponse
	err := json.Unmarshal(s.fixtures["mixed_events.json"], &response)
	s.Require().NoError(err, "Failed to unmarshal fixture")

	channelID := "UC_test_channel"
	messages, err := innertube.ParseMessages(
		response.ContinuationContents.LiveChatContinuation.Actions,
		channelID,
	)
	s.Require().NoError(err, "ParseMessages should not error")

	// Publish to Redis Stream
	streamName := "chat:raw"
	for _, msg := range messages {
		jsonData, err := json.Marshal(msg)
		s.Require().NoError(err, "Failed to marshal message")

		err = s.redisClient.XAdd(s.ctx, &redis.XAddArgs{
			Stream: streamName,
			Values: map[string]interface{}{
				"data": string(jsonData),
			},
		}).Err()
		s.Require().NoError(err, "Failed to publish to Redis")
	}

	// Read messages in order
	result, err := s.redisClient.XRange(s.ctx, streamName, "-", "+").Result()
	s.Require().NoError(err, "Failed to read from Redis")
	s.Require().Len(result, 7, "Should have 7 messages")

	// Verify order: regular, regular, regular, deletion, regular, regular, deletion
	expectedOrder := []string{"", "", "", "message_deletion", "", "", "message_deletion"}

	for i, entry := range result {
		dataStr := entry.Values["data"].(string)
		var msgData map[string]interface{}
		json.Unmarshal([]byte(dataStr), &msgData)

		eventType, _ := msgData["event_type"].(string)
		s.Equal(expectedOrder[i], eventType, "Message %d should have event_type=%s", i, expectedOrder[i])
	}
}

// TestDeletionEmissionSuite runs the emission test suite
func TestDeletionEmissionSuite(t *testing.T) {
	suite.Run(t, new(DeletionEmissionSuite))
}
