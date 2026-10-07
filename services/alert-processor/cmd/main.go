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

// The alert-processor consumes the chat:raw Redis Stream (its own consumer
// group) and routes event-typed messages to alert-capable overlays: it
// normalizes them with the message-processor's normalizers, publishes alert
// envelopes to overlay:{id}:alerts, persists them to alert_events, and feeds
// 'list' overlay leaderboards (ADR-0064).
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/caesar/all-chat/services/alert-processor/consumer"
	"github.com/caesar/all-chat/services/alert-processor/leaderboard"
	"github.com/caesar/all-chat/services/alert-processor/processor"
	"github.com/caesar/all-chat/services/alert-processor/publisher"
	"github.com/caesar/all-chat/services/alert-processor/repository"
	"github.com/caesar/all-chat/shared/database"
	"github.com/caesar/all-chat/shared/logger"
	"github.com/caesar/all-chat/shared/tracing"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func main() {
	log := logger.NewLogger("alert-processor", getEnv("LOG_LEVEL", "info"))
	defer log.Sync()

	log.Info("Starting Alert Processor",
		zap.String("version", getEnv("APP_VERSION", "0.1.0")),
	)

	// OpenTelemetry tracing — opt-in, mirroring the other Go services.
	tracingCfg := tracingConfigFromEnv()
	if tracingCfg.Enabled {
		shutdownTracer, err := tracing.InitTracer(tracingCfg, log)
		if err != nil {
			log.Error("Failed to initialize tracer (continuing without tracing)", zap.Error(err))
		} else {
			defer shutdownTracer(context.Background())
			log.Info("OpenTelemetry tracing enabled")
		}
	}

	config := loadConfig()
	if config.DatabasePassword == "" {
		log.Fatal("DATABASE_PASSWORD must be set")
	}

	connString := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		config.DatabaseUser,
		config.DatabasePassword,
		config.DatabaseHost,
		config.DatabasePort,
		config.DatabaseName,
	)
	dbPool, err := database.NewPostgresPoolWithTracing(connString, tracingCfg.Enabled)
	if err != nil {
		log.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer dbPool.Close()
	log.Info("Connected to PostgreSQL")

	// Redis is a core dependency (stream + Pub/Sub + leaderboards), unlike the
	// media-service where it only backs an optional check: without it this
	// service does nothing, so fail fast and let the orchestrator restart.
	redisClient := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", getEnv("REDIS_HOST", "localhost"), getEnv("REDIS_PORT", "6379")),
		Password: getEnv("REDIS_PASSWORD", ""),
	})
	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		log.Fatal("Failed to connect to Redis", zap.Error(err))
	}
	defer redisClient.Close()
	log.Info("Connected to Redis")

	consumerName, err := os.Hostname()
	if err != nil || consumerName == "" {
		consumerName = "alert-processor-" + getEnv("POD_NAME", "unknown")
	}

	repo := repository.NewRepository(dbPool)
	alertPublisher := publisher.New(redisClient)
	boards := leaderboard.New(redisClient)
	pipeline := processor.New(repo, alertPublisher, boards, log)
	streamConsumer := consumer.New(redisClient, log, pipeline, consumerName)

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(gin.LoggerWithConfig(gin.LoggerConfig{
		SkipPaths: []string{"/health/live", "/health/ready"},
	}))

	router.GET("/health/live", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "alive"})
	})

	router.GET("/health/ready", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := dbPool.Ping(ctx); err != nil {
			log.Error("Health check failed: database unavailable", zap.Error(err))
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "unavailable",
				"reason": "database connection failed",
			})
			return
		}
		if err := redisClient.Ping(ctx).Err(); err != nil {
			log.Error("Health check failed: redis unavailable", zap.Error(err))
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "unavailable",
				"reason": "redis connection failed",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})

	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	srv := &http.Server{
		Addr:         ":" + config.Port,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info("Alert Processor started", zap.String("port", config.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	streamConsumer.Start(context.Background())

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down...")
	streamConsumer.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Error("Server forced to shutdown", zap.Error(err))
	}

	log.Info("Server exited")
}

// Config is the service's environment-derived configuration.
type Config struct {
	Port             string
	DatabaseHost     string
	DatabasePort     string
	DatabaseUser     string
	DatabasePassword string
	DatabaseName     string
}

func loadConfig() *Config {
	return &Config{
		Port:             getEnv("PORT", "8095"),
		DatabaseHost:     getEnv("DATABASE_HOST", "localhost"),
		DatabasePort:     getEnv("DATABASE_PORT", "5432"),
		DatabaseUser:     getEnv("DATABASE_USER", "allchat"),
		DatabasePassword: getEnv("DATABASE_PASSWORD", ""),
		DatabaseName:     getEnv("DATABASE_NAME", "allchat"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// tracingConfigFromEnv reads the OTEL block the deployment manifest sets
// (caesar-deployment apps/workloads/all-chat/alert-processor-deployment.yaml)
// into a tracing.Config, mirroring the other Go services. Kept out of main()
// so the wiring is testable: a typo in a variable name would otherwise be
// dead config advertising telemetry the service does not emit.
func tracingConfigFromEnv() tracing.Config {
	return tracing.Config{
		ServiceName:    "alert-processor",
		ServiceVersion: getEnv("APP_VERSION", "0.1.0"),
		Environment:    getEnv("ENVIRONMENT", "development"),
		OTLPEndpoint:   getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317"),
		Enabled:        getEnv("OTEL_ENABLED", "false") == "true",
	}
}
