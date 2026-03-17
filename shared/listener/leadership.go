package listener

import (
	"time"

	"github.com/caesar/all-chat/shared/sourcemanager"
	"go.uber.org/zap"
)

// LeadershipListener embeds ListenerBase and adds optional leadership coordination.
// When SOURCE_MANAGER_SECRET is absent, leadership is disabled and all methods
// are nil-safe (sourcemanager.LeadershipCoordinator handles nil receiver).
type LeadershipListener struct {
	*ListenerBase
	coordinator *sourcemanager.LeadershipCoordinator
	smClient    *sourcemanager.Client
}

// NewLeadershipListenerFromEnv constructs a LeadershipListener reading SOURCE_MANAGER_SECRET
// and SOURCE_MANAGER_URL from the environment.
//
// When SOURCE_MANAGER_SECRET is absent, coordination is disabled (coordinator is nil).
// All *sourcemanager.LeadershipCoordinator methods are nil-safe, so callers may
// call l.LeadershipCoordinator().EnsureLeadership / Stop without nil-checks.
func NewLeadershipListenerFromEnv(base *ListenerBase, platform string, logger *zap.Logger) (*LeadershipListener, error) {
	secret := Env("SOURCE_MANAGER_SECRET", "")
	if secret == "" {
		logger.Info("SOURCE_MANAGER_SECRET not set — leadership coordination disabled")
		return &LeadershipListener{ListenerBase: base}, nil
	}

	smURL := Env("SOURCE_MANAGER_URL", "http://source-manager:8083")
	tokenSource := sourcemanager.NewSigningTokenSource(platform+"-listener", secret, 15*time.Minute)
	smClient, err := sourcemanager.NewClient(smURL, tokenSource)
	if err != nil {
		return nil, err
	}

	coordinator := sourcemanager.NewLeadershipCoordinator(platform, smClient, 5*time.Second, logger)

	return &LeadershipListener{
		ListenerBase: base,
		coordinator:  coordinator,
		smClient:     smClient,
	}, nil
}

// LeadershipCoordinator returns the leadership coordinator.
// May be nil when SOURCE_MANAGER_SECRET was absent — all methods on
// *sourcemanager.LeadershipCoordinator are nil-safe.
func (l *LeadershipListener) LeadershipCoordinator() *sourcemanager.LeadershipCoordinator {
	return l.coordinator
}
