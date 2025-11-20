package channels

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/caesar/all-chat/shared/metrics"
	"github.com/caesar/all-chat/shared/sourcemanager"
	"github.com/jackc/pgx/v5/pgxpool"
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
	metrics     *metrics.ListenerMetrics
	rateLimiter *rate.Limiter
	activeChans map[string]bool // Currently joined channels
	mu          sync.RWMutex
	syncTicker  *time.Ticker
	stopChan    chan struct{}
	wg          sync.WaitGroup
	dbConn      DBConnInterface // For PostgreSQL LISTEN

	leader *sourcemanager.LeadershipCoordinator
}

// DBConnInterface allows getting a raw pgxpool.Pool for LISTEN
type DBConnInterface interface {
	GetPool() interface{}
}

// NewManager creates a new channel manager
func NewManager(repo RepositoryInterface, joinParter JoinParterInterface, dbConn DBConnInterface, leader *sourcemanager.LeadershipCoordinator, logger *zap.Logger, m *metrics.ListenerMetrics) *Manager {
	return &Manager{
		repo:        repo,
		joinParter:  joinParter,
		dbConn:      dbConn,
		leader:      leader,
		logger:      logger,
		metrics:     m,
		rateLimiter: rate.NewLimiter(rate.Every(JoinRatePer/JoinRateLimit), JoinRateLimit),
		activeChans: make(map[string]bool),
		syncTicker:  time.NewTicker(SyncInterval),
		stopChan:    make(chan struct{}),
	}
}

// Start begins the periodic sync process and PostgreSQL LISTEN
func (m *Manager) Start(ctx context.Context) error {
	// Initial sync
	if err := m.SyncChannels(ctx); err != nil {
		return err
	}

	// Start periodic sync (fallback)
	m.wg.Add(1)
	go m.syncLoop(ctx)

	// Start PostgreSQL LISTEN for instant notifications
	if m.dbConn != nil {
		m.wg.Add(1)
		go m.listenForChanges(ctx)
	} else {
		m.logger.Warn("Database connection not configured, skipping LISTEN/NOTIFY watcher")
	}

	m.logger.Info("Channel manager started",
		zap.Duration("sync_interval", SyncInterval),
		zap.String("notification_channel", "chat_source_changes"),
	)

	return nil
}

// Stop gracefully stops the channel manager
func (m *Manager) Stop() {
	close(m.stopChan)
	m.syncTicker.Stop()
	m.wg.Wait()

	if m.leader != nil {
		m.leader.Stop()
	}

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

// listenForChanges listens for PostgreSQL NOTIFY events for instant source updates
func (m *Manager) listenForChanges(ctx context.Context) {
	defer m.wg.Done()

	for {
		select {
		case <-m.stopChan:
			return
		case <-ctx.Done():
			return
		default:
			// Get connection from pool for LISTEN
			pool := m.dbConn.GetPool()
			if pool == nil {
				m.logger.Error("Failed to get database pool for LISTEN")
				time.Sleep(5 * time.Second)
				continue
			}

			// Acquire connection and LISTEN
			if err := m.listenAndWait(ctx, pool); err != nil {
				m.logger.Warn("PostgreSQL LISTEN error, will retry",
					zap.Error(err),
					zap.Duration("retry_in", 5*time.Second),
				)
				time.Sleep(5 * time.Second)
			}
		}
	}
}

// listenAndWait establishes LISTEN connection and waits for notifications
func (m *Manager) listenAndWait(ctx context.Context, poolInterface interface{}) error {
	pool, ok := poolInterface.(*pgxpool.Pool)
	if !ok {
		return fmt.Errorf("invalid pool type")
	}

	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("failed to acquire connection: %w", err)
	}
	defer conn.Release()

	// Start listening for notifications
	_, err = conn.Exec(ctx, "LISTEN chat_source_changes")
	if err != nil {
		return fmt.Errorf("failed to LISTEN: %w", err)
	}

	m.logger.Info("PostgreSQL LISTEN active",
		zap.String("channel", "chat_source_changes"),
	)

	// Wait for notifications
	for {
		select {
		case <-m.stopChan:
			return nil
		case <-ctx.Done():
			return nil
		default:
			// Wait for notification with timeout
			notification, err := conn.Conn().WaitForNotification(ctx)
			if err != nil {
				return fmt.Errorf("notification wait failed: %w", err)
			}

			m.logger.Info("Source change notification received",
				zap.String("payload", notification.Payload),
			)

			// Trigger immediate sync
			if err := m.SyncChannels(ctx); err != nil {
				m.logger.Error("Failed to sync after notification", zap.Error(err))
			}
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
		m.partChannelLocked(ch, true)
	}

	// JOIN new channels with rate limiting
	for _, ch := range toJoin {
		if m.leader != nil {
			ok, err := m.leader.EnsureLeadership(ctx, ch, func(channel string) func() {
				return func() {
					m.handleLeadershipLoss(channel)
				}
			}(ch))
			if err != nil {
				m.logger.Error("Failed to claim leadership",
					zap.String("channel", ch),
					zap.Error(err),
				)
				continue
			}
			if !ok {
				m.logger.Debug("Skipping channel because another instance is leader",
					zap.String("channel", ch),
				)
				continue
			}
		}

		// Wait for rate limiter
		if err := m.rateLimiter.Wait(ctx); err != nil {
			m.logger.Warn("Rate limiter wait interrupted", zap.Error(err))
			break
		}
		m.joinChannel(ch)
	}

	// Record active sources and source events
	m.metrics.SetActiveSources("twitch", "twitch-listener", len(m.activeChans))
	for range toJoin {
		m.metrics.RecordSourceEvent("twitch", "twitch-listener", "added")
	}
	for range toPart {
		m.metrics.RecordSourceEvent("twitch", "twitch-listener", "removed")
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

// partChannelLocked parts a channel and removes from tracking. Caller must hold m.mu.
func (m *Manager) partChannelLocked(channel string, releaseLeadership bool) {
	m.joinParter.Depart(channel)
	delete(m.activeChans, channel)

	if releaseLeadership && m.leader != nil {
		m.leader.Release(channel)
	}

	m.logger.Info("Parted channel",
		zap.String("channel", channel),
	)
}

func (m *Manager) handleLeadershipLoss(channel string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.activeChans[channel] {
		return
	}

	m.joinParter.Depart(channel)
	delete(m.activeChans, channel)

	m.logger.Warn("Parted channel after losing leadership",
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
