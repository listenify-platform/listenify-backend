// Package rpc provides WebSocket-based RPC functionality.
package rpc

import (
	"sync"

	"norelock.dev/listenify/backend/internal/utils"
)

// Hub maintains the set of active clients and broadcasts messages to them.
type Hub struct {
	// clients is a map of all connected clients.
	clients map[*Client]bool

	// rooms is a map of room IDs to a map of clients in that room.
	rooms map[string]map[*Client]bool

	// userClients is a map of user IDs to a map of their clients.
	userClients map[string]map[*Client]bool

	// broadcast is a channel of messages to broadcast to all clients.
	broadcast chan []byte

	// roomBroadcast is a channel of messages to broadcast to a specific room.
	roomBroadcast chan *roomMessage

	// userBroadcast is a channel of messages to broadcast to a specific user.
	userBroadcast chan *userMessage

	// register is a channel for registering clients.
	register chan *Client

	// unregister is a channel for unregistering clients.
	unregister chan *Client

	// join is a channel for adding clients to rooms.
	join chan *roomOperation

	// leave is a channel for removing clients from rooms.
	leave chan *roomOperation

	// mutex is used to synchronize access to the maps.
	mutex sync.RWMutex

	// logger is the hub's logger.
	logger *utils.Logger
}

// roomMessage represents a message to be broadcast to a room.
type roomMessage struct {
	room    string
	message []byte
}

// userMessage represents a message to be broadcast to a user.
type userMessage struct {
	userID  string
	message []byte
}

// roomOperation represents an operation to add or remove a client from a room.
type roomOperation struct {
	client *Client
	room   string
}

// NewHub creates a new hub.
func NewHub(logger *utils.Logger) *Hub {
	return &Hub{
		clients:       make(map[*Client]bool),
		rooms:         make(map[string]map[*Client]bool),
		userClients:   make(map[string]map[*Client]bool),
		broadcast:     make(chan []byte),
		roomBroadcast: make(chan *roomMessage),
		userBroadcast: make(chan *userMessage),
		register:      make(chan *Client),
		unregister:    make(chan *Client),
		join:          make(chan *roomOperation),
		leave:         make(chan *roomOperation),
		logger:        logger.Named("hub"),
	}
}

// Run starts the hub.
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.registerClient(client)

		case client := <-h.unregister:
			h.unregisterClient(client)

		case message := <-h.broadcast:
			h.broadcastMessage(message)

		case rm := <-h.roomBroadcast:
			h.broadcastToRoom(rm.room, rm.message)

		case um := <-h.userBroadcast:
			h.broadcastToUser(um.userID, um.message)

		case op := <-h.join:
			h.addClientToRoom(op.client, op.room)

		case op := <-h.leave:
			h.removeClientFromRoom(op.client, op.room)
		}
	}
}

// registerClient registers a client with the hub.
func (h *Hub) registerClient(client *Client) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	// Add client to clients map
	h.clients[client] = true

	// Add client to user's clients map
	if client.UserID != "" {
		if _, ok := h.userClients[client.UserID]; !ok {
			h.userClients[client.UserID] = make(map[*Client]bool)
		}
		h.userClients[client.UserID][client] = true
	}

	h.logger.Debug("Client registered", "id", client.ID, "userID", client.UserID)
}

// unregisterClient unregisters a client from the hub.
func (h *Hub) unregisterClient(client *Client) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	// Remove client from clients map
	if _, ok := h.clients[client]; ok {
		delete(h.clients, client)

		// Remove client from user's clients map
		if client.UserID != "" {
			if clients, ok := h.userClients[client.UserID]; ok {
				delete(clients, client)
				if len(clients) == 0 {
					delete(h.userClients, client.UserID)
				}
			}
		}

		// Remove client from all rooms
		for room := range client.rooms {
			if clients, ok := h.rooms[room]; ok {
				delete(clients, client)
				if len(clients) == 0 {
					delete(h.rooms, room)
				}
			}
		}

		h.logger.Debug("Client unregistered", "id", client.ID, "userID", client.UserID)
	}
}

// broadcastMessage broadcasts a message to all clients.
func (h *Hub) broadcastMessage(message []byte) {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	for client := range h.clients {
		// Use safelySendMessage instead of direct channel send
		if !client.safelySendMessage(message) {
			h.unregisterClient(client)
		}
	}
}

// broadcastToRoom broadcasts a message to all clients in a room.
func (h *Hub) broadcastToRoom(room string, message []byte) {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	if clients, ok := h.rooms[room]; ok {
		for client := range clients {
			// Use safelySendMessage instead of direct channel send
			if !client.safelySendMessage(message) {
				h.unregisterClient(client)
			}
		}
	}
}

// broadcastToUser broadcasts a message to all clients of a user.
func (h *Hub) broadcastToUser(userID string, message []byte) {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	if clients, ok := h.userClients[userID]; ok {
		for client := range clients {
			// Use safelySendMessage instead of direct channel send
			if !client.safelySendMessage(message) {
				h.unregisterClient(client)
			}
		}
	}
}

// addClientToRoom adds a client to a room.
func (h *Hub) addClientToRoom(client *Client, room string) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	// Create room if it doesn't exist
	if _, ok := h.rooms[room]; !ok {
		h.rooms[room] = make(map[*Client]bool)
	}

	// Add client to room
	h.rooms[room][client] = true

	// Update client's rooms
	client.rooms[room] = true

	h.logger.Debug("Client added to room", "id", client.ID, "userID", client.UserID, "room", room)
}

// removeClientFromRoom removes a client from a room.
func (h *Hub) removeClientFromRoom(client *Client, room string) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	// Remove client from room
	if clients, ok := h.rooms[room]; ok {
		delete(clients, client)
		if len(clients) == 0 {
			delete(h.rooms, room)
		}
	}

	// Update client's rooms
	delete(client.rooms, room)

	h.logger.Debug("Client removed from room", "id", client.ID, "userID", client.UserID, "room", room)
}

// Broadcast sends a message to all connected clients.
func (h *Hub) Broadcast(message []byte) {
	h.broadcast <- message
}

// BroadcastToRoom sends a message to all clients in a room.
func (h *Hub) BroadcastToRoom(room string, message []byte) {
	h.roomBroadcast <- &roomMessage{room: room, message: message}
}

// BroadcastToUser sends a message to all clients of a user.
func (h *Hub) BroadcastToUser(userID string, message []byte) {
	h.userBroadcast <- &userMessage{userID: userID, message: message}
}

// AddClientToRoom adds a client to a room.
func (h *Hub) AddClientToRoom(client *Client, room string) {
	h.join <- &roomOperation{client: client, room: room}
}

// RemoveClientFromRoom removes a client from a room.
func (h *Hub) RemoveClientFromRoom(client *Client, room string) {
	h.leave <- &roomOperation{client: client, room: room}
}

// GetClientsInRoom gets all clients in a room.
func (h *Hub) GetClientsInRoom(room string) []*Client {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	clients := make([]*Client, 0)
	if roomClients, ok := h.rooms[room]; ok {
		for client := range roomClients {
			clients = append(clients, client)
		}
	}
	return clients
}

// GetClientCount gets the number of connected clients.
func (h *Hub) GetClientCount() int {
	h.mutex.RLock()
	defer h.mutex.RUnlock()
	return len(h.clients)
}

// GetRoomCount gets the number of active rooms.
func (h *Hub) GetRoomCount() int {
	h.mutex.RLock()
	defer h.mutex.RUnlock()
	return len(h.rooms)
}

// GetUserCount gets the number of connected users.
func (h *Hub) GetUserCount() int {
	h.mutex.RLock()
	defer h.mutex.RUnlock()
	return len(h.userClients)
}
