package ranker

import (
	"math"
	"sort"
	"time"

	"github.com/caesar/all-chat/services/clip-manager/models"
	"go.uber.org/zap"
)

// ClipRanker ranks clips based on views, recency, and duration
type ClipRanker struct {
	logger *zap.Logger
}

// NewClipRanker creates a new clip ranker
func NewClipRanker(logger *zap.Logger) *ClipRanker {
	return &ClipRanker{
		logger: logger,
	}
}

// RankingWeights defines the weights for ranking algorithm
type RankingWeights struct {
	ViewScore     float64 // Weight for view count (0-1)
	RecencyScore  float64 // Weight for recency (0-1)
	DurationScore float64 // Weight for ideal duration (0-1)
}

// DefaultWeights returns the default ranking weights
func DefaultWeights() RankingWeights {
	return RankingWeights{
		ViewScore:     0.6, // 60% weight on views
		RecencyScore:  0.3, // 30% weight on recency
		DurationScore: 0.1, // 10% weight on duration
	}
}

// RankClips calculates rank scores for clips and sorts them
func (r *ClipRanker) RankClips(clips []models.Clip, streamStart, streamEnd time.Time) []models.Clip {
	if len(clips) == 0 {
		return clips
	}

	weights := DefaultWeights()

	// Find max views for normalization
	maxViews := 0
	for _, clip := range clips {
		if clip.ViewCount > maxViews {
			maxViews = clip.ViewCount
		}
	}

	if maxViews == 0 {
		maxViews = 1 // Avoid division by zero
	}

	// Calculate stream duration
	streamDuration := streamEnd.Sub(streamStart).Seconds()
	if streamDuration <= 0 {
		streamDuration = 1 // Fallback
	}

	// Calculate scores for each clip
	for i := range clips {
		clip := &clips[i]

		// View score (normalized 0-100)
		viewScore := (float64(clip.ViewCount) / float64(maxViews)) * 100

		// Recency score (clips created early in stream rank higher)
		recencyScore := 100.0
		if clip.CreatedAtPlatform != nil {
			timeSinceStart := clip.CreatedAtPlatform.Sub(streamStart).Seconds()
			if timeSinceStart >= 0 && timeSinceStart <= streamDuration {
				// Clips from first 20% of stream get highest recency score
				recencyScore = math.Max(0, 100-(timeSinceStart/streamDuration)*100)
			}
		}

		// Duration score (prefer 15-45 second clips, ideal = 30s)
		durationScore := 100.0
		if clip.DurationSeconds != nil {
			idealDuration := 30.0
			durationDiff := math.Abs(float64(*clip.DurationSeconds) - idealDuration)
			// Penalty increases with distance from ideal
			durationScore = math.Max(0, 100-(durationDiff*3))
		}

		// Calculate final score
		finalScore := (viewScore * weights.ViewScore) +
			(recencyScore * weights.RecencyScore) +
			(durationScore * weights.DurationScore)

		clip.RankScore = &finalScore

		r.logger.Debug("Clip ranked",
			zap.String("clip_id", clip.ID.String()),
			zap.String("title", clipTitle(clip.Title)),
			zap.Float64("view_score", viewScore),
			zap.Float64("recency_score", recencyScore),
			zap.Float64("duration_score", durationScore),
			zap.Float64("final_score", finalScore),
		)
	}

	// Sort by score descending
	sort.Slice(clips, func(i, j int) bool {
		if clips[i].RankScore == nil {
			return false
		}
		if clips[j].RankScore == nil {
			return true
		}
		return *clips[i].RankScore > *clips[j].RankScore
	})

	return clips
}

// SelectClips selects the best clips with platform diversification
func (r *ClipRanker) SelectClips(rankedClips []models.Clip, maxClips int, preferDiversity bool) []models.Clip {
	if len(rankedClips) <= maxClips {
		return rankedClips
	}

	selected := make([]models.Clip, 0, maxClips)
	lastPlatform := ""

	for _, clip := range rankedClips {
		// If diversity is preferred, avoid consecutive clips from same platform
		if preferDiversity && clip.Platform == lastPlatform && len(rankedClips) > maxClips*2 {
			continue
		}

		selected = append(selected, clip)
		lastPlatform = clip.Platform

		if len(selected) >= maxClips {
			break
		}
	}

	r.logger.Info("Selected clips",
		zap.Int("selected", len(selected)),
		zap.Int("from_total", len(rankedClips)),
	)

	return selected
}

// FilterClipsByDuration filters clips by duration range
func (r *ClipRanker) FilterClipsByDuration(clips []models.Clip, minDuration, maxDuration int) []models.Clip {
	filtered := make([]models.Clip, 0, len(clips))

	for _, clip := range clips {
		if clip.DurationSeconds == nil {
			continue
		}

		duration := *clip.DurationSeconds
		if duration >= minDuration && duration <= maxDuration {
			filtered = append(filtered, clip)
		}
	}

	r.logger.Debug("Filtered clips by duration",
		zap.Int("original", len(clips)),
		zap.Int("filtered", len(filtered)),
		zap.Int("min_duration", minDuration),
		zap.Int("max_duration", maxDuration),
	)

	return filtered
}

func clipTitle(title *string) string {
	if title == nil {
		return "(untitled)"
	}
	return *title
}
