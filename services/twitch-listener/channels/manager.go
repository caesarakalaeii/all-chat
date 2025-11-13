package channels

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

const (
	// Twitch rate limits for authenticated connections
	// JOIN: 20 channels per 10 seconds
	JoinRateLimit = 20
	JoinRatePer   = 10 * time.Second

	// Sync interval for checking database for channel changes
	SyncInterval = 30 * time.Second
)

// JoinParterInterface defines the interface for joining/parting channels
// This allows for mocking in tests
type JoinParterInterface interface {
	Join(channel string)
	Depart(channel string)
}

// Manager manages which Twitch channels to monitor
type Manager struct {
	repo        RepositoryInterface
	joinParter  JoinParterInterface
	logger      *zap.Logger
	rateLimiter *rate.Limiter
	activeChans map[string]bool // Currently joined channels
	mu          sync.RWMutex
	syncTicker  *time.Ticker
	stopChan    chan struct{}
	wg          sync.WaitGroup
}

// NewManager creates a new channel manager
func NewManager(repo RepositoryInterface, joinParter JoinParterInterface, logger *zap.Logger) *Manager {
	return &Manager{
		repo:        repo,
		joinParter:  joinParter,
		logger:      logger,
		rateLimiter: rate.NewLimiter(rate.Every(JoinRatePer/JoinRateLimit), JoinRateLimit),
		activeChans: make(map[string]bool),
		syncTicker:  time.NewTicker(SyncInterval),
		stopChan:    make(chan struct{}),
	}
}

// Start begins the periodic sync process
func (m *Manager) Start(ctx context.Context) error {
	// Initial sync
	if err := m.SyncChannels(ctx); err != nil {
		return err
	}

	// Start periodic sync
	m.wg.Add(1)
	go m.syncLoop(ctx)

	m.logger.Info("Channel manager started",
		zap.Duration("sync_interval", SyncInterval),
	)

	return nil
}

// Stop gracefully stops the channel manager
func (m *Manager) Stop() {
	close(m.stopChan)
	m.syncTicker.Stop()
	m.wg.Wait()

	m.logger.Info("Channel manager stopped")
}

// syncLoop periodically syncs channels from database
func (m *Manager) syncLoop(ctx context.Context) {
	defer m.wg.Done()

	for {
		select {
		case <-m.syncTicker.C:
			if err := m.SyncChannels(ctx); err != nil {
				m.logger.Error("Failed to sync channels", zap.Error(err))
			}
		case <-m.stopChan:
			return
		case <-ctx.Done():
			return
		}
	}
}

// SyncChannels queries the database and updates joined channels
func (m *Manager) SyncChannels(ctx context.Context) error {
	// Get unique channels from database
	desiredChannels, err := m.repo.GetUniqueChannels(ctx)
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Convert to map for easier lookup
	desiredMap := make(map[string]bool)
	for _, ch := range desiredChannels {
		desiredMap[ch] = true
	}

	// Find channels to JOIN (in desired but not active)
	toJoin := make([]string, 0)
	for ch := range desiredMap {
		if !m.activeChans[ch] {
			toJoin = append(toJoin, ch)
		}
	}

	// Find channels to PART (in active but not desired)
	toPart := make([]string, 0)
	for ch := range m.activeChans {
		if !desiredMap[ch] {
			toPart = append(toPart, ch)
		}
	}

	// PART channels first (no rate limit)
	for _, ch := range toPart {
		m.partChannel(ch)
	}

	// JOIN new channels with rate limiting
	for _, ch := range toJoin {
		// Wait for rate limiter
		if err := m.rateLimiter.Wait(ctx); err != nil {
			m.logger.Warn("Rate limiter wait interrupted", zap.Error(err))
			break
		}
		m.joinChannel(ch)
	}

	m.logger.Info("Channel sync completed",
		zap.Int("total_active", len(m.activeChans)),
		zap.Int("joined", len(toJoin)),
		zap.Int("parted", len(toPart)),
	)

	return nil
}

// joinChannel joins a channel and tracks it
func (m *Manager) joinChannel(channel string) {
	m.joinParter.Join(channel)
	m.activeChans[channel] = true

	m.logger.Info("Joined channel",
		zap.String("channel", channel),
	)
}

// partChannel parts a channel and removes from tracking
func (m *Manager) partChannel(channel string) {
	m.joinParter.Depart(channel)
	delete(m.activeChans, channel)

	m.logger.Info("Parted channel",
		zap.String("channel", channel),
	)
}

// GetActiveChannels returns the list of currently joined channels
func (m *Manager) GetActiveChannels() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	channels := make([]string, 0, len(m.activeChans))
	for ch := range m.activeChans {
		channels = append(channels, ch)
	}
	return channels
}

// GetActiveChannelCount returns the number of currently joined channels
func (m *Manager) GetActiveChannelCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.activeChans)
}

// IsChannelActive checks if a channel is currently joined
func (m *Manager) IsChannelActive(channel string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.activeChans[channel]
}
