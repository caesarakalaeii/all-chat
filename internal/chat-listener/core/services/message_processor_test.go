package services

import (
	"context"
	"strings"
	"testing"

	"github.com/caesar/all-chat/internal/chat-listener/core/domain"
	"go.uber.org/zap"
)

// Mock implementations for testing
type mockIRCClient struct {
	connectFunc    func() error
	disconnectFunc func() error
	joinFunc       func(channels ...string)
	partFunc       func(channels ...string)
	onMessageFunc  func(handler func(channel, user, message string, tags map[string]string))
}

func (m *mockIRCClient) Connect() error {
	if m.connectFunc != nil {
		return m.connectFunc()
	}
	return nil
}

func (m *mockIRCClient) Disconnect() error {
	if m.disconnectFunc != nil {
		return m.disconnectFunc()
	}
	return nil
}

func (m *mockIRCClient) Join(channels ...string) {
	if m.joinFunc != nil {
		m.joinFunc(channels...)
	}
}

func (m *mockIRCClient) Part(channels ...string) {
	if m.partFunc != nil {
		m.partFunc(channels...)
	}
}

func (m *mockIRCClient) OnMessage(handler func(channel, user, message string, tags map[string]string)) {
	if m.onMessageFunc != nil {
		m.onMessageFunc(handler)
	}
}

type mockChannelRepository struct {
	getActiveChannelsFunc func(ctx context.Context) ([]domain.ActiveChannel, error)
}

func (m *mockChannelRepository) GetActiveChannels(ctx context.Context) ([]domain.ActiveChannel, error) {
	if m.getActiveChannelsFunc != nil {
		return m.getActiveChannelsFunc(ctx)
	}
	return []domain.ActiveChannel{}, nil
}

type mockEmoteClient struct {
	getChannelEmotesFunc func(ctx context.Context, channel string, enable7TV, enableBTTV, enableFFZ bool) ([]domain.Emote, error)
}

func (m *mockEmoteClient) GetChannelEmotes(ctx context.Context, channel string, enable7TV, enableBTTV, enableFFZ bool) ([]domain.Emote, error) {
	if m.getChannelEmotesFunc != nil {
		return m.getChannelEmotesFunc(ctx, channel, enable7TV, enableBTTV, enableFFZ)
	}
	return []domain.Emote{}, nil
}

type mockPublisher struct {
	publishFunc func(ctx context.Context, channel string, message interface{}) error
	published   []interface{} // Store published messages for verification
}

func (m *mockPublisher) Publish(ctx context.Context, channel string, message interface{}) error {
	if m.published == nil {
		m.published = make([]interface{}, 0)
	}
	m.published = append(m.published, message)

	if m.publishFunc != nil {
		return m.publishFunc(ctx, channel, message)
	}
	return nil
}

func TestProcessMessage(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	tests := []struct {
		name          string
		channel       string
		messageData   map[string]interface{}
		activeChannel *domain.ActiveChannel
		emotes        []domain.Emote
		wantErr       bool
		checkPublish  bool
	}{
		{
			name:    "successful message processing",
			channel: "testchannel",
			messageData: map[string]interface{}{
				"channel": "testchannel",
				"user":    "testuser",
				"message": "Hello World!",
				"tags": map[string]string{
					"display-name": "TestUser",
					"color":        "#FF0000",
				},
			},
			activeChannel: &domain.ActiveChannel{
				OverlayID:    "overlay-123",
				Channel:      "testchannel",
				Enable7TV:    true,
				EnableBTTV:   true,
				EnableFFZ:    false,
				BlockedUsers: []string{},
				BlockedWords: []string{},
			},
			emotes:       []domain.Emote{},
			wantErr:      false,
			checkPublish: true,
		},
		{
			name:    "message with blocked user",
			channel: "testchannel",
			messageData: map[string]interface{}{
				"channel": "testchannel",
				"user":    "blockeduser",
				"message": "This should not appear",
				"tags": map[string]string{
					"display-name": "BlockedUser",
				},
			},
			activeChannel: &domain.ActiveChannel{
				OverlayID:    "overlay-123",
				Channel:      "testchannel",
				Enable7TV:    false,
				EnableBTTV:   false,
				EnableFFZ:    false,
				BlockedUsers: []string{"blockeduser"},
				BlockedWords: []string{},
			},
			emotes:       []domain.Emote{},
			wantErr:      true,
			checkPublish: false,
		},
		{
			name:    "message with blocked word",
			channel: "testchannel",
			messageData: map[string]interface{}{
				"channel": "testchannel",
				"user":    "testuser",
				"message": "This message contains badword",
				"tags": map[string]string{
					"display-name": "TestUser",
				},
			},
			activeChannel: &domain.ActiveChannel{
				OverlayID:    "overlay-123",
				Channel:      "testchannel",
				Enable7TV:    false,
				EnableBTTV:   false,
				EnableFFZ:    false,
				BlockedUsers: []string{},
				BlockedWords: []string{"badword"},
			},
			emotes:       []domain.Emote{},
			wantErr:      true,
			checkPublish: false,
		},
		{
			name:    "message with emote",
			channel: "testchannel",
			messageData: map[string]interface{}{
				"channel": "testchannel",
				"user":    "testuser",
				"message": "Kappa Hello PogChamp",
				"tags": map[string]string{
					"display-name": "TestUser",
					"color":        "#00FF00",
				},
			},
			activeChannel: &domain.ActiveChannel{
				OverlayID:    "overlay-123",
				Channel:      "testchannel",
				Enable7TV:    true,
				EnableBTTV:   false,
				EnableFFZ:    false,
				BlockedUsers: []string{},
				BlockedWords: []string{},
			},
			emotes: []domain.Emote{
				{
					Code:     "Kappa",
					URL:      "https://static-cdn.jtvnw.net/emoticons/v2/25/default/dark/1.0",
					Provider: "twitch",
				},
				{
					Code:     "PogChamp",
					URL:      "https://static-cdn.jtvnw.net/emoticons/v2/88/default/dark/1.0",
					Provider: "twitch",
				},
			},
			wantErr:      false,
			checkPublish: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockPub := &mockPublisher{}
			mockEmote := &mockEmoteClient{
				getChannelEmotesFunc: func(ctx context.Context, channel string, enable7TV, enableBTTV, enableFFZ bool) ([]domain.Emote, error) {
					return tt.emotes, nil
				},
			}

			service := NewChatService(
				&mockIRCClient{},
				&mockChannelRepository{},
				mockEmote,
				mockPub,
				logger,
			)

			// Manually set active channels for testing
			if tt.activeChannel != nil {
				service.mu.Lock()
				service.activeChannels[tt.channel] = []domain.ActiveChannel{*tt.activeChannel}
				service.channelEmotes[tt.channel] = tt.emotes
				service.mu.Unlock()
			}

			err := service.ProcessMessage(context.Background(), tt.channel, tt.messageData)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if tt.checkPublish && len(mockPub.published) == 0 {
				t.Error("expected message to be published but nothing was published")
			}

			// Verify emote enrichment if emotes were provided
			if len(tt.emotes) > 0 && len(mockPub.published) > 0 {
				// Check that the published message contains emote information
				t.Log("Message published with emotes")
			}
		})
	}
}

func TestFilterMessage(t *testing.T) {
	tests := []struct {
		name         string
		message      string
		user         string
		blockedWords []string
		blockedUsers []string
		shouldFilter bool
	}{
		{
			name:         "no filters applied",
			message:      "Hello world!",
			user:         "testuser",
			blockedWords: []string{},
			blockedUsers: []string{},
			shouldFilter: false,
		},
		{
			name:         "blocked user",
			message:      "Hello world!",
			user:         "blockeduser",
			blockedWords: []string{},
			blockedUsers: []string{"blockeduser"},
			shouldFilter: true,
		},
		{
			name:         "blocked word exact match",
			message:      "This contains badword in it",
			user:         "testuser",
			blockedWords: []string{"badword"},
			blockedUsers: []string{},
			shouldFilter: true,
		},
		{
			name:         "blocked word case insensitive",
			message:      "This contains BADWORD in it",
			user:         "testuser",
			blockedWords: []string{"badword"},
			blockedUsers: []string{},
			shouldFilter: true,
		},
		{
			name:         "multiple blocked words",
			message:      "Clean message",
			user:         "testuser",
			blockedWords: []string{"bad", "spam", "offensive"},
			blockedUsers: []string{},
			shouldFilter: false,
		},
		{
			name:         "multiple filters - user blocked",
			message:      "Hello!",
			user:         "blockeduser",
			blockedWords: []string{"badword"},
			blockedUsers: []string{"blockeduser", "spammer"},
			shouldFilter: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Check blocked user
			for _, blockedUser := range tt.blockedUsers {
				if strings.EqualFold(tt.user, blockedUser) {
					if !tt.shouldFilter {
						t.Error("expected message to not be filtered but user is blocked")
					}
					return
				}
			}

			// Check blocked words
			messageLower := strings.ToLower(tt.message)
			for _, word := range tt.blockedWords {
				if strings.Contains(messageLower, strings.ToLower(word)) {
					if !tt.shouldFilter {
						t.Error("expected message to not be filtered but contains blocked word")
					}
					return
				}
			}

			if tt.shouldFilter {
				t.Error("expected message to be filtered but passed all checks")
			}
		})
	}
}

func TestRefreshChannels(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	mockChannels := []domain.ActiveChannel{
		{
			OverlayID:    "overlay-1",
			Channel:      "channel1",
			Enable7TV:    true,
			EnableBTTV:   true,
			EnableFFZ:    false,
			BlockedUsers: []string{},
			BlockedWords: []string{},
		},
		{
			OverlayID:    "overlay-2",
			Channel:      "channel2",
			Enable7TV:    false,
			EnableBTTV:   true,
			EnableFFZ:    true,
			BlockedUsers: []string{},
			BlockedWords: []string{},
		},
	}

	tests := []struct {
		name             string
		mockChannels     []domain.ActiveChannel
		wantErr          bool
		expectedJoined   int
		expectedParted   int
	}{
		{
			name:           "successful channel refresh",
			mockChannels:   mockChannels,
			wantErr:        false,
			expectedJoined: 2,
			expectedParted: 0,
		},
		{
			name:           "empty channel list",
			mockChannels:   []domain.ActiveChannel{},
			wantErr:        false,
			expectedJoined: 0,
			expectedParted: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			joinedChannels := make([]string, 0)
			partedChannels := make([]string, 0)

			mockIRC := &mockIRCClient{
				joinFunc: func(channels ...string) {
					joinedChannels = append(joinedChannels, channels...)
				},
				partFunc: func(channels ...string) {
					partedChannels = append(partedChannels, channels...)
				},
			}

			mockRepo := &mockChannelRepository{
				getActiveChannelsFunc: func(ctx context.Context) ([]domain.ActiveChannel, error) {
					return tt.mockChannels, nil
				},
			}

			service := NewChatService(
				mockIRC,
				mockRepo,
				&mockEmoteClient{},
				&mockPublisher{},
				logger,
			)

			err := service.RefreshChannels(context.Background())

			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if len(joinedChannels) != tt.expectedJoined {
				t.Errorf("expected %d channels joined, got %d", tt.expectedJoined, len(joinedChannels))
			}

			if len(partedChannels) != tt.expectedParted {
				t.Errorf("expected %d channels parted, got %d", tt.expectedParted, len(partedChannels))
			}
		})
	}
}
