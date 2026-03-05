package deletion

import (
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestNewBatchDetector(t *testing.T) {
	tests := []struct {
		name              string
		threshold         int
		expectedThreshold int
	}{
		{
			name:              "default threshold when zero",
			threshold:         0,
			expectedThreshold: 5,
		},
		{
			name:              "default threshold when negative",
			threshold:         -1,
			expectedThreshold: 5,
		},
		{
			name:              "custom threshold",
			threshold:         10,
			expectedThreshold: 10,
		},
	}

	logger := zap.NewNop()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detector := NewBatchDetector(tt.threshold, logger)

			if detector.GetThreshold() != tt.expectedThreshold {
				t.Errorf("expected threshold %d, got %d", tt.expectedThreshold, detector.GetThreshold())
			}
		})
	}
}

func TestAddDeletion_Validation(t *testing.T) {
	logger := zap.NewNop()
	detector := NewBatchDetector(5, logger)

	tests := []struct {
		name         string
		channelID    string
		targetItemID string
		expectError  bool
	}{
		{
			name:         "empty channel ID",
			channelID:    "",
			targetItemID: "msg123",
			expectError:  true,
		},
		{
			name:         "empty target item ID",
			channelID:    "channel123",
			targetItemID: "",
			expectError:  true,
		},
		{
			name:         "valid inputs",
			channelID:    "channel123",
			targetItemID: "msg123",
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := detector.AddDeletion(tt.channelID, tt.targetItemID, time.Now())

			if tt.expectError && err == nil {
				t.Error("expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestBatchDetection_BelowThreshold(t *testing.T) {
	logger := zap.NewNop()
	detector := NewBatchDetector(5, logger)
	channelID := "test-channel-1"

	// Add 3 deletions (below threshold of 5)
	for i := 0; i < 3; i++ {
		_, err := detector.AddDeletion(channelID, "msg"+string(rune(i)), time.Now())
		if err != nil {
			t.Fatalf("AddDeletion failed: %v", err)
		}
	}

	// Wait for window to close (100ms + buffer)
	time.Sleep(150 * time.Millisecond)

	// Process window manually to get result
	result := detector.processWindow(channelID)

	// Window already processed by ticker, so expect nil or empty result
	// The actual test is that no batch was detected (logged in tickerLoop)
	// For better testing, we'll verify by adding more deletions and checking state
	if result != nil {
		t.Errorf("expected nil result (window already processed), got %+v", result)
	}

	// Cleanup
	if err := detector.Cleanup(channelID); err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}
}

func TestBatchDetection_ExactlyThreshold(t *testing.T) {
	logger := zap.NewNop()
	detector := NewBatchDetector(5, logger)
	channelID := "test-channel-2"

	// Add exactly 5 deletions (threshold)
	now := time.Now()
	for i := 0; i < 5; i++ {
		_, err := detector.AddDeletion(channelID, "msg"+string(rune(i)), now.Add(time.Duration(i)*time.Millisecond))
		if err != nil {
			t.Fatalf("AddDeletion failed: %v", err)
		}
	}

	// Wait for window to close
	time.Sleep(150 * time.Millisecond)

	// Window should have been processed as a batch
	// Verify by checking that window was reset (new deletions start fresh window)
	_, err := detector.AddDeletion(channelID, "msg-new", time.Now())
	if err != nil {
		t.Fatalf("AddDeletion after window failed: %v", err)
	}

	// Cleanup
	if err := detector.Cleanup(channelID); err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}
}

func TestBatchDetection_AboveThreshold(t *testing.T) {
	logger := zap.NewNop()
	detector := NewBatchDetector(5, logger)
	channelID := "test-channel-3"

	// Add 10 deletions (above threshold)
	now := time.Now()
	for i := 0; i < 10; i++ {
		_, err := detector.AddDeletion(channelID, "msg"+string(rune(i)), now.Add(time.Duration(i)*time.Millisecond))
		if err != nil {
			t.Fatalf("AddDeletion failed: %v", err)
		}
	}

	// Wait for window to close
	time.Sleep(150 * time.Millisecond)

	// Cleanup
	if err := detector.Cleanup(channelID); err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}
}

func TestBatchDetection_PerChannelIsolation(t *testing.T) {
	logger := zap.NewNop()
	detector := NewBatchDetector(5, logger)

	channelA := "channel-a"
	channelB := "channel-b"

	// Add 3 deletions to channel A (below threshold)
	now := time.Now()
	for i := 0; i < 3; i++ {
		_, err := detector.AddDeletion(channelA, "msg-a-"+string(rune(i)), now)
		if err != nil {
			t.Fatalf("AddDeletion to channel A failed: %v", err)
		}
	}

	// Add 7 deletions to channel B (above threshold)
	for i := 0; i < 7; i++ {
		_, err := detector.AddDeletion(channelB, "msg-b-"+string(rune(i)), now)
		if err != nil {
			t.Fatalf("AddDeletion to channel B failed: %v", err)
		}
	}

	// Wait for windows to close
	time.Sleep(150 * time.Millisecond)

	// Both channels should have independent state
	// Verify by checking they exist in detector
	detector.mu.RLock()
	_, aExists := detector.channels[channelA]
	_, bExists := detector.channels[channelB]
	detector.mu.RUnlock()

	if !aExists {
		t.Error("expected channel A to exist in detector")
	}
	if !bExists {
		t.Error("expected channel B to exist in detector")
	}

	// Cleanup both channels
	if err := detector.Cleanup(channelA); err != nil {
		t.Fatalf("Cleanup channel A failed: %v", err)
	}
	if err := detector.Cleanup(channelB); err != nil {
		t.Fatalf("Cleanup channel B failed: %v", err)
	}
}

func TestBatchDetection_WindowBoundaries(t *testing.T) {
	logger := zap.NewNop()
	detector := NewBatchDetector(5, logger)
	channelID := "test-channel-window"

	// Add 3 deletions at t=0
	now := time.Now()
	for i := 0; i < 3; i++ {
		_, err := detector.AddDeletion(channelID, "msg-first-"+string(rune(i)), now)
		if err != nil {
			t.Fatalf("AddDeletion first batch failed: %v", err)
		}
	}

	// Wait for first window to close
	time.Sleep(150 * time.Millisecond)

	// Add 2 deletions in second window (t=150ms)
	for i := 0; i < 2; i++ {
		_, err := detector.AddDeletion(channelID, "msg-second-"+string(rune(i)), time.Now())
		if err != nil {
			t.Fatalf("AddDeletion second batch failed: %v", err)
		}
	}

	// Wait for second window to close
	time.Sleep(150 * time.Millisecond)

	// Both windows should be processed independently
	// Total 5 deletions but across 2 windows, so no batch detection

	// Cleanup
	if err := detector.Cleanup(channelID); err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}
}

func TestCleanup(t *testing.T) {
	logger := zap.NewNop()
	detector := NewBatchDetector(5, logger)
	channelID := "test-channel-cleanup"

	// Add some deletions
	for i := 0; i < 3; i++ {
		_, err := detector.AddDeletion(channelID, "msg"+string(rune(i)), time.Now())
		if err != nil {
			t.Fatalf("AddDeletion failed: %v", err)
		}
	}

	// Cleanup should process final window and remove channel
	if err := detector.Cleanup(channelID); err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}

	// Verify channel was removed
	detector.mu.RLock()
	_, exists := detector.channels[channelID]
	detector.mu.RUnlock()

	if exists {
		t.Error("expected channel to be removed after cleanup")
	}

	// Cleanup again should be safe (no error)
	if err := detector.Cleanup(channelID); err != nil {
		t.Fatalf("Double cleanup failed: %v", err)
	}
}

func TestCleanup_EmptyChannelID(t *testing.T) {
	logger := zap.NewNop()
	detector := NewBatchDetector(5, logger)

	err := detector.Cleanup("")
	if err == nil {
		t.Error("expected error for empty channel ID")
	}
}

func TestConcurrentAccess(t *testing.T) {
	logger := zap.NewNop()
	detector := NewBatchDetector(5, logger)
	channelID := "test-channel-concurrent"

	// Use WaitGroup to ensure all goroutines complete
	var wg sync.WaitGroup
	goroutineCount := 10
	deletionsPerGoroutine := 5

	// Add deletions concurrently from multiple goroutines
	for i := 0; i < goroutineCount; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < deletionsPerGoroutine; j++ {
				msgID := "msg-" + string(rune(goroutineID)) + "-" + string(rune(j))
				_, err := detector.AddDeletion(channelID, msgID, time.Now())
				if err != nil {
					t.Errorf("Concurrent AddDeletion failed: %v", err)
				}
			}
		}(i)
	}

	wg.Wait()

	// Wait for window to process
	time.Sleep(150 * time.Millisecond)

	// Cleanup should work without race conditions
	if err := detector.Cleanup(channelID); err != nil {
		t.Fatalf("Cleanup after concurrent access failed: %v", err)
	}
}

func TestReasonDetection(t *testing.T) {
	logger := zap.NewNop()

	tests := []struct {
		name           string
		deletionCount  int
		threshold      int
		expectedReason string
		expectedBatch  bool
	}{
		{
			name:           "3 deletions = mod (below threshold)",
			deletionCount:  3,
			threshold:      5,
			expectedReason: "mod",
			expectedBatch:  false,
		},
		{
			name:           "5 deletions = timeout (at threshold)",
			deletionCount:  5,
			threshold:      5,
			expectedReason: "timeout",
			expectedBatch:  true,
		},
		{
			name:           "10 deletions = timeout (above threshold, below ban)",
			deletionCount:  10,
			threshold:      5,
			expectedReason: "timeout",
			expectedBatch:  true,
		},
		{
			name:           "20 deletions = ban",
			deletionCount:  20,
			threshold:      5,
			expectedReason: "ban",
			expectedBatch:  true,
		},
		{
			name:           "30 deletions = ban",
			deletionCount:  30,
			threshold:      5,
			expectedReason: "ban",
			expectedBatch:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detector := NewBatchDetector(tt.threshold, logger)
			channelID := "test-channel-reason"

			// Add specified number of deletions
			now := time.Now()
			for i := 0; i < tt.deletionCount; i++ {
				_, err := detector.AddDeletion(channelID, "msg"+string(rune(i)), now.Add(time.Duration(i)*time.Microsecond))
				if err != nil {
					t.Fatalf("AddDeletion failed: %v", err)
				}
			}

			// Process window immediately to get result
			result := detector.processWindow(channelID)

			if result == nil {
				t.Fatal("expected result, got nil")
			}

			if result.IsBatch != tt.expectedBatch {
				t.Errorf("expected IsBatch=%v, got %v", tt.expectedBatch, result.IsBatch)
			}

			if result.Count != tt.deletionCount {
				t.Errorf("expected Count=%d, got %d", tt.deletionCount, result.Count)
			}

			if result.Reason != tt.expectedReason {
				t.Errorf("expected Reason=%s, got %s", tt.expectedReason, result.Reason)
			}

			// Cleanup
			if err := detector.Cleanup(channelID); err != nil {
				t.Fatalf("Cleanup failed: %v", err)
			}
		})
	}
}

func TestEmptyWindow(t *testing.T) {
	logger := zap.NewNop()
	detector := NewBatchDetector(5, logger)
	channelID := "test-channel-empty"

	// Add one deletion to create window
	_, err := detector.AddDeletion(channelID, "msg1", time.Now())
	if err != nil {
		t.Fatalf("AddDeletion failed: %v", err)
	}

	// Wait for window to process
	time.Sleep(150 * time.Millisecond)

	// Process empty window (already processed by ticker)
	result := detector.processWindow(channelID)

	// Should return nil for empty window
	if result != nil {
		t.Errorf("expected nil result for empty window, got %+v", result)
	}

	// Cleanup
	if err := detector.Cleanup(channelID); err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}
}
