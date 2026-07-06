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

// Package announcer posts an opt-in "round started" message to the streamer's chat
// when a poll/prediction opens (issue #523, H4-2). engagement-service has no chat-send
// capability of its own, so it resolves the overlay owner + its sendable source
// platforms and calls auth-service's internal announce endpoint (which reuses the
// tested per-platform streamer-send path) with a short-lived service JWT. Everything
// here is best-effort: a failed or disabled announce never blocks or fails the round.
package announcer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/caesar/all-chat/services/engagement-service/models"
	"github.com/caesar/all-chat/services/engagement-service/repository"
	sharedAuth "github.com/caesar/all-chat/shared/auth"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// sendablePlatforms are the platforms auth-service can post a streamer message to.
// Discord/TikTok have no streamer-send path, so a round on a Discord/TikTok-only
// overlay simply isn't announced (viewers there still see the overlay + participate page).
var sendablePlatforms = map[string]bool{"twitch": true, "youtube": true, "kick": true}

const (
	maxAnnounceLen  = 500 // matches auth-service maxMessageLength
	announceTimeout = 8 * time.Second
)

// store is the slice of the repository the announcer needs.
type store interface {
	GetEarnConfig(ctx context.Context, overlayID uuid.UUID) (models.EarnConfig, error)
	OverlayOwner(ctx context.Context, overlayID uuid.UUID) (string, error)
	SourceChannelsForOverlay(ctx context.Context, overlayID uuid.UUID) ([]repository.ChannelRef, error)
}

// Announcer sends round-start announcements to chat.
type Announcer struct {
	store       store
	http        *http.Client
	authBaseURL string
	svcKeys     *sharedAuth.KeyChain
	publicBase  string
	log         *zap.Logger
	enabled     bool
}

// New builds an Announcer. It is disabled (every Announce* is a no-op) unless both the
// auth-service base URL and the service JWT key chain are configured, so a cluster
// without the announce wiring degrades cleanly instead of erroring on every round.
func New(st store, authBaseURL, publicBase string, svcKeys *sharedAuth.KeyChain, log *zap.Logger) *Announcer {
	enabled := authBaseURL != "" && svcKeys != nil
	if !enabled {
		log.Info("engagement chat announce disabled (AUTH_SERVICE_URL and/or SERVICE_JWT_SECRET_V1 not set)")
	}
	return &Announcer{
		store:       st,
		http:        &http.Client{Timeout: announceTimeout},
		authBaseURL: strings.TrimRight(authBaseURL, "/"),
		svcKeys:     svcKeys,
		publicBase:  strings.TrimRight(publicBase, "/"),
		log:         log,
		enabled:     enabled,
	}
}

// AnnouncePoll announces a newly-opened poll (numbered options + participate link) if
// the overlay opted in. Non-blocking and best-effort.
func (a *Announcer) AnnouncePoll(overlayID uuid.UUID, poll *models.Poll) {
	if a == nil || !a.enabled || poll == nil {
		return
	}
	a.dispatch(overlayID, "📊", poll.Question, optionLines(len(poll.Options), func(i int) (int, string) {
		return poll.Options[i].Idx, poll.Options[i].Label
	}), "Vote in chat (!vote N or just N) or here")
}

// AnnouncePrediction announces a newly-opened prediction if the overlay opted in.
func (a *Announcer) AnnouncePrediction(overlayID uuid.UUID, pred *models.Prediction) {
	if a == nil || !a.enabled || pred == nil {
		return
	}
	a.dispatch(overlayID, "🔮", pred.Title, optionLines(len(pred.Outcomes), func(i int) (int, string) {
		return pred.Outcomes[i].Idx, pred.Outcomes[i].Label
	}), "Predict in chat (!predict N amount) or here")
}

func optionLines(n int, at func(i int) (int, string)) []string {
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		idx, label := at(i)
		out = append(out, fmt.Sprintf("%d. %s", idx, label))
	}
	return out
}

// dispatch runs the announce on a detached context in the background so it never
// blocks or fails the round-create response.
func (a *Announcer) dispatch(overlayID uuid.UUID, icon, title string, options []string, cta string) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), announceTimeout)
		defer cancel()
		if err := a.announce(ctx, overlayID, icon, title, options, cta); err != nil {
			a.log.Warn("engagement chat announce failed", zap.String("overlay", overlayID.String()), zap.Error(err))
		}
	}()
}

func (a *Announcer) announce(ctx context.Context, overlayID uuid.UUID, icon, title string, options []string, cta string) error {
	cfg, err := a.store.GetEarnConfig(ctx, overlayID)
	if err != nil {
		return fmt.Errorf("get earn config: %w", err)
	}
	if !cfg.AnnounceOnStart {
		return nil // overlay didn't opt in
	}
	owner, err := a.store.OverlayOwner(ctx, overlayID)
	if err != nil {
		return fmt.Errorf("resolve overlay owner: %w", err)
	}
	channels, err := a.store.SourceChannelsForOverlay(ctx, overlayID)
	if err != nil {
		return fmt.Errorf("source channels: %w", err)
	}
	platforms := sendableFrom(channels)
	if len(platforms) == 0 {
		return nil // nothing we can post to (e.g. Discord/TikTok-only overlay)
	}
	return a.postAnnounce(ctx, owner, a.buildMessage(icon, title, options, cta, overlayID), platforms)
}

// buildMessage assembles a single-line announcement (platform chats are single-line)
// and keeps it within the send length limit, dropping the inline option list before
// the title/link since the options are already numbered on the overlay + participate page.
func (a *Announcer) buildMessage(icon, title string, options []string, cta string, overlayID uuid.UUID) string {
	url := fmt.Sprintf("%s/overlay/%s/participate", a.publicBase, overlayID)
	tail := cta + ": " + url
	head := strings.TrimSpace(icon + " " + strings.TrimSpace(title))

	msg := head + " — " + strings.Join(options, "  ") + " · " + tail
	if len(msg) <= maxAnnounceLen {
		return msg
	}
	msg = head + " · " + tail // drop inline options
	if len(msg) <= maxAnnounceLen {
		return msg
	}
	// Extreme title length: trim the head (from the end, so the leading icon survives),
	// backing off to a rune boundary so we never emit invalid UTF-8 (a split multibyte
	// rune would be rejected/mangled by the platform send APIs).
	if over := len(msg) - maxAnnounceLen; over < len(head) {
		trimmed := head[:len(head)-over]
		for len(trimmed) > 0 && !utf8.ValidString(trimmed) {
			trimmed = trimmed[:len(trimmed)-1]
		}
		head = strings.TrimSpace(trimmed)
		msg = head + " · " + tail
	}
	return msg
}

func (a *Announcer) postAnnounce(ctx context.Context, userID, message string, platforms []string) error {
	body, err := json.Marshal(announceRequest{UserID: userID, Message: message, Platforms: platforms})
	if err != nil {
		return fmt.Errorf("marshal announce: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.authBaseURL+"/internal/chat/announce", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build announce request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	tok, err := sharedAuth.GenerateServiceJWTWithKid(a.svcKeys.LatestKid(), "engagement-service", string(a.svcKeys.LatestSecret()), 30*time.Second)
	if err != nil {
		return fmt.Errorf("mint service jwt: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+tok)

	resp, err := a.http.Do(req)
	if err != nil {
		return fmt.Errorf("post announce: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("announce endpoint returned %d", resp.StatusCode)
	}
	return nil
}

// sendableFrom returns the unique lowercase platforms among an overlay's source
// channels that auth-service can actually post to.
func sendableFrom(channels []repository.ChannelRef) []string {
	seen := map[string]bool{}
	var out []string
	for _, c := range channels {
		p := strings.ToLower(c.Platform)
		if sendablePlatforms[p] && !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	return out
}

// announceRequest is the body of POST /internal/chat/announce on auth-service.
type announceRequest struct {
	UserID    string   `json:"user_id"`
	Message   string   `json:"message"`
	Platforms []string `json:"platforms"`
}
