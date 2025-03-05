// Package rpc provides WebSocket-based RPC functionality.
package rpc

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"norelock.dev/listenify/backend/internal/auth"
	"norelock.dev/listenify/backend/internal/db/redis/managers"
	"norelock.dev/listenify/backend/internal/utils"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512 * 1024 // 512KB
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins in development
	},
}

// Server handles WebSocket connections and RPC requests.
type Server struct {
	hub          *Hub
	router       *Router
	authProvider auth.Provider
	sessionMgr   managers.SessionManager
	presenceMgr  managers.PresenceManager
	logger       *utils.Logger
	clients      map[*Client]bool
	register     chan *Client
	unregister   chan *Client
	mutex        sync.Mutex
}

// NewServer creates a new WebSocket server.
func NewServer(
	router *Router,
	authProvider auth.Provider,
	sessionMgr managers.SessionManager,
	presenceMgr managers.PresenceManager,
	logger *utils.Logger,
) *Server {
	hub := NewHub(logger)
	go hub.Run()

	server := &Server{
		hub:          hub,
		router:       router,
		authProvider: authProvider,
		sessionMgr:   sessionMgr,
		presenceMgr:  presenceMgr,
		logger:       logger.Named("rpc_server"),
		clients:      make(map[*Client]bool),
		register:     make(chan *Client),
		unregister:   make(chan *Client),
	}

	go server.run()

	logger.Debug("RPC server started", "router", router)

	return server
}

// run processes client registration and unregistration.
func (s *Server) run() {
	for {
		select {
		case client := <-s.register:
			s.mutex.Lock()
			s.clients[client] = true
			s.mutex.Unlock()
			s.logger.Debug("Client registered", "id", client.ID, "userID", client.UserID)

		case client := <-s.unregister:
			s.mutex.Lock()
			if _, ok := s.clients[client]; ok {
				delete(s.clients, client)
				close(client.send)
				s.logger.Debug("Client unregistered", "id", client.ID, "userID", client.UserID)
			}
			s.mutex.Unlock()
		}
	}
}

// HandleWebSocket upgrades an HTTP connection to WebSocket and handles the connection.
func (s *Server) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Upgrade connection to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error("Failed to upgrade connection", err)
		return
	}

	// Get token from query parameters
	token := r.URL.Query().Get("token")
	if token == "" {
		s.logger.Warn("No token provided")

		err := conn.WriteMessage(websocket.TextMessage, []byte(`{"error": "No token provided"}`))
		if err != nil {
			s.logger.Error("Failed to send error message", err)
		}

		conn.Close()
		return
	}

	// Validate token
	claims, err := s.authProvider.ValidateToken(token)
	if err != nil {
		s.logger.Warn("Invalid token", "error", err)

		err := conn.WriteMessage(websocket.TextMessage, []byte(`{"error": "Invalid token"}`))
		if err != nil {
			s.logger.Error("Failed to send error message", err)
		}

		conn.Close()
		return
	}

	// Verify session
	session, err := s.sessionMgr.GetSession(r.Context(), token)
	if err != nil || session == nil {
		s.logger.Warn("Invalid session", "error", err)

		err := conn.WriteMessage(websocket.TextMessage, []byte(`{"error": "Invalid session"}`))
		if err != nil {
			s.logger.Error("Failed to send error message", err)
		}

		conn.Close()
		return
	}

	// Create client
	clientID, err := utils.GenerateID("client")
	if err != nil {
		s.logger.Error("Failed to generate client ID", err)

		err := conn.WriteMessage(websocket.TextMessage, []byte(`{"error": "Failed to generate client ID"}`))
		if err != nil {
			s.logger.Error("Failed to send error message", err)
		}

		conn.Close()
		return
	}

	client := &Client{
		ID:       clientID,
		UserID:   claims.UserID,
		Username: claims.Username,
		server:   s,
		conn:     conn,
		send:     make(chan []byte, 256),
		rooms:    make(map[string]bool),
		logger:   s.logger.Named("client"),
	}

	// Register client
	s.register <- client

	// Update presence
	// Note: Assuming the PresenceManager has a method to mark a user as online
	// If this method doesn't exist, it needs to be implemented in the PresenceManager
	// For now, we'll log a message and continue
	s.logger.Info("User connected", "userID", claims.UserID)

	// Start client goroutines
	go client.readPump()
	go client.writePump()

	s.logger.Info("WebSocket connection established", "clientID", client.ID, "userID", client.UserID)
}

// Broadcast sends a message to all connected clients.
func (s *Server) Broadcast(message []byte) {
	s.hub.Broadcast(message)
}

// BroadcastToRoom sends a message to all clients in a room.
func (s *Server) BroadcastToRoom(roomID string, message []byte) {
	s.hub.BroadcastToRoom(roomID, message)
}

// BroadcastToUser sends a message to a specific user.
func (s *Server) BroadcastToUser(userID string, message []byte) {
	s.hub.BroadcastToUser(userID, message)
}

// AddClientToRoom adds a client to a room.
func (s *Server) AddClientToRoom(client *Client, roomID string) {
	s.hub.AddClientToRoom(client, roomID)
}

// RemoveClientFromRoom removes a client from a room.
func (s *Server) RemoveClientFromRoom(client *Client, roomID string) {
	s.hub.RemoveClientFromRoom(client, roomID)
}

// GetClientsInRoom gets all clients in a room.
func (s *Server) GetClientsInRoom(roomID string) []*Client {
	return s.hub.GetClientsInRoom(roomID)
}

// GetClientCount gets the number of connected clients.
func (s *Server) GetClientCount() int {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return len(s.clients)
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("Shutting down RPC server")

	// Close all client connections
	s.mutex.Lock()
	for client := range s.clients {
		client.conn.Close()
		delete(s.clients, client)
	}
	s.mutex.Unlock()

	return nil
}
