package refresher_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	authOAuth "github.com/caesar/all-chat/services/auth-service/oauth"
	"github.com/caesar/all-chat/services/token-refresh-service/refresher"
	"github.com/caesar/all-chat/services/token-refresh-service/repository"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

// ---------------------------------------------------------------------------
// Fakes / test doubles
// ---------------------------------------------------------------------------

// fakeProvider is an OAuthProvider that returns a configurable response.
type fakeProvider struct {
	platform authOAuth.Platform
	token    *oauth2.Token
	err      error
}

func (f *fakeProvider) GetAuthURL(state string) string { return "" }
func (f *fakeProvider) ExchangeCode(_ context.Context, _ string) (*oauth2.Token, error) {
	return nil, nil
}
func (f *fakeProvider) GetUserInfo(_ context.Context, _ string) (authOAuth.PlatformUserInfo, error) {
	return nil, nil
}
func (f *fakeProvider) RefreshToken(_ context.Context, _ string) (*oauth2.Token, error) {
	return f.token, f.err
}
func (f *fakeProvider) GetPlatform() authOAuth.Platform { return f.platform }

// fakeRepo records calls made to the mark-permanently-failed methods.
type fakeRepo struct {
	mu sync.Mutex

	markedUsers   []markedUserCall
	markedViewers []markedViewerCall
	markedYT      []markedYTCall

	// If non-nil, UpdateUserTokens / UpdateViewerTokens / UpdateYouTubeTokens return this.
	updateErr error
}

type markedUserCall struct {
	id               string
	suppressDuration time.Duration
}
type markedViewerCall struct {
	sessionID        string
	suppressDuration time.Duration
}
type markedYTCall struct {
	userID           string
	channelID        string
	suppressDuration time.Duration
}

func (r *fakeRepo) GetExpiringUserTokens(_ context.Context, _ time.Duration) ([]*repository.ExpiringToken, error) {
	return nil, nil
}
func (r *fakeRepo) GetExpiringViewerTokens(_ context.Context, _ time.Duration) ([]*repository.ExpiringToken, error) {
	return nil, nil
}
func (r *fakeRepo) GetExpiringYouTubeTokens(_ context.Context, _ time.Duration) ([]*repository.ExpiringToken, error) {
	return nil, nil
}
func (r *fakeRepo) UpdateUserTokens(_ context.Context, _ string, _ *oauth2.Token) error {
	return r.updateErr
}
func (r *fakeRepo) UpdateViewerTokens(_ context.Context, _ string, _ *oauth2.Token) error {
	return r.updateErr
}
func (r *fakeRepo) UpdateYouTubeTokens(_ context.Context, _, _ string, _ *oauth2.Token) error {
	return r.updateErr
}
func (r *fakeRepo) GetUserOverlays(_ context.Context, _ string) ([]string, error) {
	return nil, nil
}

func (r *fakeRepo) MarkUserTokenPermanentlyFailed(_ context.Context, id string, d time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.markedUsers = append(r.markedUsers, markedUserCall{id: id, suppressDuration: d})
	return nil
}
func (r *fakeRepo) MarkViewerTokenPermanentlyFailed(_ context.Context, sessionID string, d time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.markedViewers = append(r.markedViewers, markedViewerCall{sessionID: sessionID, suppressDuration: d})
	return nil
}
func (r *fakeRepo) MarkYouTubeTokenPermanentlyFailed(_ context.Context, userID, channelID string, d time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.markedYT = append(r.markedYT, markedYTCall{userID: userID, channelID: channelID, suppressDuration: d})
	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func newTestManager(repo refresher.TokenRepo, providers map[authOAuth.Platform]authOAuth.OAuthProvider) *refresher.Manager {
	logger := zap.NewNop()
	// Use an isolated Prometheus registry per manager instance so that parallel
	// tests do not trigger duplicate-metric-registration panics.
	reg := prometheus.NewRegistry()
	return refresher.NewManagerWithRepo(
		repo,
		providers,
		nil, // redis not needed for these unit tests
		logger,
		time.Hour,      // refreshInterval (not used in unit tests)
		10*time.Minute, // expiryBuffer
		100,            // batchSize
		1,              // retryAttempts — one attempt so non-retryable fires immediately
		reg,
	)
}

// ---------------------------------------------------------------------------
// Tests — non-retryable errors mark the token in the DB
// ---------------------------------------------------------------------------

// TestRefreshPlatform_NonRetryableError_MarksUserToken verifies that when a user
// token refresh fails with a non-retryable OAuth error (e.g. invalid_grant), the
// manager calls MarkUserTokenPermanentlyFailed so that the token is excluded from
// subsequent batches.
func TestRefreshPlatform_NonRetryableError_MarksUserToken(t *testing.T) {
	repo := &fakeRepo{}
	provider := &fakeProvider{
		platform: authOAuth.PlatformTwitch,
		err:      errors.New("invalid_grant: token has been revoked"),
	}
	providers := map[authOAuth.Platform]authOAuth.OAuthProvider{
		authOAuth.PlatformTwitch: provider,
	}
	mgr := newTestManager(repo, providers)

	token := &repository.ExpiringToken{
		ID:           "user-123",
		Platform:     "twitch",
		Username:     "testuser",
		TokenType:    "user",
		RefreshToken: "bad-refresh-token",
		ExpiresAt:    time.Now().Add(-1 * time.Hour),
	}

	mgr.ExposedRefreshPlatform(context.Background(), authOAuth.PlatformTwitch, []*repository.ExpiringToken{token})

	repo.mu.Lock()
	marked := repo.markedUsers
	repo.mu.Unlock()

	if len(marked) != 1 {
		t.Fatalf("expected 1 MarkUserTokenPermanentlyFailed call, got %d", len(marked))
	}
	if marked[0].id != "user-123" {
		t.Errorf("expected id=user-123, got %s", marked[0].id)
	}
	if marked[0].suppressDuration <= 0 {
		t.Errorf("expected positive suppress duration, got %v", marked[0].suppressDuration)
	}
}

// TestRefreshPlatform_NonRetryableError_MarksViewerToken verifies the same
// behaviour for viewer session tokens.
func TestRefreshPlatform_NonRetryableError_MarksViewerToken(t *testing.T) {
	repo := &fakeRepo{}
	provider := &fakeProvider{
		platform: authOAuth.PlatformTwitch,
		err:      errors.New("invalid_grant: token has been revoked"),
	}
	providers := map[authOAuth.Platform]authOAuth.OAuthProvider{
		authOAuth.PlatformTwitch: provider,
	}
	mgr := newTestManager(repo, providers)

	token := &repository.ExpiringToken{
		ID:           "user-456",
		SessionID:    "session-789",
		Platform:     "twitch",
		Username:     "vieweruser",
		TokenType:    "viewer",
		RefreshToken: "bad-refresh-token",
		ExpiresAt:    time.Now().Add(-2 * time.Hour),
	}

	mgr.ExposedRefreshPlatform(context.Background(), authOAuth.PlatformTwitch, []*repository.ExpiringToken{token})

	repo.mu.Lock()
	marked := repo.markedViewers
	repo.mu.Unlock()

	if len(marked) != 1 {
		t.Fatalf("expected 1 MarkViewerTokenPermanentlyFailed call, got %d", len(marked))
	}
	if marked[0].sessionID != "session-789" {
		t.Errorf("expected sessionID=session-789, got %s", marked[0].sessionID)
	}
}

// TestRefreshPlatform_NonRetryableError_MarksYouTubeToken verifies the same
// behaviour for YouTube channel tokens.
func TestRefreshPlatform_NonRetryableError_MarksYouTubeToken(t *testing.T) {
	repo := &fakeRepo{}
	provider := &fakeProvider{
		platform: authOAuth.PlatformYouTube,
		err:      errors.New("invalid_grant: Token has been expired or revoked"),
	}
	providers := map[authOAuth.Platform]authOAuth.OAuthProvider{
		authOAuth.PlatformYouTube: provider,
	}
	mgr := newTestManager(repo, providers)

	token := &repository.ExpiringToken{
		ID:           "user-yt",
		ChannelID:    "channel-abc",
		Platform:     "youtube",
		TokenType:    "youtube_channel",
		RefreshToken: "bad-yt-refresh-token",
		ExpiresAt:    time.Now().Add(-5 * time.Hour),
	}

	mgr.ExposedRefreshPlatform(context.Background(), authOAuth.PlatformYouTube, []*repository.ExpiringToken{token})

	repo.mu.Lock()
	marked := repo.markedYT
	repo.mu.Unlock()

	if len(marked) != 1 {
		t.Fatalf("expected 1 MarkYouTubeTokenPermanentlyFailed call, got %d", len(marked))
	}
	if marked[0].userID != "user-yt" {
		t.Errorf("expected userID=user-yt, got %s", marked[0].userID)
	}
	if marked[0].channelID != "channel-abc" {
		t.Errorf("expected channelID=channel-abc, got %s", marked[0].channelID)
	}
}

// TestRefreshPlatform_RetryableError_DoesNotMarkToken verifies that transient
// network errors do NOT trigger the permanent-failure mark.
func TestRefreshPlatform_RetryableError_DoesNotMarkToken(t *testing.T) {
	repo := &fakeRepo{}
	provider := &fakeProvider{
		platform: authOAuth.PlatformTwitch,
		err:      errors.New("connection timeout: upstream unreachable"),
	}
	providers := map[authOAuth.Platform]authOAuth.OAuthProvider{
		authOAuth.PlatformTwitch: provider,
	}
	mgr := newTestManager(repo, providers)

	token := &repository.ExpiringToken{
		ID:           "user-transient",
		Platform:     "twitch",
		Username:     "transientuser",
		TokenType:    "user",
		RefreshToken: "maybe-valid-token",
		ExpiresAt:    time.Now().Add(-30 * time.Minute),
	}

	mgr.ExposedRefreshPlatform(context.Background(), authOAuth.PlatformTwitch, []*repository.ExpiringToken{token})

	repo.mu.Lock()
	markedUsers := len(repo.markedUsers)
	markedViewers := len(repo.markedViewers)
	markedYT := len(repo.markedYT)
	repo.mu.Unlock()

	if markedUsers+markedViewers+markedYT > 0 {
		t.Errorf("expected no permanent-failure marks for retryable error, got users=%d viewers=%d yt=%d",
			markedUsers, markedViewers, markedYT)
	}
}

// TestRefreshPlatform_SuccessfulRefresh_DoesNotMarkToken verifies that a
// successful token refresh does not trigger the permanent-failure mark.
func TestRefreshPlatform_SuccessfulRefresh_DoesNotMarkToken(t *testing.T) {
	repo := &fakeRepo{}
	provider := &fakeProvider{
		platform: authOAuth.PlatformTwitch,
		token: &oauth2.Token{
			AccessToken:  "new-access",
			RefreshToken: "new-refresh",
			Expiry:       time.Now().Add(4 * time.Hour),
		},
	}
	providers := map[authOAuth.Platform]authOAuth.OAuthProvider{
		authOAuth.PlatformTwitch: provider,
	}
	mgr := newTestManager(repo, providers)

	token := &repository.ExpiringToken{
		ID:           "user-ok",
		Platform:     "twitch",
		Username:     "healthyuser",
		TokenType:    "user",
		RefreshToken: "valid-refresh-token",
		ExpiresAt:    time.Now().Add(5 * time.Minute),
	}

	mgr.ExposedRefreshPlatform(context.Background(), authOAuth.PlatformTwitch, []*repository.ExpiringToken{token})

	repo.mu.Lock()
	markedUsers := len(repo.markedUsers)
	repo.mu.Unlock()

	if markedUsers > 0 {
		t.Errorf("expected no permanent-failure marks for successful refresh, got %d", markedUsers)
	}
}
