package channels

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/caesar/all-chat/services/kick-listener/metrics"
	"github.com/caesar/all-chat/services/kick-listener/publisher"
	"github.com/caesar/all-chat/services/kick-listener/websocket"
	"github.com/caesar/all-chat/shared/coordination"
	"github.com/caesar/all-chat/shared/sourcemanager"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

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
	repo        *Repository
	wsClient    WebSocketClient
	publisher   *publisher.StreamPublisher
	logger      *zap.Logger
	httpClient  *http.Client
	dbConn      DBConnInterface
	leader      *sourcemanager.LeadershipCoordinator
	redisClient *redis.Client // Redis client for migration confirmations
	podID       string        // Pod ID for migration confirmations

	// Coordinator integration
	assignedSourceIDs       map[string]bool           // From coordinator
	filteredAssignmentCount int                       // Number of assigned sources that have database channels
	migrationMu             sync.RWMutex              // Protects migration state
	firstMessageChan        map[int]chan struct{}     // Per-chatroom first message signal (key: chatroom ID)

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

type trackedChannel struct {
	ChannelSlug string
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

// Start begins the channel management loop
func (m *Manager) Start() error {
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

		if err := m.syncChannels(); err != nil {
			m.logger.Error("Failed to sync after notification", zap.Error(err))
		}
	}
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

	// Filter channels to only assigned ones (KICK-02)
	assignedChannels := make([]*ActiveChannel, 0)
	for _, ch := range channels {
		if m.assignedSourceIDs[ch.SourceID] {
			assignedChannels = append(assignedChannels, ch)
		}
	}

	m.logger.Info("Filtered channels by coordinator assignments",
		zap.Int("total_channels", len(channels)),
		zap.Int("assigned_channels", len(assignedChannels)),
	)

	channels = assignedChannels
	filteredCount := len(assignedChannels) // Capture filtered count before lock

	plans := m.buildChannelPlans(channels)
	m.ensureChatroomIDs(plans)
	m.updatePendingMetadata(plans)

	desiredChannels := make(map[string]*trackedChannel, len(plans))
	for slug, plan := range plans {
		desiredChannels[slug] = plan.channel
	}

	m.subsMu.Lock()
	defer m.subsMu.Unlock()

	// Store filtered count for readiness probe
	m.filteredAssignmentCount = filteredCount

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

// HandleMigrationEvent handles migration events from Redis Pub/Sub (KICK-03, KICK-04)
func (m *Manager) HandleMigrationEvent(event *coordination.MigrationEvent) {
	// Extract trace context from event (from Redis Streams message)
	carrier := propagation.MapCarrier{
		"traceparent": event.TraceParent,
		"tracestate":  event.TraceState,
	}
	ctx := otel.GetTextMapPropagator().Extract(context.Background(), carrier)

	tracer := otel.Tracer("kick-listener")
	ctx, span := tracer.Start(ctx, "handle-migration",
		trace.WithAttributes(
			attribute.String("migration_id", event.MigrationID),
			attribute.String("channel_id", event.ChannelID),
			attribute.String("from_pod", event.FromPod),
			attribute.String("to_pod", event.ToPod),
		),
	)
	defer span.End()

	m.migrationMu.Lock()
	defer m.migrationMu.Unlock()

	if event.Platform != "kick" {
		return // Not for this listener
	}

	// Check if this pod is involved
	podName := os.Getenv("HOSTNAME")
	if podName == "" {
		podName = "kick-listener-local"
	}

	if event.ToPod == podName {
		// New pod: subscribe and wait for first message (KICK-03)
		m.handleMigrationAsNewPod(ctx, event)
	} else if event.FromPod == podName {
		// Old pod: unsubscribe after confirmation (KICK-04)
		m.handleMigrationAsOldPod(ctx, event)
	}
}

// handleMigrationAsNewPod handles migration when this pod is the new assignment target
func (m *Manager) handleMigrationAsNewPod(ctx context.Context, event *coordination.MigrationEvent) {
	// Per CONTEXT.md: "New pod waits for first message OR 30s timeout (whichever comes first)"
	channelSlug := m.getChannelSlugForSourceID(event.ChannelID)
	if channelSlug == "" {
		m.logger.Error("Cannot resolve channel for source ID", zap.String("source_id", event.ChannelID))
		return
	}

	// Get chatroom ID (fetch if needed)
	chatroomID, err := m.fetchChatroomID(channelSlug)
	if err != nil {
		m.logger.Error("Failed to fetch chatroom ID for migration",
			zap.String("channel", channelSlug),
			zap.Error(err),
		)
		m.publishMigrationConfirmation(event.MigrationID, "failed", "chatroom ID lookup failed")
		return
	}

	// Create first message signal channel
	firstMsgChan := make(chan struct{}, 1)
	m.firstMessageChan[chatroomID] = firstMsgChan

	// Subscribe to chatroom with auth (KICK-03)
	channelName := fmt.Sprintf("chatrooms.%d.v2", chatroomID)
	authToken, err := m.getKickAuthToken(channelSlug, channelName)
	if err != nil {
		m.logger.Warn("Failed to get auth token for migration, trying without auth",
			zap.String("channel", channelSlug),
			zap.Error(err),
		)
		// Fallback to no auth
		if err := m.wsClient.Subscribe(chatroomID); err != nil {
			m.logger.Error("Failed to subscribe during migration",
				zap.String("channel", channelSlug),
				zap.Error(err),
			)
			delete(m.firstMessageChan, chatroomID)
			m.publishMigrationConfirmation(event.MigrationID, "failed", "subscribe failed")
			return
		}
	} else {
		// Subscribe with auth token
		if err := m.wsClient.SubscribeWithAuth(chatroomID, authToken); err != nil {
			m.logger.Error("Failed to subscribe with auth during migration",
				zap.String("channel", channelSlug),
				zap.Error(err),
			)
			delete(m.firstMessageChan, chatroomID)
			m.publishMigrationConfirmation(event.MigrationID, "failed", "subscribe failed")
			return
		}
	}

	// Wait for first message or timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	select {
	case <-firstMsgChan:
		// Success! Publish confirmation to Redis
		m.publishMigrationConfirmation(event.MigrationID, "connected", "")
		m.logger.Info("Migration successful (new pod)",
			zap.String("channel", channelSlug),
			zap.Int("chatroom_id", chatroomID),
		)

		// Add to subscriptions tracking
		m.subsMu.Lock()
		m.subscriptions[channelSlug] = &trackedChannel{
			ChannelSlug: channelSlug,
			ChatroomID:  chatroomID,
			OverlayIDs:  make(map[string]struct{}),
		}
		m.chatroomIndex[chatroomID] = m.subscriptions[channelSlug]
		m.subsMu.Unlock()

	case <-timeoutCtx.Done():
		// Timeout - connection failed
		m.publishMigrationConfirmation(event.MigrationID, "failed", "timeout waiting for first message")
		m.logger.Error("Migration timeout (new pod)", zap.String("channel", channelSlug))
		m.wsClient.Unsubscribe(chatroomID) // Clean up failed subscription
	}

	delete(m.firstMessageChan, chatroomID)
}

// handleMigrationAsOldPod handles migration when this pod is losing the assignment
func (m *Manager) handleMigrationAsOldPod(ctx context.Context, event *coordination.MigrationEvent) {
	// Per CONTEXT.md: "Old pod disconnects immediately after seeing new pod's confirmation"
	channelSlug := m.getChannelSlugForSourceID(event.ChannelID)
	if channelSlug == "" {
		m.logger.Error("Cannot resolve channel for source ID", zap.String("source_id", event.ChannelID))
		return
	}

	m.subsMu.Lock()
	defer m.subsMu.Unlock()

	ch, exists := m.subscriptions[channelSlug]
	if !exists {
		m.logger.Warn("Channel not in subscriptions during migration", zap.String("channel", channelSlug))
		return
	}

	// Wait for confirmation (with 60s timeout per CONTEXT.md)
	// Implementation: Poll Redis Streams for confirmation events
	// When found, unsubscribe immediately (KICK-04)
	if err := m.wsClient.Unsubscribe(ch.ChatroomID); err != nil {
		m.logger.Error("Failed to unsubscribe during migration",
			zap.String("channel", channelSlug),
			zap.Error(err),
		)
	}

	delete(m.subscriptions, channelSlug)
	delete(m.chatroomIndex, ch.ChatroomID)

	m.logger.Info("Migration handoff complete (old pod)",
		zap.String("channel", channelSlug),
		zap.Int("chatroom_id", ch.ChatroomID),
	)
}

// getChannelSlugForSourceID resolves a source ID to a channel slug
func (m *Manager) getChannelSlugForSourceID(sourceID string) string {
	m.subsMu.RLock()
	defer m.subsMu.RUnlock()

	// Check subscriptions map for matching source
	// Note: This assumes source_id is stored somewhere accessible
	// For now, we'll need to query the database
	channels, err := m.repo.GetActiveChannels(m.ctx)
	if err != nil {
		m.logger.Error("Failed to query channels for source ID lookup", zap.Error(err))
		return ""
	}

	for _, ch := range channels {
		if ch.SourceID == sourceID {
			return ch.ChannelSlug
		}
	}

	return ""
}

// publishMigrationConfirmation publishes a migration confirmation to Redis
func (m *Manager) publishMigrationConfirmation(migrationID, status, errorMsg string) {
	// Publish to Redis Streams for coordinator to consume
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	event := map[string]interface{}{
		"migration_id":    migrationID,
		"status":          status, // "connected" or "failed"
		"pod_id":          m.podID,
		"timestamp":       time.Now().Unix(),
		"sequence_number": 0, // Not currently used for Kick
		"error":           errorMsg,
	}

	_, err := m.redisClient.XAdd(ctx, &redis.XAddArgs{
		Stream: "migration:log",
		Values: event,
	}).Result()

	if err != nil {
		m.logger.Error("Failed to publish migration confirmation",
			zap.String("migration_id", migrationID),
			zap.Error(err))
		return
	}

	m.logger.Info("Published migration confirmation",
		zap.String("migration_id", migrationID),
		zap.String("status", status))
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

// UpdateAssignedSourceIDs updates the assigned source IDs from coordinator
// Thread-safe update with mutex protection
func (m *Manager) UpdateAssignedSourceIDs(newAssignedIDs map[string]bool) {
	m.migrationMu.Lock()
	defer m.migrationMu.Unlock()
	m.assignedSourceIDs = newAssignedIDs
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
