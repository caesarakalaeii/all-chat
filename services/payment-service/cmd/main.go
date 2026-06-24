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
	"strconv"
	"syscall"
	"time"

	"github.com/caesar/all-chat/services/payment-service/entitlement"
	"github.com/caesar/all-chat/services/payment-service/handlers"
	"github.com/caesar/all-chat/services/payment-service/patreon"
	"github.com/caesar/all-chat/services/payment-service/reconcile"
	"github.com/caesar/all-chat/services/payment-service/repository"
	sharedAuth "github.com/caesar/all-chat/shared/auth"
	"github.com/caesar/all-chat/shared/database"
	"github.com/caesar/all-chat/shared/encryption"
	"github.com/caesar/all-chat/shared/logger"
	"github.com/caesar/all-chat/shared/middleware"
	"github.com/caesar/all-chat/shared/premium"
	sharedredis "github.com/caesar/all-chat/shared/redis"
	"github.com/caesar/all-chat/shared/tracing"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

func main() {
	logLevel := getEnv("LOG_LEVEL", "info")
	log := logger.NewLogger("payment-service", logLevel)
	defer log.Sync()

	log.Info("Starting Payment Service", zap.String("version", getEnv("APP_VERSION", "dev")))

	// Tracing (optional).
	tracingEnabled := getEnv("OTEL_ENABLED", "false") == "true"
	if tracingEnabled {
		shutdownTracer, err := tracing.InitTracer(tracing.Config{
			ServiceName:    "payment-service",
			ServiceVersion: getEnv("APP_VERSION", "dev"),
			Environment:    getEnv("ENVIRONMENT", "development"),
			OTLPEndpoint:   getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317"),
			Enabled:        true,
		}, log)
		if err != nil {
			log.Error("Failed to initialize tracer", zap.Error(err))
		} else {
			defer shutdownTracer(context.Background())
		}
	}

	// Required Patreon configuration.
	patreonClientID := os.Getenv("PATREON_CLIENT_ID")
	patreonClientSecret := os.Getenv("PATREON_CLIENT_SECRET")
	patreonCampaignID := os.Getenv("PATREON_CAMPAIGN_ID")
	patreonRedirectURL := os.Getenv("PATREON_REDIRECT_URL")
	patreonWebhookSecret := os.Getenv("PATREON_WEBHOOK_SECRET")
	if patreonClientID == "" || patreonClientSecret == "" {
		log.Fatal("PATREON_CLIENT_ID and PATREON_CLIENT_SECRET are required")
	}
	if patreonCampaignID == "" {
		log.Fatal("PATREON_CAMPAIGN_ID is required")
	}
	if patreonRedirectURL == "" {
		log.Fatal("PATREON_REDIRECT_URL is required")
	}
	if patreonWebhookSecret == "" {
		log.Fatal("PATREON_WEBHOOK_SECRET is required")
	}

	minCents := getInt("PATREON_MIN_TIER_CENTS", 500)
	viewerMinCents := getInt("PATREON_VIEWER_MIN_TIER_CENTS", 200)
	frontendURL := getEnv("FRONTEND_URL", "http://localhost:3000")

	// JWT key chain for the user-facing connect/status endpoints.
	userKeyChain, err := sharedAuth.NewKeyChainFromEnv("JWT_SECRET")
	if err != nil {
		log.Fatal("JWT key chain init failed (JWT_SECRET_V1 must be set)", zap.Error(err))
	}

	// Token encryption (TOKEN_ENCRYPTION_KEY_V1).
	encryptor, err := encryption.NewMultiKeyEncryptorFromEnv()
	if err != nil {
		log.Fatal("Failed to initialize encryption (TOKEN_ENCRYPTION_KEY_V1 must be set)", zap.Error(err))
	}

	// PostgreSQL.
	connString := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		getEnv("DATABASE_USER", "allchat"),
		getEnv("DATABASE_PASSWORD", "allchat_dev_password"),
		getEnv("DATABASE_HOST", "localhost"),
		getEnv("DATABASE_PORT", "5432"),
		getEnv("DATABASE_NAME", "allchat"))
	db, err := database.NewPostgresPool(connString)
	if err != nil {
		log.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer db.Close()
	log.Info("Connected to PostgreSQL")

	// Redis (OAuth state + webhook dedup). Retry the initial connection with
	// backoff so a transient Redis outage (e.g. the pod being rescheduled onto
	// another node) does not crash-loop this service. The retry is cancelled on
	// shutdown signals so SIGTERM still terminates the process promptly while it
	// is waiting for Redis.
	rootCtx := context.Background()
	redisAddr := sharedredis.BuildDSN(getEnv("REDIS_HOST", "localhost"), getEnv("REDIS_PORT", "6379"))

	startupCtx, stopStartup := signal.NotifyContext(rootCtx, syscall.SIGINT, syscall.SIGTERM)
	redisClient, err := sharedredis.NewClientWithRetry(startupCtx, redisAddr, getEnv("REDIS_PASSWORD", ""), tracingEnabled,
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

	// Wiring.
	patreonOAuth := patreon.NewOAuth(patreonClientID, patreonClientSecret, patreonRedirectURL)
	tokenRepo := repository.NewTokenRepository(db, encryptor, log)
	subRepo := repository.NewSubscriptionRepository(db, log)
	recomputer := premium.NewRecomputer(db, log)
	entitlementSvc := entitlement.NewService(subRepo, recomputer, minCents, viewerMinCents, log)

	oauthHandler := handlers.NewOAuthHandler(patreonOAuth, redisClient, tokenRepo, entitlementSvc, patreonCampaignID, frontendURL, log)
	statusHandler := handlers.NewStatusHandler(subRepo, tokenRepo, recomputer, log)
	webhookHandler := handlers.NewWebhookHandler(patreonWebhookSecret, redisClient, tokenRepo, entitlementSvc, log)

	// Reconcile job (single replica in production).
	reconcileMgr := reconcile.NewManager(
		patreonOAuth, tokenRepo, entitlementSvc, patreonCampaignID,
		getDuration("PAYMENT_RECONCILE_INTERVAL", 6*time.Hour),
		getDuration("PATREON_TOKEN_REFRESH_BUFFER", 24*time.Hour),
		getInt("PAYMENT_RECONCILE_BATCH_SIZE", 500),
		log,
	)
	reconcileCtx, reconcileCancel := context.WithCancel(rootCtx)
	defer reconcileCancel()
	go func() {
		if err := reconcileMgr.Start(reconcileCtx); err != nil && err != context.Canceled {
			log.Error("Reconcile manager stopped with error", zap.Error(err))
		}
	}()

	// HTTP server.
	httpRequests := promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total", Help: "Total HTTP requests",
	}, []string{"service", "method", "path", "status"})
	httpDuration := promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "http_request_duration_seconds", Help: "HTTP request duration in seconds", Buckets: prometheus.DefBuckets,
	}, []string{"service", "method", "path"})

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(httpMetricsMiddleware(httpRequests, httpDuration, "payment-service"))
	if tracingEnabled {
		router.Use(tracing.GinMiddleware("payment-service"))
	}

	router.GET("/health/live", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "alive"}) })
	router.GET("/health/ready", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := db.Ping(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable", "reason": "database"})
			return
		}
		if err := redisClient.Ping(ctx).Err(); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable", "reason": "redis"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Public routes (no JWT): Patreon's HMAC is the auth for the webhook; the
	// callback is authenticated by the one-time Redis state.
	router.POST("/api/v1/webhooks/patreon", webhookHandler.Handle)
	router.GET("/api/v1/payment/patreon/callback", oauthHandler.Callback)

	// Authenticated routes. JWTAuth accepts both user and viewer tokens (same
	// keychain); each handler reads its own subject id (user_id / viewer_id) and
	// 401s if the token is the wrong kind.
	api := router.Group("/api/v1")
	api.Use(middleware.JWTAuth(userKeyChain))
	{
		// Streamer premium (ADR-0018).
		api.GET("/payment/patreon/connect", oauthHandler.Connect)
		api.GET("/payment/status", statusHandler.Status)
		api.DELETE("/payment/patreon/connection", statusHandler.Disconnect)
		// Viewer premium (ADR-0019).
		api.GET("/payment/viewer/patreon/connect", oauthHandler.ConnectViewer)
		api.GET("/payment/viewer/status", statusHandler.ViewerStatus)
		api.DELETE("/payment/viewer/patreon/connection", statusHandler.ViewerDisconnect)
	}

	port := getEnv("PORT", "8091")
	srv := &http.Server{Addr: ":" + port, Handler: router, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		log.Info("HTTP server starting", zap.String("port", port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("HTTP server failed", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info("Shutting down gracefully...")

	reconcileCancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("HTTP server shutdown error", zap.Error(err))
	}
	log.Info("Service stopped")
}

func getEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

func getDuration(key string, defaultValue time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return defaultValue
}

func getInt(key string, defaultValue int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil && i >= 0 {
			return i
		}
	}
	return defaultValue
}

func httpMetricsMiddleware(requests *prometheus.CounterVec, duration *prometheus.HistogramVec, service string) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		status := strconv.Itoa(c.Writer.Status())
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}
		requests.WithLabelValues(service, c.Request.Method, path, status).Inc()
		duration.WithLabelValues(service, c.Request.Method, path).Observe(time.Since(start).Seconds())
	}
}
