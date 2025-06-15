package game

import (
	"encoding/json"
	"log"
	"math/rand"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"battle-wordle/server/pkg/models"
)

// QueueEntry represents a player in the matchmaking queue
type QueueEntry struct {
	PlayerId string
	Conn     *websocket.Conn
}

// MatchmakingService handles matchmaking between players
type MatchmakingService struct {
	queue       []QueueEntry
	mutex       sync.Mutex
	gameService *Service
}

// NewMatchmakingService creates a new matchmaking service
func NewMatchmakingService(gameService *Service) *MatchmakingService {
	return &MatchmakingService{
		queue:       make([]QueueEntry, 0),
		gameService: gameService,
	}
}

// AddToQueue adds a player to the matchmaking queue
func (s *MatchmakingService) AddToQueue(playerId string, conn *websocket.Conn) {
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

		// Create new game with a new UUID
		gameId := uuid.New().String()
		game, err := s.gameService.CreateGame(gameId)
		if err != nil {
			log.Printf("Error creating game: %v", err)
			return
		}

		// Randomly assign player order
		if rand.Intn(2) == 0 {
			game.Players = []string{player1.PlayerId, player2.PlayerId}
		} else {
			game.Players = []string{player2.PlayerId, player1.PlayerId}
		}
		game.CurrentPlayer = game.Players[0]

		// Store player associations in database
		for _, playerId := range game.Players {
			_, err := s.gameService.db.Exec(`
				INSERT INTO game_players (game_id, player_id)
				VALUES (?, ?)
			`, gameId, playerId)
			if err != nil {
				log.Printf("Error storing player association for player %s: %v", playerId, err)
				return
			}
		}

		// Update current player in database
		_, err = s.gameService.db.Exec(`
			UPDATE games SET current_player = ? WHERE id = ?
		`, game.CurrentPlayer, gameId)
		if err != nil {
			log.Printf("Error updating current player: %v", err)
			return
		}
		log.Printf("Set current player to %s for game %s", game.CurrentPlayer, gameId)

		log.Printf("Match found! Game ID: %s, Players: %v, First player: %s, Solution: %s",
			gameId, game.Players, game.CurrentPlayer, game.Solution)

		// Get player names
		playerNames := s.gameService.GetPlayerNames(game.Players)
		log.Printf("[AddToQueue] Player names for game %s: %v", gameId, playerNames)

		// Send match found message to both players
		matchMsg := models.Message{
			Type:        models.MATCH_FOUND,
			GameId:      gameId,
			Players:     game.Players,
			Solution:    game.Solution,
			PlayerNames: playerNames,
		}

		data, err := json.Marshal(matchMsg)
		if err != nil {
			log.Printf("Error marshaling match found message: %v", err)
			return
		}

		log.Printf("[AddToQueue] Sending match found message to players: %v", game.Players)
		// Send to both players
		if err := player1.Conn.WriteMessage(websocket.TextMessage, data); err != nil {
			log.Printf("Error sending match found message to player %s: %v", player1.PlayerId, err)
		}
		if err := player2.Conn.WriteMessage(websocket.TextMessage, data); err != nil {
			log.Printf("Error sending match found message to player %s: %v", player2.PlayerId, err)
		}
	}
}

// RemoveFromQueue removes a player from the matchmaking queue
func (s *MatchmakingService) RemoveFromQueue(playerId string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	for i, entry := range s.queue {
		if entry.PlayerId == playerId {
			s.queue = append(s.queue[:i], s.queue[i+1:]...)
			return nil
		}
	}
	return nil
}
