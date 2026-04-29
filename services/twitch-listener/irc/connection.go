// This file is part of All-Chat.
// Copyright (C) 2026 caesarakalaeii
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package irc

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/caesar/all-chat/services/message-processor/registry"
	"github.com/caesar/all-chat/services/twitch-listener/publisher"
	"github.com/caesar/all-chat/services/twitch-listener/status"
	"github.com/caesar/all-chat/services/twitch-listener/zombie"
	"github.com/caesar/all-chat/shared/metrics"
	"github.com/gempir/go-twitch-irc/v4"
	"go.uber.org/zap"
)

// zombieTracker is the subset of zombie.Detector used by ConnectionManager.
// Defined as an interface so tests can inject a no-op without importing the zombie package.
type zombieTracker interface {
	RecordReceived()
	RecordPublished()
}

// ConnectionManager manages the Twitch IRC connection
type ConnectionManager struct {
	client           *twitch.Client
	config           Config
	parser           *Parser
	publisher        *publisher.StreamPublisher
	registry         registry.MessageIDRegistry // Registry for message ID tracking
	logger           *zap.Logger
	metrics          *metrics.ListenerMetrics
	connected        bool
	connectedAt      time.Time
	lastActivityAt   time.Time // Last time any IRC message (chat, event, PING) was received
	mu               sync.RWMutex
	stopChan         chan struct{}
	wg               sync.WaitGroup
	firstMessageChan map[string]chan struct{} // Per-channel first message signal for migration
	activeChannelsFn func() []string          // Returns currently active channels for reconnect status publish
	statusPublisher  *status.Publisher        // Publishes platform status on IRC reconnect
	onDisconnect     func()                   // Called when IRC connection is lost (for channel manager reset)
	onConnect        func()                   // Called when a new IRC connection is established (for channel re-join)
	zombieDetector   zombieTracker            // Tracks received-vs-published drift for zombie detection

	// pendingJoins tracks channels for which a JOIN was sent on the wire but Twitch
	// has not yet acknowledged via SELFJOIN. Long-lived IRC sessions can silently
	// drop new JOINs (the gempir lib's local map is updated, but Twitch never starts
	// delivering PRIVMSGs); the joinAckWatchdog forces a reconnect when entries here
	// exceed joinAckTimeout. Keyed by lowercase channel name to match Twitch's wire
	// format and the gempir lib's internal map.
	pendingJoins   map[string]time.Time
	pendingJoinsMu sync.Mutex

	// bannedChannels marks channels Twitch has explicitly rejected our JOIN for
	// (msg_banned, msg_channel_suspended, msg_room_not_found, or
	// msg_concurrent_channel_limit_reached). Re-joining these on the next sync
	// or after a reconnect is wasted effort: bans/suspensions never recover, and
	// the cap is per-account so a fresh connection hits the same wall.
	//
	// Value semantics:
	//   zero time     -> permanent (never re-attempt for this process lifetime)
	//   non-zero time -> back off until this instant, then allow a re-attempt
	//
	// Without this guard the joinAckWatchdog enters a tight reconnect loop on a
	// pod that holds a single banned/cap-rejected channel: ack never arrives,
	// watchdog reconnects, fresh JOIN gets the same NOTICE, repeat.
	bannedChannels   map[string]time.Time
	bannedChannelsMu sync.Mutex
}

// Config holds the configuration for IRC connection
type Config struct {
	Username string // Twitch bot username
	OAuth    string // OAuth token (oauth:abc123...)
}

// NewConnectionManager creates a new IRC connection manager
func NewConnectionManager(
	config Config,
	parser *Parser,
	pub *publisher.StreamPublisher,
	reg registry.MessageIDRegistry,
	logger *zap.Logger,
	m *metrics.ListenerMetrics,
) *ConnectionManager {
	client := twitch.NewClient(config.Username, config.OAuth)

	cm := &ConnectionManager{
		client:           client,
		config:           config,
		parser:           parser,
		publisher:        pub,
		registry:         reg,
		logger:           logger,
		metrics:          m,
		stopChan:         make(chan struct{}),
		firstMessageChan: make(map[string]chan struct{}),
		pendingJoins:     make(map[string]time.Time),
		bannedChannels:   make(map[string]time.Time),
	}

	cm.setupClientCallbacks(client)

	return cm
}

// setupClientCallbacks wires all event callbacks onto a freshly created twitch client.
// Called from NewConnectionManager and from the reconnect loop when creating a new client.
func (cm *ConnectionManager) setupClientCallbacks(client *twitch.Client) {
	// Chat and event handlers
	client.OnPrivateMessage(cm.handlePrivateMessage)
	client.OnUserNoticeMessage(cm.handleUserNotice)

	// Deletion handlers
	client.OnClearMessage(cm.handleClearMessage)
	client.OnClearChatMessage(cm.handleClearChat)

	// Connection handler
	client.OnConnect(cm.handleConnect)

	// JOIN ack — Twitch echoes our own JOINs back to us. We use this to confirm
	// JOINs reached Twitch; missing acks within joinAckTimeout indicate the
	// connection silently dropped the JOIN and the watchdog forces a reconnect.
	client.OnSelfJoinMessage(cm.handleSelfJoin)

	// NOTICE — surfaces error replies (msg_banned, msg_channel_suspended, etc.)
	// that would otherwise be silent. Without this, failed JOINs leave no trace.
	client.OnNoticeMessage(cm.handleNotice)

	// PING sent by our client — update activity so the watchdog knows the TCP
	// connection is still alive even when no chat messages are flowing.
	client.OnPingSent(cm.recordActivity)
}

const (
	// staleConnectionTimeout is how long without any IRC activity (PING/PONG or chat)
	// before the watchdog considers the connection dead and forces a reconnect.
	// Twitch sends PINGs every ~5 minutes; we also call recordActivity on OnPingSent
	// so 6 minutes without any activity means the connection is zombie.
	staleConnectionTimeout = 6 * time.Minute

	// staleLivenessThreshold is how long without any IRC activity before the
	// liveness probe returns 503. This is intentionally longer than
	// staleConnectionTimeout to give the watchdog multiple attempts to recover
	// before Kubernetes restarts the pod.
	staleLivenessThreshold = 10 * time.Minute

	// joinAckTimeout is how long we wait for Twitch to send a SELFJOIN ack
	// for a JOIN we issued. Twitch normally responds within seconds, but
	// the gempir lib's JOIN rate-limiter throttles us to 20 channels per
	// 10s — a 94-channel sync burst takes ~47s on the wire before the last
	// JOIN is even sent, and any cap pressure (msg_concurrent_channel_limit
	// _reached) further delays NOTICE replies. The original 60s value
	// blew through every burst and looped the watchdog into reconnect
	// storms. 180s leaves comfortable headroom for the burst + ack while
	// still catching genuinely silently-dropped JOINs (the original PR #279
	// motivation) within ~3 minutes.
	joinAckTimeout = 180 * time.Second

	// joinAckWatchdogInterval is how often the joinAckWatchdog scans
	// pendingJoins for expired entries. Tuned to ~1/3 of joinAckTimeout so
	// a stuck JOIN is detected within ~1.3x the timeout in the worst case.
	joinAckWatchdogInterval = 60 * time.Second

	// concurrentChannelLimitBackoff is how long a channel is skipped after
	// Twitch returns msg_concurrent_channel_limit_reached. The cap is per
	// bot-account and shared across pods, so reconnecting doesn't free a
	// slot — backing off and letting other PARTs/idle-aging make room is
	// the only useful response.
	concurrentChannelLimitBackoff = 10 * time.Minute
)

// JOIN-rejection NOTICE msg_id values that mean "do not re-attempt this JOIN".
// See https://dev.twitch.tv/docs/irc/msg-id/ for the full list.
const (
	twitchMsgIDBanned                  = "msg_banned"
	twitchMsgIDChannelSuspended        = "msg_channel_suspended"
	twitchMsgIDRoomNotFound            = "msg_room_not_found"
	twitchMsgIDConcurrentChannelLimit  = "msg_concurrent_channel_limit_reached"
)

// Connect establishes connection to Twitch IRC with automatic reconnection.
// If the underlying client.Connect() returns (connection permanently lost),
// a new client is created and reconnected after a backoff delay.
func (cm *ConnectionManager) Connect(ctx context.Context) error {
	cm.logger.Info("Connecting to Twitch IRC")
	cm.metrics.RecordConnectionAttempt("twitch", "twitch-listener", "attempting")

	// Start watchdog that detects zombie connections
	cm.wg.Add(1)
	go cm.connectionWatchdog(ctx)

	// Start watchdog that detects JOINs that never got a SELFJOIN ack from Twitch
	cm.wg.Add(1)
	go cm.joinAckWatchdog(ctx)

	cm.wg.Add(1)
	go func() {
		defer cm.wg.Done()
		backoff := 5 * time.Second
		const maxBackoff = 60 * time.Second

		for {
			err := cm.client.Connect()

			// Mark disconnected immediately when Connect() returns
			cm.mu.Lock()
			wasConnected := cm.connected
			cm.connected = false
			cm.mu.Unlock()

			if wasConnected {
				cm.metrics.RecordConnection("twitch", "twitch-listener", "irc", false)
				cm.logger.Warn("IRC connection lost", zap.Error(err))

				// Drop pending join tracking — JOINs will be re-issued on the
				// fresh client by the next SyncChannels and re-recorded then.
				cm.clearPendingJoins()

				// Notify channel manager to clear stale activeChans
				cm.mu.RLock()
				onDisconnect := cm.onDisconnect
				cm.mu.RUnlock()
				if onDisconnect != nil {
					onDisconnect()
				}
			}

			// Check if we should stop (graceful shutdown)
			select {
			case <-cm.stopChan:
				cm.logger.Info("IRC reconnect loop stopped (shutdown)")
				return
			default:
			}

			if err != nil {
				cm.logger.Error("IRC connection error, reconnecting",
					zap.Error(err),
					zap.Duration("backoff", backoff),
				)
				cm.metrics.RecordConnectionAttempt("twitch", "twitch-listener", "failed")
				cm.metrics.RecordError("twitch", "twitch-listener", "connection", "error")
			} else {
				cm.logger.Warn("IRC connection closed cleanly, reconnecting",
					zap.Duration("backoff", backoff),
				)
			}

			// Wait before reconnecting (with shutdown check)
			select {
			case <-cm.stopChan:
				cm.logger.Info("IRC reconnect loop stopped during backoff (shutdown)")
				return
			case <-time.After(backoff):
			}

			// Increase backoff for next attempt (capped)
			backoff = backoff * 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}

			// Create fresh client — the old client's internal state is stale after Connect() returns
			cm.logger.Info("Creating new IRC client for reconnection")
			cm.metrics.RecordConnectionAttempt("twitch", "twitch-listener", "attempting")

			newClient := twitch.NewClient(cm.config.Username, cm.config.OAuth)
			cm.setupClientCallbacks(newClient)

			cm.mu.Lock()
			cm.client = newClient
			cm.mu.Unlock()

			// Reset backoff on successful connect (handleConnect sets connected=true)
			backoff = 5 * time.Second
		}
	}()

	return nil
}

// connectionWatchdog periodically checks for zombie IRC connections.
// Twitch sends PING every ~5 minutes; if we receive no IRC activity at all
// for staleConnectionTimeout, the connection is silently dead.
// Forces a disconnect to trigger the reconnect loop.
func (cm *ConnectionManager) connectionWatchdog(ctx context.Context) {
	defer cm.wg.Done()
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-cm.stopChan:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			cm.mu.RLock()
			connected := cm.connected
			lastActivity := cm.lastActivityAt
			client := cm.client
			cm.mu.RUnlock()

			if !connected || lastActivity.IsZero() {
				continue
			}

			stale := time.Since(lastActivity)
			if stale > staleConnectionTimeout {
				cm.logger.Warn("IRC connection appears zombie, forcing reconnect",
					zap.Duration("idle_duration", stale),
					zap.Duration("threshold", staleConnectionTimeout),
				)
				cm.metrics.RecordError("twitch", "twitch-listener", "connection", "stale_reconnect")

				// Disconnect triggers the reconnect loop in Connect()
				if err := client.Disconnect(); err != nil {
					cm.logger.Warn("Error forcing disconnect on stale connection", zap.Error(err))
				}
			}
		}
	}
}

// Disconnect gracefully disconnects from Twitch IRC
func (cm *ConnectionManager) Disconnect() error {
	cm.logger.Info("Disconnecting from Twitch IRC")

	close(cm.stopChan)

	// Record connection duration if we were connected
	cm.mu.RLock()
	if cm.connected && !cm.connectedAt.IsZero() {
		duration := time.Since(cm.connectedAt).Seconds()
		cm.metrics.ConnectionDuration.WithLabelValues("twitch", "twitch-listener", "normal").Observe(duration)
	}
	cm.mu.RUnlock()

	cm.mu.RLock()
	client := cm.client
	cm.mu.RUnlock()
	if err := client.Disconnect(); err != nil {
		cm.logger.Warn("Error during disconnect", zap.Error(err))
		cm.metrics.RecordError("twitch", "twitch-listener", "connection", "warning")
	}

	cm.wg.Wait()

	// Mark as disconnected
	cm.metrics.RecordConnection("twitch", "twitch-listener", "irc", false)

	cm.logger.Info("Disconnected from Twitch IRC")
	return nil
}

// Join joins a Twitch channel.
// Records the channel as a pending JOIN so the joinAckWatchdog can detect
// silent failures where Twitch never returns the SELFJOIN ack.
//
// Channels in the bannedChannels skip list (populated by handleNotice on
// JOIN-rejection NOTICEs) are short-circuited: re-attempting them just churns
// the connection without producing any messages, and historically caused the
// joinAckWatchdog to enter a reconnect loop.
func (cm *ConnectionManager) Join(channel string) {
	lower := strings.ToLower(channel)

	if cm.isJoinBanned(lower) {
		cm.logger.Debug("Skipping JOIN for banned/cap-rejected channel",
			zap.String("channel", lower),
		)
		return
	}

	cm.mu.RLock()
	client := cm.client
	connected := cm.connected
	cm.mu.RUnlock()

	client.Join(channel)

	// Only track pending acks while connected; if we're offline, the gempir
	// lib defers the wire send and the next reconnect re-issues all JOINs.
	if connected {
		cm.pendingJoinsMu.Lock()
		cm.pendingJoins[lower] = time.Now()
		cm.pendingJoinsMu.Unlock()
	}

	cm.logger.Debug("IRC JOIN", zap.String("channel", channel))
}

// isJoinBanned reports whether channel is currently in the JOIN-rejection
// skip list. Permanent entries (zero time) always return true; transient
// entries return true until their backoff expires, after which the entry is
// evicted lazily so the next attempt proceeds normally.
func (cm *ConnectionManager) isJoinBanned(channel string) bool {
	cm.bannedChannelsMu.Lock()
	defer cm.bannedChannelsMu.Unlock()

	until, exists := cm.bannedChannels[channel]
	if !exists {
		return false
	}
	if until.IsZero() {
		return true
	}
	if time.Now().Before(until) {
		return true
	}
	delete(cm.bannedChannels, channel)
	return false
}

// Depart leaves a Twitch channel.
// The channel name is lowercased to match the go-twitch-irc library's internal
// tracking (Join lowercases but Depart does not), preventing ghost entries that
// cause subsequent Join calls to be silently skipped.
func (cm *ConnectionManager) Depart(channel string) {
	cm.mu.RLock()
	client := cm.client
	cm.mu.RUnlock()
	lower := strings.ToLower(channel)
	client.Depart(lower)

	// Drop any pending ack — Departing means we no longer expect to be in this channel.
	cm.pendingJoinsMu.Lock()
	delete(cm.pendingJoins, lower)
	cm.pendingJoinsMu.Unlock()

	cm.logger.Debug("IRC PART", zap.String("channel", channel))
}

// clearPendingJoins drops all pending JOIN ack tracking. Called on disconnect
// (and at the start of a fresh connection) because reconnects discard the
// gempir client's internal channel map; JOINs are re-issued by the next sync.
func (cm *ConnectionManager) clearPendingJoins() {
	cm.pendingJoinsMu.Lock()
	if len(cm.pendingJoins) > 0 {
		cm.pendingJoins = make(map[string]time.Time)
	}
	cm.pendingJoinsMu.Unlock()
}

// handleSelfJoin records that Twitch has acknowledged our JOIN by echoing it
// back. Removing from pendingJoins prevents the watchdog from forcing a
// reconnect for this channel.
func (cm *ConnectionManager) handleSelfJoin(message twitch.UserJoinMessage) {
	cm.recordActivity()

	channel := strings.ToLower(message.Channel)
	cm.pendingJoinsMu.Lock()
	_, wasPending := cm.pendingJoins[channel]
	delete(cm.pendingJoins, channel)
	cm.pendingJoinsMu.Unlock()

	if wasPending {
		cm.logger.Debug("JOIN acknowledged by Twitch",
			zap.String("channel", channel),
		)
	}
}

// handleNotice surfaces NOTICE replies from Twitch. The msg-id tag indicates
// the kind of notice (msg_banned, msg_channel_suspended, msg_room_not_found,
// etc.) — without this handler, failed JOINs and other server-side errors are
// invisible to operators.
func (cm *ConnectionManager) handleNotice(message twitch.NoticeMessage) {
	cm.recordActivity()

	channel := strings.ToLower(message.Channel)
	cm.logger.Warn("Twitch IRC NOTICE",
		zap.String("channel", channel),
		zap.String("msg_id", message.MsgID),
		zap.String("message", message.Message),
	)
	cm.metrics.RecordError("twitch", "twitch-listener", "irc_notice", "warning")

	// JOIN-rejection NOTICEs: Twitch is telling us this JOIN will not produce
	// a SELFJOIN. Drop the pendingJoins entry so the watchdog stops counting
	// it as stuck, and remember the channel so future Join() calls short-
	// circuit. Without this guard the watchdog enters a tight reconnect loop
	// because the post-reconnect re-JOIN gets the same NOTICE.
	var blockUntil time.Time
	switch message.MsgID {
	case twitchMsgIDBanned, twitchMsgIDChannelSuspended, twitchMsgIDRoomNotFound:
		// Permanent within this process lifetime. zero time = forever.
	case twitchMsgIDConcurrentChannelLimit:
		// Per-account cap is shared across pods, so reconnecting to retry
		// won't free a slot. Back off and let normal PARTs / idle-aging
		// reduce demand before re-attempting.
		blockUntil = time.Now().Add(concurrentChannelLimitBackoff)
	default:
		return
	}

	if channel == "" {
		// Unparseable channel — nothing to skip.
		return
	}

	cm.bannedChannelsMu.Lock()
	cm.bannedChannels[channel] = blockUntil
	cm.bannedChannelsMu.Unlock()

	cm.pendingJoinsMu.Lock()
	delete(cm.pendingJoins, channel)
	cm.pendingJoinsMu.Unlock()
}

// joinAckWatchdog scans pendingJoins on a periodic ticker and forces a
// reconnect when entries exceed joinAckTimeout. This recovers from the
// long-lived-session bug where the gempir client's internal map shows the
// channel as joined but Twitch silently dropped our JOIN and never delivers
// PRIVMSGs. A fresh client (created by the reconnect loop) re-issues every
// JOIN cleanly.
func (cm *ConnectionManager) joinAckWatchdog(ctx context.Context) {
	defer cm.wg.Done()
	ticker := time.NewTicker(joinAckWatchdogInterval)
	defer ticker.Stop()

	for {
		select {
		case <-cm.stopChan:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			cm.mu.RLock()
			connected := cm.connected
			client := cm.client
			cm.mu.RUnlock()

			if !connected {
				continue
			}

			now := time.Now()
			var stuck []string
			var oldest time.Duration

			cm.pendingJoinsMu.Lock()
			for ch, sentAt := range cm.pendingJoins {
				age := now.Sub(sentAt)
				if age > joinAckTimeout {
					stuck = append(stuck, ch)
					if age > oldest {
						oldest = age
					}
				}
			}
			cm.pendingJoinsMu.Unlock()

			if len(stuck) == 0 {
				continue
			}

			cm.logger.Warn("JOIN ack missing past timeout, forcing reconnect",
				zap.Strings("channels", stuck),
				zap.Duration("oldest_age", oldest),
				zap.Duration("threshold", joinAckTimeout),
			)
			cm.metrics.RecordError("twitch", "twitch-listener", "join_ack_timeout", "warning")

			// Disconnect triggers the reconnect loop — fresh client re-joins all channels.
			if err := client.Disconnect(); err != nil {
				cm.logger.Warn("Error forcing disconnect for stuck JOINs", zap.Error(err))
			}
		}
	}
}

// IsConnected returns whether the IRC client is connected
func (cm *ConnectionManager) IsConnected() bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.connected
}

// SetFirstMessageChan sets the first message channel map for migration coordination
func (cm *ConnectionManager) SetFirstMessageChan(firstMessageChan map[string]chan struct{}) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.firstMessageChan = firstMessageChan
}

// SetActiveChannelsFn sets a callback that returns the list of currently active channels.
// This is used on IRC reconnect to re-publish connected status for all channels.
func (cm *ConnectionManager) SetActiveChannelsFn(fn func() []string, pub *status.Publisher) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.activeChannelsFn = fn
	cm.statusPublisher = pub
}

// handleConnect is called when connection is established
func (cm *ConnectionManager) handleConnect() {
	now := time.Now()
	cm.mu.Lock()
	cm.connected = true
	cm.connectedAt = now
	cm.lastActivityAt = now
	activeChannelsFn := cm.activeChannelsFn
	statusPublisher := cm.statusPublisher
	onConnect := cm.onConnect
	cm.mu.Unlock()

	// Discard any stale pending JOIN tracking from a previous client; the next
	// SyncChannels will re-issue all JOINs on this fresh client and re-record them.
	cm.clearPendingJoins()

	// Record successful connection
	cm.metrics.RecordConnectionAttempt("twitch", "twitch-listener", "success")
	cm.metrics.RecordConnection("twitch", "twitch-listener", "irc", true)

	cm.logger.Info("Connected to Twitch IRC")

	// Clear stale activeChans populated during the reconnect backoff window.
	// When the watchdog forces a disconnect, the channel manager's periodic sync may
	// run and issue Join() calls on the OLD disconnected client (during the 5-second
	// backoff before the new client is created). Those joins fail silently but mark
	// channels as active. Clearing activeChans here ensures the next SyncChannels
	// call re-joins all channels on the freshly connected client.
	if onConnect != nil {
		onConnect()
	}

	// On IRC reconnect, re-publish connected status for all currently active channels.
	// This covers the case where the IRC client reconnects after a network interruption.
	// Note: this runs AFTER onConnect clears activeChans, so we query the now-empty list.
	// The channels will be re-published once they are re-joined via SyncChannels.
	if activeChannelsFn != nil && statusPublisher != nil {
		channels := activeChannelsFn()
		if len(channels) > 0 {
			ctx := context.Background()
			for _, ch := range channels {
				statusPublisher.Publish(ctx, status.Message{
					Platform:  "twitch",
					ChannelID: strings.ToLower(ch),
					Status:    "connected",
				})
			}
			cm.logger.Info("Re-published connected status for active channels after IRC connect",
				zap.Int("channel_count", len(channels)),
			)
		}
	}
}

// recordActivity updates the last activity timestamp.
// Called on every incoming IRC message to detect stale connections.
func (cm *ConnectionManager) recordActivity() {
	cm.mu.Lock()
	cm.lastActivityAt = time.Now()
	cm.mu.Unlock()
}

// LastActivityAt returns when the last IRC message was received.
// Used by health checks to detect silently dead connections.
func (cm *ConnectionManager) LastActivityAt() time.Time {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.lastActivityAt
}

// IsStale returns true when the connection has gone silent for longer than
// staleLivenessThreshold.  A zero lastActivityAt (never connected) returns
// false so the pod is not killed before the initial connection is established.
// Used by the liveness probe to trigger a Kubernetes pod restart when the
// watchdog's Disconnect() call fails to unblock the stuck Connect() goroutine.
func (cm *ConnectionManager) IsStale() bool {
	cm.mu.RLock()
	last := cm.lastActivityAt
	cm.mu.RUnlock()
	if last.IsZero() {
		return false
	}
	return time.Since(last) > staleLivenessThreshold
}

// SetOnDisconnect registers a callback invoked when the IRC connection is lost.
// The channel manager uses this to clear stale activeChans so the next sync re-joins.
func (cm *ConnectionManager) SetOnDisconnect(fn func()) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.onDisconnect = fn
}

// SetOnConnect registers a callback invoked when a new IRC connection is established.
// The channel manager uses this to clear activeChans that were populated while the
// previous client was disconnected (race between the periodic sync and reconnect backoff),
// so the next sync re-joins all channels on the fresh client.
func (cm *ConnectionManager) SetOnConnect(fn func()) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.onConnect = fn
}

// SetZombieDetector wires a zombie.Detector into the connection manager.
// RecordReceived() is called on each incoming PRIVMSG; RecordPublished() is called
// after each successful publish to the ring buffer. This powers the liveness probe's
// received-vs-published drift check (Z-01).
func (cm *ConnectionManager) SetZombieDetector(d *zombie.Detector) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.zombieDetector = d
}

// handlePrivateMessage processes incoming PRIVMSG from Twitch
func (cm *ConnectionManager) handlePrivateMessage(message twitch.PrivateMessage) {
	cm.recordActivity()
	start := time.Now()

	// Record message received for zombie drift detection (Z-01).
	cm.mu.RLock()
	zd := cm.zombieDetector
	cm.mu.RUnlock()
	if zd != nil {
		zd.RecordReceived()
	}

	// Record message received
	cm.metrics.RecordMessage("twitch", "twitch-listener", message.Channel, "chat")

	// Parse IRC message into RawChatMessage
	rawMsg, err := cm.parser.ParsePrivateMessage(message)
	if err != nil {
		cm.logger.Warn("Failed to parse IRC message",
			zap.String("channel", message.Channel),
			zap.String("user", message.User.Name),
			zap.Error(err),
		)
		cm.metrics.RecordError("twitch", "twitch-listener", "parsing", "warning")
		return
	}

	// Add to registry IMMEDIATELY at capture point (per CONTEXT.md user decision)
	// This happens BEFORE publishing to Redis Streams
	if platformMsgID := rawMsg.Tags["id"]; platformMsgID != "" {
		ctx := context.Background()
		if err := cm.registry.Add(ctx, rawMsg.Platform, rawMsg.ChannelID, platformMsgID, rawMsg.MessageID); err != nil {
			cm.logger.Error("Failed to add message to registry at listener",
				zap.Error(err),
				zap.String("platform_msg_id", platformMsgID),
				zap.String("internal_uuid", rawMsg.MessageID),
			)
			// Continue processing - registry is best-effort
		} else {
			cm.logger.Debug("Added message to registry at capture",
				zap.String("platform_msg_id", platformMsgID),
				zap.String("internal_uuid", rawMsg.MessageID),
			)
		}
	}

	// Publish to Redis Streams (AFTER registry)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := cm.publisher.Publish(ctx, rawMsg); err != nil {
		cm.logger.Error("Failed to publish message",
			zap.String("message_id", rawMsg.MessageID),
			zap.String("channel", rawMsg.ChannelID),
			zap.Error(err),
		)
		cm.metrics.RecordPublish("twitch", "twitch-listener", "failed")
		cm.metrics.RecordError("twitch", "twitch-listener", "internal", "error")
		return
	}

	// Record successful publish and latency
	cm.metrics.RecordPublish("twitch", "twitch-listener", "success")
	cm.metrics.MessageLatency.WithLabelValues("twitch", "twitch-listener").Observe(time.Since(start).Seconds())

	// Record publish for zombie drift detection (Z-01).
	// Called after ring buffer accept — the ring buffer absorbs transient XADD failures
	// so this counter reflects messages accepted by the delivery pipeline, not XADD success.
	if zd != nil {
		zd.RecordPublished()
	}

	cm.logger.Debug("Published message",
		zap.String("message_id", rawMsg.MessageID),
		zap.String("channel", rawMsg.ChannelID),
		zap.String("username", rawMsg.Username),
		zap.String("text", rawMsg.Text),
	)

	// Signal first message for migration confirmation
	// Use non-blocking send to avoid deadlock if no migration is waiting
	if cm.firstMessageChan != nil {
		select {
		case cm.firstMessageChan[message.Channel] <- struct{}{}:
			cm.logger.Debug("Signaled first message for migration", zap.String("channel", message.Channel))
		default:
			// Channel not waiting or already signaled - this is normal
		}
	}
}

// handleUserNotice processes incoming USERNOTICE events from Twitch (subs, raids, bits, etc.)
func (cm *ConnectionManager) handleUserNotice(message twitch.UserNoticeMessage) {
	cm.recordActivity()
	start := time.Now()

	// Record event received
	cm.metrics.RecordMessage("twitch", "twitch-listener", message.Channel, "event")

	// Parse USERNOTICE into RawChatMessage with event fields
	rawMsg, err := cm.parser.ParseUserNotice(message)
	if err != nil {
		cm.logger.Warn("Failed to parse USERNOTICE",
			zap.String("channel", message.Channel),
			zap.String("msg-id", message.MsgID),
			zap.String("user", message.User.Name),
			zap.Error(err),
		)
		cm.metrics.RecordError("twitch", "twitch-listener", "parsing", "warning")
		return
	}

	// Publish to Redis Streams (same flow as chat messages)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := cm.publisher.Publish(ctx, rawMsg); err != nil {
		cm.logger.Error("Failed to publish event",
			zap.String("message_id", rawMsg.MessageID),
			zap.String("channel", rawMsg.ChannelID),
			zap.String("event_type", rawMsg.EventType),
			zap.Error(err),
		)
		cm.metrics.RecordPublish("twitch", "twitch-listener", "failed")
		cm.metrics.RecordError("twitch", "twitch-listener", "internal", "error")
		return
	}

	// Record successful publish and latency
	cm.metrics.RecordPublish("twitch", "twitch-listener", "success")
	cm.metrics.MessageLatency.WithLabelValues("twitch", "twitch-listener").Observe(time.Since(start).Seconds())

	cm.logger.Info("Published Twitch event",
		zap.String("message_id", rawMsg.MessageID),
		zap.String("channel", rawMsg.ChannelID),
		zap.String("event_type", rawMsg.EventType),
		zap.String("msg-id", message.MsgID),
		zap.String("username", rawMsg.Username),
	)
}

// handleClearMessage processes CLEARMSG (single message deletion)
func (cm *ConnectionManager) handleClearMessage(message twitch.ClearMessage) {
	cm.recordActivity()
	start := time.Now()

	// Record deletion event received
	cm.metrics.RecordMessage("twitch", "twitch-listener", message.Channel, "deletion")

	// Parse to RawChatMessage with EventType="message_deletion"
	rawMsg := cm.parser.ParseClearMessage(message)
	if rawMsg == nil {
		cm.logger.Error("Failed to parse CLEARMSG",
			zap.String("channel", message.Channel),
			zap.String("target_msg_id", message.TargetMsgID),
		)
		cm.metrics.RecordError("twitch", "twitch-listener", "parsing", "error")
		return
	}

	// Publish to Redis Streams
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := cm.publisher.Publish(ctx, rawMsg); err != nil {
		cm.logger.Error("Failed to publish deletion event",
			zap.Error(err),
			zap.String("channel", message.Channel),
			zap.String("target_msg_id", message.TargetMsgID),
		)
		cm.metrics.RecordPublish("twitch", "twitch-listener", "failed")
		cm.metrics.RecordError("twitch", "twitch-listener", "internal", "error")
		return
	}

	// Record successful processing
	cm.metrics.RecordPublish("twitch", "twitch-listener", "success")
	cm.metrics.MessageLatency.WithLabelValues("twitch", "twitch-listener").Observe(time.Since(start).Seconds())

	cm.logger.Debug("Processed CLEARMSG",
		zap.String("channel", message.Channel),
		zap.String("target_msg_id", message.TargetMsgID),
		zap.Duration("duration", time.Since(start)),
	)
}

// handleClearChat processes CLEARCHAT (user timeout/ban or full clear)
func (cm *ConnectionManager) handleClearChat(message twitch.ClearChatMessage) {
	cm.recordActivity()
	start := time.Now()

	cm.metrics.RecordMessage("twitch", "twitch-listener", message.Channel, "deletion")

	// Parse to RawChatMessage
	rawMsg := cm.parser.ParseClearChat(message)
	if rawMsg == nil {
		cm.logger.Error("Failed to parse CLEARCHAT",
			zap.String("channel", message.Channel),
			zap.String("target_user", message.TargetUsername),
		)
		cm.metrics.RecordError("twitch", "twitch-listener", "parsing", "error")
		return
	}

	// Publish to Redis Streams
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := cm.publisher.Publish(ctx, rawMsg); err != nil {
		cm.logger.Error("Failed to publish deletion event",
			zap.Error(err),
			zap.String("channel", message.Channel),
		)
		cm.metrics.RecordPublish("twitch", "twitch-listener", "failed")
		cm.metrics.RecordError("twitch", "twitch-listener", "internal", "error")
		return
	}

	cm.metrics.RecordPublish("twitch", "twitch-listener", "success")
	cm.metrics.MessageLatency.WithLabelValues("twitch", "twitch-listener").Observe(time.Since(start).Seconds())

	cm.logger.Debug("Processed CLEARCHAT",
		zap.String("channel", message.Channel),
		zap.String("target_user", message.TargetUsername),
		zap.Duration("duration", time.Since(start)),
	)
}

// GetClient returns the underlying Twitch client (for testing)
func (cm *ConnectionManager) GetClient() *twitch.Client {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.client
}
