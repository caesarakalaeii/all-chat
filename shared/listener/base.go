package listener

import (
	"context"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/caesar/all-chat/shared/coordination"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// coordinatorClient abstracts the coordinator HTTP calls used by ListenerBase.
// *coordination.CoordinatorClient satisfies this interface.
type coordinatorClient interface {
	PublishHeartbeat(ctx context.Context, podID string) error
	QueryAssignments(ctx context.Context, podID string) ([]*coordination.Assignment, error)
	StartJWTRefresh(ctx context.Context)
	StopJWTRefresh()
}

// ListenerBase manages the four background goroutine loops shared by all listeners:
// heartbeat, assignment refresh, migration subscriber, and demand subscriber.
type ListenerBase struct {
	config               ListenerConfig
	client               coordinatorClient
	redisClient          *redis.Client
	logger               *zap.Logger
	podID                string
	cancel               context.CancelFunc
	wg                   sync.WaitGroup
	hasInitialAssignments atomic.Bool

	// assignedMu protects assignedSourceIDs.
	assignedMu        sync.RWMutex
	assignedSourceIDs map[string]bool
}

// NewListenerBase creates a new ListenerBase.
// redisClient may be nil — when nil, the migration subscriber loop is disabled.
func NewListenerBase(config ListenerConfig, client coordinatorClient, redisClient *redis.Client, podID string, logger *zap.Logger) *ListenerBase {
	return &ListenerBase{
		config:      config,
		client:      client,
		redisClient: redisClient,
		logger:      logger,
		podID:       podID,
	}
}

// Start initialises assignments, starts the channel manager, and launches 3 background loops.
// It blocks briefly to query initial assignments before starting the manager.
func (b *ListenerBase) Start(ctx context.Context, mgr ChannelManager) error {
	// Apply startup jitter to spread pod start times.
	if b.config.StartupJitterMax > 0 {
		//nolint:gosec // non-cryptographic random jitter is intentional
		jitter := time.Duration(rand.Int63n(int64(b.config.StartupJitterMax)))
		b.logger.Info("Applying startup jitter",
			zap.Duration("jitter", jitter),
			zap.Duration("max", b.config.StartupJitterMax),
		)
		time.Sleep(jitter)
	}

	// Query initial assignments from coordinator.
	var assignedIDs map[string]bool
	if !b.config.DisableCoordinatorFiltering {
		assignments, err := b.client.QueryAssignments(ctx, b.podID)
		if err != nil {
			return err
		}
		assignedIDs = make(map[string]bool, len(assignments))
		for _, a := range assignments {
			assignedIDs[a.SourceID] = true
		}
	} else {
		assignedIDs = map[string]bool{}
	}

	b.assignedMu.Lock()
	b.assignedSourceIDs = assignedIDs
	b.assignedMu.Unlock()

	mgr.UpdateAssignedSourceIDs(assignedIDs)
	b.hasInitialAssignments.Store(true)

	if err := mgr.Start(ctx); err != nil {
		return err
	}

	internalCtx, cancel := context.WithCancel(ctx)
	b.cancel = cancel

	b.client.StartJWTRefresh(internalCtx)

	b.wg.Add(4)
	go b.startHeartbeatLoop(internalCtx)
	go b.startAssignmentRefreshLoop(internalCtx, mgr)
	go b.startMigrationSubscriberLoop(internalCtx, mgr)
	go b.startDemandSubscriberLoop(internalCtx, mgr)

	return nil
}

// Stop signals all background goroutines to stop and waits for them to exit.
func (b *ListenerBase) Stop() {
	if b.cancel != nil {
		b.cancel()
	}
	b.wg.Wait()
	b.client.StopJWTRefresh()
}

// startHeartbeatLoop publishes heartbeats on config.HeartbeatInterval ticks.
func (b *ListenerBase) startHeartbeatLoop(ctx context.Context) {
	defer b.wg.Done()

	ticker := time.NewTicker(b.config.HeartbeatInterval)
	defer ticker.Stop()

	backoff := time.Second
	const maxBackoff = 30 * time.Second

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := b.client.PublishHeartbeat(ctx, b.podID); err != nil {
				b.logger.Error("Failed to publish heartbeat",
					zap.String("pod_id", b.podID),
					zap.Duration("backoff", backoff),
					zap.Error(err),
				)
				if b.config.OnFatalError != nil {
					b.config.OnFatalError("heartbeat", err)
				}
				// Retry with backoff on next iteration.
				select {
				case <-ctx.Done():
					return
				case <-time.After(backoff):
				}
				backoff *= 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
				continue
			}
			backoff = time.Second
		}
	}
}

// startAssignmentRefreshLoop refreshes assignments on config.AssignmentRefreshInterval ticks.
func (b *ListenerBase) startAssignmentRefreshLoop(ctx context.Context, mgr ChannelManager) {
	defer b.wg.Done()

	ticker := time.NewTicker(b.config.AssignmentRefreshInterval)
	defer ticker.Stop()

	backoff := time.Second
	const maxBackoff = 30 * time.Second

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			assignments, err := b.client.QueryAssignments(ctx, b.podID)
			if err != nil {
				b.logger.Error("Failed to refresh assignments",
					zap.String("pod_id", b.podID),
					zap.Duration("backoff", backoff),
					zap.Error(err),
				)
				if b.config.OnFatalError != nil {
					b.config.OnFatalError("assignment-refresh", err)
				}
				select {
				case <-ctx.Done():
					return
				case <-time.After(backoff):
				}
				backoff *= 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
				continue
			}
			backoff = time.Second

			if !b.config.DisableCoordinatorFiltering {
				ids := make(map[string]bool, len(assignments))
				for _, a := range assignments {
					ids[a.SourceID] = true
				}
				b.assignedMu.Lock()
				b.assignedSourceIDs = ids
				b.assignedMu.Unlock()
				mgr.UpdateAssignedSourceIDs(ids)
			}
		}
	}
}

// startMigrationSubscriberLoop subscribes to migration events from Redis Pub/Sub.
// If b.redisClient is nil, this loop exits immediately (nil-safe for tests).
func (b *ListenerBase) startMigrationSubscriberLoop(ctx context.Context, mgr ChannelManager) {
	defer b.wg.Done()

	if b.redisClient == nil {
		return
	}

	backoff := time.Second
	const maxBackoff = 30 * time.Second

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		subscriber := coordination.NewMigrationSubscriber(b.redisClient, mgr.HandleMigrationEvent, b.logger)
		if b.config.Platform != "" {
			subscriber = subscriber.WithPlatform(b.config.Platform)
		}
		if err := subscriber.Subscribe(ctx); err != nil {
			b.logger.Error("Migration subscriber failed, retrying",
				zap.Duration("backoff", backoff),
				zap.Error(err),
			)
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}

		// Subscribe returned nil — wait for context cancellation.
		<-ctx.Done()
		return
	}
}
