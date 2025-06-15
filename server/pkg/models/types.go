package models

import (
	"net/http"

	"github.com/gorilla/websocket"
)

// Constants
const (
	MAX_GUESSES = 6
	WORD_LENGTH = 5
	JOIN_MSG    = "join"
	GAME_STATE  = "game_state"
	GAME_OVER   = "game_over"
	GUESS_MSG   = "guess"
	PLAYER_ID   = "player_id"
	PLACEHOLDER = "waiting_for_opponent"
	QUEUE_MSG   = "queue"
	MATCH_FOUND = "match_found"
)

// Message represents a WebSocket message
type Message struct {
	Type          string            `json:"type"`
	From          string            `json:"from"`
	Guess         string            `json:"guess"`
	Solution      string            `json:"solution"`
	Guesses       []string          `json:"guesses"`
	CurrentPlayer string            `json:"currentPlayer"`
	GameOver      bool              `json:"gameOver"`
	LoserId       string            `json:"loserId"`
	Players       []string          `json:"players"`
	PlayerNames   map[string]string `json:"playerNames"`
	RematchGameId string            `json:"rematchGameId"`
	GameId        string            `json:"gameId"`
}

// Game represents a game instance
type Game struct {
	Id            string
	Solution      string
	CurrentPlayer string
	Connections   map[string]*websocket.Conn
	Players       []string
	Guesses       []string
	GameOver      bool
	LoserId       string
	RematchGameId string
}

// Player represents a player in the system
type Player struct {
	ID     string
	Name   string
	Wins   int
	Losses int
	Draws  int
}

// GameService defines the interface for game business logic
type GameService interface {
	CreateGame() (*Game, error)
	JoinGame(gameId string, playerId string, conn *websocket.Conn) error
	MakeGuess(gameId string, playerId string, guess string) error
	HandleGameOver(game *Game) error
}

// MatchmakingService defines the interface for matchmaking logic
type MatchmakingService interface {
	AddToQueue(playerId string, conn *websocket.Conn) error
	RemoveFromQueue(playerId string) error
	ProcessQueue() error
}

// WebSocketHandler defines the interface for WebSocket operations
type WebSocketHandler interface {
	HandleConnection(w http.ResponseWriter, r *http.Request)
	SendMessage(conn *websocket.Conn, msg *Message) error
	BroadcastMessage(game *Game, msg *Message) error
}
