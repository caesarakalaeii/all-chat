package coordination

import (
	"context"
	"fmt"
	"os"
	"time"

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
	k8sClient        *kubernetes.Clientset
	registry         *AssignmentRegistry
	assigner         *Assigner
	sourceRepo       *registry.Repository
	redisClient      *redis.Client
	heartbeatMonitor *HeartbeatMonitor
	metrics          *metrics.ShardMetrics
	logger           *zap.Logger

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
	shardMetrics *metrics.ShardMetrics,
	logger *zap.Logger,
) *Coordinator {
	return &Coordinator{
		registry:          registry,
		assigner:          assigner,
		sourceRepo:        sourceRepo,
		redisClient:       redisClient,
		heartbeatMonitor:  heartbeatMonitor,
		metrics:           shardMetrics,
		logger:            logger,
		reconcileInterval: 30 * time.Second, // Default: 30s per user constraint
		stopCh:            make(chan struct{}),
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

	// Step 2: Query active sources from source-manager registry
	sources, err := c.sourceRepo.GetAllActiveSources(ctx)
	if err != nil {
		c.logger.Error("Failed to query active sources", zap.Error(err))
		return fmt.Errorf("failed to query active sources: %w", err)
	}

	// Step 3: Query active listener pods from Kubernetes API
	// Filter will exclude failed pods detected by heartbeat monitor
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

	// Step 4: Update assigner with healthy pod list
	c.assigner = NewAssigner(podIDs)

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

		if pod.Status.Phase != corev1.PodRunning {
			continue
		}

		// Check if pod is ready
		ready := false
		for _, condition := range pod.Status.Conditions {
			if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
				ready = true
				break
			}
		}

		if ready {
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

// Stop gracefully stops the coordinator
func (c *Coordinator) Stop() {
	c.logger.Info("Stopping coordinator")
	close(c.stopCh)
}
