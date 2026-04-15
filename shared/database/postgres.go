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
	"time"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPostgresPoolWithTracing creates a PostgreSQL pool with optional OpenTelemetry tracing
func NewPostgresPoolWithTracing(connString string, tracingEnabled bool) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse connection string: %w", err)
	}

	// Connection pool configuration
	config.MaxConns = 20                       // Max connections per service instance
	config.MinConns = 5                        // Keep warm connections
	config.MaxConnLifetime = 1 * time.Hour     // Recycle connections after 1 hour
	config.MaxConnIdleTime = 10 * time.Minute  // Close idle connections
	config.HealthCheckPeriod = 1 * time.Minute // Verify connections health

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

// HealthCheck verifies the database connection is healthy
func HealthCheck(pool *pgxpool.Pool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	return pool.Ping(ctx)
}
