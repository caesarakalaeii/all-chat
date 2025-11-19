package fetchers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/caesar/all-chat/services/clip-manager/models"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// TwitchFetcher fetches clips from Twitch Helix API
type TwitchFetcher struct {
	clientID    string
	accessToken string
	httpClient  *http.Client
	logger      *zap.Logger
}

// TwitchClipResponse represents the Helix API response
type TwitchClipResponse struct {
	Data []TwitchClip `json:"data"`
	Pagination struct {
		Cursor string `json:"cursor"`
	} `json:"pagination"`
}

// TwitchClip represents a single clip from Twitch
type TwitchClip struct {
	ID              string    `json:"id"`
	URL             string    `json:"url"`
	EmbedURL        string    `json:"embed_url"`
	BroadcasterID   string    `json:"broadcaster_id"`
	BroadcasterName string    `json:"broadcaster_name"`
	CreatorID       string    `json:"creator_id"`
	CreatorName     string    `json:"creator_name"`
	VideoID         string    `json:"video_id"`
	GameID          string    `json:"game_id"`
	Language        string    `json:"language"`
	Title           string    `json:"title"`
	ViewCount       int       `json:"view_count"`
	CreatedAt       time.Time `json:"created_at"`
	ThumbnailURL    string    `json:"thumbnail_url"`
	Duration        float64   `json:"duration"`
	VodOffset       *int      `json:"vod_offset"`
}

// NewTwitchFetcher creates a new Twitch clip fetcher
func NewTwitchFetcher(clientID, accessToken string, logger *zap.Logger) *TwitchFetcher {
	return &TwitchFetcher{
		clientID:    clientID,
		accessToken: accessToken,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		logger: logger,
	}
}

// FetchClips fetches clips for a broadcaster within a date range
func (f *TwitchFetcher) FetchClips(
	ctx context.Context,
	broadcasterID string,
	startedAt time.Time,
	endedAt time.Time,
	maxClips int,
) ([]models.Clip, error) {
	baseURL := "https://api.twitch.tv/helix/clips"

	params := url.Values{}
	params.Add("broadcaster_id", broadcasterID)
	params.Add("started_at", startedAt.Format(time.RFC3339))
	params.Add("ended_at", endedAt.Format(time.RFC3339))
	params.Add("first", fmt.Sprintf("%d", min(maxClips, 100))) // Max 100 per request

	requestURL := baseURL + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+f.accessToken)
	req.Header.Set("Client-Id", f.clientID)

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch clips: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("twitch API returned status %d: %s", resp.StatusCode, string(body))
	}

	var clipResp TwitchClipResponse
	if err := json.NewDecoder(resp.Body).Decode(&clipResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	f.logger.Info("Fetched Twitch clips",
		zap.String("broadcaster_id", broadcasterID),
		zap.Int("count", len(clipResp.Data)),
	)

	// Convert to our Clip model
	clips := make([]models.Clip, 0, len(clipResp.Data))
	for _, twitchClip := range clipResp.Data {
		durationSeconds := int(twitchClip.Duration)

		clip := models.Clip{
			ID:                uuid.New(),
			Platform:          models.PlatformTwitch,
			PlatformClipID:    &twitchClip.ID,
			ClipURL:           twitchClip.URL,
			EmbedURL:          &twitchClip.EmbedURL,
			ThumbnailURL:      &twitchClip.ThumbnailURL,
			Title:             &twitchClip.Title,
			DurationSeconds:   &durationSeconds,
			ViewCount:         twitchClip.ViewCount,
			CreatedAtPlatform: &twitchClip.CreatedAt,
			IsUserProvided:    false,
			FetchedAt:         time.Now(),
			LastUpdated:       time.Now(),
		}

		clips = append(clips, clip)
	}

	return clips, nil
}

// FetchRecentClips fetches the most recent clips (last 30 days) as fallback
func (f *TwitchFetcher) FetchRecentClips(
	ctx context.Context,
	broadcasterID string,
	maxClips int,
) ([]models.Clip, error) {
	endedAt := time.Now()
	startedAt := endedAt.AddDate(0, 0, -30) // Last 30 days

	return f.FetchClips(ctx, broadcasterID, startedAt, endedAt, maxClips)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
