// Package methods contains RPC method handlers for the application.
package methods

import (
	"context"

	"norelock.dev/listenify/backend/internal/auth"
	"norelock.dev/listenify/backend/internal/db/redis/managers"
	"norelock.dev/listenify/backend/internal/rpc"
	"norelock.dev/listenify/backend/internal/services/media"
	"norelock.dev/listenify/backend/internal/services/playlist"
	"norelock.dev/listenify/backend/internal/services/room"
	"norelock.dev/listenify/backend/internal/services/user"
	"norelock.dev/listenify/backend/internal/utils"
)

// RegisterAllMethods initializes all RPC method handlers and registers them with the router.
func RegisterAllMethods(
	router *rpc.Router,
	authProvider auth.Provider,
	sessionMgr managers.SessionManager,
	userManager *user.Manager,
	statsService *user.StatsService,
	playlistManager *playlist.Manager,
	mediaResolver *media.Resolver,
	roomManager *room.Manager,
	chatService room.ChatService,
	queueManager *room.QueueManager,
	logger *utils.Logger,
) {
	// Create handlers
	userHandler := NewUserHandler(*userManager, statsService, logger)
	chatHandler := NewChatHandler(chatService, logger)
	mediaHandler := NewMediaHandler(mediaResolver, logger)
	playlistHandler := NewPlaylistHandler(playlistManager, userManager, logger)
	queueHandler := NewQueueHandler(queueManager, logger)
	roomHandler := NewRoomHandler(roomManager, logger)

	hr := router.Wrap(rpc.RecoveryMiddleware(logger)).Wrap(rpc.LoggingMiddleware(logger))

	// Register methods
	rpc.RegisterNoParams(hr, "ping", handlePing)

	userHandler.RegisterMethods(hr)
	chatHandler.RegisterMethods(hr)
	mediaHandler.RegisterMethods(hr)
	playlistHandler.RegisterMethods(hr)
	queueHandler.RegisterMethods(hr)
	roomHandler.RegisterMethods(hr)
	logger.Info("Registered all RPC methods")
}

func handlePing(ctx context.Context, client *rpc.Client) (any, error) {
	return "pong", nil
}
