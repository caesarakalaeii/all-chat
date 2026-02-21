package lifecycle

import (
	"context"
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/suite"
)

// TestLifecycleSuite runs the lifecycle test suite
func TestLifecycleSuite(t *testing.T) {
	suite.Run(t, new(LifecycleTestSuite))
}

// TestRedisConnectivity verifies Redis container is reachable
func (s *LifecycleTestSuite) TestRedisConnectivity() {
	ctx := context.Background()
	err := s.redisClient.Ping(ctx).Err()
	s.NoError(err, "Redis should be reachable")

	// Test basic Redis operations
	err = s.redisClient.Set(ctx, "test_key", "test_value", 0).Err()
	s.NoError(err, "Redis SET should succeed")

	val, err := s.redisClient.Get(ctx, "test_key").Result()
	s.NoError(err, "Redis GET should succeed")
	s.Equal("test_value", val, "Value should match")
}

// TestPostgresConnectivity verifies PostgreSQL container is reachable
func (s *LifecycleTestSuite) TestPostgresConnectivity() {
	ctx := context.Background()
	err := s.postgresClient.Ping(ctx)
	s.NoError(err, "PostgreSQL should be reachable")

	// Test schema exists
	var count int
	err = s.postgresClient.QueryRow(ctx,
		"SELECT COUNT(*) FROM information_schema.tables WHERE table_name IN ('sources', 'overlays')",
	).Scan(&count)
	s.NoError(err, "Schema query should succeed")
	s.Equal(2, count, "Both tables should exist")
}

// TestOverlaySourceCRUD verifies basic database operations
func (s *LifecycleTestSuite) TestOverlaySourceCRUD() {
	ctx := context.Background()

	// Create overlay
	overlayID := s.InsertTestOverlay(ctx, "test-overlay")
	s.NotEmpty(overlayID, "Overlay ID should be returned")

	// Create source
	sourceID := s.InsertTestSource(ctx, overlayID, "youtube", "UC123456", false)
	s.NotEmpty(sourceID, "Source ID should be returned")

	// Update source status
	s.UpdateSourceStatus(ctx, sourceID, true)

	// Verify update
	var isActive bool
	err := s.postgresClient.QueryRow(ctx,
		"SELECT is_active FROM sources WHERE id = $1",
		sourceID,
	).Scan(&isActive)
	s.NoError(err, "Query should succeed")
	s.True(isActive, "Source should be active")
}

// TestRedisStreamOperations verifies Redis Streams functionality
func (s *LifecycleTestSuite) TestRedisStreamOperations() {
	ctx := context.Background()

	streamKey := "test:stream"

	// Add messages
	for i := 0; i < 5; i++ {
		_, err := s.redisClient.XAdd(ctx, &redis.XAddArgs{
			Stream: streamKey,
			Values: map[string]interface{}{
				"data": "message",
				"seq":  i,
			},
		}).Result()
		s.NoError(err, "XAdd should succeed")
	}

	// Check stream length
	length := s.GetRedisStreamLength(ctx, streamKey)
	s.Equal(int64(5), length, "Stream should have 5 messages")
}
