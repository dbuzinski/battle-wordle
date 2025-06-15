package game

import (
	"log"
	"math/rand"
	"sync"

	"github.com/gorilla/websocket"

	"battle-wordle/server/pkg/models"
)

// QueueEntry represents a player in the matchmaking queue
type QueueEntry struct {
	PlayerId string
	Conn     *websocket.Conn
}

// MatchmakingService implements the matchmaking logic
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
func (s *MatchmakingService) AddToQueue(playerId string, conn *websocket.Conn) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Check if player is already in queue
	for _, entry := range s.queue {
		if entry.PlayerId == playerId {
			return nil // Already in queue
		}
	}

	// Add player to queue
	s.queue = append(s.queue, QueueEntry{PlayerId: playerId, Conn: conn})
	log.Printf("Player %s added to matchmaking queue. Queue length: %d", playerId, len(s.queue))

	// Try to create a match
	return s.processQueue()
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

// ProcessQueue processes the matchmaking queue
func (s *MatchmakingService) ProcessQueue() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.processQueue()
}

// processQueue is the internal implementation of queue processing
func (s *MatchmakingService) processQueue() error {
	if len(s.queue) < 2 {
		return nil
	}

	// Get first two players
	player1 := s.queue[0]
	player2 := s.queue[1]
	s.queue = s.queue[2:] // Remove matched players from queue

	// Create new game
	game, err := s.gameService.CreateGame()
	if err != nil {
		return err
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

	// Store connections
	game.Connections[player1.PlayerId] = player1.Conn
	game.Connections[player2.PlayerId] = player2.Conn

	log.Printf("Match found! Game ID: %s, Players: %v, First player: %s",
		game.Id, game.Players, game.CurrentPlayer)

	// Send match found message to both players
	matchMsg := &models.Message{
		Type:     models.MATCH_FOUND,
		GameId:   game.Id,
		Players:  game.Players,
		Solution: game.Solution,
	}

	// Send to both players
	if err := player1.Conn.WriteJSON(matchMsg); err != nil {
		log.Printf("Error sending match found message to player %s: %v", player1.PlayerId, err)
	}
	if err := player2.Conn.WriteJSON(matchMsg); err != nil {
		log.Printf("Error sending match found message to player %s: %v", player2.PlayerId, err)
	}

	return nil
}
