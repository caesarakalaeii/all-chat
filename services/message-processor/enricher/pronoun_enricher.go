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

package enricher

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/caesar/all-chat/services/message-processor/models"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	// PronounCacheTTL is how long to cache pronoun lookups (24 hours).
	PronounCacheTTL = 24 * time.Hour

	// PronounCacheKeyPrefix is the Redis key prefix for cached pronoun display text.
	PronounCacheKeyPrefix = "pronoun:"

	// alejoAPIBaseURL is the base URL of the Alejo pronouns API.
	alejoAPIBaseURL = "https://api.pronouns.alejo.io/v1"

	// pronounEmptySentinel is stored when a user has no pronouns set (404 response).
	// Empty string means "no pronouns" — distinguishable from a Redis miss.
	pronounEmptySentinel = ""
)

// alejoPronounDef represents a single pronoun definition from the Alejo API.
type alejoPronounDef struct {
	Name     string `json:"name"`
	Subject  string `json:"subject"`
	Object   string `json:"object"`
	Singular bool   `json:"singular"`
}

// alejoUserResponse is the API response from GET /v1/users/{login}.
type alejoUserResponse struct {
	ChannelID    string  `json:"channel_id"`
	ChannelLogin string  `json:"channel_login"`
	PronounID    string  `json:"pronoun_id"`
	AltPronounID *string `json:"alt_pronoun_id"`
}

// PronounEnricher fetches pronouns from the Alejo API and caches results in Redis.
// It runs after ViewerBadgeEnricher in the CHAT PATH only (never in EVENT PATH).
// On any API error, it silently skips — messages render without pronouns (D-05).
type PronounEnricher struct {
	httpClient  *http.Client
	redisClient *redis.Client
	logger      *zap.Logger
	pronounsMap map[string]alejoPronounDef // loaded once at construction
	baseURL     string                     // Alejo API base URL (overridable in tests)
}

// NewPronounEnricher constructs a PronounEnricher.
// It fetches the pronoun definitions map from the Alejo API at startup.
// If the fetch fails, it logs a warning and uses an empty map (graceful degradation).
func NewPronounEnricher(redisClient *redis.Client, logger *zap.Logger) *PronounEnricher {
	e := &PronounEnricher{
		httpClient: &http.Client{
			Timeout: 3 * time.Second,
		},
		redisClient: redisClient,
		logger:      logger,
		pronounsMap: map[string]alejoPronounDef{},
		baseURL:     alejoAPIBaseURL,
	}
	e.pronounsMap = e.fetchPronounsMap(context.Background())
	return e
}

// newPronounEnricherWithURL is an internal constructor for testing.
// It accepts a custom baseURL and pre-loaded pronounsMap to avoid network calls in unit tests.
func newPronounEnricherWithURL(redisClient *redis.Client, logger *zap.Logger, baseURL string, pronounsMap map[string]alejoPronounDef) *PronounEnricher {
	return &PronounEnricher{
		httpClient: &http.Client{
			Timeout: 3 * time.Second,
		},
		redisClient: redisClient,
		logger:      logger,
		pronounsMap: pronounsMap,
		baseURL:     baseURL,
	}
}

// fetchPronounsMap fetches the pronoun definitions from the Alejo API.
// Returns an empty map on error (graceful degradation).
func (e *PronounEnricher) fetchPronounsMap(ctx context.Context) map[string]alejoPronounDef {
	url := fmt.Sprintf("%s/pronouns", e.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		e.logger.Warn("PronounEnricher: failed to build pronouns map request", zap.Error(err))
		return map[string]alejoPronounDef{}
	}

	resp, err := e.httpClient.Do(req)
	if err != nil {
		e.logger.Warn("PronounEnricher: failed to fetch pronouns map from Alejo API — using empty map",
			zap.String("url", url),
			zap.Error(err),
		)
		return map[string]alejoPronounDef{}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		e.logger.Warn("PronounEnricher: unexpected status fetching pronouns map",
			zap.Int("status", resp.StatusCode),
		)
		return map[string]alejoPronounDef{}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		e.logger.Warn("PronounEnricher: failed to read pronouns map body", zap.Error(err))
		return map[string]alejoPronounDef{}
	}

	var pronounsMap map[string]alejoPronounDef
	if err := json.Unmarshal(body, &pronounsMap); err != nil {
		e.logger.Warn("PronounEnricher: failed to parse pronouns map", zap.Error(err))
		return map[string]alejoPronounDef{}
	}

	e.logger.Info("PronounEnricher: loaded pronouns map", zap.Int("count", len(pronounsMap)))
	return pronounsMap
}

// Enrich looks up the Twitch user's pronouns from the Alejo API, caches the result,
// and sets msg.User.Pronouns if found. Returns nil on all soft failures (D-05).
func (e *PronounEnricher) Enrich(ctx context.Context, msg *models.UnifiedChatMessage) error {
	// 1. Resolve Twitch username for the lookup.
	//    For Twitch messages: use the message's own username.
	//    For non-Twitch: use the linked TwitchUsername populated by ViewerBadgeEnricher.
	var twitchLogin string
	if msg.Platform == "twitch" {
		twitchLogin = strings.ToLower(msg.User.Username)
	} else {
		twitchLogin = strings.ToLower(msg.User.TwitchUsername)
	}
	if twitchLogin == "" {
		return nil // Non-Twitch viewer without a linked account — silently skip
	}

	cacheKey := PronounCacheKeyPrefix + twitchLogin

	// 2. Check Redis cache.
	cached, err := e.redisClient.Get(ctx, cacheKey).Result()
	if err == nil {
		// Cache hit — empty string means "no pronouns" sentinel
		if cached != "" {
			msg.User.Pronouns = cached
		}
		return nil
	}
	// On redis.Nil (cache miss) fall through to fetch; on any other error also fall through.

	// 3. Fetch from Alejo API.
	userURL := fmt.Sprintf("%s/users/%s", e.baseURL, twitchLogin)
	req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, userURL, nil)
	if reqErr != nil {
		e.logger.Warn("PronounEnricher: failed to build API request",
			zap.String("login", twitchLogin),
			zap.Error(reqErr),
		)
		return nil // D-05: silent skip
	}

	resp, doErr := e.httpClient.Do(req)
	if doErr != nil {
		e.logger.Warn("PronounEnricher: Alejo API request failed — skipping pronouns",
			zap.String("login", twitchLogin),
			zap.Error(doErr),
		)
		return nil // D-05: silent skip
	}
	defer resp.Body.Close()

	// 404 means user has no pronouns set → cache empty sentinel
	if resp.StatusCode == http.StatusNotFound {
		e.redisClient.Set(ctx, cacheKey, pronounEmptySentinel, PronounCacheTTL)
		return nil
	}

	if resp.StatusCode != http.StatusOK {
		e.logger.Warn("PronounEnricher: unexpected Alejo API status",
			zap.String("login", twitchLogin),
			zap.Int("status", resp.StatusCode),
		)
		return nil // D-05: silent skip
	}

	// 4. Parse response.
	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		e.logger.Warn("PronounEnricher: failed to read Alejo API response",
			zap.String("login", twitchLogin),
			zap.Error(readErr),
		)
		return nil
	}

	var userResp alejoUserResponse
	if parseErr := json.Unmarshal(body, &userResp); parseErr != nil {
		e.logger.Warn("PronounEnricher: failed to parse Alejo API response",
			zap.String("login", twitchLogin),
			zap.Error(parseErr),
		)
		return nil
	}

	// 5. Build display text from pronoun IDs.
	displayText := buildDisplayText(userResp.PronounID, userResp.AltPronounID, e.pronounsMap)

	// 6. Cache the display text (even if empty, to avoid future fetches for users with no pronouns).
	e.redisClient.Set(ctx, cacheKey, displayText, PronounCacheTTL)

	// 7. Set on message if non-empty.
	if displayText != "" {
		msg.User.Pronouns = displayText
	}

	return nil
}

// buildDisplayText converts Alejo pronoun IDs into a human-readable display string.
//
// Examples:
//   - primaryID="hehim", altID=nil → "he/him"
//   - primaryID="sheher", altID="theythem" → "she/they"
//   - primaryID="any", altID=nil → "any" (singular, no slash)
func buildDisplayText(primaryID string, altID *string, pronounsMap map[string]alejoPronounDef) string {
	primary, ok := pronounsMap[primaryID]
	if !ok {
		// Unknown ID: return raw ID as fallback
		return primaryID
	}

	primarySubject := strings.ToLower(primary.Subject)

	// Alt pronoun present → combine primary subject + alt subject
	if altID != nil && *altID != "" {
		if alt, altOK := pronounsMap[*altID]; altOK {
			return primarySubject + "/" + strings.ToLower(alt.Subject)
		}
		// Unknown alt: use primary/altID as fallback
		return primarySubject + "/" + strings.ToLower(*altID)
	}

	// Singular pronoun (e.g. "any", "other") → no slash
	if primary.Singular {
		return primarySubject
	}

	// Standard pronoun: subject/object (e.g. "he/him", "she/her", "they/them")
	return primarySubject + "/" + strings.ToLower(primary.Object)
}
