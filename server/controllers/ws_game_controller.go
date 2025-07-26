package controllers

import (
	"battle-wordle/server/models"
	"battle-wordle/server/services"
	"battle-wordle/server/ws"
	"context"
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

// WSGameController handles game-specific WebSocket connections.
type WSGameController struct {
	gameService   *services.GameService
	playerService *services.PlayerService
	gameHub       *ws.Hub
	upgrader      websocket.Upgrader
}

func NewWSGameController(gameService *services.GameService, playerService *services.PlayerService, gameHub *ws.Hub) *WSGameController {
	return &WSGameController{
		gameService:   gameService,
		playerService: playerService,
		gameHub:       gameHub,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin:     func(r *http.Request) bool { return true },
		},
	}
}

// HandleWebSocket handles the game WebSocket connection for a specific game.
func (c *WSGameController) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	gameID := vars["id"]
	ctx := r.Context()

	conn, err := c.upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, "Could not upgrade to WebSocket", http.StatusBadRequest)
		return
	}
	c.gameHub.AddConnection(gameID, conn)
	defer func() {
		c.gameHub.RemoveConnection(gameID, conn)
		conn.Close()
	}()

	for {
		_, msgData, err := conn.ReadMessage()
		if err != nil {
			break
		}
		var msg struct {
			Type     string `json:"type"`
			Guess    string `json:"guess,omitempty"`
			PlayerID string `json:"player_id,omitempty"`
		}
		if err := json.Unmarshal(msgData, &msg); err != nil {
			continue
		}
		switch msg.Type {
		case "join":
			game, err := c.gameService.GetByID(ctx, gameID)
			if err != nil {
				continue
			}
			c.sendGameState(conn, game)
		case "guess":
			game, err := c.gameService.GetByID(ctx, gameID)
			if err != nil || game == nil {
				continue
			}
			if msg.PlayerID != game.CurrentPlayer {
				continue
			}
			updatedGame, err := c.gameService.SubmitGuess(ctx, gameID, msg.Guess, msg.PlayerID)
			if err != nil {
				continue
			}
			c.broadcastGameState(gameID, updatedGame)
		}
	}
}

func (c *WSGameController) sendGameState(conn *websocket.Conn, game *models.Game) {
	ctx := context.Background()
	firstPlayer, _ := c.playerService.GetByID(ctx, game.FirstPlayer)
	secondPlayer, _ := c.playerService.GetByID(ctx, game.SecondPlayer)
	feedbacks := c.gameService.GetFeedbacks(game)
	feedbackStrings := make([][]string, len(feedbacks))
	for i, fb := range feedbacks {
		feedbackStrings[i] = make([]string, len(fb))
		for j, f := range fb {
			feedbackStrings[i][j] = string(f)
		}
	}
	var solutionPtr *string
	if game.Result != "" {
		solutionPtr = &game.Solution
	}
	type PlayerSummary struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	type GameStateMessage struct {
		Type          string        `json:"type"`
		ID            string        `json:"id"`
		CreatedAt     string        `json:"created_at"`
		UpdatedAt     string        `json:"updated_at"`
		FirstPlayer   PlayerSummary `json:"first_player"`
		SecondPlayer  PlayerSummary `json:"second_player"`
		CurrentPlayer string        `json:"current_player"`
		Result        string        `json:"result"`
		Guesses       []string      `json:"guesses"`
		Feedback      [][]string    `json:"feedback"`
		Solution      *string       `json:"solution,omitempty"`
	}
	msg := GameStateMessage{
		Type:          "game_state",
		ID:            game.ID,
		CreatedAt:     game.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:     game.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		FirstPlayer:   PlayerSummary{ID: game.FirstPlayer, Name: firstPlayer.Name},
		SecondPlayer:  PlayerSummary{ID: game.SecondPlayer, Name: secondPlayer.Name},
		CurrentPlayer: game.CurrentPlayer,
		Result:        game.Result,
		Guesses:       game.Guesses,
		Feedback:      feedbackStrings,
		Solution:      solutionPtr,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	conn.WriteMessage(websocket.TextMessage, data)
}

func (c *WSGameController) broadcastGameState(gameID string, game *models.Game) {
	ctx := context.Background()
	firstPlayer, _ := c.playerService.GetByID(ctx, game.FirstPlayer)
	secondPlayer, _ := c.playerService.GetByID(ctx, game.SecondPlayer)
	feedbacks := c.gameService.GetFeedbacks(game)
	feedbackStrings := make([][]string, len(feedbacks))
	for i, fb := range feedbacks {
		feedbackStrings[i] = make([]string, len(fb))
		for j, f := range fb {
			feedbackStrings[i][j] = string(f)
		}
	}
	var solutionPtr *string
	if game.Result != "" {
		solutionPtr = &game.Solution
	}
	type PlayerSummary struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	type GameStateMessage struct {
		Type          string        `json:"type"`
		ID            string        `json:"id"`
		CreatedAt     string        `json:"created_at"`
		UpdatedAt     string        `json:"updated_at"`
		FirstPlayer   PlayerSummary `json:"first_player"`
		SecondPlayer  PlayerSummary `json:"second_player"`
		CurrentPlayer string        `json:"current_player"`
		Result        string        `json:"result"`
		Guesses       []string      `json:"guesses"`
		Feedback      [][]string    `json:"feedback"`
		Solution      *string       `json:"solution,omitempty"`
	}
	msg := GameStateMessage{
		Type:          "game_state",
		ID:            game.ID,
		CreatedAt:     game.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:     game.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		FirstPlayer:   PlayerSummary{ID: game.FirstPlayer, Name: firstPlayer.Name},
		SecondPlayer:  PlayerSummary{ID: game.SecondPlayer, Name: secondPlayer.Name},
		CurrentPlayer: game.CurrentPlayer,
		Result:        game.Result,
		Guesses:       game.Guesses,
		Feedback:      feedbackStrings,
		Solution:      solutionPtr,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	c.gameHub.Broadcast(gameID, data)
}
