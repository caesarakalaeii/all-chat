package channels

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/caesar/all-chat/services/kick-listener/metrics"
	"github.com/caesar/all-chat/services/kick-listener/publisher"
	"github.com/caesar/all-chat/services/kick-listener/status"
	"github.com/caesar/all-chat/services/kick-listener/websocket"
	"github.com/caesar/all-chat/shared/listener"
	"github.com/caesar/all-chat/shared/sourcemanager"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Compile-time assertion: Manager must satisfy the SDK ChannelManager interface.
var _ listener.ChannelManager = (*Manager)(nil)

const (
	// Kick API endpoint for channel info
	kickAPIChannelURL = "https://kick.com/api/v2/channels/%s"

	// Sync interval for checking active channels
	syncInterval = 30 * time.Second

	// PostgreSQL notification channel for source changes
	notificationChannel = "chat_source_changes"

	// Delay before retrying LISTEN connection
	listenRetryDelay = 5 * time.Second
)

// DBConnInterface allows for dependency injection
type DBConnInterface interface {
	GetPool() interface{}
}

// WebSocketClient interface for testing
type WebSocketClient interface {
	Subscribe(chatroomID int) error
	SubscribeWithAuth(chatroomID int, authToken string) error
	Unsubscribe(chatroomID int) error
	IsConnected() bool
	GetSocketID() string
}

// Manager manages Kick channel subscriptions
type Manager struct {
	repo            *Repository
	wsClient        WebSocketClient
	publisher       *publisher.StreamPublisher
	logger          *zap.Logger
	httpClient      *http.Client
	dbConn          DBConnInterface
	leader          *sourcemanager.LeadershipCoordinator
	redisClient     *redis.Client     // Redis client for migration confirmations
	podID           string            // Pod ID for migration confirmations
	statusPublisher *status.Publisher // Publishes platform status to Redis Pub/Sub

	// Coordinator integration
	assignedSourceIDs       map[string]bool                    // From coordinator
	demandedSourceIDs       map[string]listener.DemandedSource // nil = no demand filtering
	filteredAssignmentCount int                                // Number of assigned sources that have database channels
	migrationMu             sync.RWMutex                       // Protects migration state
	firstMessageChan        map[int]chan struct{}               // Per-chatroom first message signal (key: chatroom ID)

	// Track active subscriptions
	subscriptions map[string]*trackedChannel // key: channel_slug
	chatroomIndex map[int]*trackedChannel    // lookup by chatroom ID
	subsMu        sync.RWMutex

	// Control
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewManager creates a new channel manager
func NewManager(
	repo *Repository,
	wsClient WebSocketClient,
	publisher *publisher.StreamPublisher,
	dbConn DBConnInterface,
	leader *sourcemanager.LeadershipCoordinator,
	assignedSourceIDs map[string]bool,
	redisClient *redis.Client,
	podID string,
	logger *zap.Logger,
) *Manager {
	ctx, cancel := context.WithCancel(context.Background())

	return &Manager{
		repo:              repo,
		wsClient:          wsClient,
		publisher:         publisher,
		logger:            logger,
		httpClient:        &http.Client{Timeout: 10 * time.Second},
		dbConn:            dbConn,
		leader:            leader,
		assignedSourceIDs: assignedSourceIDs,
		redisClient:       redisClient,
		podID:             podID,
		firstMessageChan:  make(map[int]chan struct{}),
		subscriptions:     make(map[string]*trackedChannel),
		chatroomIndex:     make(map[int]*trackedChannel),
		ctx:               ctx,
		cancel:            cancel,
	}
}

// SetStatusPublisher injects the status publisher after construction
func (m *Manager) SetStatusPublisher(pub *status.Publisher) {
	m.subsMu.Lock()
	defer m.subsMu.Unlock()
	m.statusPublisher = pub
}

type trackedChannel struct {
	ChannelSlug string
	SourceID    string // UUID from overlay_chat_sources.id
	ChatroomID  int
	OverlayIDs  map[string]struct{}
}

type channelPlan struct {
	channel            *trackedChannel
	pendingMetadataIDs map[string]struct{}
}

// ChannelSubscription represents state for status endpoint
type ChannelSubscription struct {
	ChannelSlug string
	ChatroomID  int
	OverlayIDs  []string
}

// OverlayTarget represents an overlay consuming a chatroom
type OverlayTarget struct {
	OverlayID   string
	ChannelSlug string
}

// Start begins the channel management loop. ctx is accepted for ChannelManager interface compliance;
// internal goroutines use m.ctx created by NewManager.
func (m *Manager) Start(_ context.Context) error {
	m.logger.Info("Starting Kick channel manager")

	// Initial sync
	if err := m.syncChannels(); err != nil {
		m.logger.Error("Initial channel sync failed", zap.Error(err))
		// Don't fail startup, will retry
	}

	// Start periodic sync
	m.wg.Add(1)
	go m.syncLoop()

	// Start PostgreSQL LISTEN/NOTIFY watcher
	if m.dbConn != nil {
		m.wg.Add(1)
		go m.listenForChanges()
	} else {
		m.logger.Warn("Database connection not configured, skipping LISTEN/NOTIFY watcher")
	}

	m.logger.Info("Kick channel manager started",
		zap.Duration("sync_interval", syncInterval),
		zap.String("notification_channel", notificationChannel),
	)

	return nil
}

// Stop stops the channel manager
func (m *Manager) Stop() {
	m.logger.Info("Stopping Kick channel manager")
	m.cancel()
	m.wg.Wait()
	if m.leader != nil {
		m.leader.Stop()
	}
	m.logger.Info("Kick channel manager stopped")
}

// syncLoop periodically syncs channels from database
func (m *Manager) syncLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(syncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			if err := m.syncChannels(); err != nil {
				m.logger.Error("Failed to sync channels", zap.Error(err))
			}
		}
	}
}

// listenForChanges listens for PostgreSQL NOTIFY events to trigger immediate syncs
func (m *Manager) listenForChanges() {
	defer m.wg.Done()

	for {
		select {
		case <-m.ctx.Done():
			return
		default:
		}

		poolInterface := m.dbConn.GetPool()
		if poolInterface == nil {
			m.logger.Error("Failed to get database pool for LISTEN")
			if !m.sleepWithContext(listenRetryDelay) {
				return
			}
			continue
		}

		if err := m.listenAndWait(poolInterface); err != nil {
			m.logger.Warn("PostgreSQL LISTEN error, will retry",
				zap.Error(err),
				zap.Duration("retry_in", listenRetryDelay),
			)
			if !m.sleepWithContext(listenRetryDelay) {
				return
			}
		}
	}
}

// listenAndWait establishes LISTEN connection and waits for notifications
func (m *Manager) listenAndWait(poolInterface interface{}) error {
	pool, ok := poolInterface.(*pgxpool.Pool)
	if !ok {
		return fmt.Errorf("invalid pool type for LISTEN")
	}

	ctx := m.ctx

	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("failed to acquire connection: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, fmt.Sprintf("LISTEN %s", notificationChannel)); err != nil {
		return fmt.Errorf("failed to LISTEN on %s: %w", notificationChannel, err)
	}

	m.logger.Info("PostgreSQL LISTEN active",
		zap.String("channel", notificationChannel),
	)

	for {
		select {
		case <-m.ctx.Done():
			return nil
		default:
		}

		notification, err := conn.Conn().WaitForNotification(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil
			}
			return fmt.Errorf("notification wait failed: %w", err)
		}

		m.logger.Info("Source change notification received",
			zap.String("payload", notification.Payload),
		)

		// Only sync when the notification concerns a Kick source.
		// The chat_source_changes channel fires for all platforms; skip others to
		// avoid unnecessary work.
		if !isKickNotification(notification.Payload) {
			m.logger.Debug("Ignoring source change notification for other platform",
				zap.String("payload", notification.Payload),
			)
			continue
		}

		if err := m.syncChannels(); err != nil {
			m.logger.Error("Failed to sync after notification", zap.Error(err))
		}
	}
}

// sourceChangePayload is used to parse the platform field from PostgreSQL NOTIFY payloads.
type sourceChangePayload struct {
	Platform string `json:"platform"`
}

// isKickNotification returns true when the notification payload either cannot be
// parsed (fail-open: sync anyway) or explicitly belongs to the "kick" platform.
func isKickNotification(payload string) bool {
	var p sourceChangePayload
	if err := json.Unmarshal([]byte(payload), &p); err != nil {
		// Unparseable payload — sync to be safe.
		return true
	}
	return p.Platform == "" || p.Platform == "kick"
}

// sleepWithContext waits for the provided duration or exits if context is canceled
func (m *Manager) sleepWithContext(d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-m.ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// syncChannels synchronizes channel subscriptions with database
func (m *Manager) syncChannels() error {
	m.logger.Debug("Syncing Kick channels from database")

	// Get active channels from database
	channels, err := m.repo.GetActiveChannels(m.ctx)
	if err != nil {
		return fmt.Errorf("failed to get active channels: %w", err)
	}

	// Rebalance leadership leases before acquiring new ones.
	if m.leader != nil {
		if released, err := m.leader.Rebalance(m.ctx, len(channels)); err != nil {
			m.logger.Warn("Leadership rebalance failed", zap.Error(err))
		} else if released > 0 {
			m.logger.Info("Rebalanced: released excess channels",
				zap.Int("released", released),
				zap.Int("total_desired", len(channels)),
			)
		}
	}

	// Filter channels by demand (Phase 06: demand-based, replaces assignment-based KICK-02).
	// demandedSourceIDs is populated by the SDK demand subscriber loop via UpdateDemandedSourceIDs.
	// nil means the first demand update has not yet arrived — skip filtering so the initial
	// sync does not block startup (reconcileDemand will unsubscribe non-demanded channels once
	// demand is known). An empty (non-nil) map means zero demand: subscribe to nothing.
	m.subsMu.RLock()
	demanded := m.demandedSourceIDs
	m.subsMu.RUnlock()

	if demanded != nil {
		filtered := make([]*ActiveChannel, 0, len(demanded))
		for _, ch := range channels {
			if _, ok := demanded[ch.SourceID]; ok {
				filtered = append(filtered, ch)
			}
		}
		m.logger.Info("Filtered channels by demand",
			zap.Int("total_channels", len(channels)),
			zap.Int("demanded_channels", len(filtered)),
		)
		channels = filtered
	} else {
		m.logger.Info("Demand not yet received, syncing all channels",
			zap.Int("total_channels", len(channels)),
		)
	}

	plans := m.buildChannelPlans(channels)
	m.ensureChatroomIDs(plans)
	m.updatePendingMetadata(plans)

	desiredChannels := make(map[string]*trackedChannel, len(plans))
	for slug, plan := range plans {
		desiredChannels[slug] = plan.channel
	}

	// Count only channels that have a valid chatroom ID — these are the ones the
	// subscription loop will actually subscribe to. Channels whose chatroom ID
	// lookup failed (API error or unknown slug) are skipped in the loop below and
	// must not inflate the expected subscription count used by the readiness probe.
	validChannelCount := 0
	for _, tc := range desiredChannels {
		if tc.ChatroomID != 0 {
			validChannelCount++
		}
	}

	m.subsMu.Lock()
	defer m.subsMu.Unlock()

	// Store filtered count for readiness probe
	m.filteredAssignmentCount = validChannelCount

	// Unsubscribe from channels no longer active
	for slug, ch := range m.subscriptions {
		if _, exists := desiredChannels[slug]; !exists {
			m.logger.Info("Unsubscribing from channel", zap.String("channel", slug))
			if err := m.wsClient.Unsubscribe(ch.ChatroomID); err != nil {
				m.logger.Error("Failed to unsubscribe", zap.String("channel", slug), zap.Error(err))
			}
			delete(m.subscriptions, slug)
			delete(m.chatroomIndex, ch.ChatroomID)
			m.releaseLeadership(slug)
			metrics.ObserveSubscription("unsubscribe")

			// Don't deactivate database sources when unsubscribing
			// Sources should remain active in DB even if temporarily not subscribed
			// This allows multiple overlays to share the same channel
			m.logger.Debug("Unsubscribed from channel (sources remain active in DB)",
				zap.String("channel", slug),
			)

			// Publish offline status to overlay status indicators
			if m.statusPublisher != nil {
				m.statusPublisher.Publish(m.ctx, status.Message{
					Platform:  "kick",
					ChannelID: slug,
					Status:    "offline",
				})
			}
		}
	}

	// Subscribe to new channels
	for slug, desired := range desiredChannels {
		if desired.ChatroomID == 0 {
			m.logger.Warn("Skipping channel without chatroom ID",
				zap.String("channel", slug),
				zap.Int("overlay_consumers", len(desired.OverlayIDs)),
			)
			continue
		}

		if existing, exists := m.subscriptions[slug]; exists {
			delete(m.chatroomIndex, existing.ChatroomID)
			existing.ChatroomID = desired.ChatroomID
			existing.OverlayIDs = desired.OverlayIDs
			m.chatroomIndex[existing.ChatroomID] = existing

			// Update database status even when subscription already exists
			// This ensures database reflects actual subscription state
			if err := m.repo.SetSourceActive(m.ctx, slug, true); err != nil {
				m.logger.Error("Failed to update source status for existing subscription",
					zap.String("channel", slug),
					zap.Error(err),
				)
			}
			continue
		}

		if m.leader != nil {
			ok, err := m.leader.EnsureLeadership(m.ctx, slug, func(channel string) func() {
				// Capture context for leadership loss callback
				lossCtx := context.Background()
				return func() {
					m.handleLeadershipLoss(lossCtx, channel)
				}
			}(slug))
			if err != nil {
				m.logger.Error("Failed to claim leadership",
					zap.String("channel", slug),
					zap.Error(err),
				)
				continue
			}
			if !ok {
				m.logger.Debug("Skipping subscription; leadership owned elsewhere",
					zap.String("channel", slug),
				)
				continue
			}
		}

		m.logger.Info("Subscribing to channel",
			zap.String("channel", slug),
			zap.Int("chatroom_id", desired.ChatroomID),
			zap.Int("overlay_consumers", len(desired.OverlayIDs)),
		)

		// Get Pusher auth token for private channel subscription
		channelName := fmt.Sprintf("chatrooms.%d.v2", desired.ChatroomID)
		authToken, err := m.getKickAuthToken(slug, channelName)
		if err != nil {
			m.logger.Warn("Failed to get auth token, trying without auth",
				zap.String("channel", slug),
				zap.Error(err),
			)
			// Try subscribing without auth (fallback for public channels)
			if err := m.wsClient.Subscribe(desired.ChatroomID); err != nil {
				m.logger.Error("Failed to subscribe",
					zap.String("channel", slug),
					zap.Error(err),
				)
				m.releaseLeadership(slug)
				continue
			}
		} else {
			// Subscribe with auth token
			if err := m.wsClient.SubscribeWithAuth(desired.ChatroomID, authToken); err != nil {
				m.logger.Error("Failed to subscribe with auth",
					zap.String("channel", slug),
					zap.Error(err),
				)
				m.releaseLeadership(slug)
				continue
			}
		}

		m.subscriptions[slug] = desired
		m.chatroomIndex[desired.ChatroomID] = desired
		metrics.ObserveSubscription("subscribe")

		// Update database status to active
		if err := m.repo.SetSourceActive(m.ctx, slug, true); err != nil {
			m.logger.Error("Failed to update source status after subscribe",
				zap.String("channel", slug),
				zap.Error(err),
			)
		}

		// Publish connected status to overlay status indicators
		if m.statusPublisher != nil {
			m.statusPublisher.Publish(m.ctx, status.Message{
				Platform:  "kick",
				ChannelID: slug,
				Status:    "connected",
			})
		}
	}

	metrics.SetActiveSubscriptions(len(m.subscriptions))

	m.logger.Info("Channel sync completed",
		zap.Int("active_subscriptions", len(m.subscriptions)),
	)

	return nil
}

func (m *Manager) buildChannelPlans(channels []*ActiveChannel) map[string]*channelPlan {
	plans := make(map[string]*channelPlan)
	for _, ch := range channels {
		plan, exists := plans[ch.ChannelSlug]
		if !exists {
			plan = &channelPlan{
				channel: &trackedChannel{
					ChannelSlug: ch.ChannelSlug,
					SourceID:    ch.SourceID,
					ChatroomID:  ch.ChatroomID,
					OverlayIDs:  make(map[string]struct{}),
				},
			}
			plans[ch.ChannelSlug] = plan
		}
		plan.addOverlay(ch.OverlayID, ch.ChatroomID)
	}
	return plans
}

func (p *channelPlan) addOverlay(overlayID string, chatroomID int) {
	if p.channel.OverlayIDs == nil {
		p.channel.OverlayIDs = make(map[string]struct{})
	}
	p.channel.OverlayIDs[overlayID] = struct{}{}

	if p.channel.ChatroomID == 0 && chatroomID != 0 {
		p.channel.ChatroomID = chatroomID
	}

	if chatroomID == 0 {
		if p.pendingMetadataIDs == nil {
			p.pendingMetadataIDs = make(map[string]struct{})
		}
		p.pendingMetadataIDs[overlayID] = struct{}{}
	}
}

func (m *Manager) ensureChatroomIDs(plans map[string]*channelPlan) {
	for slug, plan := range plans {
		if plan.channel.ChatroomID != 0 {
			continue
		}

		chatroomID, err := m.fetchChatroomID(slug)
		if err != nil {
			m.logger.Error("Failed to fetch chatroom ID, marking source inactive",
				zap.String("channel", slug),
				zap.Error(err),
			)
			// Mark source as inactive - channel not found or API error
			if setErr := m.repo.SetSourceActive(m.ctx, slug, false); setErr != nil {
				m.logger.Error("Failed to mark source inactive after chatroom lookup failure",
					zap.String("channel", slug),
					zap.Error(setErr),
				)
			}
			continue
		}

		plan.channel.ChatroomID = chatroomID

		if plan.pendingMetadataIDs == nil {
			plan.pendingMetadataIDs = make(map[string]struct{})
		}
		for overlayID := range plan.channel.OverlayIDs {
			plan.pendingMetadataIDs[overlayID] = struct{}{}
		}
	}
}

func (m *Manager) updatePendingMetadata(plans map[string]*channelPlan) {
	for slug, plan := range plans {
		if plan.channel.ChatroomID == 0 || len(plan.pendingMetadataIDs) == 0 {
			continue
		}

		for overlayID := range plan.pendingMetadataIDs {
			if err := m.repo.UpdateChatroomID(m.ctx, overlayID, slug, plan.channel.ChatroomID); err != nil {
				m.logger.Warn("Failed to update chatroom ID in database",
					zap.String("overlay_id", overlayID),
					zap.String("channel", slug),
					zap.Error(err),
				)
			}
		}
	}
}

func (m *Manager) releaseLeadership(channelSlug string) {
	if m.leader == nil {
		return
	}
	m.leader.Release(channelSlug)
}

func (m *Manager) handleLeadershipLoss(ctx context.Context, channelSlug string) {
	if m.leader == nil {
		return
	}

	m.subsMu.Lock()
	defer m.subsMu.Unlock()

	ch, exists := m.subscriptions[channelSlug]
	if !exists {
		return
	}

	if err := m.wsClient.Unsubscribe(ch.ChatroomID); err != nil {
		m.logger.Error("Failed to unsubscribe after losing leadership",
			zap.String("channel", channelSlug),
			zap.Error(err),
		)
	}

	delete(m.subscriptions, channelSlug)
	delete(m.chatroomIndex, ch.ChatroomID)

	// Don't deactivate database sources when losing leadership
	// Another instance will take over subscription
	// Sources should remain active in DB

	m.logger.Warn("Dropped subscription after leadership loss (sources remain active in DB)",
		zap.String("channel", channelSlug),
	)
	metrics.ObserveSubscription("unsubscribe")
}

// fetchChatroomID fetches the chatroom ID for a Kick channel from the API
func (m *Manager) fetchChatroomID(channelSlug string) (int, error) {
	url := fmt.Sprintf(kickAPIChannelURL, channelSlug)

	req, err := http.NewRequestWithContext(m.ctx, "GET", url, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "AllChat/1.0")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch channel info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var channelInfo websocket.KickChannelInfo
	if err := json.NewDecoder(resp.Body).Decode(&channelInfo); err != nil {
		return 0, fmt.Errorf("failed to decode channel info: %w", err)
	}

	if channelInfo.Chatroom.ID == 0 {
		return 0, fmt.Errorf("chatroom ID is 0 for channel %s", channelSlug)
	}

	m.logger.Info("Fetched chatroom ID",
		zap.String("channel_slug", channelSlug),
		zap.Int("chatroom_id", channelInfo.Chatroom.ID),
	)

	return channelInfo.Chatroom.ID, nil
}

// GetSubscriptions returns current subscriptions (for status endpoint)
func (m *Manager) GetSubscriptions() map[string]*ChannelSubscription {
	m.subsMu.RLock()
	defer m.subsMu.RUnlock()

	subs := make(map[string]*ChannelSubscription, len(m.subscriptions))
	for slug, ch := range m.subscriptions {
		overlayIDs := make([]string, 0, len(ch.OverlayIDs))
		for overlayID := range ch.OverlayIDs {
			overlayIDs = append(overlayIDs, overlayID)
		}

		subs[slug] = &ChannelSubscription{
			ChannelSlug: slug,
			ChatroomID:  ch.ChatroomID,
			OverlayIDs:  overlayIDs,
		}
	}
	return subs
}

// GetOverlayTargetsForChatroom returns all overlays consuming a chatroom
func (m *Manager) GetOverlayTargetsForChatroom(chatroomID int) ([]OverlayTarget, bool) {
	m.subsMu.RLock()
	defer m.subsMu.RUnlock()

	ch, exists := m.chatroomIndex[chatroomID]
	if !exists {
		return nil, false
	}

	targets := make([]OverlayTarget, 0, len(ch.OverlayIDs))
	for overlayID := range ch.OverlayIDs {
		targets = append(targets, OverlayTarget{
			OverlayID:   overlayID,
			ChannelSlug: ch.ChannelSlug,
		})
	}

	return targets, true
}


// SignalFirstMessage should be called when a message is received for a chatroom
// This is used during migrations to confirm connectivity
func (m *Manager) SignalFirstMessage(chatroomID int) {
	m.migrationMu.RLock()
	defer m.migrationMu.RUnlock()

	if ch, exists := m.firstMessageChan[chatroomID]; exists {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

// GetAssignmentCount returns the number of assignments from coordinator (KICK-05)
func (m *Manager) GetAssignmentCount() int {
	return len(m.assignedSourceIDs)
}

// GetFilteredAssignmentCount returns the number of assigned sources that have database channels
// Used by readiness probe to check if all filtered assigned sources are subscribed
func (m *Manager) GetFilteredAssignmentCount() int {
	m.subsMu.RLock()
	defer m.subsMu.RUnlock()
	return m.filteredAssignmentCount
}

// GetActiveChannels returns the slugs of all currently subscribed channels.
func (m *Manager) GetActiveChannels() []string {
	m.subsMu.RLock()
	defer m.subsMu.RUnlock()
	channels := make([]string, 0, len(m.subscriptions))
	for slug := range m.subscriptions {
		channels = append(channels, slug)
	}
	return channels
}

// GetActiveChannelCount returns the number of currently subscribed channels.
func (m *Manager) GetActiveChannelCount() int {
	m.subsMu.RLock()
	defer m.subsMu.RUnlock()
	return len(m.subscriptions)
}

// UpdateAssignedSourceIDs updates the assigned source IDs from coordinator.
// Thread-safe update with mutex protection.
func (m *Manager) UpdateAssignedSourceIDs(newAssignedIDs map[string]bool) {
	m.migrationMu.Lock()
	defer m.migrationMu.Unlock()
	m.assignedSourceIDs = newAssignedIDs
}

// UpdateDemandedSourceIDs is called by the SDK demand subscriber loop whenever
// the set of demanded sources changes. demanded is the intersection of assigned
// sources and sources with active overlay clients. An empty map means no sources
// are demanded and this listener should disconnect all active channels.
//
// The method stores the new demanded set and triggers reconciliation: channels
// that lost demand are immediately unsubscribed; newly demanded channels are
// picked up on the next syncChannels cycle.
func (m *Manager) UpdateDemandedSourceIDs(demanded map[string]listener.DemandedSource) {
	m.subsMu.Lock()
	m.demandedSourceIDs = demanded
	m.subsMu.Unlock()

	m.reconcileDemand()
}

// reconcileDemand unsubscribes channels whose source_id is no longer in demandedSourceIDs.
// Called after UpdateDemandedSourceIDs; must not hold subsMu on entry.
func (m *Manager) reconcileDemand() {
	m.subsMu.Lock()
	defer m.subsMu.Unlock()

	demanded := m.demandedSourceIDs
	// nil demandedSourceIDs means no filtering (backward compat / feature not yet applied).
	if demanded == nil {
		return
	}

	// Build a reverse index: slug -> sourceID for active subscriptions.
	// subscriptions is keyed by slug; we need source_id to cross-reference demanded.
	// We use the chatroomIndex to find trackedChannels but slugs are the primary key.
	for slug, ch := range m.subscriptions {
		// Determine the source_id for this subscription.
		sourceID := ch.SourceID
		if sourceID == "" {
			continue
		}
		if _, ok := demanded[sourceID]; !ok {
			// This source lost demand — unsubscribe immediately.
			m.logger.Info("Demand lost, unsubscribing channel",
				zap.String("channel", slug),
				zap.String("source_id", sourceID),
			)
			if err := m.wsClient.Unsubscribe(ch.ChatroomID); err != nil {
				m.logger.Error("Failed to unsubscribe on demand loss",
					zap.String("channel", slug),
					zap.Error(err),
				)
			}
			delete(m.subscriptions, slug)
			delete(m.chatroomIndex, ch.ChatroomID)
			m.releaseLeadership(slug)

			if m.statusPublisher != nil {
				m.statusPublisher.Publish(m.ctx, status.Message{
					Platform:  "kick",
					ChannelID: slug,
					Status:    "offline",
				})
			}
		}
	}

	// Keep filteredAssignmentCount in sync with the post-demand subscription count.
	// syncChannels sets filteredAssignmentCount to the number of channels with valid
	// chatroom IDs, but when demand filtering removes subscriptions that count becomes
	// stale. The readiness probe compares subscriptionCount < filteredAssignmentCount
	// to detect "still connecting" state; if filteredAssignmentCount is never updated
	// here the probe returns 503 indefinitely whenever all channels lose demand.
	m.filteredAssignmentCount = len(m.subscriptions)
}

// GetSubscriptionCount returns the number of active subscriptions (KICK-05)
func (m *Manager) GetSubscriptionCount() int {
	m.subsMu.RLock()
	defer m.subsMu.RUnlock()
	return len(m.subscriptions)
}

// IsConnected returns true if WebSocket client is connected (KICK-05)
func (m *Manager) IsConnected() bool {
	return m.wsClient != nil && m.wsClient.IsConnected()
}

// getKickAuthToken calls Kick's /broadcasting/auth endpoint to get Pusher channel auth
func (m *Manager) getKickAuthToken(channelSlug string, channelName string) (string, error) {
	// Get socket_id from WebSocket client
	socketID := m.wsClient.GetSocketID()
	if socketID == "" {
		return "", fmt.Errorf("no socket_id available (WebSocket not connected)")
	}

	// Get OAuth access token from database
	var accessToken string
	query := `
		SELECT access_token
		FROM kick_oauth_tokens
		WHERE channel_id = $1
		  AND expiry > NOW()
		ORDER BY expiry DESC
		LIMIT 1
	`

	pool := m.dbConn.GetPool().(*pgxpool.Pool)
	err := pool.QueryRow(m.ctx, query, channelSlug).Scan(&accessToken)
	if err != nil {
		return "", fmt.Errorf("failed to get Kick OAuth token for %s: %w", channelSlug, err)
	}

	// Call Kick's /broadcasting/auth endpoint
	authURL := "https://kick.com/broadcasting/auth"

	req, err := http.NewRequest("POST", authURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create auth request: %w", err)
	}

	// Set form parameters
	q := req.URL.Query()
	q.Set("socket_id", socketID)
	q.Set("channel_name", channelName)
	req.URL.RawQuery = q.Encode()

	// Set headers
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	req.Header.Set("Accept", "application/json")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to call auth endpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", fmt.Errorf("auth endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	// Parse auth response
	var authResp struct {
		Auth string `json:"auth"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return "", fmt.Errorf("failed to parse auth response: %w", err)
	}

	m.logger.Debug("Got Pusher auth token",
		zap.String("channel", channelSlug),
		zap.String("socket_id", socketID),
	)

	return authResp.Auth, nil
}
