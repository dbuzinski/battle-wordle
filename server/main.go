package main

import (
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"sync"

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
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for development
	},
}

// Game state
type Game struct {
	Solution      string
	CurrentPlayer string
	Players       map[string]*Player
	Spectators    map[string]*Player
	Guesses       []string
	GameOver      bool
	LoserId       string
	mutex         sync.Mutex
}

// Player information
type Player struct {
	Conn *websocket.Conn
}

// Message structure
type Message struct {
	Type         string   `json:"type"`
	From         string   `json:"from"`
	Guess        string   `json:"guess"`
	Solution     string   `json:"solution"`
	Guesses      []string `json:"guesses"`
	CurrentPlayer string  `json:"currentPlayer"`
	GameOver     bool     `json:"gameOver"`
	LoserId      string   `json:"loserId"`
	IsSpectator  bool     `json:"isSpectator"`
}

// WebSocket connection
type Client struct {
	conn *websocket.Conn
	send chan []byte
}

// Game server
type GameServer struct {
	games map[string]*Game
	mutex sync.RWMutex
}

// Create new game server
func NewGameServer() *GameServer {
	return &GameServer{
		games: make(map[string]*Game),
	}
}

// Handle WebSocket connections
func (s *GameServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	gameId := r.URL.Query().Get("game")
	if gameId == "" {
		http.Error(w, "Game ID required", http.StatusBadRequest)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Error upgrading connection: %v", err)
		return
	}

	game := s.getOrCreateGame(gameId)
	log.Printf("Player connecting to game: %s", gameId)

	defer func() {
		conn.Close()
		// Remove player from game when they disconnect
		game.mutex.Lock()
		for id, player := range game.Players {
			if player.Conn == conn {
				delete(game.Players, id)
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

// Write messages to WebSocket
func (s *GameServer) writePump(c *Client) {
	defer func() {
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}
		}
	}
}

// Read messages from WebSocket
func (s *GameServer) readPump(c *Client) {
	defer func() {
		c.conn.Close()
	}()

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}

		var msg Message
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("error unmarshaling message: %v", err)
			continue
		}

		s.handleMessage(msg)
	}
}

// Handle incoming messages
func (s *GameServer) handleMessage(msg Message) {
	gameId := msg.From // Using From as gameId for now
	game := s.getOrCreateGame(gameId)

	switch msg.Type {
	case JOIN_MSG:
		s.handleJoin(game, msg.From, nil) // Note: conn will be nil here
	case GUESS_MSG:
		s.handleGuess(game, msg.From, msg.Guess)
	}
}

// Handle player join
func (s *GameServer) handleJoin(game *Game, playerId string, conn *websocket.Conn) {
	game.mutex.Lock()
	defer game.mutex.Unlock()

	log.Printf("Player %s attempting to join", playerId)
	log.Printf("Current state before join - CurrentPlayer: %s, Players: %v", game.CurrentPlayer, game.Players)

	// Check if this is a spectator
	isSpectator := len(game.Players) >= 2

	// Add or update player
	if isSpectator {
		game.Spectators[playerId] = &Player{
			Conn: conn,
		}
		log.Printf("Player %s joined as spectator", playerId)
	} else {
		game.Players[playerId] = &Player{
			Conn: conn,
		}
		if game.CurrentPlayer == "" {
			game.CurrentPlayer = playerId
			log.Printf("First player %s set as current player", playerId)
		}
	}

	// Send current game state to the joining player
	s.sendGameState(game, playerId, isSpectator)

	// If this is the second player, broadcast the updated game state to all players
	if len(game.Players) == 2 {
		s.broadcastGameState(game)
	}
}

// Handle player guess
func (s *GameServer) handleGuess(game *Game, playerId string, guess string) {
	game.mutex.Lock()
	defer game.mutex.Unlock()

	if game.GameOver {
		return
	}

	if game.CurrentPlayer != playerId {
		return
	}

	log.Printf("Player %s made guess: %s", playerId, guess)
	game.Guesses = append(game.Guesses, guess)

	if strings.ToUpper(guess) == game.Solution {
		game.GameOver = true
		game.LoserId = playerId
		log.Printf("Game over! Player %s guessed the word!", playerId)
		s.broadcastGameOver(game)
		return
	}

	// Switch turns
	for id := range game.Players {
		if id != playerId {
			game.CurrentPlayer = id
			log.Printf("Switching turn to player: %s", id)
			break
		}
	}

	s.broadcastGameState(game)
}

// Start new game
func (s *GameServer) startNewGame() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Create a new game with a random ID
	gameId := getRandomWord() // Using a word as game ID
	game := &Game{
		Solution:      getRandomWord(),
		Players:       make(map[string]*Player),
		Guesses:       make([]string, 0),
		GameOver:      false,
	}
	s.games[gameId] = game
}

// Broadcast game state
func (s *GameServer) broadcastState() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	for _, game := range s.games {
		s.broadcastGameState(game)
	}
}

// Broadcast game over
func (s *GameServer) broadcastGameOver(game *Game) {
	msg := Message{
		Type:         GAME_OVER,
		Solution:     game.Solution,
		Guesses:      game.Guesses,
		LoserId:      game.LoserId,
		GameOver:     true,
		CurrentPlayer: game.CurrentPlayer,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Error marshaling game over message: %v", err)
		return
	}

	log.Printf("Broadcasting game over message: %s", string(data))

	for id, player := range game.Players {
		if err := player.Conn.WriteMessage(websocket.TextMessage, data); err != nil {
			log.Printf("Error sending game over message to player %s: %v", id, err)
		}
	}
}

// Get random word
func getRandomWord() string {
	words := []string{
		"APPLE", "BEACH", "CLOUD", "DREAM", "EARTH",
		"FLAME", "GHOST", "HEART", "IVORY", "JUICE",
		"KNIFE", "LEMON", "MONEY", "NIGHT", "OCEAN",
		"PIANO", "QUEEN", "RADIO", "SMILE", "TIGER",
		"UMBRA", "VOICE", "WATER", "XEROX", "YACHT",
		"ZEBRA",
	}
	return words[rand.Intn(len(words))]
}

// Get random player
func getRandomPlayer(players map[string]string) string {
	playerIds := make([]string, 0, len(players))
	for id := range players {
		playerIds = append(playerIds, id)
	}
	return playerIds[rand.Intn(len(playerIds))]
}

// Main function
func main() {
	server := NewGameServer()
	http.HandleFunc("/ws", server.handleWebSocket)
	log.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func (s *GameServer) sendToPlayer(playerId string, msg Message) {
	// Find the connection for this player
	conn, exists := s.games[playerId]
	if !exists {
		log.Printf("No connection found for player %s", playerId)
		return
	}

	msgBytes, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Error marshaling message: %v", err)
		return
	}

	if err := conn.Players[playerId].Conn.WriteMessage(websocket.TextMessage, msgBytes); err != nil {
		log.Printf("Error sending message to player %s: %v", playerId, err)
	}
}

func (s *GameServer) getOrCreateGame(gameId string) *Game {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	game, exists := s.games[gameId]
	if !exists {
		game = &Game{
			Solution:      getRandomWord(),
			Players:       make(map[string]*Player),
			Spectators:    make(map[string]*Player),
			Guesses:       make([]string, 0),
			GameOver:      false,
		}
		s.games[gameId] = game
		log.Printf("New game created with ID: %s, solution: %s", gameId, game.Solution)
	}
	return game
}

func (s *GameServer) sendGameState(game *Game, playerId string, isSpectator bool) {
	msg := Message{
		Type:         GAME_STATE,
		CurrentPlayer: game.CurrentPlayer,
		Solution:     game.Solution,
		Guesses:      game.Guesses,
		GameOver:     game.GameOver,
		IsSpectator:  isSpectator,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Error marshaling game state: %v", err)
		return
	}

	var conn *websocket.Conn
	if isSpectator {
		conn = game.Spectators[playerId].Conn
	} else {
		conn = game.Players[playerId].Conn
	}

	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		log.Printf("Error sending game state to player %s: %v", playerId, err)
	}
}

func (s *GameServer) broadcastGameState(game *Game) {
	msg := Message{
		Type:         GAME_STATE,
		CurrentPlayer: game.CurrentPlayer,
		Solution:     game.Solution,
		Guesses:      game.Guesses,
		GameOver:     game.GameOver,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Error marshaling game state: %v", err)
		return
	}

	// Broadcast to players
	for id, player := range game.Players {
		if err := player.Conn.WriteMessage(websocket.TextMessage, data); err != nil {
			log.Printf("Error broadcasting game state to player %s: %v", id, err)
		}
	}

	// Broadcast to spectators
	for id, spectator := range game.Spectators {
		if err := spectator.Conn.WriteMessage(websocket.TextMessage, data); err != nil {
			log.Printf("Error broadcasting game state to spectator %s: %v", id, err)
		}
	}
}