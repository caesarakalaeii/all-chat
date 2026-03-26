package relay

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	syncInterval = 30 * time.Second

	// batchFlushInterval is the maximum time to hold messages before flushing.
	// Discord webhooks allow ~30 requests/60s — a 2s window gives 30 flushes/min
	// which stays right at the limit even under sustained high traffic.
	batchFlushInterval = 2 * time.Second

	// maxBatchContentLen caps the batched content to stay under Discord's 2000 char limit.
	maxBatchContentLen = 1800

	// batchWebhookUsername is the generic username used for batched multi-user messages.
	batchWebhookUsername = "Chat Relay"
)

// relayMessage is the local representation of a normalized chat message from
// Redis Pub/Sub. It is intentionally NOT imported from message-processor to
// keep services decoupled.
type relayMessage struct {
	Platform  string `json:"platform"`
	OverlayID string `json:"overlay_id"`
	User      struct {
		Username    string `json:"username"`
		DisplayName string `json:"display_name"`
		AvatarURL   string `json:"avatar_url"`
	} `json:"user"`
	Message struct {
		Text string `json:"text"`
	} `json:"message"`
}

// Manager subscribes to Redis Pub/Sub overlay channels and relays non-Discord
// messages to Discord via DiscordPoster.
type Manager struct {
	repo        RepositoryInterface
	poster      DiscordPoster
	provisioner *WebhookProvisioner
	redisClient *redis.Client
	logger      *zap.Logger
	dbPool      *pgxpool.Pool

	activeSubs map[string]*redis.PubSub // key: overlay_id
	activeConf map[string]string        // key: overlay_id → webhook_url
	mu         sync.Mutex

	syncTicker *time.Ticker
	stopChan   chan struct{}
	wg         sync.WaitGroup
}

// NewManager constructs a relay Manager. The provisioner parameter is optional;
// pass nil to disable automatic webhook provisioning (useful for tests).
func NewManager(
	repo RepositoryInterface,
	poster DiscordPoster,
	rdb *redis.Client,
	dbPool *pgxpool.Pool,
	logger *zap.Logger,
	provisioner *WebhookProvisioner,
) *Manager {
	return &Manager{
		repo:        repo,
		poster:      poster,
		provisioner: provisioner,
		redisClient: rdb,
		dbPool:      dbPool,
		logger:      logger,
		activeSubs:  make(map[string]*redis.PubSub),
		activeConf:  make(map[string]string),
		syncTicker:  time.NewTicker(syncInterval),
		stopChan:    make(chan struct{}),
	}
}

// Start performs an initial sync then launches background goroutines.
func (m *Manager) Start(ctx context.Context) error {
	if err := m.SyncRelayConfigs(ctx); err != nil {
		return fmt.Errorf("initial relay config sync failed: %w", err)
	}

	m.wg.Add(1)
	go m.syncLoop(ctx)

	if m.dbPool != nil {
		m.wg.Add(1)
		go m.listenForChanges(ctx)
	} else {
		if m.logger != nil {
			m.logger.Warn("No DB pool provided — skipping LISTEN/NOTIFY watcher for relay manager")
		}
	}

	if m.logger != nil {
		m.logger.Info("Relay manager started", zap.Duration("sync_interval", syncInterval))
	}
	return nil
}

// Stop closes all subscriptions and waits for goroutines to exit.
func (m *Manager) Stop() {
	close(m.stopChan)
	m.syncTicker.Stop()
	m.wg.Wait()

	m.mu.Lock()
	defer m.mu.Unlock()
	for overlayID, sub := range m.activeSubs {
		_ = sub.Close()
		delete(m.activeSubs, overlayID)
	}
}

// SyncRelayConfigs queries the repository and reconciles subscriptions.
func (m *Manager) SyncRelayConfigs(ctx context.Context) error {
	// Auto-provision webhooks for sources that need them.
	if m.provisioner != nil {
		if err := m.provisioner.ProvisionPending(ctx); err != nil {
			if m.logger != nil {
				m.logger.Warn("Webhook provisioning had errors", zap.Error(err))
			}
		}
	}

	configs, err := m.repo.GetRelayConfigs(ctx)
	if err != nil {
		return fmt.Errorf("failed to get relay configs: %w", err)
	}

	desired := make(map[string]string, len(configs))
	for _, cfg := range configs {
		desired[cfg.OverlayID] = cfg.WebhookURL
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Subscribe to new overlays.
	for overlayID, webhookURL := range desired {
		if _, active := m.activeSubs[overlayID]; active {
			continue
		}
		sub := m.redisClient.Subscribe(ctx, "overlay:"+overlayID)
		m.activeSubs[overlayID] = sub
		m.activeConf[overlayID] = webhookURL

		m.wg.Add(1)
		go m.drainOverlay(ctx, overlayID, sub, webhookURL)
	}

	// Unsubscribe from removed overlays.
	for overlayID, sub := range m.activeSubs {
		if _, ok := desired[overlayID]; !ok {
			_ = sub.Close()
			delete(m.activeSubs, overlayID)
			delete(m.activeConf, overlayID)
		}
	}

	if m.logger != nil {
		m.logger.Info("Relay config synced",
			zap.Int("active_subscriptions", len(m.activeSubs)),
		)
	}
	return nil
}

// drainOverlay reads messages from a Redis Pub/Sub subscription and relays them.
// Uses adaptive batching: single messages are posted individually (preserving
// per-user webhook username and avatar), while bursts are batched into a single
// webhook POST to stay within Discord's ~30 req/60s rate limit.
func (m *Manager) drainOverlay(ctx context.Context, overlayID string, sub *redis.PubSub, webhookURL string) {
	defer m.wg.Done()

	ch := sub.Channel()
	var batch []RelayPayload
	ticker := time.NewTicker(batchFlushInterval)
	defer ticker.Stop()

	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				m.flushBatch(ctx, webhookURL, batch, overlayID)
				return
			}
			payload, ok := m.parseRelayPayload(msg.Payload, overlayID)
			if !ok {
				continue
			}
			batch = append(batch, payload)

			// If nothing else is queued, flush immediately — single messages
			// keep per-user username and avatar for a nicer Discord experience.
			if len(ch) == 0 {
				m.flushBatch(ctx, webhookURL, batch, overlayID)
				batch = batch[:0]
			}

		case <-ticker.C:
			if len(batch) > 0 {
				m.flushBatch(ctx, webhookURL, batch, overlayID)
				batch = batch[:0]
			}

		case <-m.stopChan:
			return
		case <-ctx.Done():
			return
		}
	}
}

// parseRelayPayload unmarshals a Pub/Sub message into a RelayPayload.
// Returns false if the message should be skipped (parse error, discord platform).
func (m *Manager) parseRelayPayload(payload string, overlayID string) (RelayPayload, bool) {
	var rm relayMessage
	if err := json.Unmarshal([]byte(payload), &rm); err != nil {
		if m.logger != nil {
			m.logger.Error("Failed to unmarshal relay message",
				zap.String("overlay_id", overlayID),
				zap.Error(err),
			)
		}
		return RelayPayload{}, false
	}
	// Loop-safety filter — never relay back to Discord.
	if rm.Platform == "discord" {
		return RelayPayload{}, false
	}
	displayName := rm.User.DisplayName
	if displayName == "" {
		displayName = rm.User.Username
	}
	return RelayPayload{
		Content:   rm.Message.Text,
		Username:  formatWebhookUsername(displayName, rm.Platform),
		AvatarURL: rm.User.AvatarURL,
	}, true
}

// flushBatch sends accumulated messages to Discord. Single messages are posted
// individually (preserving username/avatar). Multiple messages are combined into
// one webhook POST with a "**user**: message" format per line, splitting into
// multiple POSTs if the content exceeds Discord's 2000-char limit.
func (m *Manager) flushBatch(ctx context.Context, webhookURL string, batch []RelayPayload, overlayID string) {
	if len(batch) == 0 {
		return
	}

	// Single message — post normally with per-user username and avatar.
	if len(batch) == 1 {
		if err := m.poster.Post(ctx, webhookURL, batch[0]); err != nil {
			if m.logger != nil {
				m.logger.Error("Failed to post relay message to Discord",
					zap.String("overlay_id", overlayID),
					zap.Error(err),
				)
			}
		}
		return
	}

	// Multiple messages — batch into combined content POSTs.
	var contentLen int
	var lines []string

	for _, msg := range batch {
		line := fmt.Sprintf("**%s**: %s", msg.Username, msg.Content)
		lineLen := len(line) + 1 // +1 for newline

		// Flush current batch if adding this line would exceed the limit.
		if contentLen+lineLen > maxBatchContentLen && len(lines) > 0 {
			m.postBatchedContent(ctx, webhookURL, lines, overlayID)
			lines = lines[:0]
			contentLen = 0
		}

		lines = append(lines, line)
		contentLen += lineLen
	}

	// Flush remaining lines.
	if len(lines) > 0 {
		m.postBatchedContent(ctx, webhookURL, lines, overlayID)
	}
}

// postBatchedContent sends a combined multi-line message via the webhook.
func (m *Manager) postBatchedContent(ctx context.Context, webhookURL string, lines []string, overlayID string) {
	content := ""
	for i, line := range lines {
		if i > 0 {
			content += "\n"
		}
		content += line
	}

	payload := RelayPayload{
		Content:  content,
		Username: batchWebhookUsername,
	}
	if err := m.poster.Post(ctx, webhookURL, payload); err != nil {
		if m.logger != nil {
			m.logger.Error("Failed to post batched relay message to Discord",
				zap.String("overlay_id", overlayID),
				zap.Int("message_count", len(lines)),
				zap.Error(err),
			)
		}
	}
}

// HandleMessage provides a synchronous injection point for tests.
// It applies the loop-safety filter and calls poster.Post directly.
func (m *Manager) HandleMessage(ctx context.Context, platform, username, displayName, avatarURL, text, overlayID, webhookURL string) error {
	if platform == "discord" {
		return nil
	}
	name := displayName
	if name == "" {
		name = username
	}
	payload := RelayPayload{
		Content:   text,
		Username:  formatWebhookUsername(name, platform),
		AvatarURL: avatarURL,
	}
	return m.poster.Post(ctx, webhookURL, payload)
}

// syncLoop periodically re-syncs relay configs from the database.
func (m *Manager) syncLoop(ctx context.Context) {
	defer m.wg.Done()

	for {
		select {
		case <-m.syncTicker.C:
			if err := m.SyncRelayConfigs(ctx); err != nil {
				if m.logger != nil {
					m.logger.Error("Failed to sync relay configs", zap.Error(err))
				}
			}
		case <-m.stopChan:
			return
		case <-ctx.Done():
			return
		}
	}
}

// listenForChanges watches for PostgreSQL NOTIFY on chat_source_changes and
// triggers SyncRelayConfigs.
func (m *Manager) listenForChanges(ctx context.Context) {
	defer m.wg.Done()

	for {
		select {
		case <-m.stopChan:
			return
		case <-ctx.Done():
			return
		default:
			if err := m.listenAndWait(ctx); err != nil {
				if m.logger != nil {
					m.logger.Warn("Relay LISTEN error, will retry",
						zap.Error(err),
						zap.Duration("retry_in", 5*time.Second),
					)
				}
				select {
				case <-time.After(5 * time.Second):
				case <-m.stopChan:
					return
				case <-ctx.Done():
					return
				}
			}
		}
	}
}

// listenAndWait acquires a dedicated connection, issues LISTEN, and blocks on
// notifications until context cancellation or stopChan.
func (m *Manager) listenAndWait(ctx context.Context) error {
	conn, err := m.dbPool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("failed to acquire connection for LISTEN: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, "LISTEN chat_source_changes"); err != nil {
		return fmt.Errorf("failed to LISTEN: %w", err)
	}

	if m.logger != nil {
		m.logger.Info("Relay manager LISTEN active",
			zap.String("channel", "chat_source_changes"),
		)
	}

	for {
		select {
		case <-m.stopChan:
			return nil
		case <-ctx.Done():
			return nil
		default:
			notification, err := conn.Conn().WaitForNotification(ctx)
			if err != nil {
				return fmt.Errorf("notification wait failed: %w", err)
			}
			if m.logger != nil {
				m.logger.Info("Relay: source change notification received",
					zap.String("payload", notification.Payload),
				)
			}
			if err := m.SyncRelayConfigs(ctx); err != nil {
				if m.logger != nil {
					m.logger.Error("Relay: failed to sync after notification", zap.Error(err))
				}
			}
		}
	}
}
