// This file is part of All-Chat.
// Copyright (C) 2026 caesarakalaeii
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func main() {
	// Parse command-line flags
	duration := flag.Duration("duration", 24*time.Hour, "Test duration (default 24h)")
	redisHost := flag.String("redis-host", "localhost:6379", "Redis host:port")
	outputDir := flag.String("output-dir", "./artifacts", "Output directory for artifacts")
	officialPrefix := flag.String("official-prefix", "official", "Redis stream prefix for official listener")
	innertubePrefix := flag.String("innertube-prefix", "innertube", "Redis stream prefix for InnerTube listener")
	flag.Parse()

	// Initialize logger
	logger, err := zap.NewProduction()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Info("Starting dual-listener integration test",
		zap.Duration("duration", *duration),
		zap.String("redis_host", *redisHost),
		zap.String("output_dir", *outputDir),
		zap.String("official_prefix", *officialPrefix),
		zap.String("innertube_prefix", *innertubePrefix),
	)

	// Connect to Redis
	redisClient := redis.NewClient(&redis.Options{
		Addr: *redisHost,
	})
	defer redisClient.Close()

	// Test Redis connection
	ctx := context.Background()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		logger.Fatal("Failed to connect to Redis", zap.Error(err))
	}
	logger.Info("Connected to Redis", zap.String("host", *redisHost))

	// Create artifact writer
	artifactWriter, err := NewArtifactWriter(*outputDir, logger)
	if err != nil {
		logger.Fatal("Failed to create artifact writer", zap.Error(err))
	}

	// Create comparator
	streamPrefix := StreamPrefix{
		Official:  *officialPrefix,
		InnerTube: *innertubePrefix,
	}
	comparator := NewComparator(redisClient, streamPrefix, artifactWriter, logger)

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

	// Run comparison in goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- comparator.Run(ctx, *duration)
	}()

	// Wait for completion or signal
	select {
	case err := <-errChan:
		if err != nil {
			logger.Error("Comparison failed", zap.Error(err))
			os.Exit(1)
		}
		logger.Info("Comparison completed successfully")

	case sig := <-sigChan:
		logger.Info("Received signal, initiating graceful shutdown", zap.String("signal", sig.String()))
		// Comparator will handle graceful shutdown via context
		if err := <-errChan; err != nil {
			logger.Warn("Shutdown with error", zap.Error(err))
		}
	}

	// Get final stats and determine exit code
	stats := comparator.GetStats()
	threshold := 0.001 // 0.1%

	if stats.CurrentMismatchRate < threshold {
		logger.Info("TEST RESULT: PASS",
			zap.Float64("mismatch_rate_pct", stats.CurrentMismatchRate*100),
			zap.Float64("threshold_pct", threshold*100),
			zap.Int("total_messages", stats.TotalProcessed),
		)
		os.Exit(0)
	} else {
		logger.Error("TEST RESULT: FAIL",
			zap.Float64("mismatch_rate_pct", stats.CurrentMismatchRate*100),
			zap.Float64("threshold_pct", threshold*100),
			zap.Int("total_messages", stats.TotalProcessed),
			zap.Int("mismatches", stats.TotalMissingInner+stats.TotalMissingOfficial+stats.TotalContentMismatch),
		)
		os.Exit(1)
	}
}
