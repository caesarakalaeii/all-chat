package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/caesar/all-chat/internal/api-gateway/adapters/api"
	"github.com/caesar/all-chat/internal/api-gateway/adapters/proxy"
	"github.com/caesar/all-chat/internal/api-gateway/adapters/websocket"
	"github.com/caesar/all-chat/pkg/logger"
	redisClient "github.com/caesar/all-chat/pkg/redis"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func main() {
	// Initialize logger
	serviceName := getEnv("SERVICE_NAME", "api-gateway")
	environment := getEnv("ENVIRONMENT", "development")

	if err := logger.Initialize(serviceName, environment); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Info("Starting API Gateway", zap.String("environment", environment))

	// Initialize Redis
	redisConfig := redisClient.Config{
		Host:     getEnv("REDIS_HOST", "localhost"),
		Port:     getEnvAsInt("REDIS_PORT", 6379),
		Password: getEnv("REDIS_PASSWORD", ""),
		DB:       getEnvAsInt("REDIS_DB", 0),
	}

	rdb, err := redisClient.NewClient(redisConfig)
	if err != nil {
		logger.Fatal("Failed to connect to Redis", zap.Error(err))
	}
	defer rdb.Close()

	logger.Info("Redis connection established")

	// Initialize WebSocket hub
	hub := websocket.NewHub(rdb, logger.Log)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start hub in background
	go hub.Run(ctx)

	// Get service URLs
	authServiceURL := getEnv("AUTH_SERVICE_URL", "http://localhost:8081")
	overlayServiceURL := getEnv("OVERLAY_SERVICE_URL", "http://localhost:8082")
	emoteServiceURL := getEnv("EMOTE_SERVICE_URL", "http://localhost:8083")

	// Initialize service proxy
	serviceProxy, err := proxy.NewServiceProxy(authServiceURL, overlayServiceURL, emoteServiceURL, logger.Log)
	if err != nil {
		logger.Fatal("Failed to create service proxy", zap.Error(err))
	}

	// Initialize WebSocket handler
	jwtSecret := getEnv("JWT_SECRET", "default-secret-change-me")
	wsHandler := api.NewHandler(hub, jwtSecret, logger.Log)

	// Initialize Gin router
	if getEnv("ENVIRONMENT", "development") == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.Default()

	// Configure CORS
	corsConfig := cors.Config{
		AllowOrigins:     []string{"*"}, // TODO: Configure for production
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}
	router.Use(cors.New(corsConfig))

	// Health check endpoints
	router.GET("/health/live", wsHandler.HandleHealth)
	router.GET("/health/ready", wsHandler.HandleReadiness)

	// WebSocket endpoint
	router.GET("/ws/overlay/:id", wsHandler.HandleWebSocket)

	// API routes - proxy to backend services
	v1 := router.Group("/api/v1")
	{
		// Auth service routes
		auth := v1.Group("/auth")
		{
			auth.GET("/login", serviceProxy.ProxyToAuthService)
			auth.GET("/callback", serviceProxy.ProxyToAuthService)
			auth.POST("/refresh", serviceProxy.ProxyToAuthService)
			auth.POST("/logout", serviceProxy.ProxyToAuthService)
			auth.GET("/me", serviceProxy.ProxyToAuthService)
		}

		// Overlay manager routes
		overlays := v1.Group("/overlays")
		{
			overlays.GET("", serviceProxy.ProxyToOverlayManager)
			overlays.POST("", serviceProxy.ProxyToOverlayManager)
			overlays.GET("/:id", serviceProxy.ProxyToOverlayManager)
			overlays.PUT("/:id", serviceProxy.ProxyToOverlayManager)
			overlays.DELETE("/:id", serviceProxy.ProxyToOverlayManager)
			overlays.GET("/:id/config", serviceProxy.ProxyToOverlayManager)
			overlays.PUT("/:id/config", serviceProxy.ProxyToOverlayManager)
		}

		// Emote service routes
		emotes := v1.Group("/emotes")
		{
			emotes.GET("/channel/:channel", serviceProxy.ProxyToEmoteService)
			emotes.GET("/:provider/:channel", serviceProxy.ProxyToEmoteService)
		}
	}

	// Serve static files (frontend)
	staticPath := getEnv("STATIC_FILES_PATH", "./web/dist")
	if _, err := os.Stat(staticPath); err == nil {
		logger.Info("Serving static files", zap.String("path", staticPath))
		router.NoRoute(func(c *gin.Context) {
			c.File(staticPath + "/index.html")
		})
		router.Static("/assets", staticPath+"/assets")
	} else {
		logger.Warn("Static files directory not found", zap.String("path", staticPath))
	}

	// Start HTTP server
	port := getEnv("PORT", "8080")
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}

	// Start server in a goroutine
	go func() {
		logger.Info("API Gateway listening", zap.String("port", port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down API Gateway")

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer shutdownCancel()

	// Shutdown HTTP server
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("Server forced to shutdown", zap.Error(err))
	}

	// Cancel hub context
	cancel()

	logger.Info("API Gateway stopped")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var intValue int
		if _, err := fmt.Sscanf(value, "%d", &intValue); err == nil {
			return intValue
		}
	}
	return defaultValue
}
