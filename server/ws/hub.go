package ws

import (
	"sync"

	"github.com/gorilla/websocket"
)

// Hub manages WebSocket connections by key (e.g., gameID, playerID).
type Hub struct {
	mu    sync.RWMutex
	conns map[string]map[*websocket.Conn]bool
}

// NewHub creates a new Hub.
func NewHub() *Hub {
	return &Hub{
		conns: make(map[string]map[*websocket.Conn]bool),
	}
}

// AddConnection adds a connection for a given key.
func (h *Hub) AddConnection(key string, conn *websocket.Conn) {
	h.mu.Lock()
	if _, ok := h.conns[key]; !ok {
		h.conns[key] = make(map[*websocket.Conn]bool)
	}
	h.conns[key][conn] = true
	h.mu.Unlock()
}

// RemoveConnection removes a connection for a given key.
func (h *Hub) RemoveConnection(key string, conn *websocket.Conn) {
	h.mu.Lock()
	if _, ok := h.conns[key]; ok {
		delete(h.conns[key], conn)
		if len(h.conns[key]) == 0 {
			delete(h.conns, key)
		}
	}
	h.mu.Unlock()
}

// Broadcast sends a message to all connections for a given key.
func (h *Hub) Broadcast(key string, message []byte) {
	h.mu.RLock()
	conns := h.conns[key]
	h.mu.RUnlock()
	for conn := range conns {
		conn.WriteMessage(websocket.TextMessage, message)
	}
}

// Connections returns all connections for a given key.
func (h *Hub) Connections(key string) []*websocket.Conn {
	h.mu.RLock()
	defer h.mu.RUnlock()
	var result []*websocket.Conn
	for conn := range h.conns[key] {
		result = append(result, conn)
	}
	return result
}
