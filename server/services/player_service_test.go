package services

import (
	"battle-wordle/server/models"
	"context"
	"testing"
)

type mockPlayerRepo struct {
	players map[string]*models.Player
	byName  map[string]*models.Player
}

func newMockPlayerRepo() *mockPlayerRepo {
	return &mockPlayerRepo{
		players: make(map[string]*models.Player),
		byName:  make(map[string]*models.Player),
	}
}
func (m *mockPlayerRepo) CreatePlayer(ctx context.Context, player *models.Player) error {
	if _, exists := m.byName[player.Name]; exists {
		return &mockError{"username_taken"}
	}
	m.players[player.ID] = player
	m.byName[player.Name] = player
	return nil
}
func (m *mockPlayerRepo) GetByID(ctx context.Context, id string) (*models.Player, error) {
	return m.players[id], nil
}
func (m *mockPlayerRepo) GetByName(ctx context.Context, name string) (*models.Player, error) {
	return m.byName[name], nil
}
func (m *mockPlayerRepo) UpdateGuestToRegistered(ctx context.Context, id, newName, passwordHash string) error {
	p, ok := m.players[id]
	if !ok || p.Registered {
		return &mockError{"guest_not_found_or_already_registered"}
	}
	if _, exists := m.byName[newName]; exists {
		return &mockError{"username_taken"}
	}
	delete(m.byName, p.Name)
	p.Name = newName
	p.Registered = true
	p.PasswordHash = &passwordHash
	m.byName[newName] = p
	return nil
}
func (m *mockPlayerRepo) SearchByName(ctx context.Context, name string) ([]*models.Player, error) {
	var result []*models.Player
	for _, p := range m.byName {
		if p != nil && p.Name == name {
			result = append(result, p)
		}
	}
	return result, nil
}

// mockError implements error
func (e *mockError) Error() string { return e.msg }

type mockError struct{ msg string }

func TestPlayerService_GuestRegistration(t *testing.T) {
	repo := newMockPlayerRepo()
	service := NewPlayerService(repo, "testsecret")
	ctx := context.Background()
	player, token, err := service.CreatePlayer(ctx, "guest1", nil, nil)
	if err != nil {
		t.Fatalf("Guest registration failed: %v", err)
	}
	if player == nil || player.Registered {
		t.Errorf("Expected guest player, got %+v", player)
	}
	if token != nil {
		t.Errorf("Expected no token for guest, got %v", *token)
	}
}

func TestPlayerService_RegisteredRegistration(t *testing.T) {
	repo := newMockPlayerRepo()
	service := NewPlayerService(repo, "testsecret")
	ctx := context.Background()
	pw := "pw"
	player, token, err := service.CreatePlayer(ctx, "user1", &pw, nil)
	if err != nil {
		t.Fatalf("Registered registration failed: %v", err)
	}
	if player == nil || !player.Registered {
		t.Errorf("Expected registered player, got %+v", player)
	}
	if token == nil || *token == "" {
		t.Errorf("Expected token for registered user, got %v", token)
	}
}

func TestPlayerService_DuplicateUsername(t *testing.T) {
	repo := newMockPlayerRepo()
	service := NewPlayerService(repo, "testsecret")
	ctx := context.Background()
	pw := "pw"
	_, _, err := service.CreatePlayer(ctx, "user1", &pw, nil)
	if err != nil {
		t.Fatalf("First registration failed: %v", err)
	}
	_, _, err = service.CreatePlayer(ctx, "user1", &pw, nil)
	if err == nil || err.Error() != "username_taken" {
		t.Errorf("Expected username_taken error, got %v", err)
	}
}

func TestPlayerService_Login(t *testing.T) {
	repo := newMockPlayerRepo()
	service := NewPlayerService(repo, "testsecret")
	ctx := context.Background()
	pw := "pw"
	player, _, err := service.CreatePlayer(ctx, "user1", &pw, nil)
	if err != nil {
		t.Fatalf("Registration failed: %v", err)
	}
	p, tkn, err := service.Login(ctx, "user1", pw)
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	if p.ID != player.ID {
		t.Errorf("Expected player ID %s, got %s", player.ID, p.ID)
	}
	if tkn == nil || *tkn == "" {
		t.Errorf("Expected token, got %v", tkn)
	}
	_, _, err = service.Login(ctx, "user1", "wrongpw")
	if err == nil {
		t.Errorf("Expected login failure with wrong password")
	}
}

func TestPlayerService_SearchByName(t *testing.T) {
	repo := newMockPlayerRepo()
	service := NewPlayerService(repo, "testsecret")
	ctx := context.Background()
	pw := "pw"
	service.CreatePlayer(ctx, "alice", &pw, nil)
	service.CreatePlayer(ctx, "bob", &pw, nil)
	players, err := service.SearchByName(ctx, "alice")
	if err != nil {
		t.Fatalf("SearchByName failed: %v", err)
	}
	if len(players) != 1 || players[0].Name != "alice" {
		t.Errorf("Expected to find alice, got %+v", players)
	}
}
