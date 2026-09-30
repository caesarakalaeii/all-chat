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

// Package subscribers polls a channel's recent subscribers while its stream is
// live and announces new ones ("UserX just subscribed").
//
// Design constraints this package exists to uphold:
//   - Zero shared state with the chat poller. A subscriber poll failing never
//     marks a stream offline, never touches continuation state, and never fires
//     the canary alerts. This loop's only outputs are chat:raw events and its
//     own metrics.
//   - Quota is spent only while live, and learned down on quiet channels: the
//     interval decays 60s -> 2m -> 5m -> 15m -> 30m while no new subscribers
//     appear, and resets on the first new one.
//   - Tokens are read via shared/youtubetoken.ResolveByChannel (channel id, no
//     user context). Routine freshness is owned by the scheduled
//     token-refresh service; this loop only refreshes proactively when the
//     credential is within leadTime of expiry and reactively once on a 401.
//     An invalid_grant opts the channel out for the rest of the stream — the
//     refresh service's permanently-failed marking and re-auth alert own the
//     recovery.
package subscribers

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/caesar/all-chat/shared/youtubetoken"
)

// Announcer publishes a subscriber event to chat:raw. *publisher.StreamPublisher
// satisfies it; the interface keeps this package unit-testable.
type Announcer interface {
	AnnounceSubscriber(ctx context.Context, ev Event) error
}

// Event is one new public subscriber, ready to announce.
type Event struct {
	ChannelID string
	StreamID  string
	Title     string // subscriber display name
	UserID    string // subscriber channel id
	AvatarURL string
	SubAt     time.Time // snippet.publishedAt
}

// PollIntervalDecay is the ladder a quiet channel walks down. Index 0 is the
// armed (active) interval; the last entry is the cap.
var PollIntervalDecay = []time.Duration{
	60 * time.Second,
	2 * time.Minute,
	5 * time.Minute,
	15 * time.Minute,
	30 * time.Minute,
}

// Poller polls one channel's recent subscribers while its stream is live.
type Poller struct {
	channelID string
	videoID   string

	tokens   TokenSource
	api      SubscriberAPI
	quota    QuotaReserver
	announce Announcer
	logger   *zap.Logger
	metrics  *Metrics

	// stateMu guards watermark, seeding and interval; the loop is single-goroutine
	// but Stop can be called from another goroutine.
	stateMu    sync.Mutex
	watermark  time.Time // newest publishedAt already seen/announced (zero value: unset)
	seeded     bool      // baseline committed: seed ok, or first successful poll after retry
	decayIndex int

	// tokenFailures records a dead credential so the loop stops retrying it
	// for the rest of the stream. Set once, read-only afterwards.
	tokenDead bool

	leadTime time.Duration // refresh lead, overridable in tests
}

// TokenSource resolves and refreshes the channel's YouTube credential.
// *youtubetoken.YouTubeSource satisfies it.
type TokenSource interface {
	ResolveByChannel(ctx context.Context, channelID string) (*youtubetoken.YouTubeCredential, error)
	Refresh(ctx context.Context, cred *youtubetoken.YouTubeCredential) error
}

// SubscriberAPI lists recent subscribers. *Client (client.go) satisfies it.
type SubscriberAPI interface {
	ListRecentSubscribers(ctx context.Context, accessToken string) ([]Subscriber, error)
}

// Subscriber is one entry of subscriptions.list?myRecentSubscribers=true.
type Subscriber struct {
	SubscriptionID string    // subscription resource id (informational)
	Title          string    // subscriberSnippet.title
	ChannelID      string    // subscriberSnippet.channelId
	AvatarURL      string    // subscriberSnippet.thumbnails.default.url
	PublishedAt    time.Time // snippet.publishedAt
}

// QuotaReserver reserves/confirms/rolls back official API quota.
// *shared/quota.Reserver satisfies it.
type QuotaReserver interface {
	Reserve(ctx context.Context, units int) (bool, error)
	Confirm(ctx context.Context, units int) error
	Rollback(ctx context.Context, units int) error
}

// PollerOptions configures a Poller.
type PollerOptions struct {
	VideoID  string
	LeadTime time.Duration // proactive-refresh lead; default 5m
}

// NewPoller builds a subscriber poller for one channel's live stream.
func NewPoller(
	channelID string,
	tokens TokenSource,
	api SubscriberAPI,
	quota QuotaReserver,
	announce Announcer,
	logger *zap.Logger,
	metrics *Metrics,
	opts PollerOptions,
) *Poller {
	p := &Poller{
		channelID: channelID,
		videoID:   opts.VideoID,
		tokens:    tokens,
		api:       api,
		quota:     quota,
		announce:  announce,
		logger:    logger,
		metrics:   metrics,
		leadTime:  opts.LeadTime,
	}
	if p.leadTime == 0 {
		p.leadTime = 5 * time.Minute
	}
	if p.metrics == nil {
		p.metrics = NopMetrics()
	}
	return p
}

// Run polls until ctx is cancelled. It is the caller's job to start it when a
// stream goes live and cancel it when the stream ends.
func (p *Poller) Run(ctx context.Context) {
	// First poll is a silent baseline: seed the watermark from the newest
	// subscriber without announcing, so a stream start does not replay the
	// last hour of subscribers.
	p.seedWatermark(ctx)

	for {
		interval := p.currentInterval()

		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}

		p.pollOnce(ctx)
	}
}

// pollOnce runs one poll cycle: resolve token (refresh reactively if 401),
// reserve quota, list subscribers, announce those newer than the watermark,
// advance the watermark, then decay or re-arm the interval.
func (p *Poller) pollOnce(ctx context.Context) {
	if p.tokenDead {
		return
	}

	token, err := p.accessToken(ctx)
	if err != nil {
		p.metrics.TokenErrors.WithLabelValues(p.channelID, errorClass(err)).Inc()
		p.logger.Warn("subscriber poll token unavailable", zap.String("channel_id", p.channelID), zap.Error(err))
		return
	}

	// Quota reserve-confirm-rollback (ADR-0006): one list call = 1 unit.
	ok, err := p.quota.Reserve(ctx, 1)
	if err != nil || !ok {
		p.metrics.QuotaSkips.WithLabelValues(p.channelID).Inc()
		return
	}

	subs, err := p.api.ListRecentSubscribers(ctx, token)
	p.metrics.Polls.WithLabelValues(p.channelID).Inc()
	if err != nil {
		_ = p.quota.Rollback(ctx, 1)
		if !errors.Is(err, ErrUnauthorized) {
			p.metrics.APIErrors.WithLabelValues(p.channelID, errorClass(err)).Inc()
			// An errored poll decays like a quiet one: repeated failures back
			// the channel down the ladder instead of hammering the API.
			p.decay()
			return
		}

		// A 401 means the access token expired between resolve and call:
		// refresh once, reserve quota for the retry, and retry once.
		if refreshErr := p.refreshToken(ctx); refreshErr != nil {
			p.metrics.TokenErrors.WithLabelValues(p.channelID, errorClass(refreshErr)).Inc()
			if isInvalidGrant(refreshErr) {
				p.tokenDead = true
				p.logger.Warn("subscriber credential revoked; disabling subscriber polling for this stream",
					zap.String("channel_id", p.channelID))
			}
			return
		}
		if token, err = p.accessToken(ctx); err != nil {
			return
		}
		if ok, err := p.quota.Reserve(ctx, 1); err != nil || !ok {
			p.metrics.QuotaSkips.WithLabelValues(p.channelID).Inc()
			// No reservation held for the retry: skip the call and drop the
			// fetched data; nothing rolls back, nothing confirms.
			return
		}
		subs, err = p.api.ListRecentSubscribers(ctx, token)
		if err != nil {
			_ = p.quota.Rollback(ctx, 1)
			p.metrics.APIErrors.WithLabelValues(p.channelID, errorClass(err)).Inc()
			return
		}
	}
	if err := p.quota.Confirm(ctx, 1); err != nil {
		// The call happened; a confirm failure must not re-announce.
		p.logger.Warn("quota confirm failed", zap.String("channel_id", p.channelID), zap.Error(err))
	}

	// Seeding gate: a stream start must never announce a backlog. When the
	// silent seed succeeded the watermark is armed; when a seed failed and this
	// first successful poll becomes the baseline instead, it commits the
	// watermark without announcing — otherwise a seed failure would turn into a
	// burst of historical subscribers on the next healthy poll.
	p.stateMu.Lock()
	seeded := p.seeded
	p.stateMu.Unlock()
	if !seeded {
		var newest time.Time
		for _, s := range subs {
			if s.PublishedAt.After(newest) {
				newest = s.PublishedAt
			}
		}
		p.stateMu.Lock()
		if newest.After(p.watermark) {
			p.watermark = newest
		}
		p.seeded = true
		p.stateMu.Unlock()
		p.decay()
		return
	}

	if len(subs) == 0 {
		p.decay()
		return
	}

	newest, announced := p.processSubscribers(ctx, subs)
	if announced > 0 {
		p.rearm()
		p.metrics.SubscribersAnnounced.WithLabelValues(p.channelID).Add(float64(announced))
	} else {
		p.decay()
	}
	_ = newest
}

// processSubscribers announces subscribers newer than the watermark and returns
// the newest publishedAt seen (even when nothing was announced) and the count
// announced.
func (p *Poller) processSubscribers(ctx context.Context, subs []Subscriber) (time.Time, int) {
	p.stateMu.Lock()
	watermark := p.watermark
	p.stateMu.Unlock()

	var newest time.Time
	announced := 0
	for _, s := range subs {
		if s.PublishedAt.After(newest) {
			newest = s.PublishedAt
		}
		// Only announce items strictly newer than the watermark.
		if !s.PublishedAt.After(watermark) {
			continue
		}
		if err := p.announce.AnnounceSubscriber(ctx, Event{
			ChannelID: p.channelID,
			StreamID:  p.videoID,
			Title:     s.Title,
			UserID:    s.ChannelID,
			AvatarURL: s.AvatarURL,
			SubAt:     s.PublishedAt,
		}); err != nil {
			p.metrics.AnnounceErrors.WithLabelValues(p.channelID).Inc()
			p.logger.Warn("failed to announce subscriber", zap.String("channel_id", p.channelID), zap.Error(err))
			continue
		}
		announced++
	}

	// Advance the watermark to the newest seen, announced or not — items older
	// than the watermark must never re-announce.
	if newest.After(watermark) {
		p.stateMu.Lock()
		if newest.After(p.watermark) {
			p.watermark = newest
		}
		p.stateMu.Unlock()
	}
	return newest, announced
}

// seedWatermark performs the silent baseline poll at stream start.
func (p *Poller) seedWatermark(ctx context.Context) {
	token, err := p.accessToken(ctx)
	if err != nil {
		p.metrics.TokenErrors.WithLabelValues(p.channelID, errorClass(err)).Inc()
		p.logger.Warn("subscriber baseline poll token unavailable", zap.String("channel_id", p.channelID), zap.Error(err))
		return
	}
	ok, err := p.quota.Reserve(ctx, 1)
	if err != nil || !ok {
		p.metrics.QuotaSkips.WithLabelValues(p.channelID).Inc()
		return
	}
	subs, err := p.api.ListRecentSubscribers(ctx, token)
	if err != nil {
		_ = p.quota.Rollback(ctx, 1)
		p.metrics.APIErrors.WithLabelValues(p.channelID, errorClass(err)).Inc()
		return
	}
	_ = p.quota.Confirm(ctx, 1)

	var newest time.Time
	for _, s := range subs {
		if s.PublishedAt.After(newest) {
			newest = s.PublishedAt
		}
	}
	p.stateMu.Lock()
	p.watermark = newest
	p.seeded = true
	p.stateMu.Unlock()
	p.logger.Debug("subscriber watermark seeded",
		zap.String("channel_id", p.channelID),
		zap.Int("baseline_count", len(subs)))
}

// accessToken returns a usable access token, refreshing proactively when the
// cached credential is within leadTime of expiry. Resolution goes through
// the DB every time — token-refresh-service's 15-minute batches keep that row
// fresh, so a re-resolve is normally all that is needed.
func (p *Poller) accessToken(ctx context.Context) (string, error) {
	cred, err := p.tokens.ResolveByChannel(ctx, p.channelID)
	if err != nil {
		if errors.Is(err, youtubetoken.ErrNoCredential) {
			// No linked token row: this channel never consented. One log line,
			// then dead for the stream — retrying every poll would hammer the DB.
			p.tokenDead = true
			p.logger.Info("no YouTube credential for channel; subscriber alerts off for this stream",
				zap.String("channel_id", p.channelID))
		}
		return "", err
	}
	// A credential with no refresh token can never be renewed: once its
	// access token is within leadTime of expiry it is spent. Retrying every
	// poll would only re-fail the same refresh exchange.
	if cred.RefreshToken == "" && time.Until(cred.ExpiresAt) < p.leadTime {
		p.tokenDead = true
		p.logger.Info("subscriber credential spent (no refresh token, access token expiring); subscriber alerts off for this stream",
			zap.String("channel_id", p.channelID))
		return "", errSpentCredential
	}
	if time.Until(cred.ExpiresAt) < p.leadTime {
		if err := p.refreshToken(ctx); err != nil {
			// Proactive refresh failure is not fatal: the token may still work
			// (Expiry estimates can be off) — let the API call decide.
			p.logger.Warn("proactive refresh failed; trying current token",
				zap.String("channel_id", p.channelID), zap.Error(err))
			return cred.AccessToken, nil
		}
		cred, err = p.tokens.ResolveByChannel(ctx, p.channelID)
		if err != nil {
			return "", err
		}
	}
	return cred.AccessToken, nil
}

// refreshToken performs one reactive refresh via the shared source. This is
// the ONLY token write this service ever performs, and it goes through the
// shared path that preserves granted_scopes and the refresh token.
func (p *Poller) refreshToken(ctx context.Context) error {
	cred, err := p.tokens.ResolveByChannel(ctx, p.channelID)
	if err != nil {
		return err
	}
	if err := p.tokens.Refresh(ctx, cred); err != nil {
		p.metrics.Refreshes.WithLabelValues(p.channelID, "error").Inc()
		return err
	}
	p.metrics.Refreshes.WithLabelValues(p.channelID, "ok").Inc()
	return nil
}

// currentInterval returns the current poll interval.
func (p *Poller) currentInterval() time.Duration {
	p.stateMu.Lock()
	defer p.stateMu.Unlock()
	return PollIntervalDecay[p.decayIndex]
}

// decay steps the interval ladder one rung down (towards the cap).
func (p *Poller) decay() {
	p.stateMu.Lock()
	defer p.stateMu.Unlock()
	if p.decayIndex < len(PollIntervalDecay)-1 {
		p.decayIndex++
	}
}

// rearm resets the interval to the armed (fastest) rung.
func (p *Poller) rearm() {
	p.stateMu.Lock()
	defer p.stateMu.Unlock()
	p.decayIndex = 0
}

// errorClass buckets an error for metrics labels.
func errorClass(err error) string {
	switch {
	case errors.Is(err, ErrUnauthorized):
		return "unauthorized"
	case errors.Is(err, youtubetoken.ErrNoCredential):
		return "no_credential"
	case errors.Is(err, errSpentCredential):
		return "spent_credential"
	case isInvalidGrant(err):
		return "invalid_grant"
	case err != nil && strings.Contains(err.Error(), "429"):
		return "rate_limited"
	default:
		return "other"
	}
}

// isInvalidGrant reports whether err is Google's non-retryable revocation.
func isInvalidGrant(err error) bool {
	return err != nil && strings.Contains(err.Error(), "invalid_grant")
}

// ErrUnauthorized is returned by SubscriberAPI implementations when the
// platform answers 401.
var ErrUnauthorized = errors.New("subscribers: access token unauthorized")

// errSpentCredential marks a resolved credential whose access token is expiring
// with no refresh token to renew it. Not retryable for the rest of the stream.
var errSpentCredential = errors.New("subscribers: credential spent (no refresh token, access token expiring)")
