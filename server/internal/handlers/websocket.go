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
					origin == "https://www.battlewordle.app"
			},
		},
	}
}

// createGameStateMessage creates a game state message
func (h *WebSocketHandler) createGameStateMessage(game *models.Game, msgType string) *models.Message {
	return &models.Message{
		Type:          msgType,
		CurrentPlayer: game.CurrentPlayer,
		Solution:      game.Solution,
		Guesses:       game.Guesses,
		GameOver:      game.GameOver,
		Players:       game.Players,
		LoserId:       game.LoserId,
		RematchGameId: game.RematchGameId,
		PlayerNames:   h.gameService.GetPlayerNames(game.Players),
	}
}

// HandleConnection handles a new WebSocket connection
func (h *WebSocketHandler) HandleConnection(w http.ResponseWriter, r *http.Request) {
	gameId := r.URL.Query().Get("game")

	conn, err := h.upgrader.Upgrade(w, r, nil)
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
				log.Printf("Error creating game %s: %v", gameId, err)
				conn.Close()
				return
			}
		} else {
			log.Printf("Error getting game %s: %v", gameId, err)
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

		switch msg.Type {
		case models.JOIN_MSG:
			if err := h.gameService.JoinGame(gameId, msg.From, conn); err != nil {
				log.Printf("Error joining game %s: %v", gameId, err)
				continue
			}
			h.sendGameState(game, msg.From)
		case models.GUESS_MSG:
			if err := h.gameService.MakeGuess(gameId, msg.From, msg.Guess); err != nil {
				log.Printf("Error making guess in game %s: %v", gameId, err)
				continue
			}
			// Get updated game state after guess
			game, err = h.gameService.GetGame(gameId)
			if err != nil {
				log.Printf("Error getting updated game state for game %s: %v", gameId, err)
				continue
			}
			h.broadcastGameState(game)
		}
	}
}

// sendGameState sends the current game state to a specific player
func (h *WebSocketHandler) sendGameState(game *models.Game, playerId string) {
	msg := h.createGameStateMessage(game, models.GAME_STATE)
	if err := game.Connections[playerId].WriteJSON(msg); err != nil {
		log.Printf("Error sending game state to player %s in game %s: %v", playerId, game.Id, err)
	}
}

// broadcastGameState sends the current game state to all players
func (h *WebSocketHandler) broadcastGameState(game *models.Game) {
	msgType := models.GAME_STATE
	if game.GameOver {
		msgType = models.GAME_OVER
	}

	msg := h.createGameStateMessage(game, msgType)
	for id, player := range game.Connections {
		if err := player.WriteJSON(msg); err != nil {
			log.Printf("Error broadcasting game state to player %s in game %s: %v", id, game.Id, err)
		}
	}
}
