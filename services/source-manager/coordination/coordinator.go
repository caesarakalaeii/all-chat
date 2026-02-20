package coordination

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/caesar/all-chat/services/source-manager/models"
	"github.com/caesar/all-chat/services/source-manager/registry"
	"github.com/caesar/all-chat/shared/metrics"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

// Coordinator manages leader election and channel assignment computation
type Coordinator struct {
	k8sClient          *kubernetes.Clientset
	registry           *AssignmentRegistry
	assigner           *Assigner
	sourceRepo         *registry.Repository
	redisClient        *redis.Client
	heartbeatMonitor   *HeartbeatMonitor
	migrationPublisher *MigrationPublisher
	loadMonitor        *LoadMonitor
	rebalancer         *Rebalancer
	metrics            *metrics.ShardMetrics
	logger             *zap.Logger

	reconcileInterval time.Duration
	stopCh            chan struct{}
}

// NewCoordinator creates a new coordinator instance
func NewCoordinator(
	registry *AssignmentRegistry,
	assigner *Assigner,
	sourceRepo *registry.Repository,
	redisClient *redis.Client,
	heartbeatMonitor *HeartbeatMonitor,
	migrationPublisher *MigrationPublisher,
	loadMonitor *LoadMonitor,
	rebalancer *Rebalancer,
	shardMetrics *metrics.ShardMetrics,
	logger *zap.Logger,
) *Coordinator {
	return &Coordinator{
		registry:           registry,
		assigner:           assigner,
		sourceRepo:         sourceRepo,
		redisClient:        redisClient,
		heartbeatMonitor:   heartbeatMonitor,
		migrationPublisher: migrationPublisher,
		loadMonitor:        loadMonitor,
		rebalancer:         rebalancer,
		metrics:            shardMetrics,
		logger:             logger,
		reconcileInterval:  30 * time.Second, // Default: 30s per user constraint
		stopCh:             make(chan struct{}),
	}
}

// Run starts the leader election loop
func (c *Coordinator) Run(ctx context.Context) error {
	// Create in-cluster Kubernetes client
	config, err := rest.InClusterConfig()
	if err != nil {
		c.logger.Error("Failed to create in-cluster config", zap.Error(err))
		return fmt.Errorf("failed to create in-cluster config: %w", err)
	}

	c.k8sClient, err = kubernetes.NewForConfig(config)
	if err != nil {
		c.logger.Error("Failed to create Kubernetes client", zap.Error(err))
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	// Get POD_NAME and POD_NAMESPACE from environment (downward API)
	podName := os.Getenv("POD_NAME")
	podNamespace := os.Getenv("POD_NAMESPACE")
	if podName == "" {
		c.logger.Warn("POD_NAME not set, using default")
		podName = "source-manager-unknown"
	}
	if podNamespace == "" {
		c.logger.Warn("POD_NAMESPACE not set, using default")
		podNamespace = "allchat"
	}

	c.logger.Info("Starting leader election",
		zap.String("pod_name", podName),
		zap.String("pod_namespace", podNamespace),
	)

	// Create LeaseLock
	lock := &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      "shard-coordinator",
			Namespace: podNamespace,
		},
		Client: c.k8sClient.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: podName,
		},
	}

	// Configure leader election
	leaderConfig := leaderelection.LeaderElectionConfig{
		Lock:            lock,
		LeaseDuration:   30 * time.Second, // Per RESEARCH.md Pattern 2
		RenewDeadline:   15 * time.Second,
		RetryPeriod:     5 * time.Second,
		ReleaseOnCancel: true,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				c.logger.Info("Acquired leadership, starting reconciliation loop")
				c.metrics.CoordinatorIsLeader.Set(1)
				go c.reconcile(ctx)
			},
			OnStoppedLeading: func() {
				c.logger.Warn("Lost leadership, stopping reconciliation")
				c.metrics.CoordinatorIsLeader.Set(0)
				c.Stop()
			},
			OnNewLeader: func(identity string) {
				if identity == podName {
					c.logger.Info("I am the new leader", zap.String("identity", identity))
				} else {
					c.logger.Info("New leader elected", zap.String("identity", identity))
				}
			},
		},
	}

	// Run leader election (blocks until context cancelled or error)
	leaderelection.RunOrDie(ctx, leaderConfig)
	return nil
}

// reconcile is the main assignment computation loop (runs only on leader)
func (c *Coordinator) reconcile(ctx context.Context) {
	ticker := time.NewTicker(c.reconcileInterval)
	defer ticker.Stop()

	c.logger.Info("Reconciliation loop started", zap.Duration("interval", c.reconcileInterval))

	for {
		select {
		case <-ticker.C:
			if err := c.computeAssignments(ctx); err != nil {
				c.logger.Error("Failed to compute assignments", zap.Error(err))
			}
		case <-c.stopCh:
			c.logger.Info("Reconciliation loop stopped")
			return
		case <-ctx.Done():
			c.logger.Info("Context cancelled, stopping reconciliation")
			return
		}
	}
}

// computeAssignments queries active sources and listener pods, then computes assignments
func (c *Coordinator) computeAssignments(ctx context.Context) error {
	startTime := time.Now()
	defer func() {
		c.metrics.ReconciliationCycles.Inc()
	}()

	// Step 1: Detect failed pods via heartbeat monitoring
	failedPods, err := c.heartbeatMonitor.GetFailedPods(ctx)
	if err != nil {
		c.logger.Error("Failed to detect failed pods", zap.Error(err))
		c.metrics.ReconciliationErrors.Inc()
		// Continue with empty failed list - don't block reconciliation
		failedPods = []string{}
	} else if len(failedPods) > 0 {
		c.logger.Info("Detected failed pods, triggering reassignment",
			zap.Strings("failed_pods", failedPods),
		)
	}

	// Update pod health metrics
	c.metrics.FailedPods.Set(float64(len(failedPods)))

	// Step 1.5: Query active listener pods from Kubernetes API BEFORE triggering migrations
	// We need healthy pod list to select migration targets
	pods, err := c.getHealthyListenerPods(ctx, failedPods)
	if err != nil {
		c.logger.Error("Failed to query healthy listener pods", zap.Error(err))
		c.metrics.ReconciliationErrors.Inc()
		return fmt.Errorf("failed to query healthy listener pods: %w", err)
	}

	if len(pods) == 0 {
		c.logger.Warn("No healthy listener pods available, skipping assignment computation")
		c.metrics.HealthyPods.Set(0)
		return nil
	}

	// Extract pod IDs
	podIDs := make([]string, 0, len(pods))
	for _, pod := range pods {
		podIDs = append(podIDs, pod.Name)
	}

	// Update healthy pod metrics
	c.metrics.HealthyPods.Set(float64(len(podIDs)))

	// Update assigner with healthy pod list BEFORE triggering migrations
	c.assigner = NewAssigner(podIDs)

	// Step 2: Query active sources from source-manager registry (before migrations)
	sources, err := c.sourceRepo.GetAllActiveSources(ctx)
	if err != nil {
		c.logger.Error("Failed to query active sources", zap.Error(err))
		return fmt.Errorf("failed to query active sources: %w", err)
	}

	// Step 2.5: Build source lookup map for migration trigger
	sourceMap := make(map[string]*models.ActiveSource)
	for _, source := range sources {
		sourceMap[source.ID] = source
	}

	// Step 2.6: Trigger migrations for failed pods (if any)
	if len(failedPods) > 0 {
		if err := c.triggerMigrationForFailedPods(ctx, failedPods, podIDs, sourceMap); err != nil {
			c.logger.Error("Failed to trigger migrations for failed pods", zap.Error(err))
			// Continue with normal reconciliation even if migrations fail
		}
	}

	// Step 2.7: Monitor pod loads and check for imbalance (Phase 7 - Dynamic Rebalancing)
	if c.loadMonitor != nil && c.rebalancer != nil {
		loads, err := c.loadMonitor.MonitorPodLoads(ctx, podIDs)
		if err != nil {
			c.logger.Error("Failed to monitor pod loads", zap.Error(err))
			// Continue with normal reconciliation
		} else {
			report := c.loadMonitor.CalculateImbalance(loads)

			if report.ShouldRebalance {
				c.logger.Info("Load imbalance detected, planning rebalancing",
					zap.Float64("imbalance_ratio", report.ImbalanceRatio),
					zap.Float64("max_message_rate", report.MaxMessageRate),
					zap.String("reason", report.Reason),
				)

				// Plan rebalancing migrations
				plans, err := c.rebalancer.PlanRebalancing(ctx, loads, report.AvgLoad)
				if err != nil {
					c.logger.Error("Failed to plan rebalancing", zap.Error(err))
				} else {
					// Execute migration plans (triggers are sufficient, no waiting yet)
					c.executeRebalancingPlans(ctx, plans, sourceMap)
				}
			} else {
				c.logger.Debug("No rebalancing needed", zap.String("reason", report.Reason))
			}
		}
	}

	c.logger.Info("Computing assignments",
		zap.Int("source_count", len(sources)),
		zap.Int("pod_count", len(podIDs)),
		zap.Int("failed_pods", len(failedPods)),
	)

	// Step 5: Compute assignment for each source
	assignmentCount := 0
	errorCount := 0

	for _, source := range sources {
		// Compute assignment using bounded-load consistent hashing
		podID, err := c.assigner.AssignChannel(source.ID)
		if err != nil {
			c.logger.Error("Failed to assign channel",
				zap.String("source_id", source.ID),
				zap.Error(err),
			)
			c.metrics.ReconciliationErrors.Inc()
			errorCount++
			continue // Continue processing other sources
		}

		// Store assignment in Redis registry
		_, err = c.registry.StoreAssignment(ctx, source.ID, podID)
		if err != nil {
			c.logger.Error("Failed to store assignment",
				zap.String("source_id", source.ID),
				zap.String("pod_id", podID),
				zap.Error(err),
			)
			c.metrics.ReconciliationErrors.Inc()
			errorCount++
			continue // Continue processing other sources
		}

		c.metrics.AssignmentsTotal.Inc()
		assignmentCount++
	}

	// Step 6: Cleanup stale heartbeats (every cycle)
	if err := c.heartbeatMonitor.CleanupStaleHeartbeats(ctx); err != nil {
		c.logger.Error("Failed to cleanup stale heartbeats", zap.Error(err))
	}

	// Step 7: Remove orphaned assignments (every cycle)
	if err := c.heartbeatMonitor.RemoveOrphanedAssignments(ctx, c.registry, c.sourceRepo); err != nil {
		c.logger.Error("Failed to remove orphaned assignments", zap.Error(err))
	}

	duration := time.Since(startTime)
	c.logger.Info("Assignment computation complete",
		zap.Int("assignments_stored", assignmentCount),
		zap.Int("errors", errorCount),
		zap.Int("healthy_pods", len(podIDs)),
		zap.Int("failed_pods", len(failedPods)),
		zap.Duration("duration", duration),
	)

	return nil
}

// getHealthyListenerPods queries Kubernetes API for active listener pods
// and excludes pods that have failed heartbeat checks
func (c *Coordinator) getHealthyListenerPods(ctx context.Context, failedPods []string) ([]corev1.Pod, error) {
	podNamespace := os.Getenv("POD_NAMESPACE")
	if podNamespace == "" {
		podNamespace = "allchat"
	}

	// List pods with listener labels (twitch-listener, kick-listener, tiktok-listener)
	// Filter by phase=Running and ready=true
	listOptions := metav1.ListOptions{
		LabelSelector: "app in (twitch-listener,kick-listener,tiktok-listener)",
	}

	podList, err := c.k8sClient.CoreV1().Pods(podNamespace).List(ctx, listOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	// Build set of failed pod names for fast lookup
	failedPodSet := make(map[string]bool)
	for _, podID := range failedPods {
		failedPodSet[podID] = true
	}

	// Filter for Running, Ready, and NOT failed pods
	healthyPods := make([]corev1.Pod, 0)
	for _, pod := range podList.Items {
		// Skip if pod failed heartbeat check
		if failedPodSet[pod.Name] {
			c.logger.Debug("Excluding failed pod from assignment",
				zap.String("pod_name", pod.Name),
			)
			continue
		}

		// Only require Running status - readiness depends on assignments (chicken-egg problem)
		// Pod queries assignments on startup, coordinator assigns to Running pods,
		// pod becomes Ready after connecting to assigned channels
		if pod.Status.Phase == corev1.PodRunning {
			healthyPods = append(healthyPods, pod)
		}
	}

	c.logger.Debug("Queried healthy listener pods",
		zap.Int("total_pods", len(podList.Items)),
		zap.Int("healthy_pods", len(healthyPods)),
		zap.Int("excluded_failed", len(failedPods)),
	)

	return healthyPods, nil
}

// executeRebalancingPlans executes migration plans by publishing events and updating assignments
// Uses Phase 6 migration infrastructure (PublishMigrationEvent)
func (c *Coordinator) executeRebalancingPlans(ctx context.Context, plans []MigrationPlan, sourceMap map[string]*models.ActiveSource) {
	if len(plans) == 0 {
		c.logger.Debug("No rebalancing plans to execute")
		return
	}

	c.logger.Info("Executing rebalancing plans", zap.Int("plan_count", len(plans)))

	for _, plan := range plans {
		c.logger.Info("Executing rebalancing plan",
			zap.String("from_pod", plan.SourcePod),
			zap.String("to_pod", plan.TargetPod),
			zap.Int("channel_count", len(plan.Channels)),
		)

		// For each channel in the plan
		for _, channelID := range plan.Channels {
			// Get platform from sourceMap (like triggerMigrationForFailedPods does)
			platform := "unknown"
			if source, ok := sourceMap[channelID]; ok {
				platform = source.Platform
			}

			// Build migration event
			event := &MigrationEvent{
				MigrationID: fmt.Sprintf("migration-%d", time.Now().UnixNano()),
				ChannelID:   channelID,
				Platform:    platform,
				FromPod:     plan.SourcePod,
				ToPod:       plan.TargetPod,
				Timestamp:   time.Now(),
				Reason:      "rebalancing",
			}

			// Publish migration event (Phase 6 infrastructure)
			if err := c.migrationPublisher.PublishMigrationEvent(ctx, event); err != nil {
				c.logger.Error("Failed to publish rebalancing migration event",
					zap.String("migration_id", event.MigrationID),
					zap.String("channel_id", event.ChannelID),
					zap.Error(err),
				)
				// Continue with remaining channels (partial rebalancing acceptable)
				continue
			}

			// Update assignment registry
			_, err := c.registry.StoreAssignment(ctx, channelID, plan.TargetPod)
			if err != nil {
				c.logger.Error("Failed to update assignment after rebalancing",
					zap.String("channel_id", channelID),
					zap.String("target_pod", plan.TargetPod),
					zap.Error(err),
				)
				// Continue with remaining channels
				continue
			}

			c.logger.Debug("Rebalancing migration event published",
				zap.String("migration_id", event.MigrationID),
				zap.String("channel_id", event.ChannelID),
				zap.String("from_pod", plan.SourcePod),
				zap.String("to_pod", plan.TargetPod),
			)
		}

		c.logger.Info("Executed rebalancing plan",
			zap.String("from_pod", plan.SourcePod),
			zap.String("to_pod", plan.TargetPod),
			zap.Int("channels_migrated", len(plan.Channels)),
		)
	}

	c.logger.Info("Rebalancing plans execution complete",
		zap.Int("plans_executed", len(plans)),
	)
}

// Stop gracefully stops the coordinator
func (c *Coordinator) Stop() {
	c.logger.Info("Stopping coordinator")
	close(c.stopCh)
}

// waitForMigrationConfirmation waits for a migration confirmation in Redis Streams (MIGRATE-03)
func (c *Coordinator) waitForMigrationConfirmation(ctx context.Context, migrationID string, timeout time.Duration) error {
	// Per CONTEXT.md: "If old pod doesn't send 'disconnected' confirmation within 60s timeout"
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	c.logger.Info("Waiting for migration confirmation",
		zap.String("migration_id", migrationID),
		zap.Duration("timeout", timeout),
	)

	startTime := time.Now()
	lastID := "0" // Start from beginning of stream

	// Poll Redis Streams migration:log for confirmation
	for {
		select {
		case <-ctx.Done():
			c.logger.Error("Migration confirmation timeout",
				zap.String("migration_id", migrationID),
				zap.Duration("elapsed", time.Since(startTime)),
			)
			return fmt.Errorf("timeout waiting for confirmation")
		default:
			// Read from migration:log stream
			result, err := c.redisClient.XRead(ctx, &redis.XReadArgs{
				Streams: []string{"migration:log", lastID},
				Count:   100,
				Block:   1 * time.Second,
			}).Result()

			if err != nil && err != redis.Nil {
				c.logger.Error("Failed to read migration:log stream",
					zap.String("migration_id", migrationID),
					zap.Error(err),
				)
				time.Sleep(1 * time.Second)
				continue
			}

			// Parse messages and check for our migration_id with status="connected" or "failed"
			for _, stream := range result {
				for _, message := range stream.Messages {
					msgMigrationID, _ := message.Values["migration_id"].(string)
					msgStatus, _ := message.Values["status"].(string)

					if msgMigrationID == migrationID {
						if msgStatus == "connected" {
							c.logger.Info("Migration confirmed successfully",
								zap.String("migration_id", migrationID),
								zap.Duration("elapsed", time.Since(startTime)),
							)
							return nil
						} else if msgStatus == "failed" {
							errMsg, _ := message.Values["error"].(string)
							c.logger.Error("Migration failed",
								zap.String("migration_id", migrationID),
								zap.String("error", errMsg),
							)
							return fmt.Errorf("migration failed: %s", errMsg)
						}
					}

					// Update lastID for next iteration
					lastID = message.ID
				}
			}

			time.Sleep(1 * time.Second)
		}
	}
}

// triggerMigrationForFailedPods triggers migrations for all channels assigned to failed pods
func (c *Coordinator) triggerMigrationForFailedPods(ctx context.Context, failedPods []string, healthyPodIDs []string, sourceMap map[string]*models.ActiveSource) error {
	if len(failedPods) == 0 {
		return nil
	}

	c.logger.Info("Triggering migrations for failed pods",
		zap.Strings("failed_pods", failedPods),
		zap.Int("healthy_pods", len(healthyPodIDs)),
	)

	// For each failed pod
	for _, failedPodID := range failedPods {
		// Get channels assigned to failed pod
		assignments, err := c.registry.GetAssignmentsForPod(ctx, failedPodID)
		if err != nil {
			c.logger.Error("Failed to get assignments for failed pod",
				zap.String("failed_pod", failedPodID),
				zap.Error(err),
			)
			continue
		}

		if len(assignments) == 0 {
			c.logger.Debug("No assignments found for failed pod",
				zap.String("failed_pod", failedPodID),
			)
			continue
		}

		c.logger.Info("Migrating channels from failed pod",
			zap.String("failed_pod", failedPodID),
			zap.Int("channel_count", len(assignments)),
		)

		// Trigger migration for each channel
		for _, assignment := range assignments {
			// Use bounded-load algorithm to select target pod
			newPodID, err := c.assigner.AssignChannel(assignment.SourceID)
			if err != nil {
				c.logger.Error("Failed to select target pod for migration",
					zap.String("source_id", assignment.SourceID),
					zap.Error(err),
				)
				continue
			}

			// Get platform for source from sourceMap
			platform := "unknown"
			if source, ok := sourceMap[assignment.SourceID]; ok {
				platform = source.Platform
			}

			event := &MigrationEvent{
				MigrationID: fmt.Sprintf("migration-%d", time.Now().UnixNano()),
				ChannelID:   assignment.SourceID,
				Platform:    platform,
				FromPod:     failedPodID,
				ToPod:       newPodID,
				Timestamp:   time.Now(),
				Reason:      "pod_failure",
			}

			if err := c.migrationPublisher.PublishMigrationEvent(ctx, event); err != nil {
				c.logger.Error("Failed to publish migration event",
					zap.String("migration_id", event.MigrationID),
					zap.String("channel_id", event.ChannelID),
					zap.Error(err),
				)
				continue
			}

			// For failed pods, skip confirmation wait (pod is already dead)
			// Only wait for confirmation during live migrations (scale-up, rebalancing)
			// This prevents blocking the reconciliation loop for dead pods
			c.logger.Info("Skipping confirmation wait for failed pod migration",
				zap.String("migration_id", event.MigrationID),
				zap.String("from_pod", event.FromPod),
				zap.String("reason", "pod_already_failed"),
			)

			// Update assignment registry
			_, err = c.registry.StoreAssignment(ctx, assignment.SourceID, newPodID)
			if err != nil {
				c.logger.Error("Failed to update assignment after migration",
					zap.String("source_id", assignment.SourceID),
					zap.String("new_pod", newPodID),
					zap.Error(err),
				)
			}
		}
	}

	return nil
}
