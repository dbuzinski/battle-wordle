package game

import (
	"database/sql"
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
	defer s.mutex.Unlock()

	game, exists := s.games[gameId]
	if !exists {
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

	return nil
}

// MakeGuess handles a player making a guess
func (s *Service) MakeGuess(gameId string, playerId string, guess string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	game, exists := s.games[gameId]
	if !exists {
		return models.ErrGameNotFound
	}

	if game.GameOver {
		return models.ErrGameOver
	}

	if game.CurrentPlayer != playerId {
		return models.ErrNotYourTurn
	}

	if len(game.Guesses) > 0 && len(game.Players) < 2 {
		return models.ErrWaitingForOpponent
	}

	game.Guesses = append(game.Guesses, guess)

	if strings.ToUpper(guess) == game.Solution {
		game.GameOver = true
		game.LoserId = playerId
		return nil
	}

	if len(game.Guesses) == models.MAX_GUESSES {
		game.GameOver = true
		game.LoserId = ""
		return nil
	}

	if len(game.Players) < 2 {
		game.CurrentPlayer = models.PLACEHOLDER
	} else {
		for i, id := range game.Players {
			if id == playerId {
				nextPlayerIndex := (i + 1) % len(game.Players)
				game.CurrentPlayer = game.Players[nextPlayerIndex]
				break
			}
		}
	}

	return nil
}

// HandleGameOver handles game over state
func (s *Service) HandleGameOver(game *models.Game) error {
	rematchGameId := uuid.New().String()
	game.RematchGameId = rematchGameId

	s.mutex.Lock()
	rematchGame := &models.Game{
		Id:          rematchGameId,
		Solution:    s.getRandomWord(),
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

	return nil
}

// GetGame returns a game by ID
func (s *Service) GetGame(id string) (*models.Game, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	game, exists := s.games[id]
	if !exists {
		return nil, models.ErrGameNotFound
	}

	return game, nil
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
func (s *Service) CreateRematchGame(prevGame *models.Game) (*models.Game, error) {
	game := &models.Game{
		Id:          uuid.New().String(),
		Solution:    s.getRandomWord(),
		Connections: make(map[string]*websocket.Conn),
		Players:     make([]string, 2),
		Guesses:     make([]string, 0),
		GameOver:    false,
	}

	// Flip the player order for the rematch
	game.Players[0] = prevGame.Players[1]
	game.Players[1] = prevGame.Players[0]
	game.CurrentPlayer = game.Players[0]

	s.mutex.Lock()
	s.games[game.Id] = game
	s.mutex.Unlock()

	log.Printf("Rematch game created with ID: %s, solution: %s", game.Id, game.Solution)
	return game, nil
}

// RemoveConnection removes a connection from a game
func (s *Service) RemoveConnection(gameId string, conn *websocket.Conn) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	game, exists := s.games[gameId]
	if !exists {
		return
	}

	game.Mutex.Lock()
	for id, player := range game.Connections {
		if player == conn {
			delete(game.Connections, id)
			log.Printf("Player %s disconnected from game %s", id, gameId)
			break
		}
	}
	game.Mutex.Unlock()
}
