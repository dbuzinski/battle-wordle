package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	_ "github.com/mattn/go-sqlite3"
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
	QUEUE_MSG   = "queue"
	MATCH_FOUND = "match_found"
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
	GameId        string   `json:"gameId"`
}

// WebSocket connection
type Client struct {
	conn *websocket.Conn
	send chan []byte
}

type QueueEntry struct {
	PlayerId string
	Conn     *websocket.Conn
}

type GameServer struct {
	games map[string]*Game
	queue []QueueEntry
	mutex sync.RWMutex
	db    *sql.DB
}

func NewGameServer() *GameServer {
	db, err := sql.Open("sqlite3", "./game.db?_journal=WAL&_timeout=5000&_busy_timeout=5000&_txlock=immediate")
	if err != nil {
		log.Fatal(err)
	}

	// Set connection pool settings
	db.SetMaxOpenConns(10) // Reduced from 25 to prevent too many concurrent connections
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Create tables if they don't exist
	createTables(db)

	return &GameServer{
		games: make(map[string]*Game),
		db:    db,
	}
}

func createTables(db *sql.DB) {
	// Create players table with index
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS players (
			id TEXT PRIMARY KEY,
			wins INTEGER DEFAULT 0,
			losses INTEGER DEFAULT 0,
			draws INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_players_id ON players(id);
	`)
	if err != nil {
		log.Fatal(err)
	}

	// Create games table with index
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS games (
			id TEXT PRIMARY KEY,
			solution TEXT NOT NULL,
			loser_id TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (loser_id) REFERENCES players(id)
		);
		CREATE INDEX IF NOT EXISTS idx_games_loser_id ON games(loser_id);
	`)
	if err != nil {
		log.Fatal(err)
	}

	// Create game_players table with indexes
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS game_players (
			game_id TEXT,
			player_id TEXT,
			FOREIGN KEY (game_id) REFERENCES games(id),
			FOREIGN KEY (player_id) REFERENCES players(id),
			PRIMARY KEY (game_id, player_id)
		);
		CREATE INDEX IF NOT EXISTS idx_game_players_game_id ON game_players(game_id);
		CREATE INDEX IF NOT EXISTS idx_game_players_player_id ON game_players(player_id);
	`)
	if err != nil {
		log.Fatal(err)
	}
}

func (s *GameServer) updatePlayerStats(playerId string, isWinner bool, isDraw bool) error {
	// First ensure player exists
	_, err := s.db.Exec(`
		INSERT OR IGNORE INTO players (id) VALUES (?)
	`, playerId)
	if err != nil {
		return err
	}

	// Update stats based on game outcome
	if isDraw {
		_, err = s.db.Exec(`
			UPDATE players SET draws = draws + 1 WHERE id = ?
		`, playerId)
	} else if isWinner {
		_, err = s.db.Exec(`
			UPDATE players SET wins = wins + 1 WHERE id = ?
		`, playerId)
	} else {
		_, err = s.db.Exec(`
			UPDATE players SET losses = losses + 1 WHERE id = ?
		`, playerId)
	}

	return err
}

func (s *GameServer) recordGame(gameId string, solution string, loserId string, playerIds []string) error {
	// Use a single transaction for all operations
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Insert game and game players in a single transaction
	_, err = tx.Exec(`
		INSERT INTO games (id, solution, loser_id) VALUES (?, ?, ?);
		INSERT OR IGNORE INTO players (id) VALUES (?), (?);
		INSERT INTO game_players (game_id, player_id) VALUES (?, ?), (?, ?);
		UPDATE players SET 
			wins = CASE 
				WHEN ? = '' THEN wins + 1 
				WHEN id = ? THEN wins + 1 
				ELSE wins 
			END,
			losses = CASE 
				WHEN ? = '' THEN losses + 1 
				WHEN id = ? THEN losses + 1 
				ELSE losses 
			END,
			draws = CASE 
				WHEN ? = '' THEN draws + 1 
				ELSE draws 
			END
		WHERE id IN (?, ?)
	`,
		gameId, solution, loserId,
		playerIds[0], playerIds[1],
		gameId, playerIds[0], gameId, playerIds[1],
		loserId, playerIds[0],
		loserId, playerIds[1],
		loserId,
		playerIds[0], playerIds[1])

	if err != nil {
		return err
	}

	return tx.Commit()
}

func (s *GameServer) getPlayerStats(playerId string) (wins, losses, draws int, err error) {
	// Use a read-only transaction for better concurrency
	tx, err := s.db.BeginTx(context.Background(), &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return 0, 0, 0, err
	}
	defer tx.Rollback()

	err = tx.QueryRow(`
		SELECT wins, losses, draws FROM players WHERE id = ?
	`, playerId).Scan(&wins, &losses, &draws)

	if err == sql.ErrNoRows {
		return 0, 0, 0, nil
	}
	return wins, losses, draws, err
}

func (s *GameServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	gameId := r.URL.Query().Get("game")
	isRematch := r.URL.Query().Get("rematch") == "true"
	previousGameId := r.URL.Query().Get("previousGame")

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

			var msg Message
			if err := json.Unmarshal(message, &msg); err != nil {
				log.Printf("Error unmarshaling message: %v", err)
				continue
			}

			if msg.Type == QUEUE_MSG {
				s.handleQueue(msg.From, conn)
			}
		}
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
		s.handleGameOver(game)
		return
	}

	if len(game.Guesses) == 6 {
		game.GameOver = true
		game.LoserId = ""
		log.Printf("Game %s ended in a draw", game.Id)
		s.handleGameOver(game)
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

func (s *GameServer) handleGameOver(game *Game) {
	// Generate a new game ID for the rematch
	rematchGameId := uuid.New().String()
	game.RematchGameId = rematchGameId

	// Record the game in the database
	err := s.recordGame(game.Id, game.Solution, game.LoserId, game.Players)
	if err != nil {
		log.Printf("Error recording game: %v", err)
	}

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

	// Add CORS middleware for WebSocket
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		server.handleWebSocket(w, r)
	})

	// Add CORS middleware for stats endpoint
	http.HandleFunc("/api/stats", server.handleStats)

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

func (s *GameServer) handleQueue(playerId string, conn *websocket.Conn) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Check if player is already in queue
	for _, entry := range s.queue {
		if entry.PlayerId == playerId {
			return // Already in queue
		}
	}

	// Add player to queue
	s.queue = append(s.queue, QueueEntry{PlayerId: playerId, Conn: conn})
	log.Printf("Player %s added to matchmaking queue. Queue length: %d", playerId, len(s.queue))

	// If we have at least 2 players, create a match
	if len(s.queue) >= 2 {
		// Get first two players
		player1 := s.queue[0]
		player2 := s.queue[1]
		s.queue = s.queue[2:] // Remove matched players from queue

		// Create new game
		gameId := uuid.New().String()
		game := &Game{
			Id:          gameId,
			Solution:    getRandomWord(),
			Connections: make(map[string]*websocket.Conn),
			Players:     make([]string, 2),
			Guesses:     make([]string, 0),
			GameOver:    false,
		}

		// Randomly assign player order
		if rand.Intn(2) == 0 {
			game.Players[0] = player1.PlayerId
			game.Players[1] = player2.PlayerId
		} else {
			game.Players[0] = player2.PlayerId
			game.Players[1] = player1.PlayerId
		}
		game.CurrentPlayer = game.Players[0]

		// Store game
		s.games[gameId] = game

		// Send match found message to both players
		matchMsg := Message{
			Type:     MATCH_FOUND,
			GameId:   gameId,
			Players:  game.Players,
			Solution: game.Solution,
		}

		data, err := json.Marshal(matchMsg)
		if err != nil {
			log.Printf("Error marshaling match found message: %v", err)
			return
		}

		log.Printf("Match found! Game ID: %s, Players: %v, First player: %s",
			gameId, game.Players, game.CurrentPlayer)

		// Send to both players
		if err := player1.Conn.WriteMessage(websocket.TextMessage, data); err != nil {
			log.Printf("Error sending match found message to player %s: %v", player1.PlayerId, err)
		}
		if err := player2.Conn.WriteMessage(websocket.TextMessage, data); err != nil {
			log.Printf("Error sending match found message to player %s: %v", player2.PlayerId, err)
		}
	}
}

func (s *GameServer) handleStats(w http.ResponseWriter, r *http.Request) {
	// Add CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	// Handle preflight requests
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	playerId := r.URL.Query().Get("playerId")
	if playerId == "" {
		http.Error(w, "Player ID is required", http.StatusBadRequest)
		return
	}

	wins, losses, draws, err := s.getPlayerStats(playerId)
	if err != nil {
		if err == sql.ErrNoRows {
			// Return zero stats for new players
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]int{
				"wins":   0,
				"losses": 0,
				"draws":  0,
			})
			return
		}
		http.Error(w, "Error fetching player stats", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{
		"wins":   wins,
		"losses": losses,
		"draws":  draws,
	})
}
