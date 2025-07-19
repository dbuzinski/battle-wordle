package controllers

import (
	"battle-wordle/server/dto"
	"battle-wordle/server/middleware"
	"battle-wordle/server/services"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"battle-wordle/server/models"

	"github.com/gorilla/mux"
)

// --- Mock PlayerRepo (copied from player_service_test.go) ---
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

func setupTestServer(t *testing.T) (*httptest.Server, *mockPlayerRepo, func()) {
	repo := newMockPlayerRepo()
	playerService := services.NewPlayerService(repo, "testsecret")
	playerController := NewPlayerController(playerService)

	router := mux.NewRouter()
	router.HandleFunc("/api/player/register", playerController.Register)
	router.HandleFunc("/api/player/login", playerController.Login)
	router.HandleFunc("/api/player/{id}", playerController.GetPlayerByID)

	h := middleware.Logger(middleware.ErrorHandler(router))
	ts := httptest.NewServer(h)
	cleanup := func() { ts.Close() }
	return ts, repo, cleanup
}

func TestPlayerAPI_GuestRegistration(t *testing.T) {
	ts, _, cleanup := setupTestServer(t)
	defer cleanup()
	resp, err := http.Post(ts.URL+"/api/player/register", "application/json", strings.NewReader(`{"name":"guest1"}`))
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}
	var player dto.PlayerDTO
	if err := json.NewDecoder(resp.Body).Decode(&player); err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if player.Name != "guest1" || player.Registered {
		t.Errorf("Expected guest player, got %+v", player)
	}
}

func TestPlayerAPI_RegisteredRegistrationAndLogin(t *testing.T) {
	ts, _, cleanup := setupTestServer(t)
	defer cleanup()
	// Register
	body := `{"name":"user1","password":"pw"}`
	resp, err := http.Post(ts.URL+"/api/player/register", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}
	var reg struct {
		Player dto.PlayerDTO `json:"player"`
		Token  string        `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&reg); err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if reg.Player.Name != "user1" || !reg.Player.Registered {
		t.Errorf("Expected registered player, got %+v", reg.Player)
	}
	if reg.Token == "" {
		t.Errorf("Expected token, got empty string")
	}
	// Login
	loginBody := `{"name":"user1","password":"pw"}`
	loginResp, err := http.Post(ts.URL+"/api/player/login", "application/json", strings.NewReader(loginBody))
	if err != nil {
		t.Fatalf("Login POST failed: %v", err)
	}
	defer loginResp.Body.Close()
	if loginResp.StatusCode != 200 {
		t.Fatalf("Expected 200, got %d", loginResp.StatusCode)
	}
	var login struct {
		Player dto.PlayerDTO `json:"player"`
		Token  string        `json:"token"`
	}
	if err := json.NewDecoder(loginResp.Body).Decode(&login); err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if login.Player.Name != "user1" || !login.Player.Registered {
		t.Errorf("Expected registered player, got %+v", login.Player)
	}
	if login.Token == "" {
		t.Errorf("Expected token, got empty string")
	}
}

func TestPlayerAPI_LoginFailure(t *testing.T) {
	ts, _, cleanup := setupTestServer(t)
	defer cleanup()
	// Register
	body := `{"name":"user2","password":"pw"}`
	resp, err := http.Post(ts.URL+"/api/player/register", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	defer resp.Body.Close()
	// Login with wrong password
	loginBody := `{"name":"user2","password":"wrong"}`
	loginResp, err := http.Post(ts.URL+"/api/player/login", "application/json", strings.NewReader(loginBody))
	if err != nil {
		t.Fatalf("Login POST failed: %v", err)
	}
	defer loginResp.Body.Close()
	if loginResp.StatusCode == 200 {
		t.Errorf("Expected non-200 for bad login, got %d", loginResp.StatusCode)
	}
}

func TestPlayerAPI_GetPlayerByID(t *testing.T) {
	ts, _, cleanup := setupTestServer(t)
	defer cleanup()
	// Register
	body := `{"name":"user3","password":"pw"}`
	resp, err := http.Post(ts.URL+"/api/player/register", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	defer resp.Body.Close()
	var reg struct {
		Player dto.PlayerDTO `json:"player"`
		Token  string        `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&reg); err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	t.Logf("Registered player: %+v", reg.Player)
	// Get by ID
	getResp, err := http.Get(ts.URL + "/api/player/" + reg.Player.ID)
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer getResp.Body.Close()
	if getResp.StatusCode != 200 {
		t.Fatalf("Expected 200, got %d", getResp.StatusCode)
	}
	var got dto.PlayerDTO
	if err := json.NewDecoder(getResp.Body).Decode(&got); err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if got.ID != reg.Player.ID {
		t.Errorf("Expected player ID %s, got %s", reg.Player.ID, got.ID)
	}
}
