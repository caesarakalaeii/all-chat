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
