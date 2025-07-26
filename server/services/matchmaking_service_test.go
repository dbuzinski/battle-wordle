package services

import (
	"battle-wordle/server/models"
	"context"
	"testing"
)

type mockGameService struct {
	createdGames []struct{ p1, p2 string }
	failCreate   bool
}

func (m *mockGameService) CreateGame(ctx context.Context, p1, p2 string) (*models.Game, error) {
	if m.failCreate {
		return nil, context.DeadlineExceeded
	}
	m.createdGames = append(m.createdGames, struct{ p1, p2 string }{p1, p2})
	return &models.Game{ID: p1 + ":" + p2, FirstPlayer: p1, SecondPlayer: p2}, nil
}

func TestJoinAndLeaveQueue(t *testing.T) {
	mockGS := &mockGameService{}
	mm := NewMatchmakingService(mockGS)
	mm.JoinQueue("p1", "conn1")
	mm.JoinQueue("p2", "conn2")
	if mm.QueueLength() != 2 {
		t.Errorf("Expected queue length 2, got %d", mm.QueueLength())
	}
	mm.LeaveQueue("p1")
	if mm.QueueLength() != 1 {
		t.Errorf("Expected queue length 1 after leave, got %d", mm.QueueLength())
	}
}

func TestTryMatch(t *testing.T) {
	mockGS := &mockGameService{}
	mm := NewMatchmakingService(mockGS)
	mm.JoinQueue("p1", "conn1")
	mm.JoinQueue("p2", "conn2")
	gameID, conns, ok := mm.TryMatch(context.Background())
	if !ok || gameID == "" {
		t.Errorf("Expected match, got ok=%v, gameID=%s", ok, gameID)
	}
	if conns[0] != "conn1" || conns[1] != "conn2" {
		t.Errorf("Expected correct connections, got %v", conns)
	}
	if mm.QueueLength() != 0 {
		t.Errorf("Expected queue to be empty after match, got %d", mm.QueueLength())
	}
}

func TestTryMatch_GameCreationFails(t *testing.T) {
	mockGS := &mockGameService{failCreate: true}
	mm := NewMatchmakingService(mockGS)
	mm.JoinQueue("p1", "conn1")
	mm.JoinQueue("p2", "conn2")
	gameID, _, ok := mm.TryMatch(context.Background())
	if ok || gameID != "" {
		t.Errorf("Expected no match when game creation fails")
	}
	if mm.QueueLength() != 2 {
		t.Errorf("Expected queue to remain full, got %d", mm.QueueLength())
	}
}

func TestSendChallengeInviteAndAccept(t *testing.T) {
	mockGS := &mockGameService{}
	mm := NewMatchmakingService(mockGS)
	mm.RegisterConnection("p1", "conn1")
	mm.RegisterConnection("p2", "conn2")
	ok := mm.SendChallengeInvite("p1", "p2")
	if !ok {
		t.Errorf("Expected challenge invite to succeed")
	}
	gameID, challengerConn, challengedConn, accepted := mm.AcceptChallenge(context.Background(), "p2", true)
	if !accepted || gameID == "" {
		t.Errorf("Expected challenge to be accepted and game created")
	}
	if challengerConn != "conn1" || challengedConn != "conn2" {
		t.Errorf("Expected correct connections returned")
	}
}

func TestCancelChallenge(t *testing.T) {
	mm := NewMatchmakingService(nil)
	mm.RegisterConnection("p1", "conn1")
	mm.RegisterConnection("p2", "conn2")
	mm.SendChallengeInvite("p1", "p2")
	mm.CancelChallenge("p1", "p2")
	_, _, _, accepted := mm.AcceptChallenge(context.Background(), "p2", true)
	if accepted {
		t.Errorf("Expected challenge to be cancelled and not accepted")
	}
}

func TestOnlineConnection(t *testing.T) {
	mm := NewMatchmakingService(nil)
	mm.RegisterConnection("p1", "conn1")
	conn, ok := mm.OnlineConnection("p1")
	if !ok || conn != "conn1" {
		t.Errorf("Expected to find online connection")
	}
	mm.UnregisterConnection("p1")
	_, ok = mm.OnlineConnection("p1")
	if ok {
		t.Errorf("Expected connection to be unregistered")
	}
}
