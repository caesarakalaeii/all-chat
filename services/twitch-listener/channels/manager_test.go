package channels

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/caesar/all-chat/services/twitch-listener/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// MockJoinParter implements JoinParterInterface for testing
type MockJoinParter struct {
	mu          sync.Mutex
	joined      []string
	departed    []string
	joinCalls   int
	departCalls int
}

func NewMockJoinParter() *MockJoinParter {
	return &MockJoinParter{
		joined:   make([]string, 0),
		departed: make([]string, 0),
	}
}

func (m *MockJoinParter) Join(channel string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.joined = append(m.joined, channel)
	m.joinCalls++
}

func (m *MockJoinParter) Depart(channel string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.departed = append(m.departed, channel)
	m.departCalls++
}

func (m *MockJoinParter) GetJoined() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]string, len(m.joined))
	copy(result, m.joined)
	return result
}

func (m *MockJoinParter) GetDeparted() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]string, len(m.departed))
	copy(result, m.departed)
	return result
}

func (m *MockJoinParter) GetJoinCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.joinCalls
}

func (m *MockJoinParter) GetDepartCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.departCalls
}

// MockRepository implements RepositoryInterface for testing
type MockRepository struct {
	channels []string
	err      error
}

func (m *MockRepository) GetActiveChannels(ctx context.Context) ([]models.ChannelSource, error) {
	// Not used in these tests, but required by interface
	return nil, nil
}

func (m *MockRepository) GetUniqueChannels(ctx context.Context) ([]string, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.channels, nil
}

func (m *MockRepository) SetSourceActive(ctx context.Context, channelName string, isActive bool) error {
	// Not used in these tests, but required by interface
	return nil
}

func (m *MockRepository) GetSourceIDsForChannels(ctx context.Context, channels []string) map[string]string {
	// Return a simple map for testing - channel name maps to itself as source ID
	result := make(map[string]string)
	for _, ch := range channels {
		result[ch] = ch + "-source-id"
	}
	return result
}

func (m *MockRepository) GetOverlayIDsForChannel(ctx context.Context, channelName string) ([]string, error) {
	// Return a test overlay ID for cross-platform event testing
	return []string{"test-overlay-" + channelName}, nil
}

func TestManager_SyncChannels_InitialJoin(t *testing.T) {
	ctx := context.Background()
	logger := zaptest.NewLogger(t)

	repo := &MockRepository{
		channels: []string{"xqc", "summit1g", "shroud"},
	}

	mockJP := NewMockJoinParter()
	manager := NewManager(repo, mockJP, nil, nil, nil, nil, "", logger, nil)

	err := manager.SyncChannels(ctx)
	require.NoError(t, err)

	// Verify all channels were joined
	joined := mockJP.GetJoined()
	assert.Len(t, joined, 3)
	assert.Contains(t, joined, "xqc")
	assert.Contains(t, joined, "summit1g")
	assert.Contains(t, joined, "shroud")

	// Verify manager tracks them as active
	assert.Equal(t, 3, manager.GetActiveChannelCount())
	assert.True(t, manager.IsChannelActive("xqc"))
	assert.True(t, manager.IsChannelActive("summit1g"))
	assert.True(t, manager.IsChannelActive("shroud"))
}

func TestManager_SyncChannels_PartRemovedChannels(t *testing.T) {
	ctx := context.Background()
	logger := zaptest.NewLogger(t)

	repo := &MockRepository{
		channels: []string{"xqc", "summit1g"},
	}

	mockJP := NewMockJoinParter()
	manager := NewManager(repo, mockJP, nil, nil, nil, nil, "", logger, nil)

	// Initial sync
	err := manager.SyncChannels(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, manager.GetActiveChannelCount())

	// Update repo to remove one channel
	repo.channels = []string{"xqc"}

	// Sync again
	err = manager.SyncChannels(ctx)
	require.NoError(t, err)

	// Verify summit1g was parted
	departed := mockJP.GetDeparted()
	assert.Len(t, departed, 1)
	assert.Contains(t, departed, "summit1g")

	// Verify manager only tracks xqc now
	assert.Equal(t, 1, manager.GetActiveChannelCount())
	assert.True(t, manager.IsChannelActive("xqc"))
	assert.False(t, manager.IsChannelActive("summit1g"))
}

func TestManager_SyncChannels_JoinNewChannels(t *testing.T) {
	ctx := context.Background()
	logger := zaptest.NewLogger(t)

	repo := &MockRepository{
		channels: []string{"xqc"},
	}

	mockJP := NewMockJoinParter()
	manager := NewManager(repo, mockJP, nil, nil, nil, nil, "", logger, nil)

	// Initial sync
	err := manager.SyncChannels(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, manager.GetActiveChannelCount())

	// Add new channels
	repo.channels = []string{"xqc", "summit1g", "shroud"}

	// Sync again
	err = manager.SyncChannels(ctx)
	require.NoError(t, err)

	// Verify new channels were joined
	joined := mockJP.GetJoined()
	assert.GreaterOrEqual(t, len(joined), 3) // At least 3 (initial + new)

	// Verify manager tracks all 3 channels
	assert.Equal(t, 3, manager.GetActiveChannelCount())
	assert.True(t, manager.IsChannelActive("xqc"))
	assert.True(t, manager.IsChannelActive("summit1g"))
	assert.True(t, manager.IsChannelActive("shroud"))
}

func TestManager_SyncChannels_NoChanges(t *testing.T) {
	ctx := context.Background()
	logger := zaptest.NewLogger(t)

	repo := &MockRepository{
		channels: []string{"xqc", "summit1g"},
	}

	mockJP := NewMockJoinParter()
	manager := NewManager(repo, mockJP, nil, nil, nil, nil, "", logger, nil)

	// Initial sync
	err := manager.SyncChannels(ctx)
	require.NoError(t, err)

	initialJoinCount := mockJP.GetJoinCallCount()
	initialDepartCount := mockJP.GetDepartCallCount()

	// Sync again with no changes
	err = manager.SyncChannels(ctx)
	require.NoError(t, err)

	// Verify no new joins or parts
	assert.Equal(t, initialJoinCount, mockJP.GetJoinCallCount())
	assert.Equal(t, initialDepartCount, mockJP.GetDepartCallCount())
}

func TestManager_SyncChannels_EmptyChannels(t *testing.T) {
	ctx := context.Background()
	logger := zaptest.NewLogger(t)

	repo := &MockRepository{
		channels: []string{},
	}

	mockJP := NewMockJoinParter()
	manager := NewManager(repo, mockJP, nil, nil, nil, nil, "", logger, nil)

	err := manager.SyncChannels(ctx)
	require.NoError(t, err)

	// No channels should be joined
	assert.Equal(t, 0, manager.GetActiveChannelCount())
	assert.Equal(t, 0, mockJP.GetJoinCallCount())
}

func TestManager_RateLimiting(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping rate limiting test in short mode")
	}

	ctx := context.Background()
	logger := zaptest.NewLogger(t)

	// Create a lot of channels to test rate limiting
	channels := make([]string, 50)
	for i := 0; i < 50; i++ {
		channels[i] = string(rune('a' + i))
	}

	repo := &MockRepository{
		channels: channels,
	}

	mockJP := NewMockJoinParter()
	manager := NewManager(repo, mockJP, nil, nil, nil, nil, "", logger, nil)

	start := time.Now()
	err := manager.SyncChannels(ctx)
	require.NoError(t, err)
	duration := time.Since(start)

	// With burst allowance of 20 and 1 event per 500ms afterwards, 50 joins take >=15s
	expectedMinDuration := 15 * time.Second
	assert.GreaterOrEqual(t, duration, expectedMinDuration,
		"Rate limiting should enforce minimum duration")

	// All channels should be joined
	assert.Equal(t, 50, manager.GetActiveChannelCount())
}

func TestManager_GetActiveChannels(t *testing.T) {
	ctx := context.Background()
	logger := zaptest.NewLogger(t)

	repo := &MockRepository{
		channels: []string{"xqc", "summit1g", "shroud"},
	}

	mockJP := NewMockJoinParter()
	manager := NewManager(repo, mockJP, nil, nil, nil, nil, "", logger, nil)

	err := manager.SyncChannels(ctx)
	require.NoError(t, err)

	activeChannels := manager.GetActiveChannels()
	assert.Len(t, activeChannels, 3)
	assert.Contains(t, activeChannels, "xqc")
	assert.Contains(t, activeChannels, "summit1g")
	assert.Contains(t, activeChannels, "shroud")
}

func TestManager_StartStop(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	logger := zaptest.NewLogger(t)

	repo := &MockRepository{
		channels: []string{"xqc"},
	}

	mockJP := NewMockJoinParter()
	manager := NewManager(repo, mockJP, nil, nil, nil, nil, "", logger, nil)

	// Start manager
	err := manager.Start(ctx)
	require.NoError(t, err)

	// Let it run for a bit
	time.Sleep(100 * time.Millisecond)

	// Stop manager
	manager.Stop()

	// Verify at least initial sync happened
	assert.Greater(t, mockJP.GetJoinCallCount(), 0)
}

func TestManager_ConcurrentAccess(t *testing.T) {
	ctx := context.Background()
	logger := zaptest.NewLogger(t)

	repo := &MockRepository{
		channels: []string{"xqc"},
	}

	mockJP := NewMockJoinParter()
	manager := NewManager(repo, mockJP, nil, nil, nil, nil, "", logger, nil)

	err := manager.SyncChannels(ctx)
	require.NoError(t, err)

	// Concurrent reads should not panic
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = manager.GetActiveChannels()
			_ = manager.GetActiveChannelCount()
			_ = manager.IsChannelActive("xqc")
		}()
	}

	wg.Wait()
}
