package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/caesar/all-chat/test/shared"
	"go.uber.org/zap"
)

func TestArtifactWriter_CaptureArtifact(t *testing.T) {
	// Create temp directory for artifacts
	tmpDir := t.TempDir()

	logger := zap.NewNop()
	writer, err := NewArtifactWriter(tmpDir, logger)
	if err != nil {
		t.Fatalf("Failed to create artifact writer: %v", err)
	}

	// Create test mismatch
	timestamp := time.Now()
	mismatch := shared.MismatchDetail{
		Type:        "missing_innertube",
		Fingerprint: "test123",
		Timestamp:   timestamp,
		OfficialMessage: &shared.RawChatMessage{
			MessageID: "msg-1",
			Username:  "testuser",
			Text:      "Test message",
			Timestamp: timestamp,
		},
		InnertubeMessage: nil,
		FieldDifferences: nil,
	}

	context := SurroundingContext{
		Before: []*shared.RawChatMessage{},
		After:  []*shared.RawChatMessage{},
	}

	// Capture artifact
	if err := writer.CaptureArtifact(mismatch, context); err != nil {
		t.Fatalf("Failed to capture artifact: %v", err)
	}

	// Verify artifact file was created
	files, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to read temp dir: %v", err)
	}

	if len(files) != 1 {
		t.Errorf("Expected 1 artifact file, got %d", len(files))
	}

	// Verify filename format
	if len(files) > 0 {
		filename := files[0].Name()
		if !contains(filename, "mismatch_") || !contains(filename, "missing_innertube") {
			t.Errorf("Unexpected filename format: %s", filename)
		}
	}

	// Verify artifact paths tracked
	if len(writer.artifactPaths) != 1 {
		t.Errorf("Expected 1 artifact path tracked, got %d", len(writer.artifactPaths))
	}
}

func TestArtifactWriter_WriteFinalReport(t *testing.T) {
	// Create temp directory for reports
	tmpDir := t.TempDir()

	logger := zap.NewNop()
	writer, err := NewArtifactWriter(tmpDir, logger)
	if err != nil {
		t.Fatalf("Failed to create artifact writer: %v", err)
	}

	// Create test stats (pass threshold < 0.1%)
	stats := ComparisonStats{
		TotalProcessed:       10000,
		TotalMatched:         9995,
		TotalMissingInner:    3,
		TotalMissingOfficial: 1,
		TotalContentMismatch: 1,
		StartTime:            time.Now().Add(-24 * time.Hour),
		LastProcessedTime:    time.Now(),
	}
	stats.CurrentMismatchRate = float64(5) / float64(10000) // 0.05% < 0.1% threshold

	// Write final report
	if err := writer.WriteFinalReport(stats); err != nil {
		t.Fatalf("Failed to write final report: %v", err)
	}

	// Verify JSON report exists
	jsonPath := filepath.Join(tmpDir, "final_report.json")
	if _, err := os.Stat(jsonPath); os.IsNotExist(err) {
		t.Errorf("JSON report not created: %s", jsonPath)
	}

	// Verify Markdown report exists
	mdPath := filepath.Join(tmpDir, "REPORT.md")
	if _, err := os.Stat(mdPath); os.IsNotExist(err) {
		t.Errorf("Markdown report not created: %s", mdPath)
	}

	// Read and verify Markdown content
	content, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatalf("Failed to read Markdown report: %v", err)
	}

	contentStr := string(content)
	if !contains(contentStr, "Total Messages") {
		t.Error("Markdown report missing 'Total Messages'")
	}
	if !contains(contentStr, "10000") {
		t.Error("Markdown report missing total message count")
	}
	if !contains(contentStr, "TEST PASSED") {
		t.Error("Markdown report missing test result")
	}
}

func TestComparisonStats_MismatchRateCalculation(t *testing.T) {
	stats := ComparisonStats{
		TotalProcessed:       1000,
		TotalMatched:         995,
		TotalMissingInner:    3,
		TotalMissingOfficial: 1,
		TotalContentMismatch: 1,
	}

	// Calculate mismatch rate
	totalMismatches := stats.TotalMissingInner + stats.TotalMissingOfficial + stats.TotalContentMismatch
	stats.CurrentMismatchRate = float64(totalMismatches) / float64(stats.TotalProcessed)

	expectedRate := 5.0 / 1000.0 // 0.5%
	if stats.CurrentMismatchRate != expectedRate {
		t.Errorf("Expected mismatch rate %f, got %f", expectedRate, stats.CurrentMismatchRate)
	}

	// Verify threshold check
	threshold := 0.001 // 0.1%
	if stats.CurrentMismatchRate < threshold {
		t.Error("Expected mismatch rate to exceed threshold")
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
