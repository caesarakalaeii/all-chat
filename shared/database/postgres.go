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

package database

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Connection-pool defaults. Every service instance holds its own pool, so the
// cluster-wide connection budget is (number of instances x MaxConns) and must
// stay under the database's max_connections. These defaults are deliberately
// small: a previous MinConns=5 across ~40 instances held ~200 permanently-idle
// connections and pinned Postgres at its 200-connection ceiling, crashlooping
// newly-starting pods with "remaining connection slots are reserved..."
// (SQLSTATE 53300) on any mass restart. Raise per service with
// DATABASE_MAX_CONNS / DATABASE_MIN_CONNS when a workload genuinely needs more.
// See ADR-0039.
const (
	defaultMaxConns = 10
	defaultMinConns = 1
)

// buildPoolConfig parses the connection string and applies the shared pool
// tuning. It is separated from pool creation so it can be unit-tested without a
// live database.
func buildPoolConfig(connString string) (*pgxpool.Config, error) {
	config, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse connection string: %w", err)
	}

	// Connection pool configuration (see the connection-budget note above).
	config.MaxConns = int32(envInt("DATABASE_MAX_CONNS", defaultMaxConns))
	config.MinConns = int32(envInt("DATABASE_MIN_CONNS", defaultMinConns))
	config.MaxConnLifetime = 1 * time.Hour     // Recycle connections after 1 hour
	config.MaxConnIdleTime = 10 * time.Minute  // Close idle connections down to MinConns
	config.HealthCheckPeriod = 1 * time.Minute // Verify connection health

	// Tag connections so pg_stat_activity can attribute them to a service
	// (they were previously anonymous, making pool leaks impossible to trace).
	if name := applicationName(); name != "" {
		if config.ConnConfig.RuntimeParams == nil {
			config.ConnConfig.RuntimeParams = map[string]string{}
		}
		config.ConnConfig.RuntimeParams["application_name"] = name
	}

	return config, nil
}

// NewPostgresPoolWithTracing creates a PostgreSQL pool with optional OpenTelemetry tracing
func NewPostgresPoolWithTracing(connString string, tracingEnabled bool) (*pgxpool.Pool, error) {
	config, err := buildPoolConfig(connString)
	if err != nil {
		return nil, err
	}

	// Add OpenTelemetry tracer if enabled
	if tracingEnabled {
		config.ConnConfig.Tracer = otelpgx.NewTracer()
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return pool, nil
}

// NewPostgresPool creates a PostgreSQL pool without tracing (backward compatibility)
func NewPostgresPool(connString string) (*pgxpool.Pool, error) {
	return NewPostgresPoolWithTracing(connString, false)
}

// RetryOptions configures startup connection retry with exponential backoff.
// Mirrors shared/redis.RetryOptions so services connect to Postgres and Redis
// with the same resilience semantics.
type RetryOptions struct {
	// MaxAttempts is the maximum number of connection attempts. A value <= 0
	// means retry until the supplied context is cancelled.
	MaxAttempts int
	// InitialBackoff is the wait before the second attempt. Defaults to 1s.
	InitialBackoff time.Duration
	// MaxBackoff caps the backoff between attempts. Defaults to 30s.
	MaxBackoff time.Duration
}

// DefaultRetryOptions returns sensible startup-retry defaults: retry indefinitely
// (bounded only by the caller's context) with backoff growing from 1s to 30s.
// This keeps a service alive — reporting not-ready via its readiness probe —
// while PostgreSQL is briefly unavailable (e.g. a CNPG primary failover or the
// pod being rescheduled), instead of crash-looping on a fatal connection error.
func DefaultRetryOptions() RetryOptions {
	return RetryOptions{
		MaxAttempts:    0,
		InitialBackoff: 1 * time.Second,
		MaxBackoff:     30 * time.Second,
	}
}

// NewPostgresPoolWithRetry behaves like NewPostgresPoolWithTracing but retries the
// initial connection (pool create + ping) with exponential backoff instead of
// returning on the first failure. It stops and returns an error when MaxAttempts
// is exhausted or ctx is cancelled. onRetry, if non-nil, is invoked after each
// failed attempt (before the backoff sleep) so callers can log progress.
func NewPostgresPoolWithRetry(ctx context.Context, connString string, tracingEnabled bool, opts RetryOptions, onRetry func(attempt int, err error, backoff time.Duration)) (*pgxpool.Pool, error) {
	if opts.InitialBackoff <= 0 {
		opts.InitialBackoff = 1 * time.Second
	}
	if opts.MaxBackoff <= 0 {
		opts.MaxBackoff = 30 * time.Second
	}

	backoff := opts.InitialBackoff
	var lastErr error
	for attempt := 1; ; attempt++ {
		pool, err := NewPostgresPoolWithTracing(connString, tracingEnabled)
		if err == nil {
			return pool, nil
		}
		lastErr = err

		if opts.MaxAttempts > 0 && attempt >= opts.MaxAttempts {
			return nil, fmt.Errorf("database connection failed after %d attempts: %w", attempt, lastErr)
		}

		if onRetry != nil {
			onRetry(attempt, err, backoff)
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("database connection aborted after %d attempts (%v): %w", attempt, ctx.Err(), lastErr)
		case <-time.After(backoff):
		}

		backoff *= 2
		if backoff > opts.MaxBackoff {
			backoff = opts.MaxBackoff
		}
	}
}

// HealthCheck verifies the database connection is healthy
func HealthCheck(pool *pgxpool.Pool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	return pool.Ping(ctx)
}

// envInt returns the integer value of the named environment variable, or def
// when it is unset, empty, non-numeric, or not positive.
func envInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return def
	}
	return n
}

// applicationName resolves a label for the connection's application_name,
// preferring an explicit override, then the OTel service name, then the pod
// hostname (always set in Kubernetes, giving per-service attribution for free).
func applicationName() string {
	for _, key := range []string{"DATABASE_APP_NAME", "OTEL_SERVICE_NAME", "HOSTNAME"} {
		if v := os.Getenv(key); v != "" {
			return v
		}
	}
	return ""
}
