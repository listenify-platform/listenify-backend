package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap/zapcore"
	"norelock.dev/listenify/backend/internal/api"
	"norelock.dev/listenify/backend/internal/auth"
	"norelock.dev/listenify/backend/internal/config"
	"norelock.dev/listenify/backend/internal/db/mongo"
	"norelock.dev/listenify/backend/internal/db/mongo/repositories"
	"norelock.dev/listenify/backend/internal/db/redis"
	"norelock.dev/listenify/backend/internal/db/redis/managers"
	"norelock.dev/listenify/backend/internal/rpc"
	"norelock.dev/listenify/backend/internal/rpc/methods"
	"norelock.dev/listenify/backend/internal/services/media"
	"norelock.dev/listenify/backend/internal/services/playlist"
	"norelock.dev/listenify/backend/internal/services/room"
	"norelock.dev/listenify/backend/internal/services/system"
	"norelock.dev/listenify/backend/internal/services/user"
	"norelock.dev/listenify/backend/internal/utils"
)

// CombinedAuthProvider combines JWT and password providers to implement the full auth.Provider interface
type CombinedAuthProvider struct {
	*auth.JWTProvider
	*auth.PasswordProvider
}

// convert logger level to zapcore.Level
func hLevel(level string) zapcore.Level {
	switch level {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	case "fatal":
		return zapcore.FatalLevel
	case "panic":
		return zapcore.PanicLevel
	default:
		return zapcore.InfoLevel
	}
}

func main() {
	// Create a context that will be canceled on interrupt signal
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("Received shutdown signal")
		cancel()
	}()

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	loggerOptions := utils.LoggerOptions{
		Development: cfg.Environment == "development",
		Level:       hLevel(cfg.Logging.Level),
		OutputPaths: cfg.Logging.OutputPaths,
	}
	logger := utils.NewLogger(loggerOptions)
	logger.Info("Starting Listenify server", "environment", cfg.Environment)

	// Initialize MongoDB client
	mongoClient, err := mongo.NewClient(cfg, logger)
	if err != nil {
		logger.Fatal("Failed to connect to MongoDB", err)
	}
	defer func() {
		if err := mongoClient.Disconnect(context.Background()); err != nil {
			logger.Error("Failed to disconnect from MongoDB", err)
		}
	}()

	// Initialize Redis client
	redisClient, err := redis.NewClient(cfg, logger)
	if err != nil {
		logger.Fatal("Failed to connect to Redis", err)
	}
	defer redisClient.Close()

	// Initialize MongoDB repositories
	userRepo := repositories.NewUserRepository(mongoClient.Database(), logger)
	roomRepo := repositories.NewRoomRepository(mongoClient.Database(), logger)
	mediaRepo := repositories.NewMediaRepository(mongoClient.Database(), logger)
	playlistRepo := repositories.NewPlaylistRepository(mongoClient.Database(), logger)
	historyRepo := repositories.NewHistoryRepository(mongoClient.Database(), logger)

	// Initialize Redis managers
	sessionMgr := managers.NewSessionManager(redisClient, cfg.Auth.AccessTokenExpiry)
	presenceMgr := managers.NewPresenceManager(redisClient)
	roomStateMgr := managers.NewRoomStateManager(redisClient)

	// Initialize authentication provider
	jwtConfig := auth.JWTConfig{
		Secret:               cfg.Auth.JWTSecret,
		Issuer:               "listenify",
		Audience:             "listenify-users",
		AccessTokenDuration:  cfg.Auth.AccessTokenExpiry,
		RefreshTokenDuration: cfg.Auth.RefreshTokenExpiry,
	}
	jwtProvider := auth.NewJWTProvider(jwtConfig, logger)
	passwordProvider := auth.NewPasswordProvider(logger)
	authProvider := &CombinedAuthProvider{
		JWTProvider:      jwtProvider,
		PasswordProvider: passwordProvider,
	}

	// Initialize services
	userManager := user.NewManager(userRepo, *sessionMgr, *presenceMgr, authProvider, logger)

	// Initialize media services
	providers := make(map[string]media.Provider)
	youtubeProvider := media.NewYouTubeProvider(cfg.Media.YouTubeAPIKey, logger)
	providers["youtube"] = youtubeProvider

	if cfg.Features.EnableSoundCloud {
		soundcloudProvider := media.NewSoundCloudProvider(cfg.Media.SoundCloudAPIKey, logger)
		providers["soundcloud"] = soundcloudProvider
	}

	// Initialize media search service and use it to register providers with resolver
	_ = media.NewSearchService(providers, logger)
	mediaResolver := media.NewResolver(mediaRepo, logger)

	// Register providers with mediaResolver
	for _, provider := range providers {
		mediaResolver.RegisterProvider(provider)
	}

	// Initialize playlist services
	playlistManager := playlist.NewManager(playlistRepo, logger)

	// Initialize room services
	roomManager := room.NewManager(roomRepo, userRepo, *roomStateMgr, *presenceMgr, logger)

	// Initialize queue manager
	queueManager := room.NewQueueManager(roomManager, logger)

	// Initialize PubSub manager
	pubSubManager := managers.NewPubSubManager(redisClient)

	// Initialize chat repository and service
	chatRepo := repositories.NewChatRepository(mongoClient.Database(), logger)
	chatService := room.NewChatService(roomManager, chatRepo, userRepo, pubSubManager, logger)

	// Initialize user stats service
	statsService := user.NewStatsService(userManager, logger)

	// Initialize system services
	healthConfig := system.HealthServiceConfig{
		Version:     "1.0.0",
		Environment: cfg.Environment,
	}
	healthService := system.NewHealthService(mongoClient.Client(), redisClient, logger, healthConfig)

	// Initialize maintenance service
	maintenanceConfig := system.DefaultMaintenanceConfig()
	maintenanceService := system.NewMaintenanceService(
		maintenanceConfig,
		mongoClient.Database(),
		redisClient,
		roomRepo,
		historyRepo,
		mediaRepo,
		playlistRepo,
		userRepo,
		logger,
	)

	// Initialize API router
	router := api.NewRouter(
		authProvider,
		*sessionMgr,
		userManager,
		playlistManager,
		mediaResolver,
		healthService,
		cfg,
		logger,
	)

	// Initialize RPC router for WebSocket
	rpcRouter := rpc.NewRouter(logger)

	// Initialize RPC server
	rpcServer := rpc.NewServer(
		rpcRouter,
		authProvider,
		*sessionMgr,
		*presenceMgr,
		logger,
	)

	// Register RPC methods
	methods.RegisterAllMethods(
		rpcRouter,
		authProvider,
		*sessionMgr,
		userManager,
		statsService,
		playlistManager,
		mediaResolver,
		roomManager,
		chatService,
		queueManager,
		logger,
	)

	// Start maintenance service
	if err := maintenanceService.Start(ctx); err != nil {
		logger.Error("Failed to start maintenance service", err)
	}

	// Start health service
	healthService.Start(ctx)

	// Create HTTP server for API
	apiAddr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	server := &http.Server{
		Addr:         apiAddr,
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	// Create a separate HTTP server for WebSocket connections on a different port
	// This avoids middleware that might interfere with WebSocket upgrades
	wsPort := cfg.Server.Port + 1
	wsAddr := fmt.Sprintf("%s:%d", cfg.Server.Host, wsPort)
	wsServer := &http.Server{
		Addr:    wsAddr,
		Handler: http.HandlerFunc(rpcServer.HandleWebSocket),
	}

	// Start HTTP server for API
	go func() {
		logger.Info("Starting HTTP server", "address", apiAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("HTTP server error", err)
		}
	}()

	// Start HTTP server for WebSocket
	go func() {
		logger.Info("Starting WebSocket server", "address", wsAddr)
		if err := wsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("WebSocket server error", err)
		}
	}()

	// Wait for shutdown signal
	<-ctx.Done()
	logger.Info("Shutting down server")

	// Create a context with timeout for shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Shutdown HTTP server
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("HTTP server shutdown error", err)
	}

	// Shutdown WebSocket server
	if err := wsServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("WebSocket server shutdown error", err)
	}

	// Shutdown RPC server
	if err := rpcServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("RPC server shutdown error", err)
	}

	// Stop maintenance service
	maintenanceService.Stop()

	logger.Info("Server shutdown complete")
}
