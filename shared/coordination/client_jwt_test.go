package coordination

import (
	"context"
	"testing"
	"time"

	"github.com/caesar/all-chat/shared/auth"
	"go.uber.org/zap"
)

// TestJWTRefresh verifies that JWT tokens are refreshed automatically
func TestJWTRefresh(t *testing.T) {
	logger := zap.NewNop()
	secret := "test-secret-for-jwt-refresh"
	baseURL := "http://test-coordinator:8088"

	client := NewCoordinatorClient(baseURL, secret, logger)

	// Get initial JWT
	client.jwtMutex.RLock()
	initialJWT := client.serviceJWT
	client.jwtMutex.RUnlock()

	if initialJWT == "" {
		t.Fatal("Initial JWT should not be empty")
	}

	// Validate initial JWT
	claims, err := auth.ValidateServiceJWT(initialJWT, secret)
	if err != nil {
		t.Fatalf("Initial JWT validation failed: %v", err)
	}

	if claims.ServiceName == "" {
		t.Error("Service name should be set in JWT claims")
	}

	// Wait a bit to ensure different IssuedAt timestamp
	time.Sleep(1100 * time.Millisecond)

	// Manually trigger JWT refresh (instead of waiting 12 hours)
	client.refreshJWT(24 * time.Hour)

	// Get refreshed JWT
	client.jwtMutex.RLock()
	refreshedJWT := client.serviceJWT
	client.jwtMutex.RUnlock()

	if refreshedJWT == "" {
		t.Fatal("Refreshed JWT should not be empty")
	}

	if refreshedJWT == initialJWT {
		t.Error("Refreshed JWT should be different from initial JWT")
	}

	// Validate refreshed JWT
	refreshedClaims, err := auth.ValidateServiceJWT(refreshedJWT, secret)
	if err != nil {
		t.Fatalf("Refreshed JWT validation failed: %v", err)
	}

	if refreshedClaims.ServiceName != claims.ServiceName {
		t.Error("Service name should remain the same after refresh")
	}

	// Verify the refreshed token has a newer IssuedAt time
	if !refreshedClaims.IssuedAt.After(claims.IssuedAt.Time) {
		t.Error("Refreshed JWT should have a newer IssuedAt timestamp")
	}
}

// TestJWTRefreshConcurrency verifies thread-safe JWT access during refresh
func TestJWTRefreshConcurrency(t *testing.T) {
	logger := zap.NewNop()
	secret := "test-secret-concurrency"
	baseURL := "http://test-coordinator:8088"

	client := NewCoordinatorClient(baseURL, secret, logger)

	// Start multiple goroutines reading the JWT while refreshing
	done := make(chan struct{})
	errors := make(chan error, 100)

	// Readers
	for i := 0; i < 10; i++ {
		go func() {
			for {
				select {
				case <-done:
					return
				default:
					client.jwtMutex.RLock()
					jwt := client.serviceJWT
					client.jwtMutex.RUnlock()

					if jwt == "" {
						errors <- nil
					}

					// Validate the JWT
					_, err := auth.ValidateServiceJWT(jwt, secret)
					if err != nil {
						errors <- err
					}
				}
			}
		}()
	}

	// Writer (refresh JWT multiple times)
	for i := 0; i < 5; i++ {
		time.Sleep(10 * time.Millisecond)
		client.refreshJWT(24 * time.Hour)
	}

	// Stop readers
	close(done)
	time.Sleep(50 * time.Millisecond)

	// Check for errors
	select {
	case err := <-errors:
		if err != nil {
			t.Fatalf("Concurrent access error: %v", err)
		}
	default:
		// No errors - success
	}
}

// TestStartStopJWTRefresh verifies the refresh lifecycle
func TestStartStopJWTRefresh(t *testing.T) {
	logger := zap.NewNop()
	secret := "test-secret-lifecycle"
	baseURL := "http://test-coordinator:8088"

	client := NewCoordinatorClient(baseURL, secret, logger)
	ctx := context.Background()

	// Start refresh
	client.StartJWTRefresh(ctx)

	// Let it run briefly
	time.Sleep(50 * time.Millisecond)

	// Stop refresh
	client.StopJWTRefresh()

	// Verify we can stop multiple times without panic
	client.StopJWTRefresh()
}
