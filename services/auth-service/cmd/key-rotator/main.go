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

// key-rotator is a Kubernetes-Job-ready binary that re-encrypts ciphertext columns
// in all token tables to the current MultiKeyEncryptor.CurrentKid().
//
// Implements D-03 (lazy + scheduled background sweeper) and D-06 (sweeper as its
// own lightweight cmd binary).
//
// Usage:
//
//	DATABASE_URL=postgres://... TOKEN_ENCRYPTION_KEY_V1=... key-rotator \
//	    [--dry-run] [--batch-size=100] [--batch-delay-ms=50] \
//	    [--skip-table=tiktok_oauth_tokens] [--skip-table=...]
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/caesar/all-chat/shared/encryption"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// rotatorOptions holds all parsed flags and validated env values.
type rotatorOptions struct {
	dryRun       bool
	batchSize    int
	batchDelayMs int
	skipTables   []string
	dbURL        string
}

// repeatedString is a flag.Value that accumulates multiple --skip-table values.
type repeatedString []string

func (r *repeatedString) String() string { return strings.Join(*r, ",") }
func (r *repeatedString) Set(v string) error {
	*r = append(*r, v)
	return nil
}

// parseFlags parses argv and validates required env vars.
// Returns the options struct and any parse/validation error.
// Uses a fresh FlagSet so tests can call it repeatedly without os.Exit side-effects.
func parseFlags(args []string, env map[string]string) (*rotatorOptions, error) {
	fs := flag.NewFlagSet("key-rotator", flag.ContinueOnError)

	dryRun := fs.Bool("dry-run", false, "log rows that would be updated without mutating the database")
	batchSize := fs.Int("batch-size", 100, "rows per UPDATE batch")
	batchDelayMs := fs.Int("batch-delay-ms", 50, "milliseconds between batches to throttle DB load")
	var skipTables repeatedString
	fs.Var(&skipTables, "skip-table", "table to skip (repeatable; e.g. --skip-table=tiktok_oauth_tokens)")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	dbURL := env["DATABASE_URL"]
	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL must be set")
	}

	return &rotatorOptions{
		dryRun:       *dryRun,
		batchSize:    *batchSize,
		batchDelayMs: *batchDelayMs,
		skipTables:   []string(skipTables),
		dbURL:        dbURL,
	}, nil
}

// envMap builds a map from os.Environ() for run().
func envMap() map[string]string {
	m := make(map[string]string)
	for _, kv := range os.Environ() {
		if idx := strings.Index(kv, "="); idx >= 0 {
			m[kv[:idx]] = kv[idx+1:]
		}
	}
	return m
}

// run is the testable entry point. Returns an exit code (0 = success).
func run(ctx context.Context, args []string, env map[string]string) int {
	logger, err := zap.NewProduction()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "failed to build logger: %v\n", err)
		return 1
	}
	defer logger.Sync() //nolint:errcheck

	opts, err := parseFlags(args, env)
	if err != nil {
		logger.Error("flag parse / env validation failed", zap.Error(err))
		return 1
	}

	// Initialise encryption from env vars (TOKEN_ENCRYPTION_KEY_V1, _V2, …).
	// We temporarily set env vars from the map so NewMultiKeyEncryptorFromEnv() works.
	// In production, env vars are already set by the K8s Job spec.
	for k, v := range env {
		if err := os.Setenv(k, v); err != nil {
			logger.Error("failed to set env var", zap.String("key", k), zap.Error(err))
			return 1
		}
	}

	encryptor, err := encryption.NewMultiKeyEncryptorFromEnv()
	if err != nil {
		logger.Error("encryption init failed", zap.Error(err))
		return 1
	}

	pool, err := pgxpool.New(ctx, opts.dbURL)
	if err != nil {
		logger.Error("db pool init failed", zap.Error(err))
		return 1
	}
	defer pool.Close()

	sweepOpts := []SweeperOption{
		WithDryRun(opts.dryRun),
		WithBatchSize(opts.batchSize),
		WithBatchDelay(time.Duration(opts.batchDelayMs) * time.Millisecond),
	}
	for _, t := range opts.skipTables {
		sweepOpts = append(sweepOpts, WithSkipTable(t))
	}

	sweeper := NewSweeper(pool, encryptor, logger, sweepOpts...)

	logger.Info("starting key-rotator sweep",
		zap.Bool("dry_run", opts.dryRun),
		zap.Int("batch_size", opts.batchSize),
		zap.Duration("batch_delay", time.Duration(opts.batchDelayMs)*time.Millisecond),
		zap.Strings("skip_tables", opts.skipTables),
		zap.Uint8("current_kid", encryptor.CurrentKid()),
	)

	if err := sweeper.SweepAll(ctx); err != nil {
		logger.Error("sweep failed", zap.Error(err))
		return 1
	}

	logger.Info("sweep complete",
		zap.Any("rows_scanned", sweeper.metrics.RowsScanned),
		zap.Any("rows_re_encrypted", sweeper.metrics.RowsReEncrypted),
		zap.Any("rows_skipped", sweeper.metrics.RowsSkipped),
		zap.Any("errors", sweeper.metrics.Errors),
	)
	return 0
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	os.Exit(run(ctx, os.Args[1:], envMap()))
}
