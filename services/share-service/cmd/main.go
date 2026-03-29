package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/caesar/all-chat/services/share-service/cycles"
	"github.com/caesar/all-chat/services/share-service/featuregates"
	"github.com/caesar/all-chat/services/share-service/handlers"
	"github.com/caesar/all-chat/services/share-service/jobs"
	localMiddleware "github.com/caesar/all-chat/services/share-service/middleware"
	"github.com/caesar/all-chat/services/share-service/repository"
	"github.com/caesar/all-chat/shared/database"
	"github.com/caesar/all-chat/shared/logger"
	"github.com/caesar/all-chat/shared/middleware"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func main() {
	// Initialize logger
	logLevel := getEnv("LOG_LEVEL", "info")
	log := logger.NewLogger("share-service", logLevel)
	defer log.Sync()

	log.Info("Starting Share Service",
		zap.String("version", getEnv("APP_VERSION", "0.1.0")),
	)

	// Load configuration from environment
	config := loadConfig()

	// Get JWT secret from environment
	jwtSecret := getEnv("JWT_SECRET", "")
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET environment variable required")
	}

	// Connect to PostgreSQL
	log.Info("Connecting to PostgreSQL",
		zap.String("host", config.DatabaseHost),
		zap.String("database", config.DatabaseName),
	)

	connString := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		config.DatabaseUser,
		config.DatabasePassword,
		config.DatabaseHost,
		config.DatabasePort,
		config.DatabaseName,
	)

	dbPool, err := database.NewPostgresPool(connString)
	if err != nil {
		log.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer dbPool.Close()

	log.Info("Connected to PostgreSQL successfully")

	// Connect to Redis
	redisHost := getEnv("REDIS_HOST", "localhost")
	redisPort := getEnv("REDIS_PORT", "6379")
	redisClient := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%s", redisHost, redisPort),
	})

	var redisClientForJobs *redis.Client
	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		log.Warn("Failed to connect to Redis (lifecycle events disabled)", zap.Error(err))
		redisClient.Close()
	} else {
		log.Info("Connected to Redis")
		redisClientForJobs = redisClient
		defer redisClient.Close()
	}

	// Initialize feature gate cache (D-07: DB source of truth + Redis Pub/Sub invalidation)
	gateCache := featuregates.NewFeatureGateCache(dbPool, redisClientForJobs, log)
	if err := gateCache.Start(context.Background()); err != nil {
		log.Fatal("Failed to start feature gate cache", zap.Error(err))
	}

	// Initialize repositories
	premiumRepo := repository.NewPremiumRepository(dbPool, log)
	userSearchRepo := repository.NewUserSearchRepository(dbPool, log)
	shareRepo := repository.NewShareRepository(dbPool, log)

	// Initialize cycle detector
	cycleDetector := cycles.NewCycleDetector(shareRepo)

	// Initialize and start expiry job
	expiryJob := jobs.NewExpiryJob(shareRepo, log)
	expiryJob.Start(context.Background())

	// Start lifecycle subscriber for stream-end expiry (EXPIRY-03)
	lifecycleSub := jobs.NewLifecycleSubscriber(shareRepo, redisClientForJobs, dbPool, log.Sugar())
	lifecycleSub.Start(context.Background())

	// Initialize handlers
	adminHandler := handlers.NewAdminHandler(premiumRepo, log)
	searchHandler := handlers.NewSearchHandler(userSearchRepo, log)
	shareHandler := handlers.NewShareHandler(shareRepo, userSearchRepo, dbPool, log, cycleDetector)
	adminFGHandler := handlers.NewAdminFeatureGatesHandler(dbPool, redisClientForJobs, log)

	// Setup Gin router
	if config.GinMode == "release" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())

	// CORS is handled by API Gateway, not by individual services
	router.Use(gin.LoggerWithConfig(gin.LoggerConfig{
		SkipPaths: []string{"/health/live", "/health/ready"},
	}))

	// Health check routes (no auth required)
	router.GET("/health/live", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "alive"})
	})

	router.GET("/health/ready", func(c *gin.Context) {
		// Check database connection
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

		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})

	// Prometheus metrics endpoint
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// API routes with authentication
	api := router.Group("/api/v1")
	api.Use(middleware.JWTAuth(jwtSecret)) // All routes require auth
	{
		// User search - no premium required
		api.GET("/users/search", searchHandler.SearchUsers)

		// Share requests - GET doesn't need premium check
		// User decision from RESEARCH.md Pitfall #5: Non-premium users can VIEW but cannot CREATE/ACCEPT
		api.GET("/shares/incoming", shareHandler.ListIncoming)                    // No premium middleware
		api.GET("/shares/accepted", shareHandler.GetAcceptedShares)              // No premium - read-only informational
		api.GET("/shares/unseen-acceptances", shareHandler.GetUnseenAcceptances) // No premium - all senders can see
		api.POST("/shares/:id/mark-seen", shareHandler.MarkAcceptanceSeen)       // No premium - all senders can mark seen
		api.POST("/shares/:id/revoke", shareHandler.RevokeShareRequest)          // No premium - revoking should always be allowed

		// Premium-only routes (only creating a share requires premium)
		premiumRoutes := api.Group("")
		premiumRoutes.Use(localMiddleware.RequirePremium(dbPool, gateCache, featuregates.GateSharing, log)) // Premium middleware
		{
			premiumRoutes.POST("/shares", shareHandler.CreateRequest)
		}

		// Non-premium routes for receivers (accept/reject should not require premium)
		api.POST("/shares/:id/accept", shareHandler.AcceptRequest)
		api.POST("/shares/:id/reject", shareHandler.RejectRequest)

		// Admin routes — requires valid JWT + admin role
		// NOTE: mounted at /admin/premium/... to avoid routing conflict with auth-service /admin/users
		adminRoutes := api.Group("/admin/premium")
		adminRoutes.Use(middleware.AdminOnly())
		{
			adminRoutes.POST("/users/:id", adminHandler.SetUserPremium)
		}

		// Feature gate admin routes (separate from /admin/premium to avoid path conflict)
		featureGateRoutes := api.Group("/admin/feature-gates")
		featureGateRoutes.Use(middleware.AdminOnly())
		{
			featureGateRoutes.GET("", adminFGHandler.ListGates)
			featureGateRoutes.PATCH("/:key", adminFGHandler.UpdateGate)
		}
	}

	// Create HTTP server
	srv := &http.Server{
		Addr:         ":" + config.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		log.Info("Share Service started",
			zap.String("port", config.Port),
			zap.String("mode", config.GinMode),
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

	// CRITICAL: Stop expiry job BEFORE server shutdown
	// This ensures current expiry operation can complete
	expiryJob.Stop()

	// Graceful shutdown with 25-second timeout
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error("Server forced to shutdown", zap.Error(err))
	}

	log.Info("Server exited")
}

// Config holds application configuration
type Config struct {
	Port             string
	GinMode          string
	DatabaseHost     string
	DatabasePort     string
	DatabaseUser     string
	DatabasePassword string
	DatabaseName     string
}

// loadConfig loads configuration from environment variables
func loadConfig() *Config {
	return &Config{
		Port:             getEnv("PORT", "8090"), // Share service port
		GinMode:          getEnv("GIN_MODE", "debug"),
		DatabaseHost:     getEnv("DATABASE_HOST", "localhost"),
		DatabasePort:     getEnv("DATABASE_PORT", "5432"),
		DatabaseUser:     getEnv("DATABASE_USER", "allchat"),
		DatabasePassword: getEnv("DATABASE_PASSWORD", "allchat_dev_password"),
		DatabaseName:     getEnv("DATABASE_NAME", "allchat"),
	}
}

// getEnv gets an environment variable with a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

