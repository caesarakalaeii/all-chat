package coordination

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// setupTestRedis creates an in-memory Redis server for testing
func setupTestRedis(t *testing.T) (*redis.Client, func()) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to start miniredis: %v", err)
	}

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	cleanup := func() {
		client.Close()
		mr.Close()
	}

	return client, cleanup
}

// TestAssignmentStorageRetrieval verifies basic store/get operations
func TestAssignmentStorageRetrieval(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	registry := NewAssignmentRegistry(client)
	ctx := context.Background()

	// Store assignment
	version, err := registry.StoreAssignment(ctx, "source-1", "pod-1")
	if err != nil {
		t.Fatalf("Failed to store assignment: %v", err)
	}

	if version != 1 {
		t.Errorf("Expected version 1, got %d", version)
	}

	// Retrieve assignment
	assignment, err := registry.GetAssignment(ctx, "source-1")
	if err != nil {
		t.Fatalf("Failed to get assignment: %v", err)
	}

	if assignment.SourceID != "source-1" {
		t.Errorf("Expected source_id=source-1, got %s", assignment.SourceID)
	}

	if assignment.PodID != "pod-1" {
		t.Errorf("Expected pod_id=pod-1, got %s", assignment.PodID)
	}

	if assignment.Version != 1 {
		t.Errorf("Expected version=1, got %d", assignment.Version)
	}

	// Verify timestamp is recent (within 1 second)
	age := time.Since(assignment.Timestamp)
	if age > time.Second {
		t.Errorf("Assignment timestamp too old: %v", age)
	}
}

// TestLoadTracking verifies pod load increments correctly
func TestLoadTracking(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	registry := NewAssignmentRegistry(client)
	ctx := context.Background()

	// Store 3 assignments to pod-1
	_, err := registry.StoreAssignment(ctx, "source-1", "pod-1")
	if err != nil {
		t.Fatalf("Failed to store assignment 1: %v", err)
	}

	_, err = registry.StoreAssignment(ctx, "source-2", "pod-1")
	if err != nil {
		t.Fatalf("Failed to store assignment 2: %v", err)
	}

	_, err = registry.StoreAssignment(ctx, "source-3", "pod-1")
	if err != nil {
		t.Fatalf("Failed to store assignment 3: %v", err)
	}

	// Get pod load
	load, err := registry.GetPodLoad(ctx, "pod-1")
	if err != nil {
		t.Fatalf("Failed to get pod load: %v", err)
	}

	if load != 3 {
		t.Errorf("Expected pod-1 load=3, got %d", load)
	}
}

// TestGetLeastLoadedPod verifies finding pod with minimum load
func TestGetLeastLoadedPod(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	registry := NewAssignmentRegistry(client)
	ctx := context.Background()

	// Create load distribution: pod-1=5, pod-2=10, pod-3=3
	for i := 0; i < 5; i++ {
		registry.StoreAssignment(ctx, "source-pod1-"+string(rune(i)), "pod-1")
	}
	for i := 0; i < 10; i++ {
		registry.StoreAssignment(ctx, "source-pod2-"+string(rune(i)), "pod-2")
	}
	for i := 0; i < 3; i++ {
		registry.StoreAssignment(ctx, "source-pod3-"+string(rune(i)), "pod-3")
	}

	// Get least loaded pod
	podID, load, err := registry.GetLeastLoadedPod(ctx)
	if err != nil {
		t.Fatalf("Failed to get least loaded pod: %v", err)
	}

	if podID != "pod-3" {
		t.Errorf("Expected pod-3 as least loaded, got %s", podID)
	}

	if load != 3 {
		t.Errorf("Expected load=3 for pod-3, got %d", load)
	}
}

// TestVersionIncrement verifies global version counter increments
func TestVersionIncrement(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	registry := NewAssignmentRegistry(client)
	ctx := context.Background()

	// Store 3 assignments, verify version increments
	v1, _ := registry.StoreAssignment(ctx, "source-1", "pod-1")
	v2, _ := registry.StoreAssignment(ctx, "source-2", "pod-2")
	v3, _ := registry.StoreAssignment(ctx, "source-3", "pod-3")

	if v1 != 1 || v2 != 2 || v3 != 3 {
		t.Errorf("Version sequence incorrect: v1=%d, v2=%d, v3=%d (expected 1,2,3)", v1, v2, v3)
	}

	// Verify global version
	globalVersion, err := registry.GetGlobalVersion(ctx)
	if err != nil {
		t.Fatalf("Failed to get global version: %v", err)
	}

	if globalVersion != 3 {
		t.Errorf("Expected global version=3, got %d", globalVersion)
	}
}

// TestGetNonExistentAssignment verifies error handling
func TestGetNonExistentAssignment(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	registry := NewAssignmentRegistry(client)
	ctx := context.Background()

	// Try to get non-existent assignment
	_, err := registry.GetAssignment(ctx, "non-existent")
	if err == nil {
		t.Error("Expected error for non-existent assignment, got nil")
	}
}

// TestAssignmentUpdate verifies updating existing assignment
func TestAssignmentUpdate(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	registry := NewAssignmentRegistry(client)
	ctx := context.Background()

	// Store initial assignment
	v1, _ := registry.StoreAssignment(ctx, "source-1", "pod-1")

	// Update assignment to different pod
	v2, _ := registry.StoreAssignment(ctx, "source-1", "pod-2")

	// Verify version incremented
	if v2 != v1+1 {
		t.Errorf("Expected version to increment: v1=%d, v2=%d", v1, v2)
	}

	// Verify new pod assigned
	assignment, _ := registry.GetAssignment(ctx, "source-1")
	if assignment.PodID != "pod-2" {
		t.Errorf("Expected pod-2 after update, got %s", assignment.PodID)
	}

	// Note: Load tracking for reassignments will be handled in coordination logic
	// The registry just tracks current assignments
}

// TestConcurrentWrites verifies thread-safety of version counter
func TestConcurrentWrites(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	registry := NewAssignmentRegistry(client)
	ctx := context.Background()

	// Run 10 goroutines writing assignments concurrently
	done := make(chan int64, 10)
	for g := 0; g < 10; g++ {
		go func(id int) {
			for i := 0; i < 10; i++ {
				sourceID := string(rune(id*10 + i))
				v, err := registry.StoreAssignment(ctx, "source-g"+sourceID, "pod-1")
				if err != nil {
					t.Errorf("Concurrent write failed: %v", err)
				}
				done <- v
			}
		}(g)
	}

	// Collect all versions
	versions := make(map[int64]bool)
	for i := 0; i < 100; i++ {
		v := <-done
		if versions[v] {
			t.Errorf("Duplicate version detected: %d (not thread-safe)", v)
		}
		versions[v] = true
	}

	// Verify all versions 1-100 exist (no gaps, no duplicates)
	if len(versions) != 100 {
		t.Errorf("Expected 100 unique versions, got %d", len(versions))
	}
}

// TestRemoveAssignment verifies assignment deletion
func TestRemoveAssignment(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	registry := NewAssignmentRegistry(client)
	ctx := context.Background()

	// Store assignment
	registry.StoreAssignment(ctx, "source-1", "pod-1")

	// Remove assignment
	err := registry.RemoveAssignment(ctx, "source-1", "pod-1")
	if err != nil {
		t.Fatalf("Failed to remove assignment: %v", err)
	}

	// Verify assignment gone
	_, err = registry.GetAssignment(ctx, "source-1")
	if err == nil {
		t.Error("Expected error after removal, got nil")
	}

	// Verify load decremented
	load, _ := registry.GetPodLoad(ctx, "pod-1")
	if load != 0 {
		t.Errorf("Expected load=0 after removal, got %d", load)
	}
}

// TestGetAllAssignments verifies batch retrieval
func TestGetAllAssignments(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	registry := NewAssignmentRegistry(client)
	ctx := context.Background()

	// Store multiple assignments
	registry.StoreAssignment(ctx, "source-1", "pod-1")
	registry.StoreAssignment(ctx, "source-2", "pod-2")
	registry.StoreAssignment(ctx, "source-3", "pod-1")

	// Get all assignments for pod-1
	assignments, err := registry.GetAssignmentsForPod(ctx, "pod-1")
	if err != nil {
		t.Fatalf("Failed to get assignments for pod: %v", err)
	}

	if len(assignments) != 2 {
		t.Errorf("Expected 2 assignments for pod-1, got %d", len(assignments))
	}

	// Verify source IDs
	sourceIDs := make(map[string]bool)
	for _, a := range assignments {
		sourceIDs[a.SourceID] = true
	}

	if !sourceIDs["source-1"] || !sourceIDs["source-3"] {
		t.Errorf("Expected source-1 and source-3, got %v", sourceIDs)
	}
}
