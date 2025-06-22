package controllers

import (
	"battle-wordle/server/models"
	"battle-wordle/server/services"
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

const (
	JOIN_MSG  = "join"
	GUESS_MSG = "guess"
)

type WSMessage struct {
	Type  string `json:"type"`
	Guess string `json:"guess,omitempty"`
}

type WSController struct {
	gameService *services.GameService
	upgrader    websocket.Upgrader

	connections map[string]map[*websocket.Conn]bool
	mu          sync.RWMutex
}

func NewWSController(gameService *services.GameService) *WSController {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     func(r *http.Request) bool { return true },
	}

	return &WSController{
		gameService: gameService,
		upgrader:    upgrader,
		connections: make(map[string]map[*websocket.Conn]bool),
	}
}

func (ws *WSController) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	gameID := vars["id"]
	ctx := r.Context()

	conn, err := ws.upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, "Could not upgrade to WebSocket", http.StatusBadRequest)
		return
	}

	// Register connection
	ws.mu.Lock()
	if _, ok := ws.connections[gameID]; !ok {
		ws.connections[gameID] = make(map[*websocket.Conn]bool)
	}
	ws.connections[gameID][conn] = true
	ws.mu.Unlock()

	defer func() {
		conn.Close()
		ws.mu.Lock()
		delete(ws.connections[gameID], conn)
		if len(ws.connections[gameID]) == 0 {
			delete(ws.connections, gameID)
		}
		ws.mu.Unlock()
	}()

	for {
		_, msgData, err := conn.ReadMessage()
		if err != nil {
			log.Printf("WebSocket read error: %v", err)
			break
		}

		var msg WSMessage
		if err := json.Unmarshal(msgData, &msg); err != nil {
			log.Printf("Invalid JSON: %v", err)
			continue
		}

		switch msg.Type {
		case JOIN_MSG:
			game, err := ws.gameService.GetByID(ctx, gameID)
			if err != nil {
				log.Printf("Failed to get game for JOIN: %v", err)
				continue
			}
			ws.sendGameState(conn, game)

		case GUESS_MSG:
			game, err := ws.gameService.SubmitGuess(ctx, gameID, msg.Guess)
			if err != nil {
				log.Printf("Failed to submit guess: %v", err)
				continue
			}
			ws.broadcastGameState(gameID, game)

		default:
			log.Printf("Unknown message type: %s", msg.Type)
		}
	}
}

func (ws *WSController) sendGameState(conn *websocket.Conn, game *models.Game) {
	data, err := json.Marshal(game)
	if err != nil {
		log.Printf("Failed to marshal game state: %v", err)
		return
	}
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		log.Printf("Failed to send game state to client: %v", err)
	}
}

func (ws *WSController) broadcastGameState(gameID string, game *models.Game) {
	data, err := json.Marshal(game)
	if err != nil {
		log.Printf("Failed to marshal game state: %v", err)
		return
	}

	ws.mu.RLock()
	conns := ws.connections[gameID]
	ws.mu.RUnlock()

	for conn := range conns {
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			log.Printf("Broadcast failed, cleaning up connection: %v", err)
			conn.Close()
			ws.mu.Lock()
			delete(ws.connections[gameID], conn)
			if len(ws.connections[gameID]) == 0 {
				delete(ws.connections, gameID)
			}
			ws.mu.Unlock()
		}
	}
}
