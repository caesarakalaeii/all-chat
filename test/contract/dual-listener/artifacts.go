package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/caesar/all-chat/test/shared"
	"go.uber.org/zap"
)

// ArtifactWriter handles writing mismatch artifacts and final reports
type ArtifactWriter struct {
	outputDir     string
	logger        *zap.Logger
	artifactPaths []string
}

// SurroundingContext holds messages before/after a mismatch
type SurroundingContext struct {
	Before []*shared.RawChatMessage `json:"before"`
	After  []*shared.RawChatMessage `json:"after"`
}

// LatencyMetrics captures timing information for mismatch analysis
type LatencyMetrics struct {
	OfficialReceivedAt  *time.Time `json:"official_received_at,omitempty"`
	InnertubeReceivedAt *time.Time `json:"innertube_received_at,omitempty"`
	LatencyDelta        *float64   `json:"latency_delta_ms,omitempty"` // milliseconds
}

// MismatchArtifact represents a single mismatch with full context
type MismatchArtifact struct {
	Type              string                    `json:"type"`
	Timestamp         time.Time                 `json:"timestamp"`
	OfficialMessage   *shared.RawChatMessage    `json:"official_message"`
	InnertubeMessage  *shared.RawChatMessage    `json:"innertube_message"`
	FieldDifferences  map[string]shared.FieldDiff `json:"field_differences,omitempty"`
	SurroundingContext SurroundingContext       `json:"surrounding_context"`
	LatencyMetrics    LatencyMetrics            `json:"latency_metrics"`
	Fingerprint       string                    `json:"fingerprint"`
}

// FinalReport contains overall test results and validation
type FinalReport struct {
	TestDurationHours    float64  `json:"test_duration_hours"`
	TotalMessages        int      `json:"total_messages"`
	Matched              int      `json:"matched"`
	MissingInnerTube     int      `json:"missing_innertube"`
	MissingOfficial      int      `json:"missing_official"`
	ContentMismatches    int      `json:"content_mismatches"`
	MismatchRate         float64  `json:"mismatch_rate"`
	ThresholdMet         bool     `json:"threshold_met"`
	ArtifactCount        int      `json:"artifact_count"`
	ArtifactPaths        []string `json:"artifact_paths"`
	StartTime            time.Time `json:"start_time"`
	EndTime              time.Time `json:"end_time"`
}

// NewArtifactWriter creates a new artifact writer
func NewArtifactWriter(outputDir string, logger *zap.Logger) (*ArtifactWriter, error) {
	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("create output directory: %w", err)
	}

	return &ArtifactWriter{
		outputDir:     outputDir,
		logger:        logger,
		artifactPaths: make([]string, 0),
	}, nil
}

// CaptureArtifact writes a mismatch artifact with full context
func (w *ArtifactWriter) CaptureArtifact(mismatch shared.MismatchDetail, context SurroundingContext) error {
	// Calculate latency metrics
	latency := w.calculateLatency(mismatch)

	// Build artifact
	artifact := MismatchArtifact{
		Type:               mismatch.Type,
		Timestamp:          mismatch.Timestamp,
		OfficialMessage:    mismatch.OfficialMessage,
		InnertubeMessage:   mismatch.InnertubeMessage,
		FieldDifferences:   mismatch.FieldDifferences,
		SurroundingContext: context,
		LatencyMetrics:     latency,
		Fingerprint:        mismatch.Fingerprint,
	}

	// Generate filename
	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("mismatch_%s_%s.json", timestamp, mismatch.Type)
	filepath := filepath.Join(w.outputDir, filename)

	// Write JSON
	data, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal artifact: %w", err)
	}

	if err := os.WriteFile(filepath, data, 0644); err != nil {
		return fmt.Errorf("write artifact file: %w", err)
	}

	w.artifactPaths = append(w.artifactPaths, filepath)
	w.logger.Debug("Captured mismatch artifact",
		zap.String("type", mismatch.Type),
		zap.String("path", filepath),
		zap.String("fingerprint", mismatch.Fingerprint),
	)

	return nil
}

// calculateLatency extracts timing information from messages
func (w *ArtifactWriter) calculateLatency(mismatch shared.MismatchDetail) LatencyMetrics {
	metrics := LatencyMetrics{}

	if mismatch.OfficialMessage != nil {
		metrics.OfficialReceivedAt = &mismatch.OfficialMessage.Timestamp
	}

	if mismatch.InnertubeMessage != nil {
		metrics.InnertubeReceivedAt = &mismatch.InnertubeMessage.Timestamp
	}

	// Calculate delta if both timestamps available
	if metrics.OfficialReceivedAt != nil && metrics.InnertubeReceivedAt != nil {
		delta := metrics.InnertubeReceivedAt.Sub(*metrics.OfficialReceivedAt).Milliseconds()
		deltaFloat := float64(delta)
		metrics.LatencyDelta = &deltaFloat
	}

	return metrics
}

// WriteFinalReport generates the final test report (JSON + Markdown)
func (w *ArtifactWriter) WriteFinalReport(stats ComparisonStats) error {
	duration := time.Since(stats.StartTime)
	durationHours := duration.Hours()

	// Calculate threshold validation
	threshold := 0.001 // 0.1%
	thresholdMet := stats.CurrentMismatchRate < threshold

	// Build report
	report := FinalReport{
		TestDurationHours: durationHours,
		TotalMessages:     stats.TotalProcessed,
		Matched:           stats.TotalMatched,
		MissingInnerTube:  stats.TotalMissingInner,
		MissingOfficial:   stats.TotalMissingOfficial,
		ContentMismatches: stats.TotalContentMismatch,
		MismatchRate:      stats.CurrentMismatchRate,
		ThresholdMet:      thresholdMet,
		ArtifactCount:     len(w.artifactPaths),
		ArtifactPaths:     w.artifactPaths,
		StartTime:         stats.StartTime,
		EndTime:           time.Now(),
	}

	// Write JSON report
	if err := w.writeJSONReport(report); err != nil {
		return fmt.Errorf("write JSON report: %w", err)
	}

	// Write Markdown report
	if err := w.writeMarkdownReport(report); err != nil {
		return fmt.Errorf("write Markdown report: %w", err)
	}

	w.logger.Info("Final report written",
		zap.String("json", filepath.Join(w.outputDir, "final_report.json")),
		zap.String("markdown", filepath.Join(w.outputDir, "REPORT.md")),
	)

	return nil
}

// writeJSONReport writes the final report as JSON
func (w *ArtifactWriter) writeJSONReport(report FinalReport) error {
	filepath := filepath.Join(w.outputDir, "final_report.json")

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath, data, 0644)
}

// writeMarkdownReport writes a human-readable Markdown report
func (w *ArtifactWriter) writeMarkdownReport(report FinalReport) error {
	reportPath := filepath.Join(w.outputDir, "REPORT.md")

	// Generate Markdown content
	content := fmt.Sprintf(`# Dual-Listener Integration Test Report

## Test Configuration

- **Duration**: %.2f hours
- **Start Time**: %s
- **End Time**: %s

## Results Summary

| Metric | Count |
|--------|-------|
| Total Messages | %d |
| Matched | %d |
| Missing in InnerTube | %d |
| Missing in Official | %d |
| Content Mismatches | %d |

## Mismatch Analysis

- **Mismatch Rate**: %.4f%% (%.0f mismatches / %d total)
- **Threshold**: 0.1%%
- **Result**: %s

## Mismatch Breakdown

- **Missing in InnerTube**: %d (%.4f%%)
- **Missing in Official**: %d (%.4f%%)
- **Content Differences**: %d (%.4f%%)

## Artifacts

- **Total Artifacts**: %d
- **Artifact Directory**: %s

`,
		report.TestDurationHours,
		report.StartTime.Format(time.RFC3339),
		report.EndTime.Format(time.RFC3339),
		report.TotalMessages,
		report.Matched,
		report.MissingInnerTube,
		report.MissingOfficial,
		report.ContentMismatches,
		report.MismatchRate*100,
		float64(report.MissingInnerTube+report.MissingOfficial+report.ContentMismatches),
		report.TotalMessages,
		w.formatResult(report.ThresholdMet),
		report.MissingInnerTube,
		w.calculatePercentage(report.MissingInnerTube, report.TotalMessages),
		report.MissingOfficial,
		w.calculatePercentage(report.MissingOfficial, report.TotalMessages),
		report.ContentMismatches,
		w.calculatePercentage(report.ContentMismatches, report.TotalMessages),
		report.ArtifactCount,
		w.outputDir,
	)

	// Add artifact list
	if len(report.ArtifactPaths) > 0 {
		content += "### Artifact Files\n\n"
		for _, path := range report.ArtifactPaths {
			content += fmt.Sprintf("- `%s`\n", filepath.Base(path))
		}
		content += "\n"
	}

	// Add conclusion
	content += "## Conclusion\n\n"
	if report.ThresholdMet {
		content += fmt.Sprintf("✅ **TEST PASSED**: Mismatch rate %.4f%% < 0.1%% threshold\n\n", report.MismatchRate*100)
		content += "Behavioral equivalence validated. InnerTube listener is ready for production.\n"
	} else {
		content += fmt.Sprintf("❌ **TEST FAILED**: Mismatch rate %.4f%% > 0.1%% threshold\n\n", report.MismatchRate*100)
		content += "Review mismatch artifacts to identify behavioral differences before production rollout.\n"
	}

	return os.WriteFile(reportPath, []byte(content), 0644)
}

// formatResult returns a colored emoji result string
func (w *ArtifactWriter) formatResult(passed bool) string {
	if passed {
		return "✅ PASSED"
	}
	return "❌ FAILED"
}

// calculatePercentage returns percentage of count/total
func (w *ArtifactWriter) calculatePercentage(count, total int) float64 {
	if total == 0 {
		return 0.0
	}
	return (float64(count) / float64(total)) * 100
}
