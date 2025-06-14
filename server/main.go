package main

import (
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	MAX_GUESSES = 6
	WORD_LENGTH = 5
	JOIN_MSG    = "join"
	GAME_STATE  = "game_state"
	GAME_OVER   = "game_over"
	GUESS_MSG   = "guess"
	PLAYER_ID   = "player_id"
	PLACEHOLDER = "waiting_for_opponent"
)

type Game struct {
	Id            string
	Solution      string
	CurrentPlayer string
	Connections   map[string]*websocket.Conn
	Players       []string
	Guesses       []string
	GameOver      bool
	LoserId       string
	mutex         sync.Mutex
	RematchGameId string
}

type Message struct {
	Type          string   `json:"type"`
	From          string   `json:"from"`
	Guess         string   `json:"guess"`
	Solution      string   `json:"solution"`
	Guesses       []string `json:"guesses"`
	CurrentPlayer string   `json:"currentPlayer"`
	GameOver      bool     `json:"gameOver"`
	LoserId       string   `json:"loserId"`
	Players       []string `json:"players"`
	RematchGameId string   `json:"rematchGameId"`
}

// WebSocket connection
type Client struct {
	conn *websocket.Conn
	send chan []byte
}

type GameServer struct {
	games map[string]*Game
	mutex sync.RWMutex
}

func NewGameServer() *GameServer {
	return &GameServer{
		games: make(map[string]*Game),
	}
}

func (s *GameServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	gameId := r.URL.Query().Get("game")
	isRematch := r.URL.Query().Get("rematch") == "true"
	previousGameId := r.URL.Query().Get("previousGame")

	if gameId == "" {
		http.Error(w, "Game ID required", http.StatusBadRequest)
		return
	}

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

	game := s.getOrCreateGame(gameId, isRematch, previousGameId)
	log.Printf("Player connecting to game: %s", gameId)

	defer func() {
		conn.Close()
		game.mutex.Lock()
		for id, player := range game.Connections {
			if player == conn {
				delete(game.Connections, id)
				log.Printf("Player %s disconnected from game %s", id, gameId)
				break
			}
		}
		game.mutex.Unlock()
	}()

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Printf("Error reading message: %v", err)
			break
		}

		var msg Message
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("Error unmarshaling message: %v", err)
			continue
		}

		log.Printf("Received message type: %s from player: %s in game: %s", msg.Type, msg.From, gameId)

		switch msg.Type {
		case JOIN_MSG:
			s.handleJoin(game, msg.From, conn)
		case GUESS_MSG:
			s.handleGuess(game, msg.From, msg.Guess)
		}
	}
}

func (s *GameServer) handleJoin(game *Game, playerId string, conn *websocket.Conn) {
	game.mutex.Lock()
	defer game.mutex.Unlock()

	log.Printf("Player %s joined game %s", playerId, game.Id)

	game.Connections[playerId] = conn

	if len(game.Players) < 2 {
		if !slices.Contains(game.Players, playerId) {
			game.Players = append(game.Players, playerId)
		}
		if game.CurrentPlayer == "" {
			game.CurrentPlayer = playerId
			log.Printf("First player %s set as current player", playerId)
		} else if game.CurrentPlayer == PLACEHOLDER {
			game.CurrentPlayer = playerId
			log.Printf("Second player %s joined, replacing placeholder", playerId)
		}
	}

	s.sendGameState(game, playerId)
}

func (s *GameServer) handleGuess(game *Game, playerId string, guess string) {
	game.mutex.Lock()
	defer game.mutex.Unlock()

	if game.GameOver {
		return
	}

	if game.CurrentPlayer != playerId {
		return
	}

	if len(game.Guesses) > 0 && len(game.Players) < 2 {
		return
	}

	log.Printf("Player %s made guess: %s", playerId, guess)
	game.Guesses = append(game.Guesses, guess)

	if strings.ToUpper(guess) == game.Solution {
		game.GameOver = true
		game.LoserId = playerId
		log.Printf("Player %s lost game %s", playerId, game.Id)
		s.broadcastGameOver(game)
		return
	}

	if len(game.Guesses) == 6 {
		game.GameOver = true
		game.LoserId = ""
		log.Printf("Game %s ended in a draw", game.Id)
		s.broadcastGameOver(game)
		return
	}

	if len(game.Players) < 2 {
		game.CurrentPlayer = PLACEHOLDER
	} else {
		for i, id := range game.Players {
			if id == playerId {
				nextPlayerIndex := (i + 1) % len(game.Players)
				game.CurrentPlayer = game.Players[nextPlayerIndex]
				log.Printf("Switching turn to player: %s", game.CurrentPlayer)
				break
			}
		}
	}

	s.broadcastGameState(game)
}

func (s *GameServer) broadcastGameOver(game *Game) {
	// Generate a new game ID for the rematch
	rematchGameId := uuid.New().String()
	game.RematchGameId = rematchGameId

	// Create the rematch game instance
	s.mutex.Lock()
	rematchGame := &Game{
		Id:          rematchGameId,
		Solution:    getRandomWord(),
		Connections: make(map[string]*websocket.Conn),
		Players:     make([]string, 2),
		Guesses:     make([]string, 0),
		GameOver:    false,
	}
	// Flip the player order for the rematch
	rematchGame.Players[0] = game.Players[1]
	rematchGame.Players[1] = game.Players[0]
	rematchGame.CurrentPlayer = rematchGame.Players[0]
	s.games[rematchGameId] = rematchGame
	s.mutex.Unlock()

	log.Printf("Created rematch game %s with solution: %s", rematchGameId, rematchGame.Solution)

	msg := Message{
		Type:          GAME_OVER,
		Players:       game.Players,
		Solution:      game.Solution,
		Guesses:       game.Guesses,
		LoserId:       game.LoserId,
		GameOver:      true,
		CurrentPlayer: game.CurrentPlayer,
		RematchGameId: rematchGameId,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Error marshaling game over message: %v", err)
		return
	}

	log.Printf("Broadcasting game over message with rematch game ID: %s", rematchGameId)

	for id, player := range game.Connections {
		if err := player.WriteMessage(websocket.TextMessage, data); err != nil {
			log.Printf("Error sending game over message to player %s: %v", id, err)
		}
	}
}

func getRandomWord() string {
	content, err := os.ReadFile("word_list.txt")
	if err != nil {
		log.Printf("Error reading word list: %v", err)
	}

	words := strings.Split(string(content), "\n")

	var validWords []string
	for _, word := range words {
		if word != "" {
			validWords = append(validWords, strings.ToUpper(word))
		} else {
			log.Printf("Empty word found in word list")
		}
	}

	if len(validWords) == 0 {
		log.Printf("No valid words found in word list")
		return "APPLE"
	}

	return validWords[rand.Intn(len(validWords))]
}

func main() {
	server := NewGameServer()
	http.HandleFunc("/ws", server.handleWebSocket)
	log.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func (s *GameServer) getOrCreateGame(gameId string, isRematch bool, previousGameId string) *Game {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	game, exists := s.games[gameId]
	if !exists {
		game = &Game{
			Id:          gameId,
			Solution:    getRandomWord(),
			Connections: make(map[string]*websocket.Conn),
			Players:     make([]string, 0, 2),
			Guesses:     make([]string, 0),
			GameOver:    false,
		}
		if isRematch && previousGameId != "" {
			if prevGame, ok := s.games[previousGameId]; ok && len(prevGame.Players) == 2 {
				game.Players = make([]string, 2)
				game.Players[0] = prevGame.Players[1]
				game.Players[1] = prevGame.Players[0]
				game.CurrentPlayer = game.Players[0]
				log.Printf("Rematch started for game %s with flipped turn order from game %s. First player: %s, Second player: %s",
					gameId, previousGameId, game.Players[0], game.Players[1])
			}
		}
		s.games[gameId] = game
		log.Printf("New game created with ID: %s, solution: %s", gameId, game.Solution)
	}
	return game
}

func (s *GameServer) sendGameState(game *Game, playerId string) {
	msg := Message{
		Type:          GAME_STATE,
		CurrentPlayer: game.CurrentPlayer,
		Solution:      game.Solution,
		Guesses:       game.Guesses,
		GameOver:      game.GameOver,
		Players:       game.Players,
		LoserId:       game.LoserId,
		RematchGameId: game.RematchGameId,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Error marshaling game state: %v", err)
		return
	}

	if err := game.Connections[playerId].WriteMessage(websocket.TextMessage, data); err != nil {
		log.Printf("Error sending game state to player %s: %v", playerId, err)
	}
}

func (s *GameServer) broadcastGameState(game *Game) {
	msg := Message{
		Type:          GAME_STATE,
		CurrentPlayer: game.CurrentPlayer,
		Solution:      game.Solution,
		Guesses:       game.Guesses,
		GameOver:      game.GameOver,
		Players:       game.Players,
		LoserId:       game.LoserId,
		RematchGameId: game.RematchGameId,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Error marshaling game state: %v", err)
		return
	}

	for id, player := range game.Connections {
		if err := player.WriteMessage(websocket.TextMessage, data); err != nil {
			log.Printf("Error broadcasting game state to player %s: %v", id, err)
		}
	}
}
