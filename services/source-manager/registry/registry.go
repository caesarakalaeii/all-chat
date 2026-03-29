package registry

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/caesar/all-chat/services/source-manager/models"
	"go.uber.org/zap"
)

// Registry maintains the active source registry
type Registry struct {
	repository *Repository
	logger     *zap.Logger

	mu      sync.RWMutex
	sources map[string]*models.ActiveSource // source ID -> source

	syncInterval time.Duration
	stopChan     chan struct{}
	wg           sync.WaitGroup
}

// NewRegistry creates a new source registry
func NewRegistry(repository *Repository, syncInterval time.Duration, logger *zap.Logger) *Registry {
	if syncInterval == 0 {
		syncInterval = 30 * time.Second
	}

	return &Registry{
		repository:   repository,
		logger:       logger,
		sources:      make(map[string]*models.ActiveSource),
		syncInterval: syncInterval,
		stopChan:     make(chan struct{}),
	}
}

// Start begins syncing the registry from the database
func (r *Registry) Start(ctx context.Context) error {
	r.logger.Info("Starting source registry")

	// Initial sync
	if err := r.sync(ctx); err != nil {
		r.logger.Error("Failed initial sync", zap.Error(err))
		return fmt.Errorf("failed initial sync: %w", err)
	}

	// Start periodic sync
	r.wg.Add(1)
	go r.periodicSync(ctx)

	return nil
}

// Stop stops the registry
func (r *Registry) Stop() {
	r.logger.Info("Stopping source registry")
	close(r.stopChan)
	r.wg.Wait()
}

// periodicSync periodically syncs from database
func (r *Registry) periodicSync(ctx context.Context) {
	defer r.wg.Done()

	ticker := time.NewTicker(r.syncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := r.sync(ctx); err != nil {
				r.logger.Error("Failed to sync sources", zap.Error(err))
			}
		case <-r.stopChan:
			return
		}
	}
}

// sync fetches sources from database and updates registry
func (r *Registry) sync(ctx context.Context) error {
	sources, err := r.repository.GetAllActiveSources(ctx)
	if err != nil {
		return fmt.Errorf("failed to get active sources: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Clear old sources
	r.sources = make(map[string]*models.ActiveSource)

	// Add new sources
	for _, source := range sources {
		r.sources[source.ID] = source
	}

	r.logger.Info("Synced source registry",
		zap.Int("total_sources", len(sources)),
	)

	return nil
}

// GetSourcesByPlatform returns all active sources for a platform
func (r *Registry) GetSourcesByPlatform(platform string) []*models.ActiveSource {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*models.ActiveSource, 0)
	for _, source := range r.sources {
		if source.Platform == platform {
			result = append(result, source)
		}
	}

	return result
}

// GetAllSources returns all active sources
func (r *Registry) GetAllSources() []*models.ActiveSource {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*models.ActiveSource, 0, len(r.sources))
	for _, source := range r.sources {
		result = append(result, source)
	}

	return result
}

// GetSource returns a specific source by ID
func (r *Registry) GetSource(sourceID string) (*models.ActiveSource, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	source, exists := r.sources[sourceID]
	if !exists {
		return nil, fmt.Errorf("source not found: %s", sourceID)
	}

	return source, nil
}

// ActivateSource marks a source active in the DB and refreshes updated_at.
// Listeners call this when they start polling, preventing cleanup from deactivating it.
func (r *Registry) ActivateSource(ctx context.Context, platform, channelID string) (int64, error) {
	return r.repository.ActivateSource(ctx, platform, channelID)
}

// DeactivateSource marks a source inactive in the DB immediately.
// Listeners call this when they stop polling so the admin panel reflects the
// actual state instead of waiting 24 h for the cleanup job to run.
func (r *Registry) DeactivateSource(ctx context.Context, platform, channelID string) (int64, error) {
	return r.repository.DeactivateSource(ctx, platform, channelID)
}

// GetStats returns registry statistics
func (r *Registry) GetStats() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	platformCounts := make(map[string]int)
	for _, source := range r.sources {
		platformCounts[source.Platform]++
	}

	return map[string]interface{}{
		"total_sources":   len(r.sources),
		"platform_counts": platformCounts,
	}
}
