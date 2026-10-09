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

package channels

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/caesar/all-chat/services/owncast-listener/metrics"
	"github.com/caesar/all-chat/services/owncast-listener/publisher"
	"github.com/caesar/all-chat/services/owncast-listener/status"
	"github.com/caesar/all-chat/services/owncast-listener/websocket"
	"github.com/caesar/all-chat/shared/listener"
	"github.com/caesar/all-chat/shared/sourcemanager"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// syncInterval matches the other listeners (30s channel sync from DB).
const syncInterval = 30 * time.Second

// registerTimeout bounds the POST /api/chat/register call per connect attempt.
const registerTimeout = 10 * time.Second

// connState carries per-connection goroutine control.
type connState struct {
	cancel context.CancelFunc
	doneCh chan struct{}
}

// Manager manages one Owncast chat connection PER INSTANCE URL.
// Channels here are base URLs (https://watch.example.org), one stream each.
type Manager struct {
	repo        *Repository
	publisher   *publisher.StreamPublisher
	statusPub   *status.Publisher
	logger      *zap.Logger
	httpClient  *http.Client
	redisClient *redis.Client
	leader      *sourcemanager.LeadershipCoordinator
	podID       string

	mu          sync.Mutex
	conns       map[string]*connState // instanceURL -> running connection
	clients     map[string]*websocket.Client
	instances   map[string]*trackedInstance
	overlays    map[string][]string // instanceURL -> overlay IDs
	demanded    map[string]listener.DemandedSource
	filteredCnt int

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

type trackedInstance struct {
	InstanceURL string
	OverlayIDs  []string
}

// compile-time check against the SDK ChannelManager interface.
var _ listener.ChannelManager = (*Manager)(nil)

// NewManager builds the manager. leader may be nil (single-pod deploys/tests).
func NewManager(
	repo *Repository,
	pub *publisher.StreamPublisher,
	statusPub *status.Publisher,
	redisClient *redis.Client,
	leader *sourcemanager.LeadershipCoordinator,
	podID string,
	logger *zap.Logger,
) *Manager {
	log := logger
	if log == nil {
		log = zap.NewNop()
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		repo:        repo,
		publisher:   pub,
		statusPub:   statusPub,
		logger:      log,
		httpClient:  &http.Client{Timeout: registerTimeout},
		redisClient: redisClient,
		leader:      leader,
		podID:       podID,
		conns:       make(map[string]*connState),
		clients:     make(map[string]*websocket.Client),
		instances:   make(map[string]*trackedInstance),
		overlays:    make(map[string][]string),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// SetStatusPublisher injects the status publisher after construction,
// mirroring the kick-listener wiring.
func (m *Manager) SetStatusPublisher(pub *status.Publisher) {
	m.statusPub = pub
}

// Start begins the sync loop.
func (m *Manager) Start(_ context.Context) error {
	m.logger.Info("Starting Owncast channel manager")
	if err := m.sync(); err != nil {
		m.logger.Error("Initial owncast sync failed", zap.Error(err))
	}
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		ticker := time.NewTicker(syncInterval)
		defer ticker.Stop()
		for {
			select {
			case <-m.ctx.Done():
				return
			case <-ticker.C:
				if err := m.sync(); err != nil {
					m.logger.Error("Failed to sync owncast instances", zap.Error(err))
				}
			}
		}
	}()
	return nil
}

// Stop tears down all connections.
func (m *Manager) Stop() {
	m.cancel()
	m.wg.Wait()
	if m.leader != nil {
		m.leader.Stop()
	}
	m.mu.Lock()
	for _, cs := range m.conns {
		cs.cancel()
	}
	m.mu.Unlock()
	m.logger.Info("Owncast channel manager stopped")
}

// UpdateAssignedSourceIDs is a no-op retained for interface stability.
func (m *Manager) UpdateAssignedSourceIDs(_ map[string]bool) {}

// UpdateDemandedSourceIDs stores the demanded set and triggers a sync.
func (m *Manager) UpdateDemandedSourceIDs(demanded map[string]listener.DemandedSource) {
	m.mu.Lock()
	m.demanded = demanded
	m.mu.Unlock()
	if err := m.sync(); err != nil {
		m.logger.Error("Failed to sync after demand update", zap.Error(err))
	}
}

// GetFilteredAssignmentCount reports how many demanded instances exist.
func (m *Manager) GetFilteredAssignmentCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.filteredCnt
}

// GetActiveChannels returns connected instance URLs.
func (m *Manager) GetActiveChannels() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, 0, len(m.conns))
	for u := range m.conns {
		out = append(out, u)
	}
	return out
}

// GetActiveChannelCount returns the number of running connections.
func (m *Manager) GetActiveChannelCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.conns)
}

// sync reconciles running connections with the demanded instance set.
func (m *Manager) sync() error {
	instances, err := m.repo.GetActiveInstances(m.ctx)
	if err != nil {
		return fmt.Errorf("failed to get active owncast instances: %w", err)
	}

	m.mu.Lock()
	demanded := m.demanded
	m.mu.Unlock()

	// nil means no demand update yet: sync everything (startup grace).
	desired := make(map[string]*trackedInstance, len(instances))
	for _, in := range instances {
		if demanded != nil {
			if _, ok := demanded[in.SourceID]; !ok {
				continue
			}
		}
		if in.InstanceURL == "" {
			continue
		}
		tr := desired[in.InstanceURL]
		if tr == nil {
			tr = &trackedInstance{InstanceURL: in.InstanceURL}
			desired[in.InstanceURL] = tr
		}
		tr.OverlayIDs = append(tr.OverlayIDs, in.OverlayID)
	}

	m.mu.Lock()
	m.filteredCnt = len(desired)
	m.mu.Unlock()

	// Stop connections for instances no longer desired.
	m.mu.Lock()
	for u, cs := range m.conns {
		if _, ok := desired[u]; !ok {
			m.logger.Info("Stopping owncast connection", zap.String("instance", u))
			cs.cancel()
			delete(m.conns, u)
			delete(m.clients, u)
			delete(m.overlays, u)
			if m.statusPub != nil {
				m.statusPub.Publish(m.ctx, status.Message{
					Platform:  "owncast",
					ChannelID: u,
					Status:    "offline",
				})
			}
			_ = m.repo.SetSourceActive(m.ctx, u, false)
		}
	}
	m.mu.Unlock()

	// Record desired overlays and start missing connections.
	m.mu.Lock()
	m.overlays = make(map[string][]string, len(desired))
	m.instances = make(map[string]*trackedInstance, len(desired))
	m.mu.Unlock()

	for u, tr := range desired {
		m.mu.Lock()
		m.overlays[u] = tr.OverlayIDs
		m.instances[u] = tr
		_, running := m.conns[u]
		m.mu.Unlock()
		if running {
			continue
		}
		m.startConnection(u, tr)
	}

	metrics.SetActiveSubscriptions(len(desired))
	return nil
}

// startConnection launches the per-instance connect/register/read loop with
// capped backoff. An offline instance keeps this loop running slowly: never a
// crash, never Error spam (ADR-0058).
func (m *Manager) startConnection(instanceURL string, tr *trackedInstance) {
	ctx, cancel := context.WithCancel(m.ctx)
	cs := &connState{cancel: cancel, doneCh: make(chan struct{})}

	m.mu.Lock()
	m.conns[instanceURL] = cs
	m.mu.Unlock()

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		defer close(cs.doneCh)
		defer func() {
			m.mu.Lock()
			delete(m.conns, instanceURL)
			m.mu.Unlock()
		}()
		m.runLoop(ctx, instanceURL, tr)
	}()
}

// runLoop is the connect/register/read cycle for one instance.
func (m *Manager) runLoop(ctx context.Context, instanceURL string, tr *trackedInstance) {
	attempt := 0
	for {
		if ctx.Err() != nil {
			return
		}

		token, err := m.register(instanceURL)
		if err != nil {
			attempt++
			wait := nextBackoff(attempt - 1)
			m.logRetry(instanceURL, attempt, wait, err)
			m.publishStatus(instanceURL, "offline", err.Error(), wait)
			_ = m.repo.SetSourceActive(m.ctx, instanceURL, false)
			if !m.sleepCtx(ctx, wait) {
				return
			}
			continue
		}

		client := m.buildClient(instanceURL, token)
		m.mu.Lock()
		m.clients[instanceURL] = client
		m.mu.Unlock()

		if err := client.Connect(token); err != nil {
			attempt++
			wait := nextBackoff(attempt - 1)
			m.logRetry(instanceURL, attempt, wait, err)
			m.publishStatus(instanceURL, "reconnecting", err.Error(), wait)
			if !m.sleepCtx(ctx, wait) {
				return
			}
			continue
		}

		metrics.SetSocketConnected(instanceURL, true)
		m.publishStatus(instanceURL, "connected", "", 0)
		_ = m.repo.SetSourceActive(m.ctx, instanceURL, true)
		attempt = 0

		m.logger.Info("Connected to Owncast instance",
			zap.String("instance", instanceURL),
			zap.Int("overlays", len(tr.OverlayIDs)),
		)

		client.Run()
		// Read loop ended: socket dropped or stop requested.
		metrics.SetSocketConnected(instanceURL, false)
		if ctx.Err() != nil {
			return
		}
		// Fast reconnect on normal socket drop (1 short retry latch), but
		// still respect backoff if it keeps failing.
		attempt = 1
		wait := nextBackoff(0)
		m.publishStatus(instanceURL, "reconnecting", "socket closed", wait)
		if !m.sleepCtx(ctx, wait) {
			return
		}
	}
}

// register POSTs /api/chat/register (unauthenticated, per Owncast's anonymous
// chat registration) to obtain the access token used on /ws.
func (m *Manager) register(instanceURL string) (string, error) {
	endpoint := strings.TrimSuffix(instanceURL, "/") + "/api/chat/register"
	req, err := http.NewRequestWithContext(m.ctx, http.MethodPost, endpoint, strings.NewReader("{}"))
	if err != nil {
		return "", fmt.Errorf("register request invalid: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := m.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("register call failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("register returned %d: %s", resp.StatusCode, string(body))
	}

	var payload struct {
		AccessToken string `json:"accessToken"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&payload); err != nil {
		return "", fmt.Errorf("register response undecodable: %w", err)
	}
	if payload.AccessToken == "" {
		return "", fmt.Errorf("register response missing accessToken")
	}
	return payload.AccessToken, nil
}

// logRetry logs an offline instance at Warn (or Debug after several tries) so
// a permanently-down instance never spams Error.
func (m *Manager) logRetry(instanceURL string, attempt int, wait time.Duration, err error) {
	fields := []zap.Field{
		zap.String("instance", instanceURL),
		zap.Int("attempt", attempt),
		zap.Duration("retry_in", wait),
		zap.Error(err),
	}
	if attempt <= 2 {
		m.logger.Warn("Owncast instance unreachable, retrying", fields...)
	} else {
		m.logger.Debug("Owncast instance still unreachable", fields...)
	}
}

func (m *Manager) publishStatus(instanceURL, st, errMsg string, nextRetry time.Duration) {
	if m.statusPub == nil {
		return
	}
	msg := status.Message{
		Platform:    "owncast",
		ChannelID:   instanceURL,
		Status:      st,
		NextRetryAt: nil,
	}
	if errMsg != "" {
		msg.ErrorMessage = errMsg
	}
	if nextRetry > 0 {
		t := time.Now().Add(nextRetry)
		msg.NextRetryAt = &t
	}
	m.statusPub.Publish(m.ctx, msg)
}

func (m *Manager) buildClient(instanceURL, token string) *websocket.Client {
	return websocket.NewClient(instanceURL, token, m.handleMessage, m.logger)
}

// handleMessage publishes one raw CHAT event per overlay target.
func (m *Manager) handleMessage(instanceURL string, msg *websocket.OwncastChatMessage) {
	m.mu.Lock()
	overlays := m.overlays[instanceURL]
	m.mu.Unlock()
	if len(overlays) == 0 {
		return
	}

	rawJSON, err := json.Marshal(msg)
	if err != nil {
		metrics.IncDropped("marshal_error")
		return
	}

	ts := time.Now().UTC()
	if msg.Timestamp != "" {
		if parsed, perr := time.Parse(time.RFC3339Nano, msg.Timestamp); perr == nil {
			ts = parsed
		}
	}

	for _, overlayID := range overlays {
		publishCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		raw := &publisher.RawMessage{
			MessageID:   msg.ID,
			Platform:    "owncast",
			OverlayID:   overlayID,
			ChannelID:   instanceURL,
			ChannelName: instanceURL,
			UserID:      msg.User.ID,
			Username:    msg.User.DisplayName,
			Text:        msg.Body,
			Tags: map[string]string{
				"instance_url":  instanceURL,
				"display_color": fmt.Sprintf("%d", msg.User.DisplayColor),
				"visible":       fmt.Sprintf("%t", msg.Visible),
			},
			RawMessage: rawJSON,
			Timestamp:  ts,
		}
		if err := m.publisher.Publish(publishCtx, raw); err != nil {
			m.logger.Error("Failed to publish owncast message",
				zap.String("instance", instanceURL),
				zap.String("overlay_id", overlayID),
				zap.Error(err),
			)
			metrics.IncDropped("publish_error")
		} else {
			metrics.IncMessage("published", "success")
		}
		cancel()
	}
}

func (m *Manager) sleepCtx(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// NormalizeInstanceURL validates and normalizes an Owncast base URL.
// Used by overlay-manager (mirror of its own validation) and by the sync path
// as defence in depth against a legacy row that never went through validation.
func NormalizeInstanceURL(input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", fmt.Errorf("instance URL is required")
	}
	if !strings.HasPrefix(input, "http://") && !strings.HasPrefix(input, "https://") {
		return "", fmt.Errorf("instance URL must start with http:// or https://")
	}
	u, err := url.Parse(input)
	if err != nil {
		return "", fmt.Errorf("instance URL is not parseable: %w", err)
	}
	if u.Host == "" {
		return "", fmt.Errorf("instance URL is missing a host")
	}
	if u.Fragment != "" {
		return "", fmt.Errorf("instance URL must not carry a fragment")
	}
	if u.Path != "" && u.Path != "/" {
		return "", fmt.Errorf("instance URL must be a bare base URL (got path %q)", u.Path)
	}
	return u.Scheme + "://" + strings.ToLower(u.Host), nil
}
