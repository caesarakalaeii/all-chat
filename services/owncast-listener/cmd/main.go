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
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/caesar/all-chat/services/owncast-listener/channels"
	"github.com/caesar/all-chat/services/owncast-listener/handlers"
	"github.com/caesar/all-chat/services/owncast-listener/publisher"
	"github.com/caesar/all-chat/services/owncast-listener/status"
	"github.com/caesar/all-chat/shared/database"
	sharedlistener "github.com/caesar/all-chat/shared/listener"
	"github.com/caesar/all-chat/shared/logger"
	sharedredis "github.com/caesar/all-chat/shared/redis"
	"github.com/caesar/all-chat/shared/sourcemanager"
	"github.com/caesar/all-chat/shared/tracing"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

func main() {
	// Initialize logger
	logLevel := sharedlistener.Env("LOG_LEVEL", "info")
	log := logger.NewLogger("owncast-listener", logLevel)
	defer log.Sync()

	log.Info("Starting Owncast Listener",
		zap.String("version", sharedlistener.Env("APP_VERSION", "dev")),
	)

	// Initialize tracing
	tracingEnabled := sharedlistener.Env("OTEL_ENABLED", "false") == "true"
	if tracingEnabled {
		tracingCfg := tracing.Config{
			ServiceName:    "owncast-listener",
			ServiceVersion: sharedlistener.Env("APP_VERSION", "dev"),
			Environment:    sharedlistener.Env("ENVIRONMENT", "development"),
			OTLPEndpoint:   sharedlistener.Env("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317"),
			Enabled:        true,
		}
		shutdownTracer, err := tracing.InitTracer(tracingCfg, log)
		if err != nil {
			log.Error("Failed to initialize tracer", zap.Error(err))
		} else {
			defer shutdownTracer(context.Background())
			log.Info("Tracer initialized", zap.String("service", "owncast-listener"))
		}
	}

	ctx := context.Background()

	// Connect to PostgreSQL
	dbHost := sharedlistener.Env("DATABASE_HOST", "localhost")
	dbPort := sharedlistener.Env("DATABASE_PORT", "5432")
	dbUser := sharedlistener.Env("DATABASE_USER", "allchat")
	dbPassword := sharedlistener.Env("DATABASE_PASSWORD", "")
	if dbPassword == "" {
		log.Fatal("DATABASE_PASSWORD must be set")
	}
	dbName := sharedlistener.Env("DATABASE_NAME", "allchat")

	connString := fmt.Sprintf("postgres://%s:%s@%s:%s/%s",
		dbUser, dbPassword, dbHost, dbPort, dbName)

	db, err := database.NewPostgresPool(connString)
	if err != nil {
		log.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer db.Close()

	log.Info("Connected to PostgreSQL")

	// Connect to Redis, retrying with backoff so a transient Redis outage
	// does not crash-loop this service.
	redisHost := sharedlistener.Env("REDIS_HOST", "localhost")
	redisPort := sharedlistener.Env("REDIS_PORT", "6379")
	redisAddr := sharedredis.BuildDSN(redisHost, redisPort)
	startupCtx, stopStartup := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	redisClient, err := sharedredis.NewClientWithRetry(startupCtx, redisAddr, sharedlistener.Env("REDIS_PASSWORD", ""), tracingEnabled,
		sharedredis.DefaultRetryOptions(),
		func(attempt int, err error, backoff time.Duration) {
			log.Warn("Redis not reachable, retrying with backoff",
				zap.Int("attempt", attempt),
				zap.Duration("backoff", backoff),
				zap.Error(err),
			)
		})
	stopStartup()
	if err != nil {
		log.Fatal("Failed to connect to Redis", zap.Error(err))
	}
	defer redisClient.Close()

	log.Info("Connected to Redis")

	podName := sharedlistener.Env("HOSTNAME", "owncast-listener-0")

	// Leadership coordinator (may be disabled by env like other listeners).
	ll, err := sharedlistener.NewLeadershipListenerFromEnv("owncast", redisClient, log)
	if err != nil {
		log.Fatal("Failed to initialize leadership listener", zap.Error(err))
	}

	// Initialize components
	streamPublisher := publisher.NewStreamPublisher(redisClient, log)
	channelRepo := channels.NewRepository(db, log)
	statusPublisher := status.NewPublisher(redisClient, log)
	log.Info("Initialized platform status publisher")

	// Leadership coordinator wired into the manager when the SDK is enabled.
	var leader *sourcemanager.LeadershipCoordinator
	channelMgr := channels.NewManager(channelRepo, streamPublisher, statusPublisher, redisClient, leader, podName, log)
	channelMgr.SetStatusPublisher(statusPublisher)

	// HTTP server for health checks — must start BEFORE ll.Start() because
	// ll.Start() -> mgr.Start() -> sync can block on DB.
	if logLevel == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())
	if tracingEnabled {
		router.Use(tracing.GinMiddleware("owncast-listener"))
	}

	healthHandler := handlers.NewHealthHandler(streamPublisher, channelMgr)
	router.GET("/health/live", healthHandler.LivenessProbe)
	router.GET("/health/ready", healthHandler.ReadinessProbe)
	router.GET("/status", healthHandler.Status)
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	port := sharedlistener.Env("PORT", "8095")

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info("HTTP server listening", zap.String("port", port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Failed to start HTTP server", zap.Error(err))
		}
	}()

	if err := ll.Start(ctx, channelMgr); err != nil {
		log.Fatal("Failed to start listener", zap.Error(err))
	}

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down service...")

	// Stop ring buffer publisher — drains retry goroutine before stopping.
	streamPublisher.Stop()

	sharedlistener.ShutdownCoordinator(ll, channelMgr, nil, srv, log)

	log.Info("Service exited")
}

// dbConnWrapper kept for parity with other listeners; unused for now because
// the owncast manager polls the DB every 30s instead of LISTEN/NOTIFY.
type dbConnWrapper struct {
	pool *pgxpool.Pool
}

func (w *dbConnWrapper) GetPool() interface{} {
	return w.pool
}
