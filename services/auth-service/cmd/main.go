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

// Updated: 2025-12-19 20:57 - Viewer JWT checked first in middleware

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/caesar/all-chat/services/auth-service/handlers"
	"github.com/caesar/all-chat/services/auth-service/oauth"
	"github.com/caesar/all-chat/services/auth-service/repository"
	sharedAuth "github.com/caesar/all-chat/shared/auth"
	"github.com/caesar/all-chat/shared/database"
	"github.com/caesar/all-chat/shared/encryption"
	"github.com/caesar/all-chat/shared/logger"
	"github.com/caesar/all-chat/shared/metrics"
	"github.com/caesar/all-chat/shared/middleware"
	"github.com/caesar/all-chat/shared/premium"
	sharedredis "github.com/caesar/all-chat/shared/redis"
	"github.com/caesar/all-chat/shared/tracing"
	"github.com/caesar/all-chat/shared/youtubetoken"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

func main() {
	// Initialize logger
	logLevel := getEnvOrDefault("LOG_LEVEL", "info")
	log := logger.NewLogger("auth-service", logLevel)
	defer log.Sync()

	log.Info("Starting Auth Service",
		zap.String("version", getEnvOrDefault("APP_VERSION", "dev")),
	)

	// Initialize OpenTelemetry tracing
	tracingEnabled := getEnvOrDefault("OTEL_ENABLED", "false") == "true"
	if tracingEnabled {
		tracingCfg := tracing.Config{
			ServiceName:    "auth-service",
			ServiceVersion: getEnvOrDefault("APP_VERSION", "dev"),
			Environment:    getEnvOrDefault("ENVIRONMENT", "development"),
			OTLPEndpoint:   getEnvOrDefault("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317"),
			Enabled:        true,
		}

		shutdownTracer, err := tracing.InitTracer(tracingCfg, log)
		if err != nil {
			log.Error("Failed to initialize tracer (continuing without tracing)", zap.Error(err))
		} else {
			defer shutdownTracer(context.Background())
			log.Info("OpenTelemetry tracing enabled")
		}
	}

	ctx := context.Background()

	// Get environment variables
	frontendURL := strings.TrimSuffix(getEnvOrDefault("FRONTEND_URL", "http://localhost:3000"), "/")

	twitchClientID := os.Getenv("TWITCH_CLIENT_ID")
	twitchClientSecret := os.Getenv("TWITCH_CLIENT_SECRET")
	twitchRedirectURL := defaultCallbackURL(frontendURL, "http://localhost:8080", "/api/v1/auth/twitch/callback")

	youtubeClientID := os.Getenv("YOUTUBE_CLIENT_ID")
	youtubeClientSecret := os.Getenv("YOUTUBE_CLIENT_SECRET")
	youtubeAPIKey := os.Getenv("YOUTUBE_API_KEY")
	youtubeRedirectURL := defaultCallbackURL(frontendURL, "http://localhost:8080", "/api/v1/auth/youtube/callback")

	kickClientID := os.Getenv("KICK_CLIENT_ID")
	kickClientSecret := os.Getenv("KICK_CLIENT_SECRET")
	kickRedirectURL := defaultCallbackURL(frontendURL, "http://localhost:8080", "/api/v1/auth/kick/callback")

	discordClientID := os.Getenv("DISCORD_CLIENT_ID")
	discordClientSecret := os.Getenv("DISCORD_CLIENT_SECRET")
	discordBotToken := os.Getenv("DISCORD_BOT_TOKEN")
	discordRedirectURL := defaultCallbackURL(frontendURL, "http://localhost:8080", "/api/v1/auth/discord/callback")

	jwtExpiryHours := getEnvAsIntOrDefault("JWT_EXPIRY_HOURS", 24)

	if twitchClientID == "" || twitchClientSecret == "" {
		log.Fatal("TWITCH_CLIENT_ID and TWITCH_CLIENT_SECRET must be set")
	}

	if youtubeClientID == "" || youtubeClientSecret == "" {
		log.Warn("YOUTUBE_CLIENT_ID and YOUTUBE_CLIENT_SECRET not set, YouTube OAuth will not be available")
	}

	if youtubeAPIKey == "" {
		log.Warn("YOUTUBE_API_KEY not set — YouTube live chat ID lookup will fail; set YOUTUBE_API_KEY to a server-side Data API key")
	}

	if kickClientID == "" || kickClientSecret == "" {
		log.Warn("KICK_CLIENT_ID and KICK_CLIENT_SECRET not set, Kick OAuth will not be available")
	}

	if discordClientID == "" || discordClientSecret == "" || discordBotToken == "" {
		log.Warn("DISCORD_CLIENT_ID, DISCORD_CLIENT_SECRET, DISCORD_BOT_TOKEN not set — Discord integration disabled")
	}

	userKeyChain, err := sharedAuth.NewKeyChainFromEnv("JWT_SECRET")
	if err != nil {
		log.Fatal("JWT key chain init failed (JWT_SECRET_V1 must be set)", zap.Error(err))
	}
	log.Info("JWT key chain initialized", zap.String("latest_kid", userKeyChain.LatestKid()))

	tokenCipher, err := encryption.NewMultiKeyEncryptorFromEnv()
	if err != nil {
		log.Fatal("failed to initialize token cipher (TOKEN_ENCRYPTION_KEY_V1 must be set; legacy TOKEN_ENCRYPTION_KEY optional)", zap.Error(err))
	}
	log.Info("token cipher initialized", zap.Uint8("current_kid", tokenCipher.CurrentKid()))

	// Connect to PostgreSQL
	dbHost := getEnvOrDefault("DATABASE_HOST", "localhost")
	dbPort := getEnvOrDefault("DATABASE_PORT", "5432")
	dbUser := getEnvOrDefault("DATABASE_USER", "allchat")
	dbPassword := getEnvOrDefault("DATABASE_PASSWORD", "allchat_dev_password")
	dbName := getEnvOrDefault("DATABASE_NAME", "allchat")

	connString := fmt.Sprintf("postgres://%s:%s@%s:%s/%s",
		dbUser, dbPassword, dbHost, dbPort, dbName)

	db, err := database.NewPostgresPool(connString)
	if err != nil {
		log.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer db.Close()

	log.Info("Connected to PostgreSQL")

	// Connect to Redis, retrying with backoff so a transient Redis outage
	// (e.g. the pod being rescheduled onto another node) does not crash-loop
	// this service. The retry is cancelled on shutdown signals so SIGTERM still
	// terminates the process promptly while it is waiting for Redis.
	redisHost := getEnvOrDefault("REDIS_HOST", "localhost")
	redisPort := getEnvOrDefault("REDIS_PORT", "6379")
	redisAddr := sharedredis.BuildDSN(redisHost, redisPort)

	startupCtx, stopStartup := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	redisClient, err := sharedredis.NewClientWithRetry(startupCtx, redisAddr, getEnvOrDefault("REDIS_PASSWORD", ""), tracingEnabled,
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

	// Initialize metrics (available via /metrics endpoint)
	businessMetrics := metrics.NewBusinessMetrics()
	log.Info("Initialized Prometheus metrics")

	// Initialize components
	twitchOAuth := oauth.NewTwitchOAuth(twitchClientID, twitchClientSecret, twitchRedirectURL)

	var youtubeOAuth *oauth.YouTubeOAuth
	if youtubeClientID != "" && youtubeClientSecret != "" {
		youtubeOAuth = oauth.NewYouTubeOAuth(youtubeClientID, youtubeClientSecret, youtubeRedirectURL)
	}

	var kickOAuth *oauth.KickOAuth
	if kickClientID != "" && kickClientSecret != "" {
		kickOAuth = oauth.NewKickOAuth(kickClientID, kickClientSecret, kickRedirectURL)
	}

	var discordHandler *handlers.DiscordHandler
	if discordClientID != "" && discordClientSecret != "" && discordBotToken != "" {
		discordOAuth := oauth.NewDiscordOAuth(discordClientID, discordClientSecret, discordRedirectURL).
			WithBotToken(discordBotToken)
		discordRepo := repository.NewDiscordRepository(db)
		discordHandler = handlers.NewDiscordHandler(discordOAuth, discordRepo, redisClient, discordBotToken, frontendURL, log)
	}

	userRepo := repository.NewUserRepository(db, tokenCipher)

	// Create platform provider registry
	providers := make(map[oauth.Platform]oauth.OAuthProvider)
	providers[oauth.PlatformTwitch] = twitchOAuth
	if youtubeOAuth != nil {
		providers[oauth.PlatformYouTube] = youtubeOAuth
	}
	if kickOAuth != nil {
		providers[oauth.PlatformKick] = kickOAuth
	}

	overlayManagerURL := getEnvOrDefault("OVERLAY_MANAGER_URL", "http://localhost:8082")

	// Create viewer OAuth providers (with chat write scopes)
	viewerTwitchRedirectURL := defaultCallbackURL(frontendURL, "http://localhost:8080", "/api/v1/auth/viewer/twitch/callback")
	viewerTwitchOAuth := oauth.NewViewerTwitchOAuth(twitchClientID, twitchClientSecret, viewerTwitchRedirectURL)

	viewerYouTubeRedirectURL := defaultCallbackURL(frontendURL, "http://localhost:8080", "/api/v1/auth/viewer/youtube/callback")
	viewerYouTubeOAuth := oauth.NewViewerYouTubeOAuth(youtubeClientID, youtubeClientSecret, viewerYouTubeRedirectURL)

	viewerKickRedirectURL := defaultCallbackURL(frontendURL, "http://localhost:8080", "/api/v1/auth/viewer/kick/callback")
	viewerKickOAuth := oauth.NewViewerKickOAuth(kickClientID, kickClientSecret, viewerKickRedirectURL)

	// Viewer premium recompute (ADR-0019): derives viewers.is_premium from the
	// viewer admin override + active viewer subscription + linked-streamer
	// inheritance. Shared by the admin override path and the viewer↔streamer link.
	viewerPremiumRecomputer := premium.NewRecomputer(db, log)

	// Create viewer repository
	viewerRepo := repository.NewViewerRepository(db, tokenCipher, viewerPremiumRecomputer)

	// Create viewer identity repository (Phase 28: cross-platform viewer linking)
	viewerIdentityRepo := repository.NewViewerIdentityRepository(db, viewerPremiumRecomputer)

	// Create handlers
	platformAuthHandlerV2 := handlers.NewPlatformAuthHandlerV2(providers, userRepo, redisClient, userKeyChain, jwtExpiryHours, frontendURL, overlayManagerURL, log).WithMetrics(businessMetrics)
	legacyAuthHandler := handlers.NewAuthHandler(twitchOAuth, youtubeOAuth, userRepo, redisClient, userKeyChain, jwtExpiryHours, log).WithMetrics(businessMetrics)
	viewerAuthHandler := handlers.NewViewerAuthHandler(viewerTwitchOAuth, viewerYouTubeOAuth, viewerKickOAuth, viewerRepo, viewerIdentityRepo, userRepo, redisClient, userKeyChain, jwtExpiryHours, frontendURL, tokenCipher, log).WithMetrics(businessMetrics)

	// Seed the persistent total-users gauge from the database so Grafana retains
	// an accurate baseline even after pod restarts (Prometheus counters are ephemeral).
	if counts, err := userRepo.CountByAuthProvider(ctx); err != nil {
		log.Warn("Failed to seed total-users metric from database (non-fatal)", zap.Error(err))
	} else {
		businessMetrics.InitTotalUsersByPlatform(counts)
		log.Info("Seeded allchat_total_users_by_platform from database", zap.Any("counts", counts))
	}
	healthHandler := handlers.NewHealthHandler(db, redisClient)
	adminHandler := handlers.NewAdminHandler(userRepo, db, log, userKeyChain)
	viewerCosmeticsHandler := handlers.NewViewerCosmeticsHandler(viewerIdentityRepo, redisClient, log)
	chatSendHandler := handlers.NewChatSendHandler(log, viewerRepo, userRepo, db, twitchClientID, viewerTwitchOAuth, viewerYouTubeOAuth, viewerKickOAuth, tokenCipher, youtubeAPIKey, redisClient, getEnvAsIntOrDefault("QUOTA_LIMIT_DAILY", 1009000)).WithYouTubeTokenSource(youtubetoken.NewYouTubeSource(db, tokenCipher, youtubeClientID, youtubeClientSecret))
	streamerInfoHandler := handlers.NewStreamerInfoHandler(log, userRepo, db)
	adminViewerHandler := handlers.NewAdminViewerHandler(log, viewerRepo)
	adminCosmeticsHandler := handlers.NewAdminCosmeticsHandler(log, db)
	riscHandler := handlers.NewRISCHandler(log, db)

	// Set Gin mode
	if logLevel == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	// Initialize HTTP metrics
	httpRequestsTotal := promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total HTTP requests",
	}, []string{"service", "method", "path", "status"})

	httpRequestDuration := promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request duration in seconds",
		Buckets: prometheus.DefBuckets,
	}, []string{"service", "method", "path"})

	// Create router
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(middleware.Logging(log))
	router.Use(httpMetricsMiddleware(httpRequestsTotal, httpRequestDuration, "auth-service"))

	// Add tracing middleware if enabled
	if tracingEnabled {
		router.Use(tracing.GinMiddleware("auth-service"))
	}

	// CORS is handled by API Gateway, not by individual services

	// Health check endpoints
	router.GET("/health/live", healthHandler.CheckLive)
	router.GET("/health/ready", healthHandler.CheckReady)

	// Prometheus metrics endpoint
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// RISC (Cross-Account Protection) endpoints - Required for Google OAuth verification
	// These endpoints receive security events from Google about user accounts
	router.POST("/.well-known/risc-events", riscHandler.HandleSecurityEvent)
	router.GET("/.well-known/risc-configuration", riscHandler.HandleConfigurationEndpoint)
	router.GET("/.well-known/jwks.json", riscHandler.HandleJWKSEndpoint)

	// Auth routes (no /auth prefix - API Gateway strips /api/v1/auth and forwards rest)
	// Twitch OAuth legacy endpoints now route through the V2 handler for compatibility
	router.GET("/login", platformAuthHandlerV2.HandleLogin(oauth.PlatformTwitch))
	router.POST("/login", platformAuthHandlerV2.HandleLogin(oauth.PlatformTwitch))
	router.GET("/callback", platformAuthHandlerV2.HandleCallback(oauth.PlatformTwitch))

	// Platform-based OAuth routes (generalized for all platforms)
	// V2 handlers support enhanced state management for add-source flows
	router.GET("/twitch/login", platformAuthHandlerV2.HandleLogin(oauth.PlatformTwitch))
	router.GET("/twitch/callback", platformAuthHandlerV2.HandleCallback(oauth.PlatformTwitch))

	router.GET("/youtube/login", platformAuthHandlerV2.HandleLogin(oauth.PlatformYouTube))
	router.GET("/youtube/callback", platformAuthHandlerV2.HandleCallback(oauth.PlatformYouTube))

	router.GET("/kick/login", platformAuthHandlerV2.HandleLogin(oauth.PlatformKick))
	router.GET("/kick/callback", platformAuthHandlerV2.HandleCallback(oauth.PlatformKick))

	// Discord bot OAuth callback (public — no JWT required; the CSRF state encodes the user identity)
	router.GET("/discord/callback", func(c *gin.Context) {
		if discordHandler == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Discord integration not configured"})
			return
		}
		discordHandler.HandleCallback(c)
	})

	// Token refresh
	router.POST("/refresh", legacyAuthHandler.HandleRefresh)

	// Viewer auth routes (separate from streamer auth)
	router.GET("/viewer/twitch/login", viewerAuthHandler.HandleTwitchLogin)
	router.GET("/viewer/twitch/callback", viewerAuthHandler.HandleTwitchCallback)
	router.POST("/viewer/twitch/exchange", viewerAuthHandler.HandleTwitchExchange)
	router.GET("/viewer/youtube/login", viewerAuthHandler.HandleYouTubeLogin)
	router.GET("/viewer/youtube/callback", viewerAuthHandler.HandleYouTubeCallback)
	router.POST("/viewer/youtube/exchange", viewerAuthHandler.HandleYouTubeExchange)
	router.GET("/viewer/kick/login", viewerAuthHandler.HandleKickLogin)
	router.GET("/viewer/kick/callback", viewerAuthHandler.HandleKickCallback)
	router.POST("/viewer/kick/exchange", viewerAuthHandler.HandleKickExchange)

	// Auth code exchange (viewer trades short-lived code for JWT)
	router.POST("/viewer/token/exchange", viewerAuthHandler.HandleTokenExchange)

	// Public streamer info routes
	router.GET("/streamers/:username", streamerInfoHandler.HandleGetStreamerInfo)

	// Protected routes (require JWT)
	protected := router.Group("/")
	protected.Use(middleware.JWTAuth(userKeyChain))
	{
		protected.GET("/me", legacyAuthHandler.HandleGetMe)
		protected.GET("/me/data-export", legacyAuthHandler.HandleDataExport)
		protected.POST("/logout", legacyAuthHandler.HandleLogout)
		protected.DELETE("/me", legacyAuthHandler.HandleDeleteAccount)

		// Streamer chat send (uses streamer's own OAuth tokens)
		protected.POST("/chat/send", chatSendHandler.HandleStreamerSendMessage)

		// Add-source routes (require JWT for account linking)
		protected.GET("/twitch/add-source/:overlay_id", platformAuthHandlerV2.HandleAddSource(oauth.PlatformTwitch))
		protected.GET("/youtube/add-source/:overlay_id", platformAuthHandlerV2.HandleAddSource(oauth.PlatformYouTube))
		protected.GET("/kick/add-source/:overlay_id", platformAuthHandlerV2.HandleAddSource(oauth.PlatformKick))

		// Opt-in moderation re-consent (ADR-0017): requests only the moderation scopes
		// for the ?actions= being enabled, on top of the existing grant. Twitch + Kick.
		protected.GET("/twitch/moderation/:overlay_id", platformAuthHandlerV2.HandleEnableModeration(oauth.PlatformTwitch))
		protected.GET("/kick/moderation/:overlay_id", platformAuthHandlerV2.HandleEnableModeration(oauth.PlatformKick))
		protected.GET("/youtube/moderation/:overlay_id", platformAuthHandlerV2.HandleEnableModeration(oauth.PlatformYouTube))

		// Discord guild management routes (require JWT)
		if discordHandler != nil {
			protected.GET("/discord/connect", discordHandler.HandleConnect)
			protected.GET("/guilds", discordHandler.HandleGetGuilds)
			protected.GET("/guilds/:guild_id/channels", discordHandler.HandleGetGuildChannels)
			protected.DELETE("/guilds/:guild_id", discordHandler.HandleDisconnect)
		}
	}

	// Public viewer catalog routes (no JWT required — cosmetic catalogs are not sensitive)
	viewerPublic := router.Group("/viewer")
	{
		viewerPublic.GET("/catalog/frames", adminCosmeticsHandler.HandleListFrames)
		viewerPublic.GET("/catalog/flairs", adminCosmeticsHandler.HandleListFlairs)
	}

	// Viewer protected routes (require viewer JWT)
	viewerProtected := router.Group("/viewer")
	viewerProtected.Use(middleware.JWTAuth(userKeyChain))
	{
		viewerProtected.GET("/me", viewerAuthHandler.HandleMe)
		viewerProtected.POST("/logout", viewerAuthHandler.HandleLogout)
		viewerProtected.POST("/chat/send", chatSendHandler.HandleSendMessage)
		viewerProtected.GET("/cosmetics", viewerCosmeticsHandler.HandleGetCosmetics)
		viewerProtected.PATCH("/cosmetics", viewerCosmeticsHandler.HandlePatchCosmetics)
		viewerProtected.GET("/linked-platforms", viewerAuthHandler.HandleGetLinkedPlatforms)
		viewerProtected.DELETE("/linked-platforms/:platform", viewerAuthHandler.HandleUnlinkPlatform)
	}

	// Admin routes (JWT + Admin role required)
	admin := router.Group("/admin")
	admin.Use(middleware.JWTAuth(userKeyChain))
	admin.Use(middleware.AdminOnly())
	{
		admin.GET("/users", adminHandler.ListUsers)
		admin.GET("/users/:id", adminHandler.GetUser)
		admin.POST("/users/:id/impersonate", adminHandler.ImpersonateUser)

		// User ban management
		admin.POST("/users/:id/ban", adminHandler.BanUser)
		admin.POST("/users/:id/unban", adminHandler.UnbanUser)
		admin.GET("/users/banned", adminHandler.ListBannedUsers)

		// Admin stats
		admin.GET("/stats", adminHandler.GetDashboardStats)

		// Viewer management
		admin.GET("/viewers", adminViewerHandler.HandleListViewers)
		admin.POST("/viewers/:session_id/ban", adminViewerHandler.HandleBanViewer)
		admin.POST("/viewers/:session_id/unban", adminViewerHandler.HandleUnbanViewer)
		admin.POST("/viewers/:session_id/premium", adminViewerHandler.HandleSetViewerPremium)

		// Cosmetic catalog management (frames and flairs)
		admin.GET("/cosmetics/frames", adminCosmeticsHandler.HandleListFrames)
		admin.POST("/cosmetics/frames", adminCosmeticsHandler.HandleCreateFrame)
		admin.DELETE("/cosmetics/frames/:id", adminCosmeticsHandler.HandleDeleteFrame)
		admin.GET("/cosmetics/flairs", adminCosmeticsHandler.HandleListFlairs)
		admin.POST("/cosmetics/flairs", adminCosmeticsHandler.HandleCreateFlair)
		admin.DELETE("/cosmetics/flairs/:id", adminCosmeticsHandler.HandleDeleteFlair)
	}

	// Get port from environment
	port := getEnvOrDefault("PORT", "8081")

	// Create HTTP server
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Info("Auth Service listening",
			zap.String("port", port),
		)

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down server...")

	// Give outstanding requests 25 seconds to complete
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error("Server forced to shutdown", zap.Error(err))
	}

	log.Info("Server exited")
}

// getEnvOrDefault gets an environment variable or returns a default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func defaultCallbackURL(frontendURL, fallbackBase, path string) string {
	normalizedPath := path
	if !strings.HasPrefix(normalizedPath, "/") {
		normalizedPath = "/" + normalizedPath
	}

	trimmedFrontend := strings.TrimSuffix(frontendURL, "/")
	trimmedFallback := strings.TrimSuffix(fallbackBase, "/")
	lowerFrontend := strings.ToLower(trimmedFrontend)

	if trimmedFrontend == "" ||
		strings.Contains(lowerFrontend, "localhost") ||
		strings.Contains(lowerFrontend, "127.0.0.1") {
		return trimmedFallback + normalizedPath
	}

	if strings.HasPrefix(trimmedFrontend, "http://") || strings.HasPrefix(trimmedFrontend, "https://") {
		return trimmedFrontend + normalizedPath
	}

	return trimmedFallback + normalizedPath
}

// getEnvAsIntOrDefault gets an environment variable as int or returns default
func getEnvAsIntOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// httpMetricsMiddleware records HTTP request count and duration metrics
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
