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
)

// Message types
const (
	JOIN_MSG    = "join"
	GAME_STATE  = "game_state"
	GAME_OVER   = "game_over"
	GUESS_MSG   = "guess"
)

// Game state
type GameState struct {
	Players     map[string]string `json:"players"`
	CurrentPlayer string          `json:"currentPlayer"`
	Solution    string           `json:"solution,omitempty"`
	Guesses     []string         `json:"guesses,omitempty"`
	LoserId     string           `json:"loserId,omitempty"`
	GameOver    bool             `json:"gameOver,omitempty"`
}

// Message structure
type Message struct {
	Type          string   `json:"type"`
	From          string   `json:"from"`
	Name          string   `json:"name,omitempty"`
	Guess         string   `json:"guess,omitempty"`
	Message       string   `json:"message,omitempty"`
	CurrentPlayer string   `json:"currentPlayer,omitempty"`
	GameStarted   bool     `json:"gameStarted,omitempty"`
	Solution      string   `json:"solution,omitempty"`
	Guesses       []string `json:"guesses,omitempty"`
	LoserId       string   `json:"loserId,omitempty"`
	GameOver      bool     `json:"gameOver,omitempty"`
}

// WebSocket connection
type Client struct {
	conn *websocket.Conn
	send chan []byte
}

// Game server
type GameServer struct {
	state         GameState
	stateMutex    sync.Mutex
	connections   map[string]*websocket.Conn
}

// Create new game server
func newGameServer() *GameServer {
	return &GameServer{
		state: GameState{
			Players:    make(map[string]string),
			Guesses:    make([]string, 0),
			GameOver:   false,
		},
		connections: make(map[string]*websocket.Conn),
	}
}

// Handle WebSocket connections
func (s *GameServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Error upgrading to WebSocket: %v", err)
		return
	}
	defer conn.Close()

	var currentPlayerId string // Track the current player ID for cleanup

	// Initialize game state if not already done
	s.stateMutex.Lock()
	if s.state.Solution == "" {
		s.state.Solution = getRandomWord()
		s.state.GameOver = false
		s.state.Guesses = make([]string, 0)
		log.Printf("New game initialized with solution: %s", s.state.Solution)
	}
	log.Printf("Current game state - Solution: %s, CurrentPlayer: %s, GameOver: %v", 
		s.state.Solution, s.state.CurrentPlayer, s.state.GameOver)
	s.stateMutex.Unlock()

	// Handle the WebSocket connection
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

		log.Printf("Received message type: %s from player: %s", msg.Type, msg.From)

		switch msg.Type {
		case JOIN_MSG:
			// Store the connection before handling join
			s.stateMutex.Lock()
			s.connections[msg.From] = conn
			currentPlayerId = msg.From // Store for cleanup
			s.stateMutex.Unlock()
			s.handleJoin(msg.From, msg.Name)
		case GUESS_MSG:
			s.handleGuess(msg.From, msg.Guess)
		}
	}

	// Clean up connection when done
	if currentPlayerId != "" {
		s.stateMutex.Lock()
		delete(s.connections, currentPlayerId)
		s.stateMutex.Unlock()
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
	s.stateMutex.Lock()
	defer s.stateMutex.Unlock()

	switch msg.Type {
	case JOIN_MSG:
		s.handleJoin(msg.From, msg.Name)
	case GUESS_MSG:
		s.handleGuess(msg.From, msg.Guess)
	}
}

// Handle player join
func (s *GameServer) handleJoin(playerId string, name string) {
	s.stateMutex.Lock()
	defer s.stateMutex.Unlock()

	log.Printf("Player %s (%s) attempting to join", name, playerId)
	log.Printf("Current state before join - CurrentPlayer: %s, Players: %v", 
		s.state.CurrentPlayer, s.state.Players)

	// Check if player already exists
	if _, exists := s.state.Players[playerId]; exists {
		log.Printf("Player %s already exists", playerId)
		return
	}

	// Add player
	s.state.Players[playerId] = name

	// If no current player is set, set this player as current
	if s.state.CurrentPlayer == "" {
		s.state.CurrentPlayer = playerId
		log.Printf("First player %s set as current player", playerId)
	}

	log.Printf("Sending game state to player %s - CurrentPlayer: %s, GameOver: %v", 
		playerId, s.state.CurrentPlayer, s.state.GameOver)

	// Send current game state to the joining player
	s.sendToPlayer(playerId, Message{
		Type:          GAME_STATE,
		CurrentPlayer: s.state.CurrentPlayer,
		GameOver:      s.state.GameOver,
		Solution:      s.state.Solution,
		Guesses:       s.state.Guesses,
	})
}

// Handle player guess
func (s *GameServer) handleGuess(playerId string, guess string) {
	s.stateMutex.Lock()
	defer s.stateMutex.Unlock()

	if s.state.GameOver {
		log.Printf("Game is over, ignoring guess from %s", playerId)
		return
	}

	if playerId != s.state.CurrentPlayer {
		log.Printf("Not player's turn: %s (current: %s)", playerId, s.state.CurrentPlayer)
		return
	}

	s.state.Guesses = append(s.state.Guesses, guess)
	log.Printf("Player %s made guess: %s", playerId, guess)

	// Check for case-insensitive match
	if strings.ToUpper(guess) == strings.ToUpper(s.state.Solution) {
		log.Printf("Game over! Player %s guessed the word!", playerId)
		s.state.GameOver = true
		s.state.LoserId = playerId
		s.broadcastGameOver()
		return
	}

	if len(s.state.Guesses) >= MAX_GUESSES {
		log.Printf("Game over! Maximum guesses reached. Player %s loses!", playerId)
		s.state.GameOver = true
		s.state.LoserId = playerId
		s.broadcastGameOver()
		return
	}

	// Switch turns
	for id := range s.state.Players {
		if id != playerId {
			s.state.CurrentPlayer = id
			log.Printf("Switching turn to player: %s", id)
			break
		}
	}

	// Broadcast updated game state to all players
	gameStateMsg := Message{
		Type:          GAME_STATE,
		CurrentPlayer: s.state.CurrentPlayer,
		GameOver:      s.state.GameOver,
		Solution:      s.state.Solution,
		Guesses:       s.state.Guesses,
	}

	msgBytes, err := json.Marshal(gameStateMsg)
	if err != nil {
		log.Printf("Error marshaling game state: %v", err)
		return
	}

	for id, conn := range s.connections {
		if err := conn.WriteMessage(websocket.TextMessage, msgBytes); err != nil {
			log.Printf("Error sending game state to player %s: %v", id, err)
		}
	}
}

// Start new game
func (s *GameServer) startNewGame() {
	log.Printf("Starting new game with players: %v", s.state.Players)
	
	// Initialize game state
	s.state.GameOver = false
	s.state.LoserId = ""
	s.state.Solution = getRandomWord()
	s.state.Guesses = []string{}
	
	// Randomly select first player
	playerIds := make([]string, 0, len(s.state.Players))
	for id := range s.state.Players {
		playerIds = append(playerIds, id)
	}
	s.state.CurrentPlayer = playerIds[rand.Intn(len(playerIds))]
	
	log.Printf("Game started with solution: %s, first player: %s", s.state.Solution, s.state.CurrentPlayer)
	
	// Broadcast game state to all players
	s.broadcastState()
}

// Broadcast game state
func (s *GameServer) broadcastState() {
	s.stateMutex.Lock()
	defer s.stateMutex.Unlock()

	msg := Message{
		Type:         GAME_STATE,
		CurrentPlayer: s.state.CurrentPlayer,
		GameOver:      s.state.GameOver,
		Solution:      s.state.Solution,
		Guesses:       s.state.Guesses,
	}

	for playerId := range s.state.Players {
		s.sendToPlayer(playerId, msg)
	}
}

// Broadcast game over
func (s *GameServer) broadcastGameOver() {
	msg := Message{
		Type:        GAME_OVER,
		From:        "",
		Solution:    s.state.Solution,
		Guesses:     s.state.Guesses,
		LoserId:     s.state.LoserId,
		GameOver:    true,
		CurrentPlayer: s.state.CurrentPlayer,
	}
	
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Error marshaling game over message: %v", err)
		return
	}
	
	log.Printf("Broadcasting game over message: %s", string(data))
	
	for id, conn := range s.connections {
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
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
	server := newGameServer()
	http.HandleFunc("/ws", server.handleWebSocket)
	log.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func (s *GameServer) sendToPlayer(playerId string, msg Message) {
	// Find the connection for this player
	conn, exists := s.connections[playerId]
	if !exists {
		log.Printf("No connection found for player %s", playerId)
		return
	}

	msgBytes, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Error marshaling message: %v", err)
		return
	}

	if err := conn.WriteMessage(websocket.TextMessage, msgBytes); err != nil {
		log.Printf("Error sending message to player %s: %v", playerId, err)
	}
}