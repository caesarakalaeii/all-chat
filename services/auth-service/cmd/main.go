package main

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
	"github.com/caesar/all-chat/shared/database"
	"github.com/caesar/all-chat/shared/encryption"
	"github.com/caesar/all-chat/shared/logger"
	"github.com/caesar/all-chat/shared/middleware"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
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

	tiktokClientKey := os.Getenv("TIKTOK_CLIENT_KEY")
	tiktokClientSecret := os.Getenv("TIKTOK_CLIENT_SECRET")
	tiktokRedirectURL := defaultCallbackURL(frontendURL, "http://localhost:8080", "/api/v1/auth/tiktok/callback")

	kickClientID := os.Getenv("KICK_CLIENT_ID")
	kickClientSecret := os.Getenv("KICK_CLIENT_SECRET")
	kickRedirectURL := defaultCallbackURL(frontendURL, "http://localhost:8080", "/api/v1/auth/kick/callback")

	jwtSecret := os.Getenv("JWT_SECRET")
	jwtExpiryHours := getEnvAsIntOrDefault("JWT_EXPIRY_HOURS", 24)
	tokenEncryptionKey := os.Getenv("TOKEN_ENCRYPTION_KEY")

	if twitchClientID == "" || twitchClientSecret == "" {
		log.Fatal("TWITCH_CLIENT_ID and TWITCH_CLIENT_SECRET must be set")
	}

	if youtubeClientID == "" || youtubeClientSecret == "" || youtubeAPIKey == "" {
		log.Warn("YOUTUBE_CLIENT_ID, YOUTUBE_CLIENT_SECRET, and YOUTUBE_API_KEY not set, YouTube OAuth will not be available")
	}

	if tiktokClientKey == "" || tiktokClientSecret == "" {
		log.Warn("TIKTOK_CLIENT_KEY and TIKTOK_CLIENT_SECRET not set, TikTok OAuth will not be available")
	}

	if kickClientID == "" || kickClientSecret == "" {
		log.Warn("KICK_CLIENT_ID and KICK_CLIENT_SECRET not set, Kick OAuth will not be available")
	}

	if jwtSecret == "" {
		log.Fatal("JWT_SECRET must be set")
	}

	if tokenEncryptionKey == "" {
		log.Fatal("TOKEN_ENCRYPTION_KEY must be set and must be 16, 24, or 32 bytes")
	}

	parsedKey, err := encryption.ParseKey(tokenEncryptionKey)
	if err != nil {
		log.Fatal("failed to parse TOKEN_ENCRYPTION_KEY", zap.Error(err))
	}

	tokenCipher, err := encryption.NewAESEncryptor(parsedKey)
	if err != nil {
		log.Fatal("failed to initialize token cipher", zap.Error(err))
	}

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

	// Connect to Redis
	redisHost := getEnvOrDefault("REDIS_HOST", "localhost")
	redisPort := getEnvOrDefault("REDIS_PORT", "6379")
	redisClient := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%s", redisHost, redisPort),
	})
	defer redisClient.Close()

	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatal("Failed to connect to Redis", zap.Error(err))
	}

	log.Info("Connected to Redis")

	// Initialize components
	twitchOAuth := oauth.NewTwitchOAuth(twitchClientID, twitchClientSecret, twitchRedirectURL)

	var youtubeOAuth *oauth.YouTubeOAuth
	if youtubeClientID != "" && youtubeClientSecret != "" && youtubeAPIKey != "" {
		youtubeOAuth = oauth.NewYouTubeOAuth(youtubeClientID, youtubeClientSecret, youtubeRedirectURL, youtubeAPIKey)
	}

	var tiktokOAuth *oauth.TikTokOAuth
	if tiktokClientKey != "" && tiktokClientSecret != "" {
		tiktokOAuth = oauth.NewTikTokOAuth(tiktokClientKey, tiktokClientSecret, tiktokRedirectURL)
	}

	var kickOAuth *oauth.KickOAuth
	if kickClientID != "" && kickClientSecret != "" {
		kickOAuth = oauth.NewKickOAuth(kickClientID, kickClientSecret, kickRedirectURL)
	}

	userRepo := repository.NewUserRepository(db, tokenCipher)

	// Create platform provider registry
	providers := make(map[oauth.Platform]oauth.OAuthProvider)
	providers[oauth.PlatformTwitch] = twitchOAuth
	if youtubeOAuth != nil {
		providers[oauth.PlatformYouTube] = youtubeOAuth
	}
	if tiktokOAuth != nil {
		providers[oauth.PlatformTikTok] = tiktokOAuth
	}
	if kickOAuth != nil {
		providers[oauth.PlatformKick] = kickOAuth
	}

	overlayManagerURL := getEnvOrDefault("OVERLAY_MANAGER_URL", "http://localhost:8082")

	// Create handlers
	platformAuthHandlerV2 := handlers.NewPlatformAuthHandlerV2(providers, userRepo, redisClient, jwtSecret, jwtExpiryHours, frontendURL, overlayManagerURL, log)
	legacyAuthHandler := handlers.NewAuthHandler(twitchOAuth, youtubeOAuth, userRepo, redisClient, jwtSecret, jwtExpiryHours, log)
	healthHandler := handlers.NewHealthHandler(db, redisClient)

	// Set Gin mode
	if logLevel == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	// Create router
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(middleware.Logging(log))
	// CORS is handled by API Gateway, not by individual services

	// Health check endpoints
	router.GET("/health/live", healthHandler.CheckLive)
	router.GET("/health/ready", healthHandler.CheckReady)

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

	router.GET("/tiktok/login", platformAuthHandlerV2.HandleLogin(oauth.PlatformTikTok))
	router.GET("/tiktok/callback", platformAuthHandlerV2.HandleCallback(oauth.PlatformTikTok))

	router.GET("/kick/login", platformAuthHandlerV2.HandleLogin(oauth.PlatformKick))
	router.GET("/kick/callback", platformAuthHandlerV2.HandleCallback(oauth.PlatformKick))

	// Token refresh
	router.POST("/refresh", legacyAuthHandler.HandleRefresh)

	// Protected routes (require JWT)
	protected := router.Group("/")
	protected.Use(middleware.JWTAuth(jwtSecret))
	{
		protected.GET("/me", legacyAuthHandler.HandleGetMe)
		protected.POST("/logout", legacyAuthHandler.HandleLogout)
		protected.DELETE("/me", legacyAuthHandler.HandleDeleteAccount)

		// Add-source routes (require JWT for account linking)
		protected.GET("/twitch/add-source/:overlay_id", platformAuthHandlerV2.HandleAddSource(oauth.PlatformTwitch))
		protected.GET("/youtube/add-source/:overlay_id", platformAuthHandlerV2.HandleAddSource(oauth.PlatformYouTube))
		protected.GET("/kick/add-source/:overlay_id", platformAuthHandlerV2.HandleAddSource(oauth.PlatformKick))
		protected.GET("/tiktok/add-source/:overlay_id", platformAuthHandlerV2.HandleAddSource(oauth.PlatformTikTok))
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
