package streams

import (
	"context"
	"fmt"

	"github.com/caesar/all-chat/services/youtube-listener/api"
	"github.com/caesar/all-chat/services/youtube-listener/models"
	"github.com/caesar/all-chat/services/youtube-listener/quota"
	"go.uber.org/zap"
)

// DetectionMethod represents the type of detection method to use
type DetectionMethod int

const (
	// MethodStatusCheck uses lightweight status check (1 quota unit)
	MethodStatusCheck DetectionMethod = iota
	// MethodFullDetection uses full search.list detection (100 quota units)
	MethodFullDetection
	// MethodSkip skips detection due to quota exhaustion
	MethodSkip
)

// LiveStreamDetector handles hybrid detection strategy for YouTube live streams
type LiveStreamDetector struct {
	client       *api.Client
	quotaTracker *quota.PerChannelTracker
	logger       *zap.Logger

	// Feature flag for hybrid detection
	enableHybridDetection bool

	// Thresholds
	statusCheckFailureThreshold int // Fallback to full detection after N failures
	offlineCacheClearThreshold  int // Clear cached video ID after N offline checks
}

// DetectorConfig holds configuration for the live stream detector
type DetectorConfig struct {
	EnableHybridDetection       bool
	StatusCheckFailureThreshold int
	OfflineCacheClearThreshold  int
}

// DefaultDetectorConfig returns default detector configuration
func DefaultDetectorConfig() DetectorConfig {
	return DetectorConfig{
		EnableHybridDetection:       true,
		StatusCheckFailureThreshold: 3,
		OfflineCacheClearThreshold:  3,
	}
}

// NewLiveStreamDetector creates a new hybrid live stream detector
func NewLiveStreamDetector(
	client *api.Client,
	quotaTracker *quota.PerChannelTracker,
	logger *zap.Logger,
	config DetectorConfig,
) *LiveStreamDetector {
	return &LiveStreamDetector{
		client:                      client,
		quotaTracker:                quotaTracker,
		logger:                      logger,
		enableHybridDetection:       config.EnableHybridDetection,
		statusCheckFailureThreshold: config.StatusCheckFailureThreshold,
		offlineCacheClearThreshold:  config.OfflineCacheClearThreshold,
	}
}

// DetectLiveStream attempts to detect if a channel is currently live using hybrid strategy
func (d *LiveStreamDetector) DetectLiveStream(ctx context.Context, channelID string) (*models.YouTubeStream, error) {
	// Get channel quota info
	channelQuota, err := d.quotaTracker.GetChannelQuota(ctx, channelID)
	if err != nil {
		d.logger.Error("Failed to get channel quota",
			zap.String("channel_id", channelID),
			zap.Error(err),
		)
		// Continue with default behavior
	}

	// Select detection method
	method := d.selectDetectionMethod(ctx, channelQuota)

	d.logger.Debug("Selected detection method",
		zap.String("channel_id", channelID),
		zap.String("method", d.methodName(method)),
		zap.String("cached_video_id", d.getCachedVideoID(channelQuota)),
	)

	switch method {
	case MethodStatusCheck:
		return d.performStatusCheck(ctx, channelID, channelQuota)

	case MethodFullDetection:
		return d.performFullDetection(ctx, channelID)

	case MethodSkip:
		return nil, fmt.Errorf("quota exhausted for channel %s", channelID)

	default:
		return nil, fmt.Errorf("unknown detection method: %d", method)
	}
}

// selectDetectionMethod determines which detection method to use
func (d *LiveStreamDetector) selectDetectionMethod(ctx context.Context, channelQuota *quota.ChannelQuota) DetectionMethod {
	// Feature flag disabled: always use full detection
	if !d.enableHybridDetection {
		return MethodFullDetection
	}

	// No quota info: use full detection
	if channelQuota == nil {
		return MethodFullDetection
	}

	// Has cached video ID: use cheap status check
	if channelQuota.CachedVideoID != nil && *channelQuota.CachedVideoID != "" {
		// But fallback to full detection if status checks keep failing
		if channelQuota.ConsecutiveStatusCheckFailures >= d.statusCheckFailureThreshold {
			d.logger.Info("Status check failures exceeded threshold, using full detection",
				zap.String("channel_id", channelQuota.ChannelID),
				zap.Int("failures", channelQuota.ConsecutiveStatusCheckFailures),
			)
			return MethodFullDetection
		}
		return MethodStatusCheck
	}

	// No cached video ID: need full detection to discover stream
	// But first check if quota allows it
	canUse, err := d.quotaTracker.CanUseQuota(ctx, channelQuota.ChannelID, 100)
	if err != nil {
		d.logger.Error("Failed to check quota availability",
			zap.String("channel_id", channelQuota.ChannelID),
			zap.Error(err),
		)
		return MethodSkip
	}

	if !canUse {
		d.logger.Warn("Insufficient quota for full detection",
			zap.String("channel_id", channelQuota.ChannelID),
			zap.String("tier", channelQuota.PriorityTier),
			zap.Int("quota_used", channelQuota.DailyQuotaUsed),
			zap.Int("quota_limit", channelQuota.DailyQuotaLimit),
		)
		return MethodSkip
	}

	return MethodFullDetection
}

// performStatusCheck performs lightweight status check (1 quota unit)
func (d *LiveStreamDetector) performStatusCheck(
	ctx context.Context,
	channelID string,
	channelQuota *quota.ChannelQuota,
) (*models.YouTubeStream, error) {
	if channelQuota == nil || channelQuota.CachedVideoID == nil {
		return nil, fmt.Errorf("no cached video ID for status check")
	}

	videoID := *channelQuota.CachedVideoID

	// Check quota availability (1 unit)
	canUse, err := d.quotaTracker.CanUseQuota(ctx, channelID, 1)
	if err != nil || !canUse {
		d.logger.Warn("Insufficient quota for status check",
			zap.String("channel_id", channelID),
			zap.Error(err),
		)
		return nil, fmt.Errorf("insufficient quota for status check")
	}

	// Perform status check using existing CheckStreamStatus method
	statusResult, err := d.client.CheckStreamStatus(ctx, videoID)

	// Record quota usage (1 unit)
	if recordErr := d.quotaTracker.RecordUsage(ctx, channelID, 1); recordErr != nil {
		d.logger.Warn("Failed to record status check quota usage",
			zap.String("channel_id", channelID),
			zap.Error(recordErr),
		)
	}

	if err != nil {
		// Status check failed - record failure
		if recordErr := d.quotaTracker.RecordStatusCheckFailure(ctx, channelID); recordErr != nil {
			d.logger.Warn("Failed to record status check failure",
				zap.String("channel_id", channelID),
				zap.Error(recordErr),
			)
		}

		d.logger.Warn("Status check failed",
			zap.String("channel_id", channelID),
			zap.String("video_id", videoID),
			zap.Error(err),
		)

		return nil, fmt.Errorf("status check failed: %w", err)
	}

	// Reset failure counter on success
	if resetErr := d.quotaTracker.ResetStatusCheckFailures(ctx, channelID); resetErr != nil {
		d.logger.Warn("Failed to reset status check failures",
			zap.String("channel_id", channelID),
			zap.Error(resetErr),
		)
	}

	if !statusResult.IsLive {
		// Stream is offline
		d.logger.Debug("Status check: stream offline",
			zap.String("channel_id", channelID),
			zap.String("video_id", videoID),
		)

		// Record offline check
		if err := d.quotaTracker.RecordOfflineCheck(ctx, channelID); err != nil {
			d.logger.Warn("Failed to record offline check",
				zap.String("channel_id", channelID),
				zap.Error(err),
			)
		}

		return nil, nil // No error, just not live
	}

	// Stream is live! Fetch additional details (costs 1 more unit)
	streamDetails, err := d.client.GetVideoDetails(ctx, videoID)
	if err != nil {
		d.logger.Warn("Failed to fetch stream details, continuing with minimal info",
			zap.String("channel_id", channelID),
			zap.String("video_id", videoID),
			zap.Error(err),
		)
		// Continue anyway with minimal info
		streamDetails = &models.YouTubeStream{
			VideoID:   videoID,
			ChannelID: channelID,
		}
	} else {
		// Record additional quota usage for GetVideoDetails (1 unit)
		if recordErr := d.quotaTracker.RecordUsage(ctx, channelID, 1); recordErr != nil {
			d.logger.Warn("Failed to record GetVideoDetails quota usage",
				zap.String("channel_id", channelID),
				zap.Error(recordErr),
			)
		}
	}

	d.logger.Info("Status check: stream is live",
		zap.String("channel_id", channelID),
		zap.String("video_id", videoID),
		zap.String("title", streamDetails.Title),
	)

	// Update cached video info
	if err := d.quotaTracker.UpdateCachedVideoID(ctx, channelID, videoID, streamDetails.Title); err != nil {
		d.logger.Warn("Failed to update cached video ID",
			zap.String("channel_id", channelID),
			zap.Error(err),
		)
	}

	// Promote channel to high tier
	if err := d.quotaTracker.PromoteChannelTier(ctx, channelID); err != nil {
		d.logger.Warn("Failed to promote channel tier",
			zap.String("channel_id", channelID),
			zap.Error(err),
		)
	}

	return streamDetails, nil
}

// performFullDetection performs full stream detection (100 quota units)
func (d *LiveStreamDetector) performFullDetection(
	ctx context.Context,
	channelID string,
) (*models.YouTubeStream, error) {
	// Check quota availability (100 units)
	canUse, err := d.quotaTracker.CanUseQuota(ctx, channelID, 100)
	if err != nil || !canUse {
		d.logger.Warn("Insufficient quota for full detection",
			zap.String("channel_id", channelID),
			zap.Error(err),
		)
		return nil, fmt.Errorf("insufficient quota for full detection")
	}

	// Perform full detection using search.list API
	streams, err := d.client.GetLiveStreams(ctx, channelID)

	// Record quota usage (100 units for search.list)
	if recordErr := d.quotaTracker.RecordUsage(ctx, channelID, 100); recordErr != nil {
		d.logger.Warn("Failed to record full detection quota usage",
			zap.String("channel_id", channelID),
			zap.Error(recordErr),
		)
	}

	if err != nil {
		d.logger.Error("Full detection failed",
			zap.String("channel_id", channelID),
			zap.Error(err),
		)
		return nil, fmt.Errorf("full detection failed: %w", err)
	}

	// No live streams found
	if len(streams) == 0 {
		d.logger.Debug("Full detection: no live streams",
			zap.String("channel_id", channelID),
		)

		// Record offline check
		if err := d.quotaTracker.RecordOfflineCheck(ctx, channelID); err != nil {
			d.logger.Warn("Failed to record offline check",
				zap.String("channel_id", channelID),
				zap.Error(err),
			)
		}

		return nil, nil // No error, just not live
	}

	// Found live stream(s) - use first one
	stream := streams[0]

	d.logger.Info("Full detection: stream is live",
		zap.String("channel_id", channelID),
		zap.String("video_id", stream.VideoID),
		zap.String("title", stream.Title),
	)

	// Cache video ID for future status checks
	if err := d.quotaTracker.UpdateCachedVideoID(ctx, channelID, stream.VideoID, stream.Title); err != nil {
		d.logger.Warn("Failed to cache video ID",
			zap.String("channel_id", channelID),
			zap.Error(err),
		)
	}

	// Promote channel to high tier
	if err := d.quotaTracker.PromoteChannelTier(ctx, channelID); err != nil {
		d.logger.Warn("Failed to promote channel tier",
			zap.String("channel_id", channelID),
			zap.Error(err),
		)
	}

	return stream, nil
}

// RecordStreamEnd records that a stream ended (for backoff reset)
func (d *LiveStreamDetector) RecordStreamEnd(ctx context.Context, channelID string) error {
	// Clear cached video ID
	if err := d.quotaTracker.ClearCachedVideoID(ctx, channelID); err != nil {
		return fmt.Errorf("failed to clear cached video ID: %w", err)
	}

	d.logger.Info("Recorded stream end",
		zap.String("channel_id", channelID),
	)

	return nil
}

// methodName returns human-readable method name for logging
func (d *LiveStreamDetector) methodName(method DetectionMethod) string {
	switch method {
	case MethodStatusCheck:
		return "status_check"
	case MethodFullDetection:
		return "full_detection"
	case MethodSkip:
		return "skip"
	default:
		return "unknown"
	}
}

// getCachedVideoID safely gets cached video ID from quota
func (d *LiveStreamDetector) getCachedVideoID(channelQuota *quota.ChannelQuota) string {
	if channelQuota == nil || channelQuota.CachedVideoID == nil {
		return ""
	}
	return *channelQuota.CachedVideoID
}

// GetDetectionStats returns statistics about detection methods used
type DetectionStats struct {
	StatusCheckCount    int
	FullDetectionCount  int
	StatusCheckSuccess  int
	FullDetectionSuccess int
	QuotaSaved          int // Units saved by using status check vs full detection
}

// Note: Stats tracking would be implemented with Prometheus metrics in production
