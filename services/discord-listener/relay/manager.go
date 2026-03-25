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

const syncInterval = 30 * time.Second

// relayMessage is the local representation of a normalized chat message from
// Redis Pub/Sub. It is intentionally NOT imported from message-processor to
// keep services decoupled.
type relayMessage struct {
	Platform  string `json:"platform"`
	OverlayID string `json:"overlay_id"`
	User      struct {
		Username string `json:"username"`
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

// NewManager constructs a relay Manager.
func NewManager(
	repo RepositoryInterface,
	poster DiscordPoster,
	rdb *redis.Client,
	dbPool *pgxpool.Pool,
	logger *zap.Logger,
) *Manager {
	return &Manager{
		repo:        repo,
		poster:      poster,
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
	configs, err := m.repo.GetRelayConfigs(ctx)
	if err != nil {
		return fmt.Errorf("failed to get relay configs: %w", err)
	}

	desired := make(map[string]string, len(configs))
	for _, cfg := range configs {
		desired[cfg.OverlayID] = cfg.RelayChannelID
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Subscribe to new overlays.
	for overlayID, channelID := range desired {
		if _, active := m.activeSubs[overlayID]; active {
			continue
		}
		sub := m.redisClient.Subscribe(ctx, "overlay:"+overlayID)
		m.activeSubs[overlayID] = sub
		m.activeConf[overlayID] = channelID

		m.wg.Add(1)
		go m.drainOverlay(ctx, overlayID, sub, channelID)
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
func (m *Manager) drainOverlay(ctx context.Context, overlayID string, sub *redis.PubSub, relayChannelID string) {
	defer m.wg.Done()

	ch := sub.Channel()
	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return
			}
			var rm relayMessage
			if err := json.Unmarshal([]byte(msg.Payload), &rm); err != nil {
				if m.logger != nil {
					m.logger.Error("Failed to unmarshal relay message",
						zap.String("overlay_id", overlayID),
						zap.Error(err),
					)
				}
				continue
			}
			// Loop-safety filter — never relay back to Discord.
			if rm.Platform == "discord" {
				continue
			}
			username := formatWebhookUsername(rm.User.Username, rm.Platform)
			payload := RelayPayload{
				Content:  rm.Message.Text,
				Username: username,
			}
			if err := m.poster.Post(ctx, relayChannelID, payload); err != nil {
				if m.logger != nil {
					m.logger.Error("Failed to post relay message to Discord",
						zap.String("overlay_id", overlayID),
						zap.String("webhook_url", relayChannelID),
						zap.Error(err),
					)
				}
			}

		case <-m.stopChan:
			return
		case <-ctx.Done():
			return
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
