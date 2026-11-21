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

// Package kickevents receives Kick's official webhook events (ADR-0094). The
// realtime feed the listener reads has no follower names and no KICKs gifts
// on the chatroom channels, so those two come from here: the service keeps a
// channel.followed and a kicks.gifted subscription for every Kick source, and
// publishes each verified delivery as a kick_follow or kick_kicks raw message.
package kickevents

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/caesar/all-chat/services/kick-listener/channels"
	"github.com/caesar/all-chat/services/kick-listener/publisher"
)

const (
	// DefaultTokenURL and DefaultAPIURL are Kick's production endpoints.
	DefaultTokenURL = "https://id.kick.com/oauth/token"
	DefaultAPIURL   = "https://api.kick.com"

	eventFollowed = "channel.followed"
	eventKicks    = "kicks.gifted"

	// ReconcileInterval is how often subscriptions are compared with the
	// sources. Kick drops a subscription whose deliveries fail for a day, so
	// this is also what brings one back after an outage.
	ReconcileInterval = 10 * time.Minute

	maxBodyBytes  = 1 << 20
	maxClockSkew  = 10 * time.Minute
	seenTTL       = time.Hour // longer than maxClockSkew, so a replay is refused by one or the other
	targetsTTL    = time.Minute
	slugsPerQuery = 50 // Kick's limit on GET /public/v1/channels

	// DefaultPace is the pause between subscription requests. Kick answered
	// 429 to a burst of about five a second on 2026-11-20; it publishes no
	// limit, so this stays far below that.
	DefaultPace = time.Second
	// maxRequestsPerPass keeps a paced pass inside the reconcile lock; the
	// next pass continues where this one stopped.
	maxRequestsPerPass = 400
)

// Sources lists the Kick sources of active overlays.
type Sources interface {
	GetActiveChannels(ctx context.Context) ([]*channels.ActiveChannel, error)
}

// Store is the shared state the pods need: a set-if-absent with expiry.
type Store interface {
	SetNX(ctx context.Context, key string, ttl time.Duration) (bool, error)
}

// Publisher writes raw messages to chat:raw.
type Publisher interface {
	Publish(ctx context.Context, msg *publisher.RawMessage) error
}

// Service is the webhook receiver and the subscription reconciler.
type Service struct {
	ClientID, ClientSecret string
	TokenURL, APIURL       string
	Sources                Sources
	Store                  Store
	Publisher              Publisher
	HTTP                   *http.Client
	Logger                 *zap.Logger
	// Pace is the pause between subscription requests; zero means none.
	Pace time.Duration

	mu          sync.Mutex
	publicKey   *rsa.PublicKey
	keyFetched  time.Time
	token       string
	tokenExpiry time.Time
	targets     map[string][]channels.OverlayTarget // Kick slug -> overlays
	targetsAt   time.Time
}

// kickSlug is the slug Kick's API uses for a stored channel name.
func kickSlug(channel string) string {
	return strings.ReplaceAll(strings.ToLower(channel), "_", "-")
}

// Run reconciles subscriptions until ctx ends.
func (s *Service) Run(ctx context.Context) {
	ticker := time.NewTicker(ReconcileInterval)
	defer ticker.Stop()
	for {
		if err := s.Reconcile(ctx); err != nil {
			s.Logger.Error("Kick event subscription reconcile failed", zap.Error(err))
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// Webhook handles POST /webhooks/kick.
func (s *Service) Webhook(c *gin.Context) {
	body, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, maxBodyBytes))
	if err != nil {
		c.Status(http.StatusRequestEntityTooLarge)
		return
	}
	id := c.GetHeader("Kick-Event-Message-Id")
	timestamp := c.GetHeader("Kick-Event-Message-Timestamp")
	sent, err := time.Parse(time.RFC3339, timestamp)
	if id == "" || err != nil || time.Since(sent).Abs() > maxClockSkew ||
		!s.verified(c.Request.Context(), id, timestamp, body, c.GetHeader("Kick-Event-Signature")) {
		c.Status(http.StatusUnauthorized)
		return
	}

	// Every verified delivery is acknowledged, whatever comes of it: Kick
	// drops a subscription whose deliveries keep failing.
	c.Status(http.StatusOK)
	first, err := s.Store.SetNX(c.Request.Context(), "kick:events:seen:"+id, seenTTL)
	if err != nil {
		s.Logger.Warn("Kick webhook dedupe unavailable, delivering anyway", zap.Error(err))
	} else if !first {
		return
	}
	if err := s.publish(c.Request.Context(), c.GetHeader("Kick-Event-Type"), id, body); err != nil {
		s.Logger.Error("Kick webhook event not published", zap.String("message_id", id), zap.Error(err))
	}
}

type kickUser struct {
	UserID      int64  `json:"user_id"`
	Username    string `json:"username"`
	ChannelSlug string `json:"channel_slug"`
}

func (s *Service) publish(ctx context.Context, eventType, id string, body []byte) error {
	var p struct {
		Broadcaster kickUser `json:"broadcaster"`
		Follower    kickUser `json:"follower"`
		Sender      kickUser `json:"sender"`
		Gift        struct {
			Amount  int    `json:"amount"`
			Name    string `json:"name"`
			Message string `json:"message"`
		} `json:"gift"`
	}
	if err := json.Unmarshal(body, &p); err != nil {
		return fmt.Errorf("decode %s: %w", eventType, err)
	}

	msg := publisher.RawMessage{MessageID: "kick-webhook-" + id, Platform: "kick", Timestamp: time.Now().UTC()}
	actor := p.Follower
	switch eventType {
	case eventFollowed:
		msg.EventType = "kick_follow"
	case eventKicks:
		actor = p.Sender
		msg.EventType, msg.Text = "kick_kicks", p.Gift.Message
		msg.EventData = map[string]any{"kicks": p.Gift.Amount, "gift_name": p.Gift.Name}
	default:
		return nil
	}
	if actor.Username == "" {
		return nil // an anonymous gift names nobody to thank
	}
	msg.Username, msg.UserID = actor.Username, strconv.FormatInt(actor.UserID, 10)

	targets, err := s.overlayTargets(ctx, p.Broadcaster.ChannelSlug)
	if err != nil {
		return err
	}
	for _, target := range targets {
		out := msg
		out.OverlayID, out.ChannelID, out.ChannelName = target.OverlayID, target.ChannelSlug, target.ChannelSlug
		if err := s.Publisher.Publish(ctx, &out); err != nil {
			return err
		}
	}
	return nil
}

// overlayTargets are the overlays reading a Kick channel, from the database:
// a delivery reaches any pod, not the one that leads the channel.
func (s *Service) overlayTargets(ctx context.Context, slug string) ([]channels.OverlayTarget, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.targets == nil || time.Since(s.targetsAt) > targetsTTL {
		sources, err := s.Sources.GetActiveChannels(ctx)
		if err != nil {
			return nil, err
		}
		s.targets = map[string][]channels.OverlayTarget{}
		for _, ch := range sources {
			key := kickSlug(ch.ChannelSlug)
			s.targets[key] = append(s.targets[key], channels.OverlayTarget{OverlayID: ch.OverlayID, ChannelSlug: ch.ChannelSlug})
		}
		s.targetsAt = time.Now()
	}
	return s.targets[kickSlug(slug)], nil
}

// verified reports whether Kick signed this delivery: RSA PKCS#1 v1.5 over
// SHA-256 of "<message id>.<timestamp>.<body>". Kick may rotate its key at any
// time, so a failure refetches the key once (at most once a minute).
func (s *Service) verified(ctx context.Context, id, timestamp string, body []byte, signature string) bool {
	sig, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return false
	}
	sum := sha256.Sum256(bytes.Join([][]byte{[]byte(id), []byte(timestamp), body}, []byte(".")))
	for _, refresh := range []bool{false, true} {
		key, err := s.kickPublicKey(ctx, refresh)
		if err != nil {
			s.Logger.Warn("Kick public key unavailable", zap.Error(err))
			return false
		}
		if rsa.VerifyPKCS1v15(key, crypto.SHA256, sum[:], sig) == nil {
			return true
		}
	}
	return false
}

func (s *Service) kickPublicKey(ctx context.Context, refresh bool) (*rsa.PublicKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.publicKey != nil && (!refresh || time.Since(s.keyFetched) < time.Minute) {
		return s.publicKey, nil
	}
	var out struct {
		Data struct {
			PublicKey string `json:"public_key"`
		} `json:"data"`
	}
	if err := s.call(ctx, http.MethodGet, "/public/v1/public-key", "", nil, &out); err != nil {
		return nil, err
	}
	block, _ := pem.Decode([]byte(out.Data.PublicKey))
	if block == nil {
		return nil, errors.New("kick public key is not PEM")
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	key, ok := parsed.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("kick public key is not RSA")
	}
	s.publicKey, s.keyFetched = key, time.Now()
	return key, nil
}

// Reconcile makes the app's subscriptions match the Kick sources: a
// channel.followed and a kicks.gifted subscription per channel, none for a
// channel that is no longer a source. One pod per interval does the work.
func (s *Service) Reconcile(ctx context.Context) error {
	if mine, err := s.Store.SetNX(ctx, "kick:events:reconcile", ReconcileInterval-time.Minute); err != nil || !mine {
		return err
	}
	sources, err := s.Sources.GetActiveChannels(ctx)
	if err != nil {
		return err
	}
	slugs := map[string]bool{}
	for _, ch := range sources {
		slugs[kickSlug(ch.ChannelSlug)] = true
	}
	token, err := s.appToken(ctx)
	if err != nil {
		return err
	}
	wanted, err := s.broadcasterIDs(ctx, token, slugs)
	if err != nil {
		return err
	}

	var existing struct {
		Data []struct {
			ID                string `json:"id"`
			BroadcasterUserID int64  `json:"broadcaster_user_id"`
			Event             string `json:"event"`
		} `json:"data"`
	}
	if err := s.call(ctx, http.MethodGet, "/public/v1/events/subscriptions", token, nil, &existing); err != nil {
		return err
	}
	have := map[int64]map[string]bool{}
	stale := url.Values{}
	for _, sub := range existing.Data {
		if !wanted[sub.BroadcasterUserID] {
			stale.Add("id", sub.ID)
			continue
		}
		if have[sub.BroadcasterUserID] == nil {
			have[sub.BroadcasterUserID] = map[string]bool{}
		}
		have[sub.BroadcasterUserID][sub.Event] = true
	}

	type event struct {
		Name    string `json:"name"`
		Version int    `json:"version"`
	}
	added, refused, requests := 0, 0, 0
	for broadcaster := range wanted {
		var missing []event
		for _, name := range []string{eventFollowed, eventKicks} {
			if !have[broadcaster][name] {
				missing = append(missing, event{name, 1})
			}
		}
		if len(missing) == 0 {
			continue
		}
		if requests == maxRequestsPerPass {
			break
		}
		requests++
		body := map[string]any{"broadcaster_user_id": broadcaster, "method": "webhook", "events": missing}
		var result struct {
			Data []struct {
				Name  string `json:"name"`
				Error string `json:"error"`
			} `json:"data"`
		}
		if requests > 1 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(s.Pace):
			}
		}
		if err := s.call(ctx, http.MethodPost, "/public/v1/events/subscriptions", token, body, &result); err != nil {
			var status statusError
			if errors.As(err, &status) && status.code == http.StatusTooManyRequests {
				// More requests would only be refused too. The next pass
				// requests what is still missing.
				s.Logger.Warn("Kick rate limit reached, rest of the subscriptions wait for the next pass",
					zap.Int("requested", added))
				break
			}
			refused++
			s.Logger.Warn("Kick event subscription refused", zap.Int64("broadcaster_user_id", broadcaster), zap.Error(err))
			continue
		}
		for _, item := range result.Data {
			if item.Error != "" {
				s.Logger.Warn("Kick event subscription refused", zap.Int64("broadcaster_user_id", broadcaster),
					zap.String("event", item.Name), zap.String("error", item.Error))
			}
		}
		added += len(missing)
	}
	if len(stale) > 0 {
		if err := s.call(ctx, http.MethodDelete, "/public/v1/events/subscriptions?"+stale.Encode(), token, nil, nil); err != nil {
			s.Logger.Warn("Kick event subscription cleanup failed", zap.Error(err))
		}
	}
	s.Logger.Info("Kick event subscriptions reconciled", zap.Int("channels", len(wanted)),
		zap.Int("requested", added), zap.Int("removed", len(stale["id"])))
	return nil
}

// broadcasterIDs resolves channel slugs to Kick's broadcaster user ids.
func (s *Service) broadcasterIDs(ctx context.Context, token string, slugs map[string]bool) (map[int64]bool, error) {
	ids := map[int64]bool{}
	batch := url.Values{}
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		var out struct {
			Data []struct {
				BroadcasterUserID int64 `json:"broadcaster_user_id"`
			} `json:"data"`
		}
		if err := s.call(ctx, http.MethodGet, "/public/v1/channels?"+batch.Encode(), token, nil, &out); err != nil {
			return err
		}
		for _, ch := range out.Data {
			ids[ch.BroadcasterUserID] = true
		}
		batch = url.Values{}
		return nil
	}
	for slug := range slugs {
		batch.Add("slug", slug)
		if len(batch["slug"]) == slugsPerQuery {
			if err := flush(); err != nil {
				return nil, err
			}
		}
	}
	return ids, flush()
}

// appToken returns the app access token (client_credentials), cached until
// shortly before it expires.
func (s *Service) appToken(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.token != "" && time.Now().Before(s.tokenExpiry) {
		return s.token, nil
	}
	form := url.Values{"grant_type": {"client_credentials"}, "client_id": {s.ClientID}, "client_secret": {s.ClientSecret}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	var out struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := s.do(req, &out); err != nil {
		return "", fmt.Errorf("kick app token: %w", err)
	}
	if out.AccessToken == "" {
		return "", errors.New("kick app token: empty access_token")
	}
	s.token, s.tokenExpiry = out.AccessToken, time.Now().Add(time.Duration(out.ExpiresIn)*time.Second-time.Minute)
	return s.token, nil
}

func (s *Service) call(ctx context.Context, method, path, token string, body, out any) error {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, s.APIURL+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return s.do(req, out)
}

// statusError is a non-2xx answer from Kick.
type statusError struct {
	code    int
	message string
}

func (e statusError) Error() string { return e.message }

func (s *Service) do(req *http.Request, out any) error {
	resp, err := s.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	payload, _ := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if resp.StatusCode >= 300 {
		return statusError{resp.StatusCode, fmt.Sprintf("%s %s: status %d: %.200s", req.Method, req.URL.Path, resp.StatusCode, payload)}
	}
	if out == nil || len(payload) == 0 {
		return nil
	}
	return json.Unmarshal(payload, out)
}
