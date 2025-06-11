package main

import (
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Message struct {
	Type    string `json:"type"`
	From    string `json:"from,omitempty"`
	Content string `json:"content,omitempty"`
	Guess   string `json:"guess,omitempty"`
}

type Client struct {
	ID   string
	Conn *websocket.Conn
}

type GameState struct {
	Players       map[string]*Client
	PlayerOrder   []string
	CurrentPlayer string
	Solution      string
	GameStarted   bool
	GameOver      bool
	Guesses       []string
	Mutex         sync.Mutex
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// Allow all origins for development
		return true
	},
}

var state = GameState{
	Players: make(map[string]*Client),
	Guesses: make([]string, 0),
}

var words = []string{
	"APPLE", "BEACH", "CLOUD", "DREAM", "EARTH",
	"FLAME", "GHOST", "HEART", "IVORY", "JUICE",
	"KNIFE", "LIGHT", "MAGIC", "NIGHT", "OCEAN",
	"PEACE", "QUEEN", "RADIO", "SMILE", "TIGER",
	"UNITY", "VOICE", "WATER", "YOUTH", "ZEBRA",
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

func getRandomWord() string {
	return words[rand.Intn(len(words))]
}

func main() {
	http.HandleFunc("/ws", handleWS)

	log.Println("Server started on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket upgrade error:", err)
		return
	}
	defer conn.Close()

	// Read first message to get player ID
	_, msg, err := conn.ReadMessage()
	if err != nil {
		log.Println("Failed to read join message:", err)
		return
	}

	var joinMsg Message
	if err := json.Unmarshal(msg, &joinMsg); err != nil {
		log.Printf("Failed to unmarshal join message: %v", err)
		return
	}
	playerID := joinMsg.From
	log.Printf("Player %s attempting to join", playerID)

	state.Mutex.Lock()
	if len(state.Players) >= 2 {
		log.Printf("Game is full, rejecting player %s", playerID)
		conn.WriteJSON(Message{Type: "error", Content: "Game is full"})
		state.Mutex.Unlock()
		return
	}

	client := &Client{ID: playerID, Conn: conn}
	state.Players[playerID] = client
	state.PlayerOrder = append(state.PlayerOrder, playerID)
	log.Printf("Player %s joined. Total players: %d", playerID, len(state.Players))

	if len(state.PlayerOrder) == 2 && !state.GameStarted {
		state.GameStarted = true
		state.CurrentPlayer = state.PlayerOrder[0]
		state.Solution = getRandomWord()
		state.Guesses = make([]string, 0)
		log.Printf("Game started! Solution: %s, First player: %s", state.Solution, state.CurrentPlayer)
	} else if state.GameStarted {
		// If game is already started, send current state to the joining player
		gameStateMsg := map[string]interface{}{
			"type":          "game_state",
			"yourId":        playerID,
			"currentPlayer": state.CurrentPlayer,
			"players":       state.PlayerOrder,
			"gameStarted":   state.GameStarted,
			"guesses":       state.Guesses,
			"solution":      state.Solution,
		}
		client.Conn.WriteJSON(gameStateMsg)
	}
	state.Mutex.Unlock()

	broadcastState()

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Printf("Connection closed for player %s: %v", playerID, err)
			break
		}

		var message Message
		if err := json.Unmarshal(msg, &message); err != nil {
			log.Printf("Failed to unmarshal message from %s: %v", playerID, err)
			continue
		}
		log.Printf("Received message from %s: %+v", playerID, message)

		if message.Type == "guess" {
			log.Printf("Processing guess from %s: %s", playerID, message.Guess)
			handleGuess(conn, playerID, message.Guess)
		}
	}
	state.Mutex.Lock()
	delete(state.Players, playerID)
	log.Printf("Player %s disconnected. Remaining players: %d", playerID, len(state.Players))
	state.Mutex.Unlock()
	broadcastState()
}

func handleGuess(conn *websocket.Conn, playerId string, guess string) {
	if state.CurrentPlayer != playerId {
		log.Printf("Not player's turn: %s (current: %s)", playerId, state.CurrentPlayer)
		return
	}

	if state.GameOver {
		log.Printf("Game is over, ignoring guess from %s", playerId)
		return
	}

	log.Printf("Processing guess from %s: %s (current player: %s)", playerId, guess, state.CurrentPlayer)
	
	// Add guess to history
	state.Guesses = append(state.Guesses, guess)
	log.Printf("Added guess to history. Total guesses: %d", len(state.Guesses))
	
	// Check if guess is correct
	if guess == state.Solution {
		log.Printf("Game over! Player %s guessed the word!", playerId)
		state.GameOver = true
		broadcastGameOver(playerId, true)
		return
	}
	
	// Switch turns
	if state.CurrentPlayer == state.PlayerOrder[0] {
		state.CurrentPlayer = state.PlayerOrder[1]
	} else {
		state.CurrentPlayer = state.PlayerOrder[0]
	}
	log.Printf("Turn switched to player %s", state.CurrentPlayer)
	
	// Broadcast updated state
	broadcastState()
}

func broadcastGameOver(loserID string, correctGuess bool) {
	log.Printf("Broadcasting game over. Loser: %s, Correct guess: %v", loserID, correctGuess)
	for _, client := range state.Players {
		gameOverMsg := map[string]interface{}{
			"type":         "game_over",
			"loserId":      loserID,
			"correctGuess": correctGuess,
			"solution":     state.Solution,
			"guesses":      state.Guesses,
		}
		if err := client.Conn.WriteJSON(gameOverMsg); err != nil {
			log.Printf("Error sending game over message to player %s: %v", client.ID, err)
		}
	}
}

func broadcastState() {
	state.Mutex.Lock()
	defer state.Mutex.Unlock()
	
	log.Printf("Broadcasting game state. Current player: %s, Total guesses: %d, Solution: %s", 
		state.CurrentPlayer, len(state.Guesses), state.Solution)
	
	for id, client := range state.Players {
		gameStateMsg := map[string]interface{}{
			"type":          "game_state",
			"yourId":        id,
			"currentPlayer": state.CurrentPlayer,
			"players":       state.PlayerOrder,
			"gameStarted":   state.GameStarted,
			"gameOver":      state.GameOver,
			"guesses":       state.Guesses,
			"solution":      state.Solution,
		}
		if err := client.Conn.WriteJSON(gameStateMsg); err != nil {
			log.Printf("Error broadcasting to player %s: %v", id, err)
		}
	}
}
