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
	log.Printf("[CreateGame] Creating new game with ID: %s", gameId)

	game := &models.Game{
		Id:          gameId,
		Solution:    s.getRandomWord(),
		Connections: make(map[string]*websocket.Conn),
		Players:     make([]string, 0, 2),
		Guesses:     make([]string, 0),
		GameOver:    false,
	}

	// Store the game in the database immediately
	guessesJson, err := json.Marshal(game.Guesses)
	if err != nil {
		log.Printf("[CreateGame] Error marshaling guesses: %v", err)
		return nil, fmt.Errorf("error marshaling guesses: %w", err)
	}

	log.Printf("[CreateGame] Storing game in database: id=%s, solution=%s, currentPlayer=%s, gameOver=%v",
		gameId, game.Solution, models.PLACEHOLDER, false)

	// Start a transaction
	tx, err := s.db.Begin()
	if err != nil {
		log.Printf("[CreateGame] Error starting transaction: %v", err)
		return nil, fmt.Errorf("error starting transaction: %w", err)
	}
	defer tx.Rollback()

	// Insert the game
	_, err = tx.Exec(`
		INSERT INTO games (
			id, 
			solution, 
			current_player,
			game_over,
			guesses
		) VALUES (?, ?, ?, ?, ?)
	`, gameId, game.Solution, models.PLACEHOLDER, false, string(guessesJson))
	if err != nil {
		log.Printf("[CreateGame] Error storing game in database: %v", err)
		return nil, fmt.Errorf("error storing game in database: %w", err)
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		log.Printf("[CreateGame] Error committing transaction: %v", err)
		return nil, fmt.Errorf("error committing transaction: %w", err)
	}

	s.mutex.Lock()
	s.games[game.Id] = game
	s.mutex.Unlock()

	log.Printf("[CreateGame] Successfully created game %s with solution %s", game.Id, game.Solution)
	return game, nil
}

// JoinGame handles a player joining a game
func (s *Service) JoinGame(gameId string, playerId string, conn *websocket.Conn) error {
	log.Printf("[JoinGame] Player %s joining game %s", playerId, gameId)

	s.mutex.Lock()
	game, exists := s.games[gameId]
	if !exists {
		s.mutex.Unlock()
		log.Printf("[JoinGame] Game %s not found", gameId)
		return models.ErrGameNotFound
	}

	game.Connections[playerId] = conn
	if len(game.Players) < 2 {
		if !contains(game.Players, playerId) {
			game.Players = append(game.Players, playerId)

			// Store player association in database
			_, err := s.db.Exec(`
				INSERT INTO game_players (game_id, player_id)
				VALUES (?, ?)
			`, gameId, playerId)
			if err != nil {
				log.Printf("[JoinGame] Error storing player association: %v", err)
				return fmt.Errorf("error storing player association: %w", err)
			}
			log.Printf("[JoinGame] Stored player association for player %s in game %s", playerId, gameId)
		}
		if game.CurrentPlayer == "" {
			game.CurrentPlayer = playerId
			// Update current player in database
			_, err := s.db.Exec(`
				UPDATE games SET current_player = ? WHERE id = ?
			`, playerId, gameId)
			if err != nil {
				log.Printf("[JoinGame] Error updating current player: %v", err)
				return fmt.Errorf("error updating current player: %w", err)
			}
			log.Printf("[JoinGame] Set current player to %s for game %s", playerId, gameId)
		} else if game.CurrentPlayer == models.PLACEHOLDER {
			game.CurrentPlayer = playerId
			// Update current player in database
			_, err := s.db.Exec(`
				UPDATE games SET current_player = ? WHERE id = ?
			`, playerId, gameId)
			if err != nil {
				log.Printf("[JoinGame] Error updating current player: %v", err)
				return fmt.Errorf("error updating current player: %w", err)
			}
			log.Printf("[JoinGame] Set current player to %s for game %s", playerId, gameId)
		}
	}
	s.mutex.Unlock()
	return nil
}

// MakeGuess handles a player making a guess
func (s *Service) MakeGuess(gameId string, playerId string, guess string) error {
	log.Printf("[MakeGuess] Player %s making guess in game %s: %s", playerId, gameId, guess)

	s.mutex.Lock()
	game, exists := s.games[gameId]
	if !exists {
		s.mutex.Unlock()
		log.Printf("[MakeGuess] Game %s not found", gameId)
		return models.ErrGameNotFound
	}

	if game.GameOver {
		s.mutex.Unlock()
		log.Printf("[MakeGuess] Game %s is already over", gameId)
		return models.ErrGameOver
	}

	if game.CurrentPlayer != playerId {
		s.mutex.Unlock()
		log.Printf("[MakeGuess] Not player %s's turn in game %s", playerId, gameId)
		return models.ErrNotYourTurn
	}

	if len(game.Guesses) > 0 && len(game.Players) < 2 {
		s.mutex.Unlock()
		log.Printf("[MakeGuess] Waiting for opponent in game %s", gameId)
		return models.ErrWaitingForOpponent
	}

	game.Guesses = append(game.Guesses, guess)
	log.Printf("[MakeGuess] Added guess to game %s, total guesses: %d", gameId, len(game.Guesses))

	// Update guesses in database
	guessesJson, err := json.Marshal(game.Guesses)
	if err != nil {
		log.Printf("[MakeGuess] Error marshaling guesses: %v", err)
		return fmt.Errorf("error marshaling guesses: %w", err)
	}

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

	// Update game state in database
	log.Printf("[MakeGuess] Updating game state in database: id=%s, currentPlayer=%s, gameOver=%v, loserId=%s",
		gameId, game.CurrentPlayer, game.GameOver, game.LoserId)

	_, err = s.db.Exec(`
		UPDATE games 
		SET guesses = ?,
			current_player = ?,
			game_over = ?,
			loser_id = ?
		WHERE id = ?
	`, string(guessesJson), game.CurrentPlayer, game.GameOver, game.LoserId, gameId)
	if err != nil {
		log.Printf("[MakeGuess] Error updating game state in database: %v", err)
		return fmt.Errorf("error updating game state in database: %w", err)
	}

	log.Printf("[MakeGuess] Successfully updated game state in database for game %s", gameId)
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

	// Update game state in database
	guessesJson, err := json.Marshal(game.Guesses)
	if err != nil {
		log.Printf("[HandleGameOver] Error marshaling guesses: %v", err)
		return fmt.Errorf("error marshaling guesses: %w", err)
	}

	_, err = s.db.Exec(`
		UPDATE games 
		SET game_over = true,
			loser_id = ?,
			guesses = ?,
			current_player = ?,
			rematch_game_id = ?
		WHERE id = ?
	`, game.LoserId, string(guessesJson), game.CurrentPlayer, rematchGame.Id, game.Id)
	if err != nil {
		log.Printf("[HandleGameOver] Error updating game state in database: %v", err)
		return fmt.Errorf("error updating game state in database: %w", err)
	}

	// Update player stats if there's a loser
	if game.LoserId != "" {
		// Update loser's losses
		_, err = s.db.Exec(`
			UPDATE players
			SET losses = losses + 1
			WHERE id = ?
		`, game.LoserId)
		if err != nil {
			log.Printf("[HandleGameOver] Error updating loser stats: %v", err)
			return fmt.Errorf("error updating loser stats: %w", err)
		}
		log.Printf("[HandleGameOver] Successfully updated loser stats for player %s", game.LoserId)

		// Update winner's wins
		winnerId := game.Players[0]
		if winnerId == game.LoserId {
			winnerId = game.Players[1]
		}
		_, err = s.db.Exec(`
			UPDATE players
			SET wins = wins + 1
			WHERE id = ?
		`, winnerId)
		if err != nil {
			log.Printf("[HandleGameOver] Error updating winner stats: %v", err)
			return fmt.Errorf("error updating winner stats: %w", err)
		}
		log.Printf("[HandleGameOver] Successfully updated winner stats for player %s", winnerId)
	} else {
		// Update draws for both players
		for _, playerId := range game.Players {
			_, err = s.db.Exec(`
				UPDATE players
				SET draws = draws + 1
				WHERE id = ?
			`, playerId)
			if err != nil {
				log.Printf("[HandleGameOver] Error updating draw stats for player %s: %v", playerId, err)
				return fmt.Errorf("error updating draw stats: %w", err)
			}
			log.Printf("[HandleGameOver] Successfully updated draw stats for player %s", playerId)
		}
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
	log.Printf("[SetPlayerName] Setting name for player %s to: %s", playerId, name)

	// First ensure player exists with default name
	_, err := s.db.Exec("INSERT OR IGNORE INTO players (id, name) VALUES (?, ?)", playerId, "Player")
	if err != nil {
		log.Printf("[SetPlayerName] Error ensuring player exists: %v", err)
		return err
	}

	// Then update their name
	result, err := s.db.Exec("UPDATE players SET name = ? WHERE id = ?", name, playerId)
	if err != nil {
		log.Printf("[SetPlayerName] Error updating player name: %v", err)
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		log.Printf("[SetPlayerName] Error getting rows affected: %v", err)
		return err
	}
	log.Printf("[SetPlayerName] Successfully updated name for player %s (rows affected: %d)", playerId, rowsAffected)

	// Verify the name was saved
	var savedName string
	err = s.db.QueryRow("SELECT name FROM players WHERE id = ?", playerId).Scan(&savedName)
	if err != nil {
		log.Printf("[SetPlayerName] Error verifying saved name: %v", err)
	} else {
		log.Printf("[SetPlayerName] Verified saved name for player %s: %s", playerId, savedName)
	}

	return nil
}

// GetRecentGames returns a player's recent games
func (s *Service) GetRecentGames(playerId string) ([]map[string]interface{}, error) {
	log.Printf("[GetRecentGames] Fetching games for player %s", playerId)

	rows, err := s.db.Query(`
		WITH player_games AS (
			SELECT 
				g.id, 
				g.created_at, 
				g.loser_id, 
				g.current_player, 
				g.game_over,
				g.guesses,
				gp2.player_id as opponent_id
			FROM games g
			JOIN game_players gp1 ON g.id = gp1.game_id AND gp1.player_id = ?
			JOIN game_players gp2 ON g.id = gp2.game_id AND gp2.player_id != ?
		)
		SELECT 
			pg.id,
			pg.created_at,
			pg.loser_id,
			pg.current_player,
			pg.game_over,
			pg.guesses,
			COALESCE(p.name, 'Player') as opponent_name,
			pg.opponent_id
		FROM player_games pg
		LEFT JOIN players p ON pg.opponent_id = p.id
		ORDER BY pg.created_at DESC
		LIMIT 50
	`, playerId, playerId)
	if err != nil {
		log.Printf("[GetRecentGames] Error querying games: %v", err)
		return nil, err
	}
	defer rows.Close()

	var games []map[string]interface{}
	for rows.Next() {
		var id, loserId, currentPlayer, opponentName, opponentId string
		var createdAt time.Time
		var gameOver bool
		var guessesJson sql.NullString
		if err := rows.Scan(
			&id,
			&createdAt,
			&loserId,
			&currentPlayer,
			&gameOver,
			&guessesJson,
			&opponentName,
			&opponentId,
		); err != nil {
			log.Printf("[GetRecentGames] Error scanning game row: %v", err)
			continue
		}

		log.Printf("[GetRecentGames] Found game %s: currentPlayer=%s, gameOver=%v, loserId=%s",
			id, currentPlayer, gameOver, loserId)

		// Parse guesses from JSON
		var guesses []string
		if guessesJson.Valid {
			if err := json.Unmarshal([]byte(guessesJson.String), &guesses); err != nil {
				log.Printf("[GetRecentGames] Error unmarshaling guesses: %v", err)
				guesses = []string{}
			}
			log.Printf("[GetRecentGames] Game %s has %d guesses", id, len(guesses))
		}

		// Get the game from memory to check if it's still active
		gameState, _ := s.GetGame(id)
		isInProgress := gameState != nil && !gameState.GameOver && len(gameState.Players) == 2
		log.Printf("[GetRecentGames] Game %s in memory: %v, isInProgress: %v",
			id, gameState != nil, isInProgress)

		game := map[string]interface{}{
			"id":            id,
			"date":          createdAt.Format("Jan 2, 2006 3:04 PM"),
			"loserId":       loserId,
			"currentPlayer": currentPlayer,
			"opponentName":  opponentName,
			"opponentId":    opponentId,
			"isInProgress":  isInProgress,
			"guesses":       guesses,
			"gameOver":      gameOver,
		}

		// Add game state if it's in memory
		if gameState != nil {
			game["solution"] = gameState.Solution
			log.Printf("[GetRecentGames] Added game state for game %s: guesses=%v, solution=%s, gameOver=%v",
				id, gameState.Guesses, gameState.Solution, gameState.GameOver)
		}

		games = append(games, game)
	}

	log.Printf("[GetRecentGames] Returning %d games for player %s", len(games), playerId)
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

	// Store the game in the database
	guessesJson, err := json.Marshal(newGame.Guesses)
	if err != nil {
		log.Printf("[CreateRematchGame] Error marshaling guesses: %v", err)
		return nil, fmt.Errorf("error marshaling guesses: %w", err)
	}

	// Start a transaction
	tx, err := s.db.Begin()
	if err != nil {
		log.Printf("[CreateRematchGame] Error starting transaction: %v", err)
		return nil, fmt.Errorf("error starting transaction: %w", err)
	}
	defer tx.Rollback()

	// Insert the game
	_, err = tx.Exec(`
		INSERT INTO games (
			id, 
			solution, 
			current_player,
			game_over,
			guesses
		) VALUES (?, ?, ?, ?, ?)
	`, rematchGameId, newGame.Solution, newGame.CurrentPlayer, false, string(guessesJson))
	if err != nil {
		log.Printf("[CreateRematchGame] Error storing game in database: %v", err)
		return nil, fmt.Errorf("error storing game in database: %w", err)
	}

	// Insert player associations
	for _, playerId := range newGame.Players {
		_, err = tx.Exec(`
			INSERT INTO game_players (game_id, player_id)
			VALUES (?, ?)
		`, rematchGameId, playerId)
		if err != nil {
			log.Printf("[CreateRematchGame] Error storing player association for player %s: %v", playerId, err)
			return nil, fmt.Errorf("error storing player association: %w", err)
		}
		log.Printf("[CreateRematchGame] Stored player association for player %s in game %s", playerId, rematchGameId)
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		log.Printf("[CreateRematchGame] Error committing transaction: %v", err)
		return nil, fmt.Errorf("error committing transaction: %w", err)
	}

	// Add the new game to the service and ensure it's properly initialized
	s.mutex.Lock()
	s.games[rematchGameId] = newGame
	s.mutex.Unlock()

	// Initialize game state in memory
	s.mutex.Lock()
	if _, exists := s.games[rematchGameId]; !exists {
		s.games[rematchGameId] = newGame
	}
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

// GetPlayerNames returns a map of player IDs to their names
func (s *Service) GetPlayerNames(playerIds []string) map[string]string {
	log.Printf("[GetPlayerNames] Getting names for players: %v", playerIds)

	names := make(map[string]string)
	if len(playerIds) == 0 {
		return names
	}

	// Build the query with the correct number of placeholders
	placeholders := make([]string, len(playerIds))
	args := make([]interface{}, len(playerIds))
	for i, id := range playerIds {
		placeholders[i] = "?"
		args[i] = id
	}
	query := fmt.Sprintf("SELECT id, name FROM players WHERE id IN (%s)", strings.Join(placeholders, ","))

	log.Printf("[GetPlayerNames] Executing query: %s with args: %v", query, args)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		log.Printf("[GetPlayerNames] Error querying player names: %v", err)
		return names
	}
	defer rows.Close()

	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			log.Printf("[GetPlayerNames] Error scanning row: %v", err)
			continue
		}
		names[id] = name
		log.Printf("[GetPlayerNames] Found name for player %s: %s", id, name)
	}

	// Set default names for any players not found
	for _, id := range playerIds {
		if _, exists := names[id]; !exists {
			names[id] = "Player"
			log.Printf("[GetPlayerNames] No name found for player %s, using default", id)
		}
	}

	log.Printf("[GetPlayerNames] Final player names map: %v", names)
	return names
}
