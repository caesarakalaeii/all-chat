package jobs

import (
	"context"
	"time"

	"github.com/caesar/all-chat/services/share-service/repository"
	"go.uber.org/zap"
)

// ExpiryJob runs a background ticker to expire old pending share requests
type ExpiryJob struct {
	repo   *repository.ShareRepository
	logger *zap.Logger
	ticker *time.Ticker
	done   chan bool
}

// NewExpiryJob creates a new expiry job with a 5-minute interval
func NewExpiryJob(repo *repository.ShareRepository, logger *zap.Logger) *ExpiryJob {
	return &ExpiryJob{
		repo:   repo,
		logger: logger,
		ticker: time.NewTicker(5 * time.Minute),
		done:   make(chan bool),
	}
}

// Start begins the expiry job in a background goroutine
// Runs immediately on start, then every 5 minutes
func (j *ExpiryJob) Start(ctx context.Context) {
	j.logger.Info("Starting share request expiry job",
		zap.String("interval", "5 minutes"))

	go func() {
		// Run immediately on start (don't wait 5 minutes)
		j.expireOldRequests(ctx)

		for {
			select {
			case <-j.done:
				j.logger.Info("Expiry job stopped")
				return
			case <-j.ticker.C:
				j.expireOldRequests(ctx)
			}
		}
	}()
}

// Stop gracefully stops the expiry job
func (j *ExpiryJob) Stop() {
	j.logger.Info("Stopping expiry job...")
	j.ticker.Stop()
	j.done <- true
}

// expireOldRequests calls the repository to expire pending requests
func (j *ExpiryJob) expireOldRequests(ctx context.Context) {
	// Add timeout to prevent indefinite blocking
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	count, err := j.repo.ExpirePendingRequests(ctx)
	if err != nil {
		j.logger.Error("Failed to expire requests", zap.Error(err))
		return
	}

	if count > 0 {
		j.logger.Info("Expired share requests",
			zap.Int("count", count))
	} else {
		j.logger.Debug("No requests to expire")
	}
}
