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
	Type     string `json:"type"`
	Guess    string `json:"guess,omitempty"`
	PlayerID string `json:"player_id,omitempty"`
}

type WSController struct {
	gameService *services.GameService
	upgrader    websocket.Upgrader

	connections map[string]map[*websocket.Conn]bool
	mu          sync.RWMutex
}

// Add matchmaking queue and handler
var matchmakingQueue = struct {
	clients []struct {
		conn     *websocket.Conn
		playerID string
	}
	sync.Mutex
}{clients: []struct {
	conn     *websocket.Conn
	playerID string
}{}}

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

	// Register connection (no log)
	ws.mu.Lock()
	if _, ok := ws.connections[gameID]; !ok {
		ws.connections[gameID] = make(map[*websocket.Conn]bool)
	}
	ws.connections[gameID][conn] = true
	ws.mu.Unlock()

	defer func() {
		// No log for normal close
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
			// Only log unexpected errors (not normal close 1000 or 1006)
			if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				log.Printf("WebSocket unexpected close for game %s: %v", gameID, err)
			}
			break
		}

		var msg WSMessage
		if err := json.Unmarshal(msgData, &msg); err != nil {
			log.Printf("Invalid JSON: %v", err)
			continue
		}

		// No log for every message received

		switch msg.Type {
		case JOIN_MSG:
			game, err := ws.gameService.GetByID(ctx, gameID)
			if err != nil {
				log.Printf("Failed to get game for JOIN: %v", err)
				continue
			}
			ws.sendGameState(conn, game)

		case GUESS_MSG:
			game, err := ws.gameService.GetByID(ctx, gameID)
			if err != nil || game == nil {
				log.Printf("Failed to get game for GUESS: %v", err)
				continue
			}
			if msg.PlayerID != game.CurrentPlayer {
				// Not this player's turn, ignore
				continue
			}
			updatedGame, err := ws.gameService.SubmitGuess(ctx, gameID, msg.Guess, msg.PlayerID)
			if err != nil {
				log.Printf("Failed to submit guess: %v", err)
				continue
			}
			ws.broadcastGameState(gameID, updatedGame)

		default:
			log.Printf("Unknown message type: %s", msg.Type)
		}
	}
}

// sendGameState sends the game state to a single connection, omitting the solution unless the game is over, and including feedback.
func (ws *WSController) sendGameState(conn *websocket.Conn, game *models.Game) {
	resp := make(map[string]interface{})
	resp["id"] = game.ID
	resp["created_at"] = game.CreatedAt
	resp["updated_at"] = game.UpdatedAt
	resp["first_player"] = game.FirstPlayer
	resp["second_player"] = game.SecondPlayer
	resp["current_player"] = game.CurrentPlayer
	resp["result"] = game.Result
	resp["guesses"] = game.Guesses
	feedbacks := ws.gameService.GetFeedbacks(game)
	resp["feedback"] = feedbacks
	// Only send solution if game is over
	if game.Result != "" {
		resp["solution"] = game.Solution
	}
	data, err := json.Marshal(resp)
	if err != nil {
		log.Printf("Failed to marshal game state: %v", err)
		return
	}
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		log.Printf("Failed to send game state to client: %v", err)
	}
}

// broadcastGameState sends the game state to all connections for a game, omitting the solution unless the game is over, and including feedback.
func (ws *WSController) broadcastGameState(gameID string, game *models.Game) {
	resp := make(map[string]interface{})
	resp["id"] = game.ID
	resp["created_at"] = game.CreatedAt
	resp["updated_at"] = game.UpdatedAt
	resp["first_player"] = game.FirstPlayer
	resp["second_player"] = game.SecondPlayer
	resp["current_player"] = game.CurrentPlayer
	resp["result"] = game.Result
	resp["guesses"] = game.Guesses
	feedbacks := ws.gameService.GetFeedbacks(game)
	resp["feedback"] = feedbacks
	if game.Result != "" {
		resp["solution"] = game.Solution
	}
	data, err := json.Marshal(resp)
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

func (ws *WSController) HandleMatchmakingWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := ws.upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, "Could not upgrade to WebSocket", http.StatusBadRequest)
		return
	}
	defer conn.Close()

	// Read player ID from first message
	_, msg, err := conn.ReadMessage()
	if err != nil {
		return
	}
	var joinMsg struct {
		Type     string `json:"type"`
		PlayerID string `json:"player_id"`
	}
	if err := json.Unmarshal(msg, &joinMsg); err != nil || joinMsg.Type != "join" || joinMsg.PlayerID == "" {
		return
	}

	// Add to matchmaking queue
	matchmakingQueue.Lock()
	matchmakingQueue.clients = append(matchmakingQueue.clients, struct {
		conn     *websocket.Conn
		playerID string
	}{conn, joinMsg.PlayerID})
	// If two players, create a game and notify both
	if len(matchmakingQueue.clients) >= 2 {
		c1 := matchmakingQueue.clients[0]
		c2 := matchmakingQueue.clients[1]
		matchmakingQueue.clients = matchmakingQueue.clients[2:]
		matchmakingQueue.Unlock()
		// Create game
		game, err := ws.gameService.CreateGame(r.Context(), c1.playerID, c2.playerID)
		if err != nil {
			return
		}
		resp := struct {
			Type   string `json:"type"`
			GameID string `json:"game_id"`
		}{Type: "match_found", GameID: game.ID}
		b, _ := json.Marshal(resp)
		c1.conn.WriteMessage(websocket.TextMessage, b)
		c2.conn.WriteMessage(websocket.TextMessage, b)
		c1.conn.Close()
		c2.conn.Close()
		return
	} else {
		matchmakingQueue.Unlock()
	}

	// Wait for match or disconnect
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			// Remove from queue if still present
			matchmakingQueue.Lock()
			for i, c := range matchmakingQueue.clients {
				if c.conn == conn {
					matchmakingQueue.clients = append(matchmakingQueue.clients[:i], matchmakingQueue.clients[i+1:]...)
					break
				}
			}
			matchmakingQueue.Unlock()
			return
		}
	}
}
