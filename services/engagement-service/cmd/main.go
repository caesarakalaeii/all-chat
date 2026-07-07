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

// Command engagement-service owns All-Chat polls, predictions, and the per-overlay
// viewer points economy (issue #523). Participation is universal: viewers vote and
// wager via native chat commands (resolved through the durable engagement:commands
// stream), via the authenticated web page, or via the extension — no install
// required. Aggregate poll/prediction state is broadcast to overlays; private
// balances are pull-only (the WS is broadcast-per-overlay).
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

	sharedAuth "github.com/caesar/all-chat/shared/auth"
	"github.com/caesar/all-chat/shared/database"
	"github.com/caesar/all-chat/shared/logger"
	"github.com/caesar/all-chat/shared/middleware"
	"github.com/caesar/all-chat/shared/ratelimit"

	"github.com/caesar/all-chat/services/engagement-service/announcer"
	"github.com/caesar/all-chat/services/engagement-service/consumer"
	"github.com/caesar/all-chat/services/engagement-service/handler"
	"github.com/caesar/all-chat/services/engagement-service/publisher"
	"github.com/caesar/all-chat/services/engagement-service/repository"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func main() {
	log := logger.NewLogger("engagement-service", getEnv("LOG_LEVEL", "info"))
	defer log.Sync()
	middleware.SetLogger(log)
	log.Info("Starting Engagement Service", zap.String("version", getEnv("APP_VERSION", "0.1.0")))

	cfg := loadConfig()
	if cfg.DatabasePassword == "" {
		log.Fatal("DATABASE_PASSWORD must be set")
	}

	keyChain, err := sharedAuth.NewKeyChainFromEnv("JWT_SECRET")
	if err != nil {
		log.Fatal("JWT key chain init failed (JWT_SECRET_V1 must be set)", zap.Error(err))
	}

	// Bound server-side query time and lock waits on THIS service's pool so a runaway
	// query or a contended row lock (Wager's SELECT … FOR UPDATE) fails fast instead
	// of pinning a connection and serializing a consumer batch (M6). pgx forwards
	// unrecognized DSN query params as startup runtime params, so these apply per
	// session — scoped to engagement-service, not the shared pool helper (used by many
	// services with different needs). Values in ms, env-tunable.
	connString := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable&statement_timeout=%d&lock_timeout=%d",
		cfg.DatabaseUser, cfg.DatabasePassword, cfg.DatabaseHost, cfg.DatabasePort, cfg.DatabaseName,
		cfg.StatementTimeoutMs, cfg.LockTimeoutMs)
	dbPool, err := database.NewPostgresPool(connString)
	if err != nil {
		log.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer dbPool.Close()
	log.Info("Connected to PostgreSQL")

	redisClient := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", getEnv("REDIS_HOST", "localhost"), getEnv("REDIS_PORT", "6379")),
		Password: getEnv("REDIS_PASSWORD", ""),
	})
	defer redisClient.Close()
	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		log.Warn("Redis ping failed; engagement broadcasts/commands buffer until Redis recovers", zap.Error(err))
	} else {
		log.Info("Connected to Redis")
	}

	repo := repository.New(dbPool)
	pub := publisher.New(redisClient, log)

	// Chat announcer (issue #523, H4-2): posts round-start messages to chat via
	// auth-service's internal send endpoint, authenticated with a service JWT. Both
	// the key chain and the auth-service URL are optional — a missing one disables the
	// announce (no-op) rather than failing round creation.
	var serviceKeys *sharedAuth.KeyChain
	if kc, err := sharedAuth.NewKeyChainFromEnv("SERVICE_JWT_SECRET"); err != nil {
		log.Warn("SERVICE_JWT_SECRET not set; chat announce disabled", zap.Error(err))
	} else {
		serviceKeys = kc
	}
	announce := announcer.New(repo, getEnv("AUTH_SERVICE_URL", "http://auth-service:8081"),
		getEnv("ENGAGEMENT_PUBLIC_BASE_URL", "https://allch.at"), serviceKeys, log)

	h := handler.New(repo, pub, announce, log)

	// Background consumers + auto-lock/close sweep share one cancellable context,
	// stopped on shutdown before the HTTP server drains.
	bgCtx, cancelBg := context.WithCancel(context.Background())
	defer cancelBg()

	hostname, _ := os.Hostname()
	consumerName := fmt.Sprintf("engagement-%s-%d", hostname, os.Getpid())
	cmdConsumer := consumer.NewCommandConsumer(redisClient, repo, pub, consumerName, log)
	earnConsumer := consumer.NewEarnConsumer(redisClient, repo, log)
	nativeConsumer := consumer.NewNativeConsumer(redisClient, repo, pub, consumerName, log)
	go cmdConsumer.Run(bgCtx)
	go earnConsumer.Run(bgCtx)
	go nativeConsumer.Run(bgCtx)
	go runSweeper(bgCtx, repo, pub, log, time.Duration(cfg.SweepIntervalSeconds)*time.Second,
		time.Duration(cfg.NativeStaleTTLSeconds)*time.Second)

	if cfg.GinMode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(gin.LoggerWithConfig(gin.LoggerConfig{SkipPaths: []string{"/health/live", "/health/ready"}}))

	router.GET("/health/live", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "alive"}) })
	router.GET("/health/ready", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := dbPool.Ping(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable", "reason": "database"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	rl := ratelimit.NewRateLimiter(ratelimit.Config{
		RequestsPerMinute: cfg.RateLimitPerMin,
		KeyPrefix:         "engrl",
		RedisClient:       redisClient,
		Logger:            log,
	})
	idemTTL := time.Duration(cfg.IdempotencyTTLSeconds) * time.Second

	// Public read endpoints (OBS overlay + web page render aggregate state; no auth).
	pubGroup := router.Group("/api/v1/engagement")
	pubGroup.GET("/overlays/:id/active-poll", h.GetActivePoll)
	pubGroup.GET("/overlays/:id/active-prediction", h.GetActivePrediction)
	// Streamer-keyed public aggregate (ADR-0031): the extension / no-install viewer
	// page resolve by username, never learning the overlay id.
	pubGroup.GET("/streamers/:username/active", h.GetStreamerActive)

	// Authenticated routes. The shared middleware accepts either a streamer (owner)
	// or viewer token; handlers enforce which one each route needs.
	auth := router.Group("/api/v1/engagement")
	auth.Use(middleware.JWTAuthWithRevocation(keyChain, redisClient))
	auth.Use(rl.Middleware())
	auth.Use(handler.IdempotencyMiddleware(redisClient, idemTTL))

	// Owner-only management.
	auth.POST("/overlays/:id/polls", h.CreatePoll)
	auth.POST("/overlays/:id/polls/:pollId/close", h.ClosePoll)
	auth.POST("/overlays/:id/predictions", h.CreatePrediction)
	auth.POST("/overlays/:id/predictions/:pid/lock", h.LockPrediction)
	auth.POST("/overlays/:id/predictions/:pid/resolve", h.ResolvePrediction)
	auth.POST("/overlays/:id/predictions/:pid/cancel", h.CancelPrediction)
	auth.GET("/overlays/:id/points/config", h.GetConfig)
	auth.PUT("/overlays/:id/points/config", h.PutConfig)

	// Viewer participation (web page / extension).
	auth.POST("/overlays/:id/polls/:pollId/vote", h.WebVote)
	auth.POST("/overlays/:id/predictions/:pid/wager", h.WebWager)
	auth.GET("/viewers/me/points", h.GetBalance)
	auth.GET("/viewers/me/engagement", h.GetEngagement)
	auth.POST("/viewers/me/heartbeat", h.Heartbeat)

	// Streamer-keyed viewer participation (ADR-0031): the browser extension knows the
	// streamer username, not the overlay id. These resolve username -> public overlay
	// server-side and reuse the overlay-keyed vote/wager/balance logic above.
	auth.GET("/streamers/:username/me", h.StreamerEngagement)
	auth.POST("/streamers/:username/vote", h.StreamerVote)
	auth.POST("/streamers/:username/wager", h.StreamerWager)
	auth.POST("/streamers/:username/heartbeat", h.StreamerHeartbeat)

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	go func() {
		log.Info("Engagement Service listening", zap.String("port", cfg.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info("Shutting down...")
	cancelBg()

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Error("Server forced to shutdown", zap.Error(err))
	}
	log.Info("Server exited")
}

// runSweeper periodically locks predictions past their auto_lock_at and closes
// polls past their ends_at, then broadcasts the new state and clears the active
// flags. It also force-closes mirrored twitch_native rounds stranded live past
// nativeStaleTTL (a never-delivered terminal EventSub would otherwise 409-block
// All-Chat rounds on that overlay forever, M1). Restart-safe: state lives in the DB,
// not in-memory timers.
func runSweeper(ctx context.Context, repo *repository.Repository, pub *publisher.Publisher, log *zap.Logger, interval, nativeStaleTTL time.Duration) {
	if interval <= 0 {
		interval = 10 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			locked, err := repo.LockExpired(ctx)
			if err != nil {
				log.Warn("auto-lock sweep failed", zap.Error(err))
			}
			for _, ref := range locked {
				if pred, err := repo.GetPrediction(ctx, ref.PredictionID); err == nil {
					pub.PublishPrediction(ctx, pred)
				}
				pub.ClearActive(ctx, ref.PredictionID)
			}
			closed, err := repo.CloseExpired(ctx)
			if err != nil {
				log.Warn("auto-close sweep failed", zap.Error(err))
			}
			for _, ref := range closed {
				if poll, err := repo.GetPoll(ctx, ref.PollID); err == nil {
					pub.PublishPoll(ctx, poll)
				}
				pub.ClearActive(ctx, ref.PollID)
			}
			reconcileActiveFlags(ctx, repo, pub, log)
			sweepStaleNative(ctx, repo, pub, log, nativeStaleTTL)
		}
	}
}

// reconcileActiveFlags re-arms the hot-path active flag for every live All-Chat round.
// CreatePoll/CreatePrediction set the flag AFTER committing the round, and SetActive only
// logs (and now counts) on a Redis failure — so a transient blip at create time can leave
// a round live and visible while its engagement:active:{platform}:{channel} SET was never
// written, silently dropping every chat vote for the round. This periodic, idempotent
// re-SetActive self-heals that (mirroring the DB-is-source-of-truth auto-lock/close sweep),
// also covers the sub-millisecond commit→markActive crash window, and refreshes the 24h
// active-flag TTL so a round living longer than a day never loses its gate (P2-2). It only
// ARMS flags for ACTIVE rounds; ClearActive + the TTL handle teardown.
func reconcileActiveFlags(ctx context.Context, repo *repository.Repository, pub *publisher.Publisher, log *zap.Logger) {
	rounds, err := repo.ActiveAllChatEngagements(ctx)
	if err != nil {
		log.Warn("active-flag reconcile: list live rounds failed", zap.Error(err))
		return
	}
	for _, ref := range rounds {
		channels, err := repo.SourceChannelsForOverlay(ctx, ref.OverlayID)
		if err != nil {
			log.Warn("active-flag reconcile: source channels failed",
				zap.String("overlay", ref.OverlayID.String()), zap.Error(err))
			continue
		}
		if len(channels) == 0 {
			continue
		}
		pub.SetActive(ctx, ref.EngagementID, channels)
	}
}

// sweepStaleNative force-closes mirrored twitch_native rounds that have been live
// past nativeStaleTTL and broadcasts their now-terminal state so overlays clear.
// Native rounds never set All-Chat active flags (they run on Twitch) and never move
// viewer_points, so there is nothing to un-flag or refund. Disabled when
// nativeStaleTTL <= 0.
func sweepStaleNative(ctx context.Context, repo *repository.Repository, pub *publisher.Publisher, log *zap.Logger, nativeStaleTTL time.Duration) {
	if nativeStaleTTL <= 0 {
		return
	}
	stalePolls, err := repo.ForceCloseStaleNativePolls(ctx, nativeStaleTTL)
	if err != nil {
		log.Warn("native stale-poll sweep failed", zap.Error(err))
	}
	for _, ref := range stalePolls {
		if poll, err := repo.GetPoll(ctx, ref.PollID); err == nil {
			pub.PublishPoll(ctx, poll)
		}
	}
	stalePreds, err := repo.ForceCloseStaleNativePredictions(ctx, nativeStaleTTL)
	if err != nil {
		log.Warn("native stale-prediction sweep failed", zap.Error(err))
	}
	for _, ref := range stalePreds {
		if pred, err := repo.GetPrediction(ctx, ref.PredictionID); err == nil {
			pub.PublishPrediction(ctx, pred)
		}
	}
}

// Config holds runtime configuration.
type Config struct {
	Port                  string
	GinMode               string
	DatabaseHost          string
	DatabasePort          string
	DatabaseUser          string
	DatabasePassword      string
	DatabaseName          string
	RateLimitPerMin       int
	IdempotencyTTLSeconds int
	SweepIntervalSeconds  int
	NativeStaleTTLSeconds int
	StatementTimeoutMs    int
	LockTimeoutMs         int
}

func loadConfig() *Config {
	return &Config{
		Port:                  getEnv("PORT", "8093"),
		GinMode:               getEnv("GIN_MODE", "debug"),
		DatabaseHost:          getEnv("DATABASE_HOST", "localhost"),
		DatabasePort:          getEnv("DATABASE_PORT", "5432"),
		DatabaseUser:          getEnv("DATABASE_USER", "allchat"),
		DatabasePassword:      getEnv("DATABASE_PASSWORD", ""),
		DatabaseName:          getEnv("DATABASE_NAME", "allchat"),
		RateLimitPerMin:       getEnvInt("ENGAGEMENT_RATE_PER_MIN", 120),
		IdempotencyTTLSeconds: getEnvInt("ENGAGEMENT_IDEMPOTENCY_TTL_SECONDS", 60),
		SweepIntervalSeconds:  getEnvInt("ENGAGEMENT_SWEEP_INTERVAL_SECONDS", 10),
		// 4h: comfortably longer than any real Twitch round (predictions cap ~30m of
		// open + resolution window) yet well below "stranded forever" (M1).
		NativeStaleTTLSeconds: getEnvInt("ENGAGEMENT_NATIVE_STALE_TTL_SECONDS", 14400),
		StatementTimeoutMs:    getEnvInt("ENGAGEMENT_STATEMENT_TIMEOUT_MS", 5000),
		LockTimeoutMs:         getEnvInt("ENGAGEMENT_LOCK_TIMEOUT_MS", 3000),
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
