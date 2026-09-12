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

// instagram-listener polls a streamer's Instagram professional account for
// live-broadcast comments via the Instagram Graph API and publishes them to
// chat:raw (ADR-0062). Sister surface to facebook-listener (ADR-0060): same
// Meta app, same Graph host, own token table and own pollers keyed on the IG
// user id. No quota subsystem: IG rate limits are BUC-based like Facebook's
// and a hobby deployment cannot approach them. Single-pod by design (see the
// k8s deployment): two replicas would both poll, and leader election buys
// nothing for a comment-id cursor.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/caesar/all-chat/services/instagram-listener/channels"
	"github.com/caesar/all-chat/services/instagram-listener/client"
	"github.com/caesar/all-chat/services/instagram-listener/handlers"
	"github.com/caesar/all-chat/services/instagram-listener/publisher"
	"github.com/caesar/all-chat/shared/database"
	"github.com/caesar/all-chat/shared/encryption"
	"github.com/caesar/all-chat/shared/listener"
	"github.com/caesar/all-chat/shared/logger"
	sharedredis "github.com/caesar/all-chat/shared/redis"
	"github.com/caesar/all-chat/shared/tracing"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func main() {
	logLevel := listener.Env("LOG_LEVEL", "info")
	log := logger.NewLogger("instagram-listener", logLevel)
	defer log.Sync()

	log.Info("Starting Instagram Listener",
		zap.String("version", listener.Env("APP_VERSION", "dev")),
	)

	tracingEnabled := listener.Env("OTEL_ENABLED", "false") == "true"
	if tracingEnabled {
		tracingCfg := tracing.Config{
			ServiceName:    "instagram-listener",
			ServiceVersion: listener.Env("APP_VERSION", "dev"),
			Environment:    listener.Env("ENVIRONMENT", "development"),
			OTLPEndpoint:   listener.Env("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317"),
			Enabled:        true,
		}
		shutdownTracer, err := tracing.InitTracer(tracingCfg, log)
		if err != nil {
			log.Error("Failed to initialize tracer", zap.Error(err))
		} else {
			defer shutdownTracer(context.Background())
			log.Info("Tracer initialized", zap.String("service", "instagram-listener"))
		}
	}

	ctx := context.Background()

	// PostgreSQL (source registry + IG user tokens)
	dbHost := listener.Env("DATABASE_HOST", "localhost")
	dbPort := listener.Env("DATABASE_PORT", "5432")
	dbUser := listener.Env("DATABASE_USER", "allchat")
	dbPassword := listener.Env("DATABASE_PASSWORD", "")
	if dbPassword == "" {
		log.Fatal("DATABASE_PASSWORD must be set")
	}
	dbName := listener.Env("DATABASE_NAME", "allchat")

	db, err := database.NewPostgresPool(fmt.Sprintf("postgres://%s:%s@%s:%s/%s",
		dbUser, dbPassword, dbHost, dbPort, dbName))
	if err != nil {
		log.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer db.Close()
	log.Info("Connected to PostgreSQL")

	// Redis, with startup retry so a transient outage does not crash-loop.
	startupCtx, stopStartup := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	redisClient, err := sharedredis.NewClientWithRetry(startupCtx,
		sharedredis.BuildDSN(listener.Env("REDIS_HOST", "localhost"), listener.Env("REDIS_PORT", "6379")),
		listener.Env("REDIS_PASSWORD", ""), tracingEnabled,
		sharedredis.DefaultRetryOptions(),
		func(attempt int, err error, backoff time.Duration) {
			log.Warn("Redis not reachable, retrying with backoff",
				zap.Int("attempt", attempt), zap.Duration("backoff", backoff), zap.Error(err))
		})
	stopStartup()
	if err != nil {
		log.Fatal("Failed to connect to Redis", zap.Error(err))
	}
	defer redisClient.Close()
	log.Info("Connected to Redis")

	// Optional token cipher (TOKEN_ENCRYPTION_KEY_V1); plaintext rows still work.
	var tokenCipher *encryption.MultiKeyEncryptor
	if cipher, cipherErr := encryption.NewMultiKeyEncryptorFromEnvWithLogger(log); cipherErr == nil {
		tokenCipher = cipher
		log.Info("instagram token cipher initialized", zap.Uint8("current_kid", tokenCipher.CurrentKid()))
	} else {
		log.Warn("instagram token cipher not configured — encrypted instagram_oauth_tokens will fail",
			zap.Error(cipherErr))
	}

	graph := client.NewClient(
		listener.Env("INSTAGRAM_GRAPH_URL", ""),
		listener.Env("INSTAGRAM_GRAPH_VERSION", ""),
	)

	// INSTAGRAM_APP_ID/SECRET belong to auth-service (OAuth); the listener needs
	// neither — it uses stored tokens.
	if os.Getenv("INSTAGRAM_APP_ID") == "" {
		log.Info("INSTAGRAM_APP_ID not set — Instagram OAuth lives in auth-service; the listener only needs stored IG tokens")
	}

	streamPublisher := publisher.NewStreamPublisher(redisClient, log)
	sourceRepo := channels.NewRepository(db)
	stateStore := channels.NewStateStore(redisClient)
	tokenStore := channels.NewPgTokenStore(db, tokenCipher, log)

	pollEvery := time.Duration(parseIntEnv("INSTAGRAM_SOURCE_SYNC_SECONDS", 30)) * time.Second
	pollGap := time.Duration(parseIntEnv("INSTAGRAM_POLLING_INTERVAL_MS", 10000)) * time.Millisecond

	mgr := channels.NewManager(sourceRepo, stateStore, graph, streamPublisher, tokenStore, log, pollEvery, pollGap)

	// Health endpoints — start before mgr so probes never block on init.
	if logLevel == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()
	router.Use(gin.Recovery())
	if tracingEnabled {
		router.Use(tracing.GinMiddleware("instagram-listener"))
	}
	healthHandler := handlers.NewHealthHandler(streamPublisher, mgr)
	router.GET("/health/live", healthHandler.LivenessProbe)
	router.GET("/health/ready", healthHandler.ReadinessProbe)
	router.GET("/status", healthHandler.Status)
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	port := listener.Env("PORT", "8100")
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

	mgr.Start(ctx)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down service...")
	mgr.Stop()
	streamPublisher.Stop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("HTTP server forced to shutdown", zap.Error(err))
	}
	log.Info("Service exited")
}

// parseIntEnv parses an integer environment variable or returns a default.
func parseIntEnv(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := parsePositiveInt(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}

func parsePositiveInt(s string) (int, error) {
	n := 0
	if s == "" {
		return 0, fmt.Errorf("empty")
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("not a number: %s", s)
		}
		n = n*10 + int(r-'0')
	}
	return n, nil
}

var _ = redis.Nil // keep go-redis imported for the retry helper signature
