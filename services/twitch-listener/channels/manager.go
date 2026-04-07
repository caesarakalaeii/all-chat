package channels

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/caesar/all-chat/services/twitch-listener/status"
	"github.com/caesar/all-chat/shared/listener"
	"github.com/caesar/all-chat/shared/metrics"
	"github.com/caesar/all-chat/shared/sourcemanager"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

// Compile-time assertion: Manager must satisfy the SDK ChannelManager interface.
var _ listener.ChannelManager = (*Manager)(nil)

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
	repo             RepositoryInterface
	joinParter       JoinParterInterface
	logger           *zap.Logger
	metrics          *metrics.ListenerMetrics
	rateLimiter      *rate.Limiter
	activeChans      map[string]bool              // Currently joined channels
	mu               sync.RWMutex
	syncTicker       *time.Ticker
	stopChan         chan struct{}
	wg               sync.WaitGroup
	dbConn           DBConnInterface              // For PostgreSQL LISTEN
	leader                *sourcemanager.LeadershipCoordinator
	assignedSourceIDs     map[string]bool             // From coordinator
	filteredAssignmentCount int                         // Number of assigned sources that have database channels
	ircClients            []JoinParterInterface        // Multiple IRC connections for >100 channels
	migrationMu      sync.RWMutex                 // Protects migration state
	firstMessageChan map[string]chan struct{}     // Per-channel first message signal
	redisClient      *redis.Client                // Redis client for migration confirmations
	podID            string                       // Pod ID for migration confirmations
	statusPublisher  *status.Publisher            // Publishes platform status to Redis Pub/Sub
	initialSyncDone  bool                         // Set to true after the first SyncChannels completes
}

// DBConnInterface allows getting a raw pgxpool.Pool for LISTEN
type DBConnInterface interface {
	GetPool() interface{}
}

// NewManager creates a new channel manager
func NewManager(repo RepositoryInterface, joinParter JoinParterInterface, dbConn DBConnInterface, leader *sourcemanager.LeadershipCoordinator, assignedSourceIDs map[string]bool, redisClient *redis.Client, podID string, logger *zap.Logger, m *metrics.ListenerMetrics) *Manager {
	return &Manager{
		repo:              repo,
		joinParter:        joinParter,
		dbConn:            dbConn,
		leader:            leader,
		assignedSourceIDs: assignedSourceIDs,
		redisClient:       redisClient,
		podID:             podID,
		logger:            logger,
		metrics:           m,
		rateLimiter:       rate.NewLimiter(rate.Every(JoinRatePer/JoinRateLimit), JoinRateLimit),
		activeChans:       make(map[string]bool),
		ircClients:        make([]JoinParterInterface, 0),
		firstMessageChan:  make(map[string]chan struct{}),
		syncTicker:        time.NewTicker(SyncInterval),
		stopChan:          make(chan struct{}),
	}
}

// SetStatusPublisher injects the status publisher after construction
func (m *Manager) SetStatusPublisher(pub *status.Publisher) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.statusPublisher = pub
}

// GetFirstMessageChan returns the first message channel map for migration coordination
func (m *Manager) GetFirstMessageChan() map[string]chan struct{} {
	m.migrationMu.RLock()
	defer m.migrationMu.RUnlock()
	return m.firstMessageChan
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

// ClearActiveChannels resets the active channel map.
// Called when the IRC connection is lost so the next sync cycle re-joins all channels
// instead of believing they are still connected.
func (m *Manager) ClearActiveChannels() {
	m.mu.Lock()
	count := len(m.activeChans)
	m.activeChans = make(map[string]bool)
	m.mu.Unlock()
	m.logger.Warn("Cleared active channels due to IRC disconnect",
		zap.Int("cleared_count", count),
	)
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

// SyncChannels queries the database and updates joined channels.
//
// The mutex (m.mu) is held ONLY for brief, non-blocking snapshots of shared state.
// All time-consuming operations — IRC rate-limited joins, leadership election, PART
// notifications — happen WITHOUT the lock so that concurrent readiness-probe HTTP
// handlers are never blocked waiting for a slow write-lock acquisition.
func (m *Manager) SyncChannels(ctx context.Context) error {
	// Get unique channels from database
	desiredChannels, err := m.repo.GetUniqueChannels(ctx)
	if err != nil {
		return err
	}

	// Rebalance leadership leases before acquiring new ones.
	// This ensures that when pods scale up/down, excess leases are shed so
	// new pods can claim their fair share on the next sync cycle.
	// Released channels are removed from desiredChannels so the PART logic
	// disconnects them from IRC.
	var rebalancedOut map[string]bool
	if m.leader != nil {
		if released, err := m.leader.Rebalance(ctx, len(desiredChannels)); err != nil {
			m.logger.Warn("Leadership rebalance failed", zap.Error(err))
		} else if len(released) > 0 {
			rebalancedOut = make(map[string]bool, len(released))
			for _, id := range released {
				rebalancedOut[id] = true
			}
			m.logger.Info("Rebalanced: released excess channels",
				zap.Int("released", len(released)),
				zap.Int("total_desired", len(desiredChannels)),
			)
		}
	}

	// Filter channels by coordinator assignments (TWITCH-02)
	// Always filter when coordinator integration is enabled (assignedSourceIDs != nil)
	// Even if empty map (0 assignments), should connect to 0 channels
	filteredCount := len(desiredChannels) // Default: all channels
	if m.assignedSourceIDs != nil {
		// Get UUID-to-channel-name mapping from database
		sourceIDMap := m.repo.GetSourceIDsForChannels(ctx, desiredChannels)

		// Build reverse map: UUID -> channel_name for filtering
		uuidToChannelMap := make(map[string]string)
		for channelName, uuid := range sourceIDMap {
			uuidToChannelMap[uuid] = channelName
		}

		// Filter to only assigned channels
		// Coordinator uses composite keys for Twitch (e.g. "uuid:twitch") — strip platform suffix
		filteredChannels := make([]string, 0, len(m.assignedSourceIDs))
		for compositeID := range m.assignedSourceIDs {
			bareID := compositeID
			if colonIdx := strings.LastIndexByte(compositeID, ':'); colonIdx != -1 {
				bareID = compositeID[:colonIdx]
			}
			if channelName, ok := uuidToChannelMap[bareID]; ok {
				filteredChannels = append(filteredChannels, channelName)
			}
		}

		// CRITICAL: Verify 100% coverage before filtering
		coverageComplete := m.verifyCoverageComplete(ctx, sourceIDMap)

		if !coverageComplete {
			// SAFETY: Coverage gaps detected, disable filtering
			m.logger.Error("Coverage verification FAILED - filtering disabled for safety",
				zap.Int("total_channels", len(desiredChannels)),
				zap.Int("assigned_channels", len(filteredChannels)),
				zap.Int("missing", len(desiredChannels)-len(filteredChannels)),
			)
			filteredCount = len(desiredChannels) // Use all channels
		} else {
			// Coverage verified - safe to filter
			m.logger.Info("Coverage verified - filtering enabled",
				zap.Int("total_sources", len(sourceIDMap)),
				zap.Int("assigned_channels", len(filteredChannels)),
			)
			desiredChannels = filteredChannels
			filteredCount = len(filteredChannels)
		}
	}

	// --- Phase 1: Brief lock to snapshot shared state ---
	// Compute toJoin/toPart from activeChans and update filteredAssignmentCount.
	// The lock is released before any slow IRC or leadership-election operations.
	var toJoin, toPart []string
	func() {
		m.mu.Lock()
		defer m.mu.Unlock()

		// Store filtered count for readiness probe
		m.filteredAssignmentCount = filteredCount

		// Convert to map for easier lookup, excluding channels shed by rebalancing
		desiredMap := make(map[string]bool)
		for _, ch := range desiredChannels {
			if rebalancedOut != nil && rebalancedOut[ch] {
				continue // Leadership was released during rebalance; let another pod take it
			}
			desiredMap[ch] = true
		}

		// Find channels to JOIN (in desired but not active)
		for ch := range desiredMap {
			if !m.activeChans[ch] {
				toJoin = append(toJoin, ch)
			}
		}

		// Find channels to PART (in active but not desired)
		for ch := range m.activeChans {
			if !desiredMap[ch] {
				toPart = append(toPart, ch)
			}
		}
	}()

	// --- Phase 2: IRC operations — no lock held ---
	// PART channels first (no rate limit); each call acquires a brief lock internally.
	for _, ch := range toPart {
		m.partChannel(ctx, ch, true)
	}

	// JOIN new channels with rate limiting.
	// Use multiple IRC connections if >=100 channels (TWITCH-03).
	var joined []string
	if len(toJoin) >= 100 {
		joined = m.joinChannelsMultipleConnectionsUnlocked(ctx, toJoin)
	} else {
		// Single connection JOIN with rate limiting (existing logic)
		for _, ch := range toJoin {
			if m.leader != nil {
				ok, err := m.leader.EnsureLeadership(ctx, ch, func(channel string) func() {
					// Capture context for leadership loss callback
					lossCtx := context.Background()
					return func() {
						m.handleLeadershipLoss(lossCtx, channel)
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
			m.joinChannel(ctx, ch)
			joined = append(joined, ch)
		}
	}

	// --- Phase 3: Brief lock to snapshot active channels for DB updates ---
	var activeSnapshot []string
	var activeCount int
	func() {
		m.mu.RLock()
		defer m.mu.RUnlock()
		activeCount = len(m.activeChans)
		activeSnapshot = make([]string, 0, activeCount)
		for ch := range m.activeChans {
			activeSnapshot = append(activeSnapshot, ch)
		}
	}()

	// --- Phase 4: DB status updates and metrics — no lock needed ---
	// Update database status for all active channels (including already-connected ones)
	// This ensures the database reflects actual IRC connection state
	statusUpdates := 0
	for _, ch := range activeSnapshot {
		if err := m.repo.SetSourceActive(ctx, ch, true); err != nil {
			m.logger.Error("Failed to update source status during sync",
				zap.String("channel", ch),
				zap.Error(err),
			)
		} else {
			statusUpdates++
		}
	}

	// Record active sources and source events
	if m.metrics != nil {
		m.metrics.SetActiveSources("twitch", "twitch-listener", activeCount)
		for range joined {
			m.metrics.RecordSourceEvent("twitch", "twitch-listener", "added")
		}
		for range toPart {
			m.metrics.RecordSourceEvent("twitch", "twitch-listener", "removed")
		}
	}

	m.logger.Info("Channel sync completed",
		zap.Int("total_active", activeCount),
		zap.Int("joined", len(joined)),
		zap.Int("parted", len(toPart)),
		zap.Int("status_updates", statusUpdates),
	)

	// Mark initial sync as done after first successful completion.
	// When leadership is enabled the pod may own 0 channels (all locks held by peer),
	// so the readiness probe uses this flag instead of requiring activeChannelCount > 0.
	m.mu.Lock()
	m.initialSyncDone = true
	m.mu.Unlock()

	return nil
}

// verifyCoverageComplete checks if all database sources have coordinator assignments
// Returns false if any source lacks assignment (prevents message loss)
// Queries Redis for global assignment coverage across all pods
func (m *Manager) verifyCoverageComplete(ctx context.Context, sourceIDMap map[string]string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Query all assignments from Redis for global coverage check
	globalAssignedIDs := make(map[string]bool)
	iter := m.redisClient.Scan(ctx, 0, "shard:assignment:*", 0).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()
		// Extract source ID from key (may be composite like "uuid:platform")
		sourceID := key[len("shard:assignment:"):]
		// Strip platform suffix if present (e.g., "abc123:twitch" → "abc123")
		if colonIdx := strings.LastIndexByte(sourceID, ':'); colonIdx != -1 {
			sourceID = sourceID[:colonIdx]
		}
		globalAssignedIDs[sourceID] = true
	}
	if err := iter.Err(); err != nil {
		m.logger.Warn("Failed to scan Redis assignments for coverage check, using local assignments",
			zap.Error(err),
		)
		// Fallback to local assignments on error
		globalAssignedIDs = m.assignedSourceIDs
	}

	unassignedSources := make([]string, 0)
	unassignedUUIDs := make([]string, 0)

	for channelName, uuid := range sourceIDMap {
		if !globalAssignedIDs[uuid] {
			unassignedSources = append(unassignedSources, channelName)
			unassignedUUIDs = append(unassignedUUIDs, uuid)
		}
	}

	if len(unassignedSources) > 0 {
		// Helper function for min
		minInt := func(a, b int) int {
			if a < b {
				return a
			}
			return b
		}

		m.logger.Error("Coverage verification failed - unassigned sources detected",
			zap.Int("unassigned_count", len(unassignedSources)),
			zap.Strings("sample_channels", unassignedSources[:minInt(5, len(unassignedSources))]),
			zap.Strings("sample_uuids", unassignedUUIDs[:minInt(5, len(unassignedUUIDs))]),
		)

		// Emit metric for alerting
		m.metrics.RecordSourceEvent("twitch", "twitch-listener", "coverage_gap_detected")

		return false
	}

	m.logger.Debug("Coverage verification passed",
		zap.Int("total_sources", len(sourceIDMap)),
		zap.String("pod_name", m.podID),
	)

	return true
}

// joinChannel joins a channel and tracks it.
// It departs first to clear any stale state in the go-twitch-irc library's
// internal channels map. The library skips Join() calls for channels already
// in its map, but after IRC reconnects or race conditions the map entry can
// exist while the server-side JOIN was never acknowledged. Departing first
// removes the map entry so the subsequent Join() always sends a real IRC JOIN.
//
// This method acquires m.mu briefly to update activeChans, so it must NOT be
// called while m.mu is already held.
func (m *Manager) joinChannel(ctx context.Context, channel string) {
	m.joinParter.Depart(channel)
	m.joinParter.Join(channel)

	m.mu.Lock()
	m.activeChans[channel] = true
	m.mu.Unlock()

	// Update database status
	if err := m.repo.SetSourceActive(ctx, channel, true); err != nil {
		m.logger.Error("Failed to update source status after join",
			zap.String("channel", channel),
			zap.Error(err),
		)
	}

	// Publish connected status to overlay status indicators.
	// Use lowercase channel name because the API gateway looks up status by
	// overlay_chat_sources.channel_id which is stored in lowercase, while
	// GetUniqueChannels returns channel_name which may be mixed-case.
	if m.statusPublisher != nil {
		m.statusPublisher.Publish(ctx, status.Message{
			Platform:  "twitch",
			ChannelID: strings.ToLower(channel),
			Status:    "connected",
		})
	}

	m.logger.Info("Joined channel",
		zap.String("channel", channel),
	)
}

// partChannel parts a channel and removes it from tracking. It acquires m.mu
// internally so callers must NOT hold m.mu when calling this method.
func (m *Manager) partChannel(ctx context.Context, channel string, releaseLeadership bool) {
	m.joinParter.Depart(channel)

	m.mu.Lock()
	delete(m.activeChans, channel)
	m.mu.Unlock()

	if releaseLeadership && m.leader != nil {
		m.leader.Release(channel)
	}

	// Don't deactivate database sources when parting.
	// Sources should remain active in DB even if temporarily not connected —
	// this allows multiple overlays to share the same channel.

	// Publish offline status to overlay status indicators (lowercase to match channel_id in DB)
	if m.statusPublisher != nil {
		m.statusPublisher.Publish(ctx, status.Message{
			Platform:  "twitch",
			ChannelID: strings.ToLower(channel),
			Status:    "offline",
		})
	}

	m.logger.Info("Parted channel (sources remain active in DB)",
		zap.String("channel", channel),
	)
}

func (m *Manager) handleLeadershipLoss(ctx context.Context, channel string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.activeChans[channel] {
		return
	}

	m.joinParter.Depart(channel)
	delete(m.activeChans, channel)

	// Don't deactivate database sources when losing leadership
	// Another instance will take over polling
	// Sources should remain active in DB

	m.logger.Warn("Parted channel after losing leadership (sources remain active in DB)",
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

// IsCoordinationEnabled reports whether leadership coordination is active.
// Returns false when SOURCE_MANAGER_SECRET was not set and assignedSourceIDs was
// passed as nil — in that case assignment-based readiness checks are meaningless.
func (m *Manager) IsCoordinationEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.assignedSourceIDs != nil
}

// GetAssignmentCount returns the number of assigned sources
func (m *Manager) GetAssignmentCount() int {
	return len(m.assignedSourceIDs)
}

// GetFilteredAssignmentCount returns the number of assigned sources that have database channels
// Used by readiness probe to check if all filtered assigned channels are active
func (m *Manager) GetFilteredAssignmentCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.filteredAssignmentCount
}

// IsInitialSyncComplete returns true after the first SyncChannels has completed successfully.
// When leadership is enabled a pod may legitimately own 0 channels (all Redis locks held by
// peer), so the readiness probe uses this flag rather than requiring activeChannelCount > 0.
func (m *Manager) IsInitialSyncComplete() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.initialSyncDone
}

// IsLeadershipEnabled returns true when the manager is configured with a
// LeadershipCoordinator.  In this mode multiple pods split channels between
// them, so a pod that owns 0 channels is still healthy (the peer owns them
// all).  The readiness probe must not gate on activeChannelCount in this case.
func (m *Manager) IsLeadershipEnabled() bool {
	return m.leader != nil
}

// UpdateAssignedSourceIDs updates the assigned source IDs from coordinator.
// Thread-safe update with mutex protection.
func (m *Manager) UpdateAssignedSourceIDs(newAssignedIDs map[string]bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.assignedSourceIDs = newAssignedIDs
}

// UpdateDemandedSourceIDs is a no-op for the Twitch IRC listener.
// Twitch IRC is excluded from demand-driven behavior — it always connects to all
// assigned channels because IRC is a push protocol with no per-source connection cost.
// The SDK's DisableDemandFiltering=true config prevents this method from being called
// by the demand loop, but the interface must be satisfied.
func (m *Manager) UpdateDemandedSourceIDs(_ map[string]listener.DemandedSource) {
	// No-op: Twitch IRC always connected to all assigned channels.
}

// joinChannelsMultipleConnectionsUnlocked joins >100 channels without holding m.mu.
// Per RESEARCH.md: Distribute channels evenly across connections (90 channels per
// connection, safe margin below 100). Leadership election is performed per channel
// first so that IRC connections are only created for the channels this pod actually
// owns — preventing OOM on startup when the full channel list is large but this pod
// only handles a fraction.
//
// Returns the list of channels successfully joined so that the caller can update
// activeChans under its own lock.
func (m *Manager) joinChannelsMultipleConnectionsUnlocked(ctx context.Context, channels []string) []string {
	// Filter channels through leadership first to determine how many this pod owns.
	var wonChannels []string
	for _, ch := range channels {
		if m.leader != nil {
			ok, err := m.leader.EnsureLeadership(ctx, ch, func(channel string) func() {
				lossCtx := context.Background()
				return func() {
					m.handleLeadershipLoss(lossCtx, channel)
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
		wonChannels = append(wonChannels, ch)
	}

	if len(wonChannels) == 0 {
		m.logger.Info("No channels won via leadership, skipping IRC connections")
		return nil
	}

	// Create IRC connections sized to the channels this pod actually owns.
	clientCount := (len(wonChannels) / 90) + 1

	m.logger.Info("Creating multiple IRC connections",
		zap.Int("total_candidates", len(channels)),
		zap.Int("won_channels", len(wonChannels)),
		zap.Int("client_count", clientCount),
	)

	var joined []string
	for i := 0; i < clientCount; i++ {
		start := i * 90
		end := start + 90
		if end > len(wonChannels) {
			end = len(wonChannels)
		}

		for _, ch := range wonChannels[start:end] {
			if err := m.rateLimiter.Wait(ctx); err != nil {
				m.logger.Warn("Rate limiter wait interrupted", zap.Error(err))
				break
			}
			m.joinParter.Depart(ch) // Clear stale library state
			m.joinParter.Join(ch)

			m.mu.Lock()
			m.activeChans[ch] = true
			m.mu.Unlock()

			joined = append(joined, ch)
		}
	}

	return joined
}

