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

	"github.com/caesar/all-chat/services/media-service/handlers"
	"github.com/caesar/all-chat/services/media-service/repository"
	"github.com/caesar/all-chat/services/media-service/storage"
	sharedAuth "github.com/caesar/all-chat/shared/auth"
	"github.com/caesar/all-chat/shared/database"
	"github.com/caesar/all-chat/shared/logger"
	"github.com/caesar/all-chat/shared/middleware"
	"github.com/caesar/all-chat/shared/tracing"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func main() {
	logLevel := getEnv("LOG_LEVEL", "info")
	log := logger.NewLogger("media-service", logLevel)
	defer log.Sync()
	// Surface JWT revocation (logout-blacklist) check failures from the shared
	// middleware instead of dropping them to a no-op logger.
	middleware.SetLogger(log)

	log.Info("Starting Media Service",
		zap.String("version", getEnv("APP_VERSION", "0.1.0")),
	)

	// Initialize OpenTelemetry tracing
	tracingCfg := tracingConfigFromEnv()
	tracingEnabled := tracingCfg.Enabled
	if tracingEnabled {
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

	userKeyChain, err := sharedAuth.NewKeyChainFromEnv("JWT_SECRET")
	if err != nil {
		log.Fatal("JWT key chain init failed (JWT_SECRET_V1 must be set)", zap.Error(err))
	}
	log.Info("JWT key chain initialized", zap.String("latest_kid", userKeyChain.LatestKid()))

	// Connect to PostgreSQL
	connString := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		config.DatabaseUser,
		config.DatabasePassword,
		config.DatabaseHost,
		config.DatabasePort,
		config.DatabaseName,
	)
	dbPool, err := database.NewPostgresPoolWithTracing(connString, tracingEnabled)
	if err != nil {
		log.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer dbPool.Close()
	log.Info("Connected to PostgreSQL")

	// Redis for the JWT logout-blacklist check. Optional: an unreachable
	// Redis degrades to skipping the blacklist (middleware fail-open), so a
	// Redis outage cannot lock users out of media management.
	redisClient := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", getEnv("REDIS_HOST", "localhost"), getEnv("REDIS_PORT", "6379")),
		Password: getEnv("REDIS_PASSWORD", ""),
	})
	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		log.Warn("Failed to connect to Redis (logout blacklist check disabled)", zap.Error(err))
		redisClient.Close()
		redisClient = nil
	} else {
		log.Info("Connected to Redis")
		defer redisClient.Close()
	}

	// MinIO object store. No endpoint configured is a valid degraded state:
	// the service starts and every media route answers 503, so a missing
	// MinIO deployment (issue #948) cannot take the rest of the stack down.
	storageCfg, err := storage.LoadConfig()
	if err != nil {
		log.Fatal("Invalid media storage configuration", zap.Error(err))
	}
	var objectStore handlers.ObjectStore
	if storageCfg.Endpoint == "" {
		log.Warn("MINIO_ENDPOINT not set — media routes will serve 503")
		objectStore = storage.DisabledStore{}
	} else {
		objectStore, err = storage.NewMinioStore(storageCfg)
		if err != nil {
			// minio.New only parses the endpoint; an error here is a malformed
			// MINIO_ENDPOINT. Degrade to 503s instead of crash-looping: the
			// rest of the stack does not depend on this service.
			log.Error("Failed to build MinIO client — media routes will serve 503", zap.Error(err))
			objectStore = storage.DisabledStore{}
		} else {
			log.Info("MinIO object store configured",
				zap.String("endpoint", storageCfg.Endpoint),
				zap.String("bucket", storageCfg.Bucket),
			)
		}
	}

	mediaRepo := repository.NewMediaRepository(dbPool)
	mediaHandler := handlers.NewMediaHandler(mediaRepo, objectStore, handlers.MediaConfig{
		PublicBaseURL:     storageCfg.PublicBaseURL,
		PresignExpiry:     storageCfg.PresignExpiry,
		MaxObjectsPerUser: storageCfg.MaxObjectsPerUser,
	}, log)

	if config.GinMode == "release" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

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
		// An unconfigured object store is NOT unready: media routes answer
		// 503 by design while the rest of the service is fine.
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})

	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Media routes. All JWT-authenticated; user_id comes from the token,
	// never from the request. Anonymous reads of media go straight to MinIO
	// (media.allch.at), never through this service.
	api := router.Group("/api/v1")
	api.Use(middleware.JWTAuthWithRevocation(userKeyChain, redisClient))
	media := api.Group("/media", mediaHandler.RequireStore)
	media.POST("/presign", mediaHandler.Presign)
	media.GET("", mediaHandler.List)
	media.DELETE("/*object_key", mediaHandler.Delete)

	srv := &http.Server{
		Addr:         ":" + config.Port,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info("Media Service started", zap.String("port", config.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Error("Server forced to shutdown", zap.Error(err))
	}

	log.Info("Server exited")
}

type Config struct {
	Port             string
	GinMode          string
	DatabaseHost     string
	DatabasePort     string
	DatabaseUser     string
	DatabasePassword string
	DatabaseName     string
}

func loadConfig() *Config {
	return &Config{
		Port:             getEnv("PORT", "8094"),
		GinMode:          getEnv("GIN_MODE", "debug"),
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

// tracingConfigFromEnv reads the OTEL block that the deployment manifest sets
// (caesar-deployment apps/workloads/all-chat/media-service-deployment.yaml)
// into a tracing.Config, mirroring the other Go services. Kept out of main()
// so the wiring is testable: a typo in a variable name would otherwise be
// dead config advertising telemetry the service does not emit.
func tracingConfigFromEnv() tracing.Config {
	return tracing.Config{
		ServiceName:    "media-service",
		ServiceVersion: getEnv("APP_VERSION", "0.1.0"),
		Environment:    getEnv("ENVIRONMENT", "development"),
		OTLPEndpoint:   getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317"),
		Enabled:        getEnv("OTEL_ENABLED", "false") == "true",
	}
}
