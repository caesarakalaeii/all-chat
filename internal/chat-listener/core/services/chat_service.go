package services

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/caesar/all-chat/internal/chat-listener/core/domain"
	"github.com/caesar/all-chat/internal/chat-listener/core/ports"
	"go.uber.org/zap"
)

// ChatService implements the ChatService interface
type ChatService struct {
	ircClient   ports.IRCClient
	channelRepo ports.ChannelRepository
	emoteClient ports.EmoteClient
	publisher   ports.Publisher
	logger      *zap.Logger

	mu               sync.RWMutex
	activeChannels   map[string][]domain.ActiveChannel // channel -> overlays
	channelEmotes    map[string][]domain.Emote         // channel -> emotes
	emoteCacheTimes  map[string]time.Time              // channel -> last fetch time
	emoteCacheTTL    time.Duration
}

// NewChatService creates a new chat service
func NewChatService(
	ircClient ports.IRCClient,
	channelRepo ports.ChannelRepository,
	emoteClient ports.EmoteClient,
	publisher ports.Publisher,
	logger *zap.Logger,
) *ChatService {
	return &ChatService{
		ircClient:       ircClient,
		channelRepo:     channelRepo,
		emoteClient:     emoteClient,
		publisher:       publisher,
		logger:          logger,
		activeChannels:  make(map[string][]domain.ActiveChannel),
		channelEmotes:   make(map[string][]domain.Emote),
		emoteCacheTimes: make(map[string]time.Time),
		emoteCacheTTL:   15 * time.Minute,
	}
}

// Start begins listening to all active channels
func (s *ChatService) Start(ctx context.Context) error {
	s.logger.Info("Starting chat service")

	// Load active channels
	if err := s.RefreshChannels(ctx); err != nil {
		return fmt.Errorf("failed to load channels: %w", err)
	}

	// Register message handler
	s.ircClient.OnMessage(func(channel, user, message string, tags map[string]string) {
		if err := s.ProcessMessage(context.Background(), channel, map[string]interface{}{
			"channel": channel,
			"user":    user,
			"message": message,
			"tags":    tags,
		}); err != nil {
			s.logger.Error("Failed to process message",
				zap.String("channel", channel),
				zap.Error(err))
		}
	})

	// Connect to IRC
	if err := s.ircClient.Connect(); err != nil {
		return fmt.Errorf("failed to connect to IRC: %w", err)
	}

	s.logger.Info("Chat service started successfully")
	return nil
}

// Stop gracefully shuts down the service
func (s *ChatService) Stop() error {
	s.logger.Info("Stopping chat service")
	return s.ircClient.Disconnect()
}

// RefreshChannels reloads the list of active channels
func (s *ChatService) RefreshChannels(ctx context.Context) error {
	channels, err := s.channelRepo.GetActiveChannels(ctx)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Build new channel map
	newChannels := make(map[string][]domain.ActiveChannel)
	for _, ch := range channels {
		newChannels[ch.Channel] = append(newChannels[ch.Channel], ch)
	}

	// Determine channels to join and part
	oldChannelNames := s.getChannelNames()
	newChannelNames := getMapKeys(newChannels)

	toJoin := difference(newChannelNames, oldChannelNames)
	toPart := difference(oldChannelNames, newChannelNames)

	// Update active channels
	s.activeChannels = newChannels

	// Join new channels
	if len(toJoin) > 0 {
		s.logger.Info("Joining channels", zap.Strings("channels", toJoin))
		s.ircClient.Join(toJoin...)
	}

	// Part from old channels
	if len(toPart) > 0 {
		s.logger.Info("Leaving channels", zap.Strings("channels", toPart))
		s.ircClient.Part(toPart...)
	}

	s.logger.Info("Channel refresh complete",
		zap.Int("active_channels", len(newChannels)),
		zap.Int("joined", len(toJoin)),
		zap.Int("parted", len(toPart)))

	return nil
}

// ProcessMessage enriches and publishes a chat message
func (s *ChatService) ProcessMessage(ctx context.Context, channel string, rawMessage interface{}) error {
	msg, ok := rawMessage.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid message format")
	}

	messageText := msg["message"].(string)
	userName := msg["user"].(string)
	tags := msg["tags"].(map[string]string)

	s.mu.RLock()
	overlays := s.activeChannels[channel]
	s.mu.RUnlock()

	if len(overlays) == 0 {
		return nil // No overlays for this channel
	}

	// Process for each overlay
	for _, overlay := range overlays {
		// Check filters
		if s.isBlocked(userName, messageText, overlay) {
			continue
		}

		// Get emotes for the channel
		emotes, err := s.getChannelEmotes(ctx, channel, overlay)
		if err != nil {
			s.logger.Warn("Failed to fetch emotes",
				zap.String("channel", channel),
				zap.Error(err))
			emotes = []domain.Emote{}
		}

		// Enrich message with emotes
		enrichedMessage := s.enrichMessageWithEmotes(messageText, emotes)

		// Build chat message
		chatMsg := domain.ChatMessage{
			OverlayID: overlay.OverlayID,
			Channel:   channel,
			User: domain.User{
				Name:        userName,
				DisplayName: tags["display-name"],
				Color:       tags["color"],
				Badges:      strings.Split(tags["badges"], ","),
			},
			Message:   enrichedMessage,
			Timestamp: time.Now(),
		}

		// Publish to Redis
		redisChannel := fmt.Sprintf("overlay:%s", overlay.OverlayID)
		if err := s.publisher.Publish(ctx, redisChannel, chatMsg); err != nil {
			s.logger.Error("Failed to publish message",
				zap.String("overlay_id", overlay.OverlayID),
				zap.Error(err))
		}
	}

	return nil
}

// getChannelEmotes fetches and caches emotes for a channel
func (s *ChatService) getChannelEmotes(ctx context.Context, channel string, overlay domain.ActiveChannel) ([]domain.Emote, error) {
	s.mu.RLock()
	cachedEmotes, hasCached := s.channelEmotes[channel]
	cacheTime, hasCacheTime := s.emoteCacheTimes[channel]
	s.mu.RUnlock()

	// Check if cache is valid
	if hasCached && hasCacheTime && time.Since(cacheTime) < s.emoteCacheTTL {
		return cachedEmotes, nil
	}

	// Fetch fresh emotes
	emotes, err := s.emoteClient.GetChannelEmotes(ctx, channel, overlay.Enable7TV, overlay.EnableBTTV, overlay.EnableFFZ)
	if err != nil {
		return cachedEmotes, err // Return cached emotes if fetch fails
	}

	// Update cache
	s.mu.Lock()
	s.channelEmotes[channel] = emotes
	s.emoteCacheTimes[channel] = time.Now()
	s.mu.Unlock()

	return emotes, nil
}

// enrichMessageWithEmotes replaces emote codes with emote objects
func (s *ChatService) enrichMessageWithEmotes(text string, emotes []domain.Emote) domain.Message {
	// Create emote map for quick lookup
	emoteMap := make(map[string]domain.Emote)
	for _, emote := range emotes {
		emoteMap[emote.Code] = emote
	}

	// Find emotes in the message
	words := strings.Split(text, " ")
	foundEmotes := make([]domain.Emote, 0)

	for _, word := range words {
		if emote, exists := emoteMap[word]; exists {
			foundEmotes = append(foundEmotes, emote)
		}
	}

	return domain.Message{
		Text:   text,
		Emotes: foundEmotes,
	}
}

// isBlocked checks if a message should be filtered
func (s *ChatService) isBlocked(user, message string, overlay domain.ActiveChannel) bool {
	// Check blocked users
	for _, blockedUser := range overlay.BlockedUsers {
		if strings.EqualFold(user, blockedUser) {
			return true
		}
	}

	// Check blocked words
	lowerMessage := strings.ToLower(message)
	for _, blockedWord := range overlay.BlockedWords {
		if strings.Contains(lowerMessage, strings.ToLower(blockedWord)) {
			return true
		}
	}

	return false
}

// getChannelNames returns a list of currently active channel names
func (s *ChatService) getChannelNames() []string {
	names := make([]string, 0, len(s.activeChannels))
	for name := range s.activeChannels {
		names = append(names, name)
	}
	return names
}

// Helper functions

func getMapKeys(m map[string][]domain.ActiveChannel) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func difference(a, b []string) []string {
	mb := make(map[string]bool, len(b))
	for _, x := range b {
		mb[x] = true
	}

	diff := make([]string, 0)
	for _, x := range a {
		if !mb[x] {
			diff = append(diff, x)
		}
	}
	return diff
}
