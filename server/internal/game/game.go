package game

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"battle-wordle/server/pkg/models"
)

// Service implements the GameService interface
type Service struct {
	games    map[string]*models.Game
	mutex    sync.RWMutex
	wordList []string
	db       *sql.DB
}

// NewService creates a new game service
func NewService(db *sql.DB) (*Service, error) {
	wordList, err := loadWordList()
	if err != nil {
		return nil, err
	}

	return &Service{
		games:    make(map[string]*models.Game),
		wordList: wordList,
		db:       db,
	}, nil
}

// loadWordList loads the word list from file
func loadWordList() ([]string, error) {
	content, err := os.ReadFile("word_list.txt")
	if err != nil {
		return nil, err
	}

	words := strings.Split(string(content), "\n")
	var validWords []string
	for _, word := range words {
		if word != "" {
			validWords = append(validWords, strings.ToUpper(word))
		}
	}

	return validWords, nil
}

// getRandomWord returns a random word from the word list
func (s *Service) getRandomWord() string {
	if len(s.wordList) == 0 {
		return "APPLE" // Fallback word
	}
	return s.wordList[rand.Intn(len(s.wordList))]
}

// GetGame returns a game by ID
func (s *Service) GetGame(id string) (*models.Game, error) {
	s.mutex.RLock()
	game, exists := s.games[id]
	s.mutex.RUnlock()

	if !exists {
		return nil, models.ErrGameNotFound
	}
	return game, nil
}

// CreateGame creates a new game with the specified ID
func (s *Service) CreateGame(gameId string) (*models.Game, error) {
	game := &models.Game{
		Id:          gameId,
		Solution:    s.getRandomWord(),
		Connections: make(map[string]*websocket.Conn),
		Players:     make([]string, 0, 2),
		Guesses:     make([]string, 0),
		GameOver:    false,
	}

	s.mutex.Lock()
	s.games[game.Id] = game
	s.mutex.Unlock()

	log.Printf("New game created with ID: %s, solution: %s", game.Id, game.Solution)
	return game, nil
}

// JoinGame handles a player joining a game
func (s *Service) JoinGame(gameId string, playerId string, conn *websocket.Conn) error {
	s.mutex.Lock()
	game, exists := s.games[gameId]
	if !exists {
		s.mutex.Unlock()
		return models.ErrGameNotFound
	}

	game.Connections[playerId] = conn
	if len(game.Players) < 2 {
		if !contains(game.Players, playerId) {
			game.Players = append(game.Players, playerId)
		}
		if game.CurrentPlayer == "" {
			game.CurrentPlayer = playerId
		} else if game.CurrentPlayer == models.PLACEHOLDER {
			game.CurrentPlayer = playerId
		}
	}
	s.mutex.Unlock()
	return nil
}

// MakeGuess handles a player making a guess
func (s *Service) MakeGuess(gameId string, playerId string, guess string) error {
	s.mutex.Lock()
	game, exists := s.games[gameId]
	if !exists {
		s.mutex.Unlock()
		return models.ErrGameNotFound
	}

	if game.GameOver {
		s.mutex.Unlock()
		return models.ErrGameOver
	}

	if game.CurrentPlayer != playerId {
		s.mutex.Unlock()
		return models.ErrNotYourTurn
	}

	if len(game.Guesses) > 0 && len(game.Players) < 2 {
		s.mutex.Unlock()
		return models.ErrWaitingForOpponent
	}

	game.Guesses = append(game.Guesses, guess)

	if strings.ToUpper(guess) == game.Solution {
		game.GameOver = true
		game.LoserId = playerId
		s.mutex.Unlock()
		log.Printf("[MakeGuess] Player %s lost game %s", playerId, gameId)
		return s.HandleGameOver(game)
	}

	if len(game.Guesses) == models.MAX_GUESSES {
		game.GameOver = true
		game.LoserId = ""
		s.mutex.Unlock()
		log.Printf("[MakeGuess] Game %s ended in a draw", gameId)
		return s.HandleGameOver(game)
	}

	if len(game.Players) < 2 {
		game.CurrentPlayer = models.PLACEHOLDER
	} else {
		for i, id := range game.Players {
			if id == playerId {
				nextPlayerIndex := (i + 1) % len(game.Players)
				game.CurrentPlayer = game.Players[nextPlayerIndex]
				log.Printf("[MakeGuess] Switching turn to player: %s", game.CurrentPlayer)
				break
			}
		}
	}
	s.mutex.Unlock()
	return nil
}

// HandleGameOver handles the end of a game and creates a rematch game
func (s *Service) HandleGameOver(game *models.Game) error {
	if game == nil {
		return fmt.Errorf("nil game object")
	}

	log.Printf("[HandleGameOver] Starting game over handling for game %s", game.Id)

	// Create rematch game first
	rematchGame, err := s.CreateRematchGame(game.Id)
	if err != nil {
		return err
	}

	// Update the game with rematch ID
	s.mutex.Lock()
	game.RematchGameId = rematchGame.Id
	s.mutex.Unlock()

	// Record the game in the database
	if err := s.recordGame(game.Id, game.Solution, game.LoserId, game.Players); err != nil {
		return err
	}

	// Broadcast game over message with rematch game ID
	msg := models.Message{
		Type:          models.GAME_OVER,
		Players:       game.Players,
		Solution:      game.Solution,
		Guesses:       game.Guesses,
		LoserId:       game.LoserId,
		GameOver:      true,
		CurrentPlayer: game.CurrentPlayer,
		RematchGameId: rematchGame.Id,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	// Send messages to all players
	s.mutex.RLock()
	connections := make(map[string]*websocket.Conn)
	for id, conn := range game.Connections {
		connections[id] = conn
	}
	s.mutex.RUnlock()

	for id, player := range connections {
		if err := player.WriteMessage(websocket.TextMessage, data); err != nil {
			log.Printf("Error sending game over message to player %s: %v", id, err)
		}
	}

	return nil
}

// GetPlayerStats returns a player's stats
func (s *Service) GetPlayerStats(playerId string) (wins, losses, draws int, err error) {
	err = s.db.QueryRow(`
		SELECT wins, losses, draws FROM players WHERE id = ?
	`, playerId).Scan(&wins, &losses, &draws)

	if err == sql.ErrNoRows {
		return 0, 0, 0, nil
	}
	return wins, losses, draws, err
}

// SetPlayerName sets a player's name
func (s *Service) SetPlayerName(playerId string, name string) error {
	// First ensure player exists
	_, err := s.db.Exec("INSERT OR IGNORE INTO players (id) VALUES (?)", playerId)
	if err != nil {
		return err
	}

	// Then update their name
	_, err = s.db.Exec("UPDATE players SET name = ? WHERE id = ?", name, playerId)
	return err
}

// GetRecentGames returns a player's recent games
func (s *Service) GetRecentGames(playerId string) ([]map[string]interface{}, error) {
	rows, err := s.db.Query(`
		WITH player_games AS (
			SELECT g.id, g.created_at, g.loser_id, g.current_player, gp2.player_id as opponent_id
			FROM games g
			JOIN game_players gp1 ON g.id = gp1.game_id AND gp1.player_id = ?
			JOIN game_players gp2 ON g.id = gp2.game_id AND gp2.player_id != ?
		)
		SELECT 
			pg.id,
			pg.created_at,
			pg.loser_id,
			pg.current_player,
			COALESCE(p.name, 'Player') as opponent_name,
			pg.opponent_id
		FROM player_games pg
		LEFT JOIN players p ON pg.opponent_id = p.id
		ORDER BY pg.created_at DESC
		LIMIT 50
	`, playerId, playerId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var games []map[string]interface{}
	for rows.Next() {
		var id, loserId, currentPlayer, opponentName, opponentId string
		var createdAt time.Time
		if err := rows.Scan(
			&id,
			&createdAt,
			&loserId,
			&currentPlayer,
			&opponentName,
			&opponentId,
		); err != nil {
			log.Printf("Error scanning game row: %v", err)
			continue
		}

		game := map[string]interface{}{
			"id":            id,
			"date":          createdAt.Format("Jan 2, 2006 3:04 PM"),
			"loserId":       loserId,
			"currentPlayer": currentPlayer,
			"opponentName":  opponentName,
			"opponentId":    opponentId,
		}
		games = append(games, game)
	}

	return games, rows.Err()
}

// GetHeadToHeadStats returns head-to-head stats between two players
func (s *Service) GetHeadToHeadStats(playerId, opponentId string) (wins, losses, draws int, err error) {
	rows, err := s.db.Query(`
		SELECT 
			CASE 
				WHEN g.loser_id = ? THEN 1
				WHEN g.loser_id = ? THEN 0
				ELSE 2
			END as result
		FROM games g
		JOIN game_players gp1 ON g.id = gp1.game_id AND gp1.player_id = ?
		JOIN game_players gp2 ON g.id = gp2.game_id AND gp2.player_id = ?
	`, playerId, opponentId, playerId, opponentId)
	if err != nil {
		return 0, 0, 0, err
	}
	defer rows.Close()

	for rows.Next() {
		var result int
		if err := rows.Scan(&result); err != nil {
			return 0, 0, 0, err
		}
		switch result {
		case 0:
			wins++
		case 1:
			losses++
		case 2:
			draws++
		}
	}

	return wins, losses, draws, nil
}

// Helper function to check if a slice contains a string
func contains(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

// CreateRematchGame creates a new game for a rematch
func (s *Service) CreateRematchGame(gameId string) (*models.Game, error) {
	log.Printf("[CreateRematchGame] Creating rematch game for game %s", gameId)

	// Get the game data first
	game, err := s.GetGame(gameId)
	if err != nil {
		log.Printf("[CreateRematchGame] Original game %s not found: %v", gameId, err)
		return nil, fmt.Errorf("game not found: %w", err)
	}

	// Create new game ID for rematch
	rematchGameId := uuid.New().String()
	log.Printf("[CreateRematchGame] Generated new rematch game ID: %s", rematchGameId)

	// Create new game with flipped player order
	newGame := &models.Game{
		Id:            rematchGameId,
		Solution:      s.getRandomWord(),
		CurrentPlayer: game.Players[1], // Flip player order
		Connections:   make(map[string]*websocket.Conn),
		Players:       []string{game.Players[1], game.Players[0]}, // Flip player order
		Guesses:       make([]string, 0),
		GameOver:      false,
	}

	// Add the new game to the service
	s.mutex.Lock()
	s.games[rematchGameId] = newGame
	s.mutex.Unlock()

	log.Printf("[CreateRematchGame] Created rematch game %s with players %v", rematchGameId, newGame.Players)
	return newGame, nil
}

// RemoveConnection removes a connection from a game
func (s *Service) RemoveConnection(gameId string, conn *websocket.Conn) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	game, exists := s.games[gameId]
	if !exists {
		return
	}

	for id, player := range game.Connections {
		if player == conn {
			delete(game.Connections, id)
			log.Printf("Player %s disconnected from game %s", id, gameId)
			break
		}
	}
}

// recordGame records a game in the database
func (s *Service) recordGame(gameId string, solution string, loserId string, playerIds []string) error {
	log.Printf("[recordGame] Starting to record game %s in database", gameId)

	// Start a transaction
	tx, err := s.db.Begin()
	if err != nil {
		log.Printf("[recordGame] Error starting transaction: %v", err)
		return fmt.Errorf("error starting transaction: %w", err)
	}
	defer tx.Rollback()

	log.Printf("[recordGame] Started transaction for game %s", gameId)

	// Insert the game
	_, err = tx.Exec(`
		INSERT INTO games (id, solution, loser_id, current_player)
		VALUES (?, ?, ?, ?)
	`, gameId, solution, loserId, playerIds[0])
	if err != nil {
		log.Printf("[recordGame] Error inserting game: %v", err)
		return fmt.Errorf("error inserting game: %w", err)
	}

	log.Printf("[recordGame] Successfully inserted game record")

	// Insert player associations
	for _, playerId := range playerIds {
		_, err = tx.Exec(`
			INSERT INTO game_players (game_id, player_id)
			VALUES (?, ?)
		`, gameId, playerId)
		if err != nil {
			log.Printf("[recordGame] Error inserting player association for player %s: %v", playerId, err)
			return fmt.Errorf("error inserting player association: %w", err)
		}
		log.Printf("[recordGame] Successfully inserted player association for player %s", playerId)
	}

	// Update player stats if there's a loser
	if loserId != "" {
		// Update loser's losses
		_, err = tx.Exec(`
			UPDATE players
			SET losses = losses + 1
			WHERE id = ?
		`, loserId)
		if err != nil {
			log.Printf("[recordGame] Error updating loser stats: %v", err)
			return fmt.Errorf("error updating loser stats: %w", err)
		}
		log.Printf("[recordGame] Successfully updated loser stats for player %s", loserId)

		// Update winner's wins
		winnerId := playerIds[0]
		if winnerId == loserId {
			winnerId = playerIds[1]
		}
		_, err = tx.Exec(`
			UPDATE players
			SET wins = wins + 1
			WHERE id = ?
		`, winnerId)
		if err != nil {
			log.Printf("[recordGame] Error updating winner stats: %v", err)
			return fmt.Errorf("error updating winner stats: %w", err)
		}
		log.Printf("[recordGame] Successfully updated winner stats for player %s", winnerId)
	} else {
		// Update draws for both players
		for _, playerId := range playerIds {
			_, err = tx.Exec(`
				UPDATE players
				SET draws = draws + 1
				WHERE id = ?
			`, playerId)
			if err != nil {
				log.Printf("[recordGame] Error updating draw stats for player %s: %v", playerId, err)
				return fmt.Errorf("error updating draw stats: %w", err)
			}
			log.Printf("[recordGame] Successfully updated draw stats for player %s", playerId)
		}
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		log.Printf("[recordGame] Error committing transaction: %v", err)
		return fmt.Errorf("error committing transaction: %w", err)
	}

	log.Printf("[recordGame] Successfully committed all database changes for game %s", gameId)
	return nil
}
