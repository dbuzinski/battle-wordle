package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Constants
const (
	MAX_GUESSES = 6
	WORD_LENGTH = 5
)

// Message types
const (
	JOIN_MSG     = "join"
	GUESS_MSG    = "guess"
	GAME_STATE   = "game_state"
	GAME_OVER    = "game_over"
)

// Game state
type GameState struct {
	Players       map[string]string
	CurrentPlayer string
	Solution      string
	Guesses       []string
	GameStarted   bool
	GameOver      bool
	LoserId       string
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
}

// WebSocket connection
type Client struct {
	conn *websocket.Conn
	send chan []byte
}

// Game server
type GameServer struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	state      *GameState
	stateMutex sync.Mutex
}

// Create new game server
func newGameServer() *GameServer {
	return &GameServer{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		state: &GameState{
			Players: make(map[string]string),
		},
	}
}

// Start game server
func (s *GameServer) run() {
	for {
		select {
		case client := <-s.register:
			s.clients[client] = true
		case client := <-s.unregister:
			if _, ok := s.clients[client]; ok {
				delete(s.clients, client)
				close(client.send)
			}
		case message := <-s.broadcast:
			for client := range s.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(s.clients, client)
				}
			}
		}
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
		log.Println(err)
		return
	}

	client := &Client{
		conn: conn,
		send: make(chan []byte, 256),
	}
	s.register <- client

	go s.writePump(client)
	go s.readPump(client)
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
		s.unregister <- c
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
		s.handleJoin(msg)
	case GUESS_MSG:
		s.handleGuess(msg)
	}
}

// Handle player join
func (s *GameServer) handleJoin(msg Message) {
	s.state.Players[msg.From] = msg.Name

	if len(s.state.Players) == 2 && !s.state.GameStarted {
		s.startNewGame()
	}

	s.broadcastState()
}

// Handle player guess
func (s *GameServer) handleGuess(msg Message) {
	s.stateMutex.Lock()
	defer s.stateMutex.Unlock()

	if s.state.GameOver {
		log.Printf("Game is over, ignoring guess from %s", msg.From)
		return
	}

	if msg.From != s.state.CurrentPlayer {
		log.Printf("Not player's turn: %s (current: %s)", msg.From, s.state.CurrentPlayer)
		return
	}

	s.state.Guesses = append(s.state.Guesses, msg.Guess)

	if msg.Guess == s.state.Solution {
		log.Printf("Game over! Player %s guessed the word!", msg.From)
		s.state.GameOver = true
		s.state.LoserId = msg.From
		s.broadcastGameOver()
		return
	}

	if len(s.state.Guesses) >= MAX_GUESSES {
		log.Printf("Game over! Maximum guesses reached. Player %s loses!", msg.From)
		s.state.GameOver = true
		s.state.LoserId = msg.From
		s.broadcastGameOver()
		return
	}

	s.switchTurn()
	s.broadcastState()
}

// Start new game
func (s *GameServer) startNewGame() {
	s.state.GameStarted = true
	s.state.GameOver = false
	s.state.Guesses = []string{}
	s.state.Solution = getRandomWord()
	s.state.CurrentPlayer = getRandomPlayer(s.state.Players)
}

// Switch player turn
func (s *GameServer) switchTurn() {
	for playerId := range s.state.Players {
		if playerId != s.state.CurrentPlayer {
			s.state.CurrentPlayer = playerId
			break
		}
	}
}

// Broadcast game state
func (s *GameServer) broadcastState() {
	s.stateMutex.Lock()
	defer s.stateMutex.Unlock()

	stateMsg := Message{
		Type:          GAME_STATE,
		CurrentPlayer: s.state.CurrentPlayer,
		GameStarted:   s.state.GameStarted,
		Solution:      s.state.Solution,
		Guesses:       s.state.Guesses,
	}

	message, err := json.Marshal(stateMsg)
	if err != nil {
		log.Printf("error marshaling state: %v", err)
		return
	}

	s.broadcast <- message
}

// Broadcast game over
func (s *GameServer) broadcastGameOver() {
	s.stateMutex.Lock()
	defer s.stateMutex.Unlock()

	gameOverMsg := Message{
		Type:     GAME_OVER,
		Solution: s.state.Solution,
		Guesses:  s.state.Guesses,
		LoserId:  s.state.LoserId,
	}

	message, err := json.Marshal(gameOverMsg)
	if err != nil {
		log.Printf("error marshaling game over: %v", err)
		return
	}

	s.broadcast <- message
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
	rand.Seed(time.Now().UnixNano())

	server := newGameServer()
	go server.run()

	http.HandleFunc("/ws", server.handleWebSocket)
	http.Handle("/", http.FileServer(http.Dir("ui")))

	fmt.Println("Server starting on :8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
