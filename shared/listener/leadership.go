package listener

import (
	"context"
	"sync"
	"time"

	"github.com/caesar/all-chat/shared/sourcemanager"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// LeadershipConfig holds the minimal configuration for LeadershipListener.
type LeadershipConfig struct {
	// Platform is the platform identifier for this listener (e.g. "kick", "twitch", "youtube").
	// When set, the demand subscriber loop filters incoming demand updates to only
	// sources matching this platform.
	Platform string

	// DisableDemandFiltering treats all sources as having demand.
	// When true, the SDK demand subscriber loop exits immediately without subscribing
	// to source:demand Pub/Sub.
	DisableDemandFiltering bool
}

// LeadershipListener is a standalone struct that owns leadership coordination and
// the demand subscriber loop. It is the sole SDK entry point for all listeners.
//
// When SOURCE_MANAGER_SECRET is absent, leadership is disabled and all methods
// are nil-safe (sourcemanager.LeadershipCoordinator handles nil receiver).
type LeadershipListener struct {
	config      LeadershipConfig
	redisClient *redis.Client
	logger      *zap.Logger
	coordinator *sourcemanager.LeadershipCoordinator
	smClient    *sourcemanager.Client
	cancel      context.CancelFunc
	wg          sync.WaitGroup
}

// NewLeadershipListenerFromEnv constructs a LeadershipListener reading SOURCE_MANAGER_SECRET
// and SOURCE_MANAGER_URL from the environment.
//
// When SOURCE_MANAGER_SECRET is absent, coordination is disabled (coordinator is nil).
// All *sourcemanager.LeadershipCoordinator methods are nil-safe, so callers may
// call l.LeadershipCoordinator().EnsureLeadership / Stop without nil-checks.
func NewLeadershipListenerFromEnv(platform string, redisClient *redis.Client, logger *zap.Logger) (*LeadershipListener, error) {
	secret := Env("SOURCE_MANAGER_SECRET", "")
	if secret == "" {
		if logger != nil {
			logger.Info("SOURCE_MANAGER_SECRET not set — leadership coordination disabled")
		}
		return &LeadershipListener{
			config:      LeadershipConfig{Platform: platform},
			redisClient: redisClient,
			logger:      logger,
		}, nil
	}

	smURL := Env("SOURCE_MANAGER_URL", "http://source-manager:8083")
	tokenSource := sourcemanager.NewSigningTokenSource(platform+"-listener", secret, 15*time.Minute)
	smClient, err := sourcemanager.NewClient(smURL, tokenSource)
	if err != nil {
		return nil, err
	}

	coordinator := sourcemanager.NewLeadershipCoordinator(platform, smClient, 5*time.Second, logger)

	return &LeadershipListener{
		config:      LeadershipConfig{Platform: platform},
		redisClient: redisClient,
		logger:      logger,
		coordinator: coordinator,
		smClient:    smClient,
	}, nil
}

// NewLeadershipListener constructs a LeadershipListener from an explicit config.
// Intended for tests — does not read environment variables, returns nil coordinator/smClient.
func NewLeadershipListener(config LeadershipConfig, redisClient *redis.Client, logger *zap.Logger) (*LeadershipListener, error) {
	return &LeadershipListener{
		config:      config,
		redisClient: redisClient,
		logger:      logger,
	}, nil
}

// Start calls mgr.Start and, unless demand filtering is disabled, launches the
// demand subscriber loop in a background goroutine.
func (ll *LeadershipListener) Start(ctx context.Context, mgr ChannelManager) error {
	if err := mgr.Start(ctx); err != nil {
		return err
	}

	internalCtx, cancel := context.WithCancel(ctx)
	ll.cancel = cancel

	if !ll.config.DisableDemandFiltering && ll.redisClient != nil {
		ll.wg.Add(1)
		go ll.startDemandSubscriberLoop(internalCtx, mgr)
	}

	return nil
}

// Stop cancels the internal context and waits for all background goroutines to exit.
func (ll *LeadershipListener) Stop() {
	if ll.cancel != nil {
		ll.cancel()
	}
	ll.wg.Wait()
}

// LeadershipCoordinator returns the leadership coordinator.
// May be nil when SOURCE_MANAGER_SECRET was absent — all methods on
// *sourcemanager.LeadershipCoordinator are nil-safe.
func (ll *LeadershipListener) LeadershipCoordinator() *sourcemanager.LeadershipCoordinator {
	return ll.coordinator
}

// SMClient returns the source manager client.
// May be nil when SOURCE_MANAGER_SECRET was absent — callers must nil-check before use.
func (ll *LeadershipListener) SMClient() *sourcemanager.Client {
	return ll.smClient
}
