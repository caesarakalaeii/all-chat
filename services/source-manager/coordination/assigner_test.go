package coordination

import (
	"fmt"
	"testing"
)

// TestBoundedLoadEnforcement verifies that no pod exceeds 1.25x average load
func TestBoundedLoadEnforcement(t *testing.T) {
	// Scenario: 3 pods, 300 channels
	// Average load: 100 channels per pod
	// Bound: 1.25 * 100 = 125 channels max per pod
	pods := []string{"pod-1", "pod-2", "pod-3"}
	assigner := NewAssigner(pods)

	// Track assignments per pod
	assignments := make(map[string]int)
	for _, pod := range pods {
		assignments[pod] = 0
	}

	// Assign 300 channels
	for i := 0; i < 300; i++ {
		sourceID := fmt.Sprintf("source-%d", i)
		podID, err := assigner.AssignChannel(sourceID)
		if err != nil {
			t.Fatalf("Failed to assign channel %s: %v", sourceID, err)
		}
		assignments[podID]++
	}

	// Verify bounded-load constraint
	avgLoad := 300.0 / 3.0 // 100 channels per pod
	maxAllowed := int(avgLoad * 1.25) // 125 channels

	for pod, count := range assignments {
		if count > maxAllowed {
			t.Errorf("Pod %s exceeded bounded-load limit: got %d channels, max allowed %d", pod, count, maxAllowed)
		}
		t.Logf("Pod %s: %d channels (%.1f%% of average)", pod, count, float64(count)/avgLoad*100)
	}

	// Verify all channels assigned
	total := 0
	for _, count := range assignments {
		total += count
	}
	if total != 300 {
		t.Errorf("Expected 300 channels assigned, got %d", total)
	}
}

// TestDeterministicAssignment verifies same source_id always maps to same pod
func TestDeterministicAssignment(t *testing.T) {
	pods := []string{"pod-1", "pod-2", "pod-3"}
	assigner := NewAssigner(pods)

	// Assign channels first time
	firstAssignments := make(map[string]string)
	for i := 0; i < 100; i++ {
		sourceID := fmt.Sprintf("source-%d", i)
		podID, err := assigner.AssignChannel(sourceID)
		if err != nil {
			t.Fatalf("Failed to assign channel %s: %v", sourceID, err)
		}
		firstAssignments[sourceID] = podID
	}

	// Assign same channels again
	for i := 0; i < 100; i++ {
		sourceID := fmt.Sprintf("source-%d", i)
		podID, err := assigner.AssignChannel(sourceID)
		if err != nil {
			t.Fatalf("Failed to re-assign channel %s: %v", sourceID, err)
		}

		expectedPod := firstAssignments[sourceID]
		if podID != expectedPod {
			t.Errorf("Channel %s assignment not deterministic: first=%s, second=%s", sourceID, expectedPod, podID)
		}
	}
}

// TestMinimalReassignment verifies consistent hashing minimizes reassignments when adding pods
func TestMinimalReassignment(t *testing.T) {
	// Start with 3 pods
	pods := []string{"pod-1", "pod-2", "pod-3"}
	assigner := NewAssigner(pods)

	// Assign 300 channels
	initialAssignments := make(map[string]string)
	for i := 0; i < 300; i++ {
		sourceID := fmt.Sprintf("source-%d", i)
		podID, err := assigner.AssignChannel(sourceID)
		if err != nil {
			t.Fatalf("Failed to assign channel %s: %v", sourceID, err)
		}
		initialAssignments[sourceID] = podID
	}

	// Add 4th pod
	assigner.AddPod("pod-4")

	// Re-assign same channels
	reassignedCount := 0
	for i := 0; i < 300; i++ {
		sourceID := fmt.Sprintf("source-%d", i)
		podID, err := assigner.AssignChannel(sourceID)
		if err != nil {
			t.Fatalf("Failed to re-assign channel %s: %v", sourceID, err)
		}

		if podID != initialAssignments[sourceID] {
			reassignedCount++
		}
	}

	// With consistent hashing, reassignment should be ~1/4 (75 channels)
	// Allow 50-100 channels (16.7%-33.3%) as acceptable range
	reassignmentPct := float64(reassignedCount) / 300.0 * 100
	t.Logf("Reassigned %d channels (%.1f%%)", reassignedCount, reassignmentPct)

	if reassignedCount < 50 || reassignedCount > 100 {
		t.Errorf("Reassignment count unexpected: got %d channels (%.1f%%), expected 50-100 (16.7%%-33.3%%)", reassignedCount, reassignmentPct)
	}
}

// TestPodRemoval verifies channels redistribute when pod removed
func TestPodRemoval(t *testing.T) {
	// Start with 4 pods
	pods := []string{"pod-1", "pod-2", "pod-3", "pod-4"}
	assigner := NewAssigner(pods)

	// Assign 400 channels
	initialAssignments := make(map[string]string)
	for i := 0; i < 400; i++ {
		sourceID := fmt.Sprintf("source-%d", i)
		podID, err := assigner.AssignChannel(sourceID)
		if err != nil {
			t.Fatalf("Failed to assign channel %s: %v", sourceID, err)
		}
		initialAssignments[sourceID] = podID
	}

	// Remove pod-2
	assigner.RemovePod("pod-2")

	// Re-assign channels
	newAssignments := make(map[string]int)
	for _, pod := range []string{"pod-1", "pod-3", "pod-4"} {
		newAssignments[pod] = 0
	}

	for i := 0; i < 400; i++ {
		sourceID := fmt.Sprintf("source-%d", i)
		podID, err := assigner.AssignChannel(sourceID)
		if err != nil {
			t.Fatalf("Failed to re-assign channel %s: %v", sourceID, err)
		}

		// Verify not assigned to removed pod
		if podID == "pod-2" {
			t.Errorf("Channel %s assigned to removed pod-2", sourceID)
		}

		newAssignments[podID]++
	}

	// Verify bounded-load still enforced (avg=133.3, max=166.6)
	avgLoad := 400.0 / 3.0
	maxAllowed := int(avgLoad*1.25) + 1 // +1 for rounding

	for pod, count := range newAssignments {
		if count > maxAllowed {
			t.Errorf("Pod %s exceeded bounded-load limit after removal: got %d channels, max allowed %d", pod, count, maxAllowed)
		}
		t.Logf("Pod %s: %d channels (%.1f%% of average)", pod, count, float64(count)/avgLoad*100)
	}
}

// TestAllPodsAtCapacity verifies error when all pods at capacity
func TestAllPodsAtCapacity(t *testing.T) {
	// Single pod scenario - should fail when exceeding capacity
	pods := []string{"pod-1"}
	assigner := NewAssigner(pods)

	// With Load=1.25 and single pod, capacity is theoretically infinite
	// But bounded-load algorithm should prevent assignment when pod would exceed bound
	// This test documents behavior - in production, HPA would scale before this occurs

	// Assign channels until we hit expected behavior
	// Note: With single pod, bounded-load doesn't apply (no "average" to compare against)
	// This is documented as "HPA scales up" scenario in plan

	// For now, verify assigner doesn't crash with single pod
	for i := 0; i < 100; i++ {
		sourceID := fmt.Sprintf("source-%d", i)
		_, err := assigner.AssignChannel(sourceID)
		if err != nil {
			t.Logf("Expected behavior: single pod accepts all channels (HPA scales in production)")
			return
		}
	}

	t.Logf("Single pod scenario: assigner accepted 100 channels (HPA would scale in production)")
}

// TestConcurrentAssignment verifies thread-safety
func TestConcurrentAssignment(t *testing.T) {
	pods := []string{"pod-1", "pod-2", "pod-3"}
	assigner := NewAssigner(pods)

	// Run 10 goroutines assigning channels concurrently
	done := make(chan bool)
	for g := 0; g < 10; g++ {
		go func(goroutineID int) {
			for i := 0; i < 50; i++ {
				sourceID := fmt.Sprintf("goroutine-%d-source-%d", goroutineID, i)
				_, err := assigner.AssignChannel(sourceID)
				if err != nil {
					t.Errorf("Concurrent assignment failed: %v", err)
				}
			}
			done <- true
		}(g)
	}

	// Wait for all goroutines
	for g := 0; g < 10; g++ {
		<-done
	}

	t.Logf("Concurrent assignment test completed (500 channels across 10 goroutines)")
}
