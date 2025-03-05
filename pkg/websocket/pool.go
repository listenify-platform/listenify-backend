// Package websocket provides WebSocket connection handling.
package websocket

import (
	"errors"
	"sync"
	"time"
)

// Pool errors
var (
	ErrConnectionNotFound = errors.New("connection not found")
	ErrPoolClosed         = errors.New("pool closed")
)

// Pool manages a set of WebSocket connections.
type Pool struct {
	// connections is a map of connection IDs to connections.
	connections map[string]*Connection

	// groups is a map of group names to a map of connection IDs.
	groups map[string]map[string]bool

	// mutex is used to synchronize access to the connections and groups maps.
	mutex sync.RWMutex

	// closed indicates whether the pool is closed.
	closed bool

	// closeMutex is used to synchronize access to the closed flag.
	closeMutex sync.RWMutex
}

// NewPool creates a new connection pool.
func NewPool() *Pool {
	return &Pool{
		connections: make(map[string]*Connection),
		groups:      make(map[string]map[string]bool),
	}
}

// Add adds a connection to the pool.
func (p *Pool) Add(id string, conn *Connection) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.IsClosed() {
		return ErrPoolClosed
	}

	p.connections[id] = conn
	return nil
}

// Remove removes a connection from the pool.
func (p *Pool) Remove(id string) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.IsClosed() {
		return ErrPoolClosed
	}

	if _, ok := p.connections[id]; !ok {
		return ErrConnectionNotFound
	}

	// Remove connection from all groups
	for group, conns := range p.groups {
		if _, ok := conns[id]; ok {
			delete(conns, id)
			if len(conns) == 0 {
				delete(p.groups, group)
			}
		}
	}

	delete(p.connections, id)
	return nil
}

// Get gets a connection from the pool.
func (p *Pool) Get(id string) (*Connection, error) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	if p.IsClosed() {
		return nil, ErrPoolClosed
	}

	conn, ok := p.connections[id]
	if !ok {
		return nil, ErrConnectionNotFound
	}

	return conn, nil
}

// AddToGroup adds a connection to a group.
func (p *Pool) AddToGroup(group, id string) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.IsClosed() {
		return ErrPoolClosed
	}

	if _, ok := p.connections[id]; !ok {
		return ErrConnectionNotFound
	}

	if _, ok := p.groups[group]; !ok {
		p.groups[group] = make(map[string]bool)
	}

	p.groups[group][id] = true
	return nil
}

// RemoveFromGroup removes a connection from a group.
func (p *Pool) RemoveFromGroup(group, id string) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.IsClosed() {
		return ErrPoolClosed
	}

	if _, ok := p.groups[group]; !ok {
		return nil
	}

	delete(p.groups[group], id)
	if len(p.groups[group]) == 0 {
		delete(p.groups, group)
	}

	return nil
}

// GetGroup gets all connections in a group.
func (p *Pool) GetGroup(group string) ([]*Connection, error) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	if p.IsClosed() {
		return nil, ErrPoolClosed
	}

	conns, ok := p.groups[group]
	if !ok {
		return []*Connection{}, nil
	}

	result := make([]*Connection, 0, len(conns))
	for id := range conns {
		if conn, ok := p.connections[id]; ok {
			result = append(result, conn)
		}
	}

	return result, nil
}

// Broadcast sends a message to all connections in the pool.
func (p *Pool) Broadcast(message *Message) error {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	if p.IsClosed() {
		return ErrPoolClosed
	}

	for _, conn := range p.connections {
		if !conn.IsClosed() {
			conn.WriteMessage(message.Type, message.Data)
		}
	}

	return nil
}

// BroadcastJSON sends a JSON message to all connections in the pool.
func (p *Pool) BroadcastJSON(v any) error {
	message, err := NewJSONMessage(v)
	if err != nil {
		return err
	}

	return p.Broadcast(message)
}

// BroadcastToGroup sends a message to all connections in a group.
func (p *Pool) BroadcastToGroup(group string, message *Message) error {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	if p.IsClosed() {
		return ErrPoolClosed
	}

	conns, ok := p.groups[group]
	if !ok {
		return nil
	}

	for id := range conns {
		if conn, ok := p.connections[id]; ok && !conn.IsClosed() {
			conn.WriteMessage(message.Type, message.Data)
		}
	}

	return nil
}

// BroadcastJSONToGroup sends a JSON message to all connections in a group.
func (p *Pool) BroadcastJSONToGroup(group string, v any) error {
	message, err := NewJSONMessage(v)
	if err != nil {
		return err
	}

	return p.BroadcastToGroup(group, message)
}

// Send sends a message to a specific connection.
func (p *Pool) Send(id string, message *Message) error {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	if p.IsClosed() {
		return ErrPoolClosed
	}

	conn, ok := p.connections[id]
	if !ok {
		return ErrConnectionNotFound
	}

	if conn.IsClosed() {
		return ErrConnectionClosed
	}

	return conn.WriteMessage(message.Type, message.Data)
}

// SendJSON sends a JSON message to a specific connection.
func (p *Pool) SendJSON(id string, v any) error {
	message, err := NewJSONMessage(v)
	if err != nil {
		return err
	}

	return p.Send(id, message)
}

// SendWithTimeout sends a message to a specific connection with a timeout.
func (p *Pool) SendWithTimeout(id string, message *Message, timeout time.Duration) error {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	if p.IsClosed() {
		return ErrPoolClosed
	}

	conn, ok := p.connections[id]
	if !ok {
		return ErrConnectionNotFound
	}

	if conn.IsClosed() {
		return ErrConnectionClosed
	}

	return conn.WriteWithTimeout(message.Type, message.Data, timeout)
}

// SendJSONWithTimeout sends a JSON message to a specific connection with a timeout.
func (p *Pool) SendJSONWithTimeout(id string, v any, timeout time.Duration) error {
	message, err := NewJSONMessage(v)
	if err != nil {
		return err
	}

	return p.SendWithTimeout(id, message, timeout)
}

// Close closes the pool and all connections.
func (p *Pool) Close() error {
	p.closeMutex.Lock()
	defer p.closeMutex.Unlock()

	if p.closed {
		return nil
	}

	p.closed = true

	p.mutex.Lock()
	defer p.mutex.Unlock()

	for _, conn := range p.connections {
		conn.Close()
	}

	p.connections = make(map[string]*Connection)
	p.groups = make(map[string]map[string]bool)

	return nil
}

// IsClosed returns whether the pool is closed.
func (p *Pool) IsClosed() bool {
	p.closeMutex.RLock()
	defer p.closeMutex.RUnlock()
	return p.closed
}

// Count returns the number of connections in the pool.
func (p *Pool) Count() int {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return len(p.connections)
}

// GroupCount returns the number of connections in a group.
func (p *Pool) GroupCount(group string) int {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	conns, ok := p.groups[group]
	if !ok {
		return 0
	}

	return len(conns)
}

// Groups returns the names of all groups.
func (p *Pool) Groups() []string {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	groups := make([]string, 0, len(p.groups))
	for group := range p.groups {
		groups = append(groups, group)
	}

	return groups
}

// ConnectionIDs returns the IDs of all connections.
func (p *Pool) ConnectionIDs() []string {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	ids := make([]string, 0, len(p.connections))
	for id := range p.connections {
		ids = append(ids, id)
	}

	return ids
}

// GroupConnectionIDs returns the IDs of all connections in a group.
func (p *Pool) GroupConnectionIDs(group string) []string {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	conns, ok := p.groups[group]
	if !ok {
		return []string{}
	}

	ids := make([]string, 0, len(conns))
	for id := range conns {
		ids = append(ids, id)
	}

	return ids
}
