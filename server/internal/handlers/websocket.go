package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/websocket"

	"battle-wordle/server/internal/game"
	"battle-wordle/server/pkg/models"
)

// WebSocketHandler handles WebSocket connections
type WebSocketHandler struct {
	gameService        *game.Service
	matchmakingService *game.MatchmakingService
	upgrader           websocket.Upgrader
}

// NewWebSocketHandler creates a new WebSocket handler
func NewWebSocketHandler(gameService *game.Service, matchmakingService *game.MatchmakingService) *WebSocketHandler {
	return &WebSocketHandler{
		gameService:        gameService,
		matchmakingService: matchmakingService,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				origin := r.Header.Get("Origin")
				return origin == "https://battlewordle.app" ||
					origin == "https://www.battlewordle.app" ||
					origin == "http://localhost:5173"
			},
		},
	}
}

// HandleConnection handles a new WebSocket connection
func (h *WebSocketHandler) HandleConnection(w http.ResponseWriter, r *http.Request) {
	gameId := r.URL.Query().Get("game")

	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			return origin == "https://battlewordle.app" || origin == "https://www.battlewordle.app" || origin == "http://localhost:5173"
		},
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Error upgrading connection: %v", err)
		return
	}

	// If no game ID, this might be a queue request
	if gameId == "" {
		defer conn.Close()
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				log.Printf("Error reading message: %v", err)
				break
			}

			var msg models.Message
			if err := json.Unmarshal(message, &msg); err != nil {
				log.Printf("Error unmarshaling message: %v", err)
				continue
			}

			if msg.Type == models.QUEUE_MSG {
				h.matchmakingService.AddToQueue(msg.From, conn)
			}
		}
		return
	}

	// Get or create the game
	game, err := h.gameService.GetGame(gameId)
	if err != nil {
		if err == models.ErrGameNotFound {
			// Create a new game if it doesn't exist
			game, err = h.gameService.CreateGame(gameId)
			if err != nil {
				log.Printf("Error creating game: %v", err)
				conn.Close()
				return
			}
			log.Printf("New game created with ID: %s, solution: %s", gameId, game.Solution)
		} else {
			log.Printf("Error getting game: %v", err)
			conn.Close()
			return
		}
	}

	defer func() {
		conn.Close()
		h.gameService.RemoveConnection(gameId, conn)
	}()

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Printf("Error reading message: %v", err)
			break
		}

		var msg models.Message
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("Error unmarshaling message: %v", err)
			continue
		}

		log.Printf("Received message type: %s from player: %s in game: %s", msg.Type, msg.From, gameId)

		switch msg.Type {
		case models.JOIN_MSG:
			if err := h.gameService.JoinGame(gameId, msg.From, conn); err != nil {
				log.Printf("Error joining game: %v", err)
				continue
			}
			h.sendGameState(game, msg.From)
		case models.GUESS_MSG:
			if err := h.gameService.MakeGuess(gameId, msg.From, msg.Guess); err != nil {
				log.Printf("Error making guess: %v", err)
				continue
			}
			h.broadcastGameState(game)
		}
	}
}

// sendGameState sends the current game state to a specific player
func (h *WebSocketHandler) sendGameState(game *models.Game, playerId string) {
	msg := &models.Message{
		Type:          models.GAME_STATE,
		CurrentPlayer: game.CurrentPlayer,
		Solution:      game.Solution,
		Guesses:       game.Guesses,
		GameOver:      game.GameOver,
		Players:       game.Players,
		LoserId:       game.LoserId,
		RematchGameId: game.RematchGameId,
	}

	if err := game.Connections[playerId].WriteJSON(msg); err != nil {
		log.Printf("Error sending game state to player %s: %v", playerId, err)
	}
}

// broadcastGameState sends the current game state to all players
func (h *WebSocketHandler) broadcastGameState(game *models.Game) {
	msg := &models.Message{
		Type:          models.GAME_STATE,
		CurrentPlayer: game.CurrentPlayer,
		Solution:      game.Solution,
		Guesses:       game.Guesses,
		GameOver:      game.GameOver,
		Players:       game.Players,
		LoserId:       game.LoserId,
		RematchGameId: game.RematchGameId,
	}

	for id, player := range game.Connections {
		if err := player.WriteJSON(msg); err != nil {
			log.Printf("Error broadcasting game state to player %s: %v", id, err)
		}
	}
}
