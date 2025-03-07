// Package api provides the HTTP API for the application.
package api

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"norelock.dev/listenify/backend/internal/api/handlers"
	appMiddleware "norelock.dev/listenify/backend/internal/api/middleware"
	"norelock.dev/listenify/backend/internal/auth"
	"norelock.dev/listenify/backend/internal/config"
	"norelock.dev/listenify/backend/internal/db/redis/managers"
	"norelock.dev/listenify/backend/internal/services/media"
	"norelock.dev/listenify/backend/internal/services/playlist"
	"norelock.dev/listenify/backend/internal/services/room"
	"norelock.dev/listenify/backend/internal/services/system"
	"norelock.dev/listenify/backend/internal/services/user"
	"norelock.dev/listenify/backend/internal/utils"
)

// Router is the main HTTP router for the API.
type Router struct {
	*chi.Mux
	logger *utils.Logger
}

// NewRouter creates a new API router.
func NewRouter(
	authProvider auth.Provider,
	sessionMgr managers.SessionManager,
	userManager *user.Manager,
	playlistManager *playlist.Manager,
	roomManager *room.Manager,
	mediaResolver *media.Resolver,
	healthService *system.HealthService,
	cfg *config.Config,
	logger *utils.Logger,
) *Router {
	r := chi.NewRouter()
	apiLogger := logger.Named("api")

	// Create middleware
	recoveryMiddleware := appMiddleware.NewRecoveryMiddleware(apiLogger)
	loggerMiddleware := appMiddleware.NewLoggerMiddleware(apiLogger)
	corsMiddleware := appMiddleware.NewCORSMiddleware(appMiddleware.DefaultCORSConfig(), apiLogger)
	authMiddleware := appMiddleware.NewAuthMiddleware(authProvider, sessionMgr, apiLogger)

	// Create handlers
	authHandler := handlers.NewAuthHandler(userManager, authProvider, apiLogger)
	userHandler := handlers.NewUserHandler(userManager, apiLogger)
	mediaHandler := handlers.NewMediaHandler(mediaResolver, apiLogger)
	playlistHandler := handlers.NewPlaylistHandler(playlistManager, apiLogger)
	roomHandler := handlers.NewRoomHandler(roomManager, apiLogger)
	healthHandler := handlers.NewHealthHandler(apiLogger, healthService, cfg)

	// Apply global middleware
	r.Use(recoveryMiddleware.Recovery)
	r.Use(loggerMiddleware.Logger)
	r.Use(corsMiddleware.CORS)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Heartbeat("/ping"))

	// Public routes
	r.Group(func(r chi.Router) {
		// Health check
		r.Get("/health", healthHandler.Check)

		// Auth routes
		r.Route("/auth", func(r chi.Router) {
			r.Post("/register", authHandler.Register)
			r.Post("/login", authHandler.Login)
			r.Post("/refresh", authHandler.Refresh)
			r.Post("/logout", authHandler.Logout)
		})
	})

	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(authMiddleware.RequireAuth)

		// User routes
		r.Route("/users", func(r chi.Router) {
			r.Get("/me", authHandler.Me)
			r.Get("/{id}", userHandler.GetUser)
			r.Put("/me", userHandler.UpdateUser)
			r.Delete("/me", userHandler.DeleteUser)
			r.Get("/search", userHandler.SearchUsers)
			r.Get("/online", userHandler.GetOnlineUsers)

			// User social routes
			r.Route("/me/social", func(r chi.Router) {
				r.Get("/following", userHandler.GetFollowing)
				r.Get("/followers", userHandler.GetFollowers)
				r.Post("/follow/{id}", userHandler.FollowUser)
				r.Delete("/unfollow/{id}", userHandler.UnfollowUser)
			})
		})

		// Media routes
		r.Route("/media", func(r chi.Router) {
			r.Get("/search", mediaHandler.Search)
			r.Get("/resolve", mediaHandler.Resolve)
			r.Get("/proxy/{provider}/{id}", mediaHandler.Proxy)
		})

		// Playlist routes
		r.Route("/playlists", func(r chi.Router) {
			r.Get("/", playlistHandler.GetPlaylists)
			r.Post("/", playlistHandler.CreatePlaylist)
			r.Get("/{id}", playlistHandler.GetPlaylist)
			r.Put("/{id}", playlistHandler.UpdatePlaylist)
			r.Delete("/{id}", playlistHandler.DeletePlaylist)
			r.Post("/{id}/items", playlistHandler.AddPlaylistItem)
			r.Delete("/{id}/items/{itemId}", playlistHandler.RemovePlaylistItem)
			r.Post("/import", playlistHandler.ImportPlaylist)
		})

		// Room routes
		r.Route("/rooms", func(r chi.Router) {
			AddCRUDRoutes(r, roomHandler)
			r.Get("/popular", roomHandler.ListPopular)
			r.Get("/favorites", roomHandler.ListFavorites)
			r.Get("/search", roomHandler.Search)
			r.Get("/{id}/state", WithID(roomHandler.GetState))
			r.Post("/{id}/join", WithID(roomHandler.PostJoin))
			r.Post("/{id}/leave", WithID(roomHandler.PostLeave))
			r.Post("/{id}/skip", WithID(roomHandler.PostSkip))
			r.Post("/{id}/vote", WithID(roomHandler.PostVote))
			r.Post("/{id}/queue/join", WithID(roomHandler.PostQueueJoin))
			r.Post("/{id}/queue/leave", WithID(roomHandler.PostQueueLeave))
			r.Post("/{id}/favorite", WithID(roomHandler.PostFavorite))
			r.Delete("/{id}/favorite", WithID(roomHandler.DeleteFavorite))
		})
	})

	// Admin routes
	r.Group(func(r chi.Router) {
		r.Use(authMiddleware.RequireAuth)
		r.Use(authMiddleware.RequireRole("admin"))

		// Admin routes go here
		r.Route("/admin", func(r chi.Router) {
			// Admin user management
			r.Get("/users", userHandler.GetAllUsers)
			r.Put("/users/{id}/activate", userHandler.ActivateUser)
			r.Put("/users/{id}/deactivate", userHandler.DeactivateUser)
			r.Delete("/users/{id}", userHandler.AdminDeleteUser)
		})
	})

	return &Router{
		Mux:    r,
		logger: apiLogger,
	}
}
