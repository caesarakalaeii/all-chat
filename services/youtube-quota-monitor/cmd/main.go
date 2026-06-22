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

// Command youtube-quota-monitor watches the shared youtube_quota_usage table and is the
// single owner of YouTube quota observability now that the quota-based youtube-listener
// is no longer deployed (ADR-0023). It exports the Prometheus gauges the alert rules
// evaluate (listener_quota_usage_percentage/listener_quota_remaining, platform=youtube),
// publishes state-transition / threshold-crossing QuotaEvents to the "quota:alerts"
// Redis channel the discord-bot renders, and sweeps stale quota reservations. Deploy as
// a single replica — the alert dedup state is in-memory.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/caesar/all-chat/services/youtube-quota-monitor/config"
	"github.com/caesar/all-chat/services/youtube-quota-monitor/handlers"
	"github.com/caesar/all-chat/services/youtube-quota-monitor/monitor"
	"github.com/caesar/all-chat/shared/database"
	"github.com/caesar/all-chat/shared/logger"
	"github.com/caesar/all-chat/shared/metrics"
	"github.com/caesar/all-chat/shared/quota"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func main() {
	cfg := config.Load()

	log := logger.NewLogger("youtube-quota-monitor", cfg.LogLevel)
	defer log.Sync()
	log.Info("Starting YouTube Quota Monitor",
		zap.Duration("interval", cfg.MonitorInterval),
		zap.String("alert_channel", cfg.AlertChannel),
		zap.Bool("notifier_enabled", cfg.NotifierEnabled),
	)

	// PostgreSQL: the shared youtube_quota_usage table is the source of truth.
	connString := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		cfg.DatabaseUser, cfg.DatabasePassword, cfg.DatabaseHost, cfg.DatabasePort, cfg.DatabaseName,
	)
	dbPool, err := database.NewPostgresPool(connString)
	if err != nil {
		log.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer dbPool.Close()
	log.Info("Connected to PostgreSQL")

	// Redis: publishes QuotaEvents to the channel the discord-bot subscribes to.
	redisClient := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%s", cfg.RedisHost, cfg.RedisPort),
	})
	defer redisClient.Close()
	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		log.Warn("Redis ping failed; quota alerts will fail until Redis recovers", zap.Error(err))
	} else {
		log.Info("Connected to Redis")
	}

	m := metrics.NewListenerMetrics("youtube", "youtube-quota-monitor")
	notifier := quota.NewNotifier(redisClient, log, cfg.NotifierEnabled, cfg.AlertChannel)
	reader := monitor.NewPgxReader(dbPool)
	mon := monitor.New(reader, notifier, m, quota.DefaultThresholds(), cfg.MonitorInterval, log)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go mon.Run(ctx)
	go runReservationCleanup(ctx, dbPool, cfg.CleanupInterval, log)

	if cfg.GinMode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(gin.LoggerWithConfig(gin.LoggerConfig{SkipPaths: []string{"/health/live", "/health/ready"}}))

	health := handlers.NewHealthHandler(dbPool, redisClient)
	status := handlers.NewStatusHandler(mon)
	router.GET("/health/live", health.Live)
	router.GET("/health/ready", health.Ready)
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))
	router.GET("/quota/status", status.GetQuotaStatus)

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info("YouTube Quota Monitor listening", zap.String("port", cfg.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info("Shutting down...")

	cancel() // stop the monitor + cleanup loops
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("Server forced to shutdown", zap.Error(err))
	}
	log.Info("Server exited")
}

// runReservationCleanup periodically sweeps stale (past-day) quota reservations left by
// crashed processes — the job the youtube-listener used to own. Cheap and idempotent.
func runReservationCleanup(ctx context.Context, db *pgxpool.Pool, interval time.Duration, log *zap.Logger) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			var recovered int
			if err := db.QueryRow(cctx, `SELECT cleanup_stale_quota_reservations()`).Scan(&recovered); err != nil {
				log.Warn("stale quota reservation cleanup failed", zap.Error(err))
			} else if recovered > 0 {
				log.Info("cleaned up stale quota reservations", zap.Int("units_recovered", recovered))
			}
			cancel()
		}
	}
}
