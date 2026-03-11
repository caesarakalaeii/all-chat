package jobs

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/caesar/all-chat/services/share-service/models"
	"github.com/caesar/all-chat/services/share-service/repository"
	"github.com/caesar/all-chat/shared/database"
	"github.com/caesar/all-chat/shared/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// getTestDatabaseURL returns the database connection string for testing
func getTestDatabaseURL() string {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		// Default to localhost for manual testing
		dbURL = "postgresql://allchat:allchat_dev_password@localhost:5432/allchat?sslmode=disable"
	}
	return dbURL
}

func TestExpiryJob(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Connect to test database
	dbPool, err := database.NewPostgresPool(getTestDatabaseURL())
	if err != nil {
		t.Skipf("Skipping test: database not available: %v", err)
	}
	defer dbPool.Close()

	log := logger.NewLogger("test", "error")
	repo := repository.NewShareRepository(dbPool, log)

	// Insert expired request manually
	query := `
		INSERT INTO share_requests (sender_user_id, sender_overlay_id, recipient_user_id, status, created_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`
	var requestID string
	err = dbPool.QueryRow(context.Background(), query,
		"test-sender-expiry",
		"test-overlay-expiry",
		"test-recipient-expiry",
		models.StatusPending,
		time.Now().Add(-8*24*time.Hour), // 8 days ago
		time.Now().Add(-1*time.Hour),    // Expired 1 hour ago
	).Scan(&requestID)
	require.NoError(t, err)

	// Clean up test data after test
	defer func() {
		_, _ = dbPool.Exec(context.Background(), "DELETE FROM share_requests WHERE id = $1", requestID)
	}()

	// Create job with short interval for testing
	job := NewExpiryJob(repo, log)
	// Override ticker for faster testing
	job.ticker.Stop()
	job.ticker = time.NewTicker(100 * time.Millisecond)

	// Start job
	job.Start(context.Background())

	// Wait for tick to execute
	time.Sleep(200 * time.Millisecond)

	// Stop job
	job.Stop()

	// Verify request was expired
	req, err := repo.GetByID(context.Background(), requestID)
	require.NoError(t, err)
	assert.Equal(t, models.StatusExpired, req.Status)
	assert.NotNil(t, req.RespondedAt)
}

func TestExpiryJob_NoPendingRequests(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Connect to test database
	dbPool, err := database.NewPostgresPool(getTestDatabaseURL())
	if err != nil {
		t.Skipf("Skipping test: database not available: %v", err)
	}
	defer dbPool.Close()

	log := logger.NewLogger("test", "error")
	repo := repository.NewShareRepository(dbPool, log)

	// Create job
	job := NewExpiryJob(repo, log)
	job.ticker.Stop()
	job.ticker = time.NewTicker(100 * time.Millisecond)

	// Start and stop immediately (should handle empty database gracefully)
	job.Start(context.Background())
	time.Sleep(200 * time.Millisecond)
	job.Stop()

	// No assertion needed - test passes if no panic or error
}

func TestExpiryJob_OnlyExpiresPending(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Connect to test database
	dbPool, err := database.NewPostgresPool(getTestDatabaseURL())
	if err != nil {
		t.Skipf("Skipping test: database not available: %v", err)
	}
	defer dbPool.Close()

	log := logger.NewLogger("test", "error")
	repo := repository.NewShareRepository(dbPool, log)

	// Insert accepted request with expired date (should NOT be expired)
	query := `
		INSERT INTO share_requests (sender_user_id, sender_overlay_id, recipient_user_id, status, created_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`
	var requestID string
	err = dbPool.QueryRow(context.Background(), query,
		"test-sender-accepted",
		"test-overlay-accepted",
		"test-recipient-accepted",
		models.StatusAccepted,
		time.Now().Add(-8*24*time.Hour),
		time.Now().Add(-1*time.Hour), // Expired date but already accepted
	).Scan(&requestID)
	require.NoError(t, err)

	// Clean up test data after test
	defer func() {
		_, _ = dbPool.Exec(context.Background(), "DELETE FROM share_requests WHERE id = $1", requestID)
	}()

	// Run expiry job
	job := NewExpiryJob(repo, log)
	job.ticker.Stop()
	job.ticker = time.NewTicker(100 * time.Millisecond)
	job.Start(context.Background())
	time.Sleep(200 * time.Millisecond)
	job.Stop()

	// Verify request is still accepted (not changed to expired)
	req, err := repo.GetByID(context.Background(), requestID)
	require.NoError(t, err)
	assert.Equal(t, models.StatusAccepted, req.Status)
}

// TestExpiryJob_TimedAcceptedShares verifies that accepted shares with a
// past share_expires_at are expired by the job (EXPIRY-04).
// Wave 0: RED stub — ExpireTimedAcceptedShares does not exist yet.
func TestExpiryJob_TimedAcceptedShares(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	// RED: repo.ExpireTimedAcceptedShares does not exist yet.
	// This test will fail to compile until Wave 1 adds the method.
	dbPool, err := database.NewPostgresPool(getTestDatabaseURL())
	if err != nil {
		t.Skipf("Skipping test: database not available: %v", err)
	}
	defer dbPool.Close()

	log := logger.NewLogger("test", "error")
	repo := repository.NewShareRepository(dbPool, log)

	count, err := repo.ExpireTimedAcceptedShares(context.Background())
	require.NoError(t, err)
	assert.GreaterOrEqual(t, count, 0)
}
