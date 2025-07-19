package services

import (
	"context"
	"log"
	"sync"
)

// MatchmakingService handles player matchmaking and challenge invitations.
type MatchmakingService struct {
	gameService       *GameService
	queue             []matchmakingClient
	mu                sync.Mutex
	online            map[string]interface{} // playerID -> connection
	pendingChallenges map[string]string      // challengedID -> challengerID
}

type matchmakingClient struct {
	PlayerID string
	Conn     interface{} // Controller can use *websocket.Conn or any connection type
}

// NewMatchmakingService creates a new MatchmakingService.
func NewMatchmakingService(gameService *GameService) *MatchmakingService {
	return &MatchmakingService{
		gameService:       gameService,
		queue:             make([]matchmakingClient, 0),
		online:            make(map[string]interface{}),
		pendingChallenges: make(map[string]string),
	}
}

// JoinQueue adds a player and their connection to the matchmaking queue.
func (m *MatchmakingService) JoinQueue(playerID string, conn interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queue = append(m.queue, matchmakingClient{PlayerID: playerID, Conn: conn})
}

// LeaveQueue removes a player from the matchmaking queue.
func (m *MatchmakingService) LeaveQueue(playerID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, c := range m.queue {
		if c.PlayerID == playerID {
			m.queue = append(m.queue[:i], m.queue[i+1:]...)
			break
		}
	}
}

// TryMatch attempts to match two players. If successful, returns the game and the paired connections.
func (m *MatchmakingService) TryMatch(ctx context.Context) (gameID string, conns [2]interface{}, ok bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.queue) < 2 {
		return "", [2]interface{}{}, false
	}
	c1 := m.queue[0]
	c2 := m.queue[1]
	m.queue = m.queue[2:]
	game, err := m.gameService.CreateGame(ctx, c1.PlayerID, c2.PlayerID)
	if err != nil {
		// If game creation fails, requeue the clients
		m.queue = append([]matchmakingClient{c1, c2}, m.queue...)
		return "", [2]interface{}{}, false
	}
	return game.ID, [2]interface{}{c1.Conn, c2.Conn}, true
}

// QueueLength returns the number of players waiting in the queue.
func (m *MatchmakingService) QueueLength() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.queue)
}

// RegisterConnection registers a player's connection for direct notifications (challenge/rematch).
func (m *MatchmakingService) RegisterConnection(playerID string, conn interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.online[playerID] = conn
	log.Printf("[debug] RegisterConnection: playerID=%s, conn type=%T", playerID, conn)
}

// UnregisterConnection removes a player's connection (on disconnect).
func (m *MatchmakingService) UnregisterConnection(playerID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.online, playerID)
	delete(m.pendingChallenges, playerID)
	// Remove as challenger if present
	for challenged, challenger := range m.pendingChallenges {
		if challenger == playerID {
			delete(m.pendingChallenges, challenged)
		}
	}
	log.Printf("[debug] UnregisterConnection: playerID=%s", playerID)
}

// SendChallengeInvite sends a challenge invite from challengerID to challengedID. Returns true if delivered.
func (m *MatchmakingService) SendChallengeInvite(challengerID, challengedID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Debug log: print all online player IDs
	log.Printf("[debug] SendChallengeInvite: online keys: %v, challengedID: %s", keys(m.online), challengedID)
	_, ok := m.online[challengedID]
	if !ok {
		return false
	}
	// Mark as pending
	m.pendingChallenges[challengedID] = challengerID
	// The controller should call this and send the message
	return true
}

// Helper function to get map keys as []string
func keys(m map[string]interface{}) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}

// AcceptChallenge handles a challenge response. If accepted, creates a game and returns (gameID, challengerConn, challengedConn, true). If declined, returns ("", challengerConn, challengedConn, false).
func (m *MatchmakingService) AcceptChallenge(ctx context.Context, challengedID string, accepted bool) (gameID string, challengerConn, challengedConn interface{}, ok bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	challengerID, found := m.pendingChallenges[challengedID]
	if !found {
		return "", nil, nil, false
	}
	challengerConn = m.online[challengerID]
	challengedConn = m.online[challengedID]
	delete(m.pendingChallenges, challengedID)
	if !accepted {
		return "", challengerConn, challengedConn, false
	}
	// Create game
	game, err := m.gameService.CreateGame(ctx, challengerID, challengedID)
	if err != nil {
		return "", challengerConn, challengedConn, false
	}
	return game.ID, challengerConn, challengedConn, true
}

// CancelChallenge removes a pending challenge from challenger to challenged.
func (m *MatchmakingService) CancelChallenge(challengerID, challengedID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.pendingChallenges[challengedID] == challengerID {
		delete(m.pendingChallenges, challengedID)
	}
}

// OnlineConnection returns the connection for a given playerID, or false if not online.
func (m *MatchmakingService) OnlineConnection(playerID string) (interface{}, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	conn, ok := m.online[playerID]
	if !ok {
		log.Printf("[debug] OnlineConnection: playerID %s not found", playerID)
	} else {
		log.Printf("[debug] OnlineConnection: playerID %s found, type: %T", playerID, conn)
	}
	return conn, ok
}
