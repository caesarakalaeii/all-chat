package handlers

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const (
	twitchBadgeGlobalURL  = "https://api.twitch.tv/helix/chat/badges/global"
	twitchBadgeChannelURL = "https://api.twitch.tv/helix/chat/badges"
	badgeCacheTTL         = 15 * time.Minute
)

type badgeCacheEntry struct {
	data    []byte
	expires time.Time
}

// TwitchBadgeHandler proxies badge requests through our API gateway.
type TwitchBadgeHandler struct {
	httpClient  *http.Client
	log         *zap.Logger
	cacheMux    sync.RWMutex
	cache       map[string]badgeCacheEntry
	clientID    string
	accessToken string
}

// NewTwitchBadgeHandler creates a new badge handler with sensible defaults.
func NewTwitchBadgeHandler(log *zap.Logger, clientID, accessToken string) *TwitchBadgeHandler {
	return &TwitchBadgeHandler{
		httpClient:  &http.Client{Timeout: 10 * time.Second},
		log:         log.Named("twitch-badges"),
		cache:       make(map[string]badgeCacheEntry),
		clientID:    clientID,
		accessToken: accessToken,
	}
}

// GetGlobalBadges proxies the Twitch global badge list.
func (h *TwitchBadgeHandler) GetGlobalBadges(c *gin.Context) {
	h.serveBadges(c, twitchBadgeGlobalURL, "global")
}

// GetChannelBadges proxies the Twitch channel-specific badge list.
func (h *TwitchBadgeHandler) GetChannelBadges(c *gin.Context) {
	roomID := strings.TrimSpace(c.Param("room_id"))
	if roomID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "room_id is required"})
		return
	}

	cacheKey := fmt.Sprintf("channel:%s", roomID)
	url := fmt.Sprintf("%s?broadcaster_id=%s", twitchBadgeChannelURL, roomID)
	h.serveBadges(c, url, cacheKey)
}

func (h *TwitchBadgeHandler) serveBadges(c *gin.Context, url, cacheKey string) {
	payload, err := h.getBadgePayload(url, cacheKey)
	if err != nil {
		h.log.Warn("Failed to fetch Twitch badges", zap.String("url", url), zap.Error(err))
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to fetch Twitch badges"})
		return
	}

	c.Header("Cache-Control", fmt.Sprintf("public, max-age=%d", int(badgeCacheTTL.Seconds())))
	c.Data(http.StatusOK, "application/json", payload)
}

func (h *TwitchBadgeHandler) getBadgePayload(url, cacheKey string) ([]byte, error) {
	if data, ok := h.getFromCache(cacheKey); ok {
		return data, nil
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Client-ID", h.clientID)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", h.accessToken))

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("twitch badge request failed: status=%d body=%s", resp.StatusCode, string(body))
	}

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	h.saveToCache(cacheKey, payload)
	return payload, nil
}

func (h *TwitchBadgeHandler) getFromCache(path string) ([]byte, bool) {
	h.cacheMux.RLock()
	entry, ok := h.cache[path]
	h.cacheMux.RUnlock()

	if !ok || time.Now().After(entry.expires) {
		return nil, false
	}
	return entry.data, true
}

func (h *TwitchBadgeHandler) saveToCache(path string, data []byte) {
	h.cacheMux.Lock()
	h.cache[path] = badgeCacheEntry{
		data:    data,
		expires: time.Now().Add(badgeCacheTTL),
	}
	h.cacheMux.Unlock()
}
