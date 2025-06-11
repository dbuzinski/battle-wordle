package main

import (
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"os"
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
	PLACEHOLDER = "waiting_for_opponent"
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
	Connections   map[string]*Player  // All connected users (players and spectators)
	Players       []string            // The two actual players in order
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
	Players      []string `json:"players"`  // The two actual players
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
		for id, player := range game.Connections {
			if player.Conn == conn {
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

	// Add or update connection
	game.Connections[playerId] = &Player{
		Conn: conn,
	}

	// If this is one of the first two players, add to Players
	if len(game.Players) < 2 {
		game.Players = append(game.Players, playerId)
		if game.CurrentPlayer == "" {
			game.CurrentPlayer = playerId
			log.Printf("First player %s set as current player", playerId)
		} else if game.CurrentPlayer == PLACEHOLDER {
			// If current player is placeholder, replace it with the new player
			game.CurrentPlayer = playerId
			log.Printf("Second player %s joined, replacing placeholder", playerId)
		}
	}

	// Send current game state to the joining player
	s.sendGameState(game, playerId)

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

	// Don't allow moves if it's not the player's turn
	if game.CurrentPlayer != playerId {
		return
	}

	// Allow first player's first move, but require two players for subsequent moves
	if len(game.Guesses) > 0 && len(game.Players) < 2 {
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

	// If this was the first player's first move and there's no second player yet,
	// set current player to placeholder
	if len(game.Players) < 2 {
		game.CurrentPlayer = PLACEHOLDER
	} else {
		// Switch turns between the two players
		for i, id := range game.Players {
			if id == playerId {
				// Set current player to the other player
				nextPlayerIndex := (i + 1) % len(game.Players)
				game.CurrentPlayer = game.Players[nextPlayerIndex]
				log.Printf("Switching turn to player: %s", game.CurrentPlayer)
				break
			}
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
		Players:       make([]string, 0),
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

	for id, player := range game.Connections {
		if err := player.Conn.WriteMessage(websocket.TextMessage, data); err != nil {
			log.Printf("Error sending game over message to player %s: %v", id, err)
		}
	}
}

// Get random word
func getRandomWord() string {
	// Read the word list file
	content, err := os.ReadFile("word_list.txt")
	if err != nil {
		log.Printf("Error reading word list: %v", err)
		// Fallback to a default word if file can't be read
		return "APPLE"
	}

	// Split the content into words
	words := strings.Split(string(content), "\n")
	
	// Filter out any empty lines
	var validWords []string
	for _, word := range words {
		if word != "" {
			validWords = append(validWords, strings.ToUpper(word))
		}
	}

	if len(validWords) == 0 {
		log.Printf("No valid words found in word list")
		return "APPLE"
	}

	// Return a random word from the list
	return validWords[rand.Intn(len(validWords))]
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

	if err := conn.Connections[playerId].Conn.WriteMessage(websocket.TextMessage, msgBytes); err != nil {
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
			Connections:   make(map[string]*Player),
			Players:       make([]string, 0, 2),
			Guesses:       make([]string, 0),
			GameOver:      false,
		}
		s.games[gameId] = game
		log.Printf("New game created with ID: %s, solution: %s", gameId, game.Solution)
	}
	return game
}

func (s *GameServer) sendGameState(game *Game, playerId string) {
	msg := Message{
		Type:         GAME_STATE,
		CurrentPlayer: game.CurrentPlayer,
		Solution:     game.Solution,
		Guesses:      game.Guesses,
		GameOver:     game.GameOver,
		Players:      game.Players,
		LoserId:      game.LoserId,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Error marshaling game state: %v", err)
		return
	}

	if err := game.Connections[playerId].Conn.WriteMessage(websocket.TextMessage, data); err != nil {
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
		Players:      game.Players,
		LoserId:      game.LoserId,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Error marshaling game state: %v", err)
		return
	}

	// Broadcast to all connected users
	for id, player := range game.Connections {
		if err := player.Conn.WriteMessage(websocket.TextMessage, data); err != nil {
			log.Printf("Error broadcasting game state to player %s: %v", id, err)
		}
	}
}