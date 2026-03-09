package cycles

import (
	"context"
	"testing"

	"github.com/caesar/all-chat/services/share-service/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockShareRepository implements a mock for testing cycle detection
type mockShareRepository struct {
	shares map[string][]models.ShareRequest // recipientUserID -> list of accepted shares
}

func newMockRepo() *mockShareRepository {
	return &mockShareRepository{
		shares: make(map[string][]models.ShareRequest),
	}
}

// addShare adds a mock accepted share for testing
func (m *mockShareRepository) addShare(recipientUserID, senderUserID string) {
	share := models.ShareRequest{
		ID:              "test-share",
		SenderUserID:    senderUserID,
		RecipientUserID: recipientUserID,
		Status:          models.StatusAccepted,
	}
	m.shares[recipientUserID] = append(m.shares[recipientUserID], share)
}

// GetAcceptedSharesByRecipient returns accepted shares where recipient_user_id = userID
func (m *mockShareRepository) GetAcceptedSharesByRecipient(ctx context.Context, userID string) ([]models.ShareRequest, error) {
	return m.shares[userID], nil
}

func TestHasCycle_NoExistingShares(t *testing.T) {
	repo := newMockRepo()
	detector := NewCycleDetector(repo)

	// A→B with no other shares should return false
	hasCycle, err := detector.HasCycle(context.Background(), "userA", "userB")
	require.NoError(t, err)
	assert.False(t, hasCycle, "Should not detect cycle when no shares exist")
}

func TestHasCycle_DirectCycle(t *testing.T) {
	repo := newMockRepo()
	detector := NewCycleDetector(repo)

	// Existing share: A→B (userB is recipient, shares to userA)
	repo.addShare("userB", "userA")

	// Attempting to create B→A should detect cycle
	hasCycle, err := detector.HasCycle(context.Background(), "userB", "userA")
	require.NoError(t, err)
	assert.True(t, hasCycle, "Should detect direct cycle A→B→A")
}

func TestHasCycle_IndirectCycle(t *testing.T) {
	repo := newMockRepo()
	detector := NewCycleDetector(repo)

	// Existing shares: A→B→C (A shares to B, B shares to C)
	repo.addShare("userB", "userA") // B is recipient, shares to A
	repo.addShare("userC", "userB") // C is recipient, shares to B

	// Attempting to create C→A should detect cycle
	hasCycle, err := detector.HasCycle(context.Background(), "userC", "userA")
	require.NoError(t, err)
	assert.True(t, hasCycle, "Should detect indirect cycle A→B→C→A")
}

func TestHasCycle_ValidChain(t *testing.T) {
	repo := newMockRepo()
	detector := NewCycleDetector(repo)

	// Existing shares: A→B→C
	repo.addShare("userB", "userA")
	repo.addShare("userC", "userB")

	// Creating D→E should not detect cycle (disconnected)
	hasCycle, err := detector.HasCycle(context.Background(), "userD", "userE")
	require.NoError(t, err)
	assert.False(t, hasCycle, "Should not detect cycle for disconnected chain")
}

func TestHasCycle_DisconnectedGraph(t *testing.T) {
	repo := newMockRepo()
	detector := NewCycleDetector(repo)

	// Chain 1: A→B→C
	repo.addShare("userB", "userA")
	repo.addShare("userC", "userB")

	// Chain 2: X→Y→Z
	repo.addShare("userY", "userX")
	repo.addShare("userZ", "userY")

	// Creating D→E (new chain) should not detect cycle
	hasCycle, err := detector.HasCycle(context.Background(), "userD", "userE")
	require.NoError(t, err)
	assert.False(t, hasCycle, "Should not detect cycle in disconnected graph")

	// But C→A should still detect cycle in chain 1
	hasCycle, err = detector.HasCycle(context.Background(), "userC", "userA")
	require.NoError(t, err)
	assert.True(t, hasCycle, "Should detect cycle within chain 1")

	// And Z→X should detect cycle in chain 2
	hasCycle, err = detector.HasCycle(context.Background(), "userZ", "userX")
	require.NoError(t, err)
	assert.True(t, hasCycle, "Should detect cycle within chain 2")
}

func TestHasCycle_SelfLoop(t *testing.T) {
	repo := newMockRepo()
	detector := NewCycleDetector(repo)

	// Attempting A→A should detect cycle (though this should be prevented by validation)
	hasCycle, err := detector.HasCycle(context.Background(), "userA", "userA")
	require.NoError(t, err)
	assert.True(t, hasCycle, "Should detect self-loop as cycle")
}

func TestHasCycle_ComplexGraph(t *testing.T) {
	repo := newMockRepo()
	detector := NewCycleDetector(repo)

	// Diamond pattern: A→B, A→C, B→D, C→D
	repo.addShare("userB", "userA")
	repo.addShare("userC", "userA")
	repo.addShare("userD", "userB")
	repo.addShare("userD", "userC")

	// D→A would create cycle through multiple paths
	hasCycle, err := detector.HasCycle(context.Background(), "userD", "userA")
	require.NoError(t, err)
	assert.True(t, hasCycle, "Should detect cycle in diamond graph")

	// E→D should be valid (no cycle)
	hasCycle, err = detector.HasCycle(context.Background(), "userE", "userD")
	require.NoError(t, err)
	assert.False(t, hasCycle, "Should not detect cycle for valid edge in diamond graph")
}

func TestHasCycle_LongChain(t *testing.T) {
	repo := newMockRepo()
	detector := NewCycleDetector(repo)

	// Long chain: A→B→C→D→E→F
	repo.addShare("userB", "userA")
	repo.addShare("userC", "userB")
	repo.addShare("userD", "userC")
	repo.addShare("userE", "userD")
	repo.addShare("userF", "userE")

	// F→A would create cycle
	hasCycle, err := detector.HasCycle(context.Background(), "userF", "userA")
	require.NoError(t, err)
	assert.True(t, hasCycle, "Should detect cycle in long chain")

	// F→G should be valid
	hasCycle, err = detector.HasCycle(context.Background(), "userF", "userG")
	require.NoError(t, err)
	assert.False(t, hasCycle, "Should not detect cycle extending chain")
}
