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
	"github.com/caesar/all-chat/services/kick-listener/websocket"
	"github.com/caesar/all-chat/shared/sourcemanager"
	"github.com/jackc/pgx/v5/pgxpool"
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
	Unsubscribe(chatroomID int) error
	IsConnected() bool
}

// Manager manages Kick channel subscriptions
type Manager struct {
	repo       *Repository
	wsClient   WebSocketClient
	publisher  *publisher.StreamPublisher
	logger     *zap.Logger
	httpClient *http.Client
	dbConn     DBConnInterface
	leader     *sourcemanager.LeadershipCoordinator

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
	logger *zap.Logger,
) *Manager {
	ctx, cancel := context.WithCancel(context.Background())

	return &Manager{
		repo:          repo,
		wsClient:      wsClient,
		publisher:     publisher,
		logger:        logger,
		httpClient:    &http.Client{Timeout: 10 * time.Second},
		dbConn:        dbConn,
		leader:        leader,
		subscriptions: make(map[string]*trackedChannel),
		chatroomIndex: make(map[int]*trackedChannel),
		ctx:           ctx,
		cancel:        cancel,
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

	plans := m.buildChannelPlans(channels)
	m.ensureChatroomIDs(plans)
	m.updatePendingMetadata(plans)

	desiredChannels := make(map[string]*trackedChannel, len(plans))
	for slug, plan := range plans {
		desiredChannels[slug] = plan.channel
	}

	m.subsMu.Lock()
	defer m.subsMu.Unlock()

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
			continue
		}

		if m.leader != nil {
			ok, err := m.leader.EnsureLeadership(m.ctx, slug, func(channel string) func() {
				return func() {
					m.handleLeadershipLoss(channel)
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

		if err := m.wsClient.Subscribe(desired.ChatroomID); err != nil {
			m.logger.Error("Failed to subscribe",
				zap.String("channel", slug),
				zap.Error(err),
			)
			m.releaseLeadership(slug)
			continue
		}

		m.subscriptions[slug] = desired
		m.chatroomIndex[desired.ChatroomID] = desired
		metrics.ObserveSubscription("subscribe")
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
			m.logger.Error("Failed to fetch chatroom ID",
				zap.String("channel", slug),
				zap.Error(err),
			)
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

func (m *Manager) handleLeadershipLoss(channelSlug string) {
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

	m.logger.Warn("Dropped subscription after leadership loss",
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
