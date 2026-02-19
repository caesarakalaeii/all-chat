package coordination

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/caesar/all-chat/services/source-manager/registry"
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
	k8sClient   *kubernetes.Clientset
	registry    *AssignmentRegistry
	assigner    *Assigner
	sourceRepo  registry.Repository
	redisClient *redis.Client
	logger      *zap.Logger

	reconcileInterval time.Duration
	stopCh            chan struct{}
}

// NewCoordinator creates a new coordinator instance
func NewCoordinator(
	registry *AssignmentRegistry,
	assigner *Assigner,
	sourceRepo registry.Repository,
	redisClient *redis.Client,
	logger *zap.Logger,
) *Coordinator {
	return &Coordinator{
		registry:          registry,
		assigner:          assigner,
		sourceRepo:        sourceRepo,
		redisClient:       redisClient,
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
				go c.reconcile(ctx)
			},
			OnStoppedLeading: func() {
				c.logger.Warn("Lost leadership, stopping reconciliation")
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

	// Query active sources from source-manager registry
	sources, err := c.sourceRepo.GetAllActiveSources(ctx)
	if err != nil {
		c.logger.Error("Failed to query active sources", zap.Error(err))
		return fmt.Errorf("failed to query active sources: %w", err)
	}

	// Query active listener pods from Kubernetes API
	pods, err := c.queryActiveListenerPods(ctx)
	if err != nil {
		c.logger.Error("Failed to query active listener pods", zap.Error(err))
		return fmt.Errorf("failed to query active listener pods: %w", err)
	}

	if len(pods) == 0 {
		c.logger.Warn("No active listener pods found, skipping assignment computation")
		return nil
	}

	// Extract pod IDs
	podIDs := make([]string, 0, len(pods))
	for _, pod := range pods {
		podIDs = append(podIDs, pod.Name)
	}

	// Update assigner with current pod list
	c.assigner = NewAssigner(podIDs)

	c.logger.Info("Computing assignments",
		zap.Int("source_count", len(sources)),
		zap.Int("pod_count", len(podIDs)),
	)

	// Compute assignment for each source
	assignmentCount := 0
	errorCount := 0

	for _, source := range sources {
		// Compute assignment using bounded-load consistent hashing
		podID := c.assigner.AssignChannel(source.ID)

		// Store assignment in Redis registry
		if err := c.registry.StoreAssignment(ctx, source.ID, podID); err != nil {
			c.logger.Error("Failed to store assignment",
				zap.String("source_id", source.ID),
				zap.String("pod_id", podID),
				zap.Error(err),
			)
			errorCount++
			continue // Continue processing other sources
		}

		assignmentCount++
	}

	duration := time.Since(startTime)
	c.logger.Info("Assignment computation complete",
		zap.Int("assignments_stored", assignmentCount),
		zap.Int("errors", errorCount),
		zap.Duration("duration", duration),
	)

	return nil
}

// queryActiveListenerPods queries Kubernetes API for active listener pods
func (c *Coordinator) queryActiveListenerPods(ctx context.Context) ([]corev1.Pod, error) {
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

	// Filter for Running and Ready pods
	activePods := make([]corev1.Pod, 0)
	for _, pod := range podList.Items {
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
			activePods = append(activePods, pod)
		}
	}

	c.logger.Debug("Queried active listener pods",
		zap.Int("total_pods", len(podList.Items)),
		zap.Int("active_pods", len(activePods)),
	)

	return activePods, nil
}

// Stop gracefully stops the coordinator
func (c *Coordinator) Stop() {
	c.logger.Info("Stopping coordinator")
	close(c.stopCh)
}
