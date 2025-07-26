package controllers

import (
	"battle-wordle/server/dto"
	"battle-wordle/server/middleware"
	"battle-wordle/server/models"
	"battle-wordle/server/services"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

// --- Mock GameRepo ---
type mockGameRepo struct {
	games    map[string]*models.Game
	byPlayer map[string][]*models.Game
}

func newMockGameRepo() *mockGameRepo {
	return &mockGameRepo{
		games:    make(map[string]*models.Game),
		byPlayer: make(map[string][]*models.Game),
	}
}
func (m *mockGameRepo) CreateGame(ctx context.Context, game *models.Game) error {
	m.games[game.ID] = game
	m.byPlayer[game.FirstPlayer] = append(m.byPlayer[game.FirstPlayer], game)
	m.byPlayer[game.SecondPlayer] = append(m.byPlayer[game.SecondPlayer], game)
	return nil
}
func (m *mockGameRepo) GetByID(ctx context.Context, id string) (*models.Game, error) {
	return m.games[id], nil
}
func (m *mockGameRepo) GetByPlayer(ctx context.Context, playerID string) ([]*models.Game, error) {
	return m.byPlayer[playerID], nil
}
func (m *mockGameRepo) UpdateGame(ctx context.Context, game *models.Game) error {
	m.games[game.ID] = game
	return nil
}

func setupGameTestServer(t *testing.T) (*httptest.Server, *mockGameRepo, *mockPlayerRepo, func()) {
	gameRepo := newMockGameRepo()
	playerRepo := newMockPlayerRepo()
	gameService := services.NewGameService(gameRepo, []string{"APPLE"})
	playerService := services.NewPlayerService(playerRepo, "testsecret")
	gameController := NewGameController(gameService, playerService)

	router := mux.NewRouter()
	router.HandleFunc("/api/game/{id}", gameController.GetGameByID)
	router.HandleFunc("/api/player/{id}/games", gameController.GetGamesByPlayer)
	router.HandleFunc("/api/game", gameController.CreateGame).Methods("POST")
	router.HandleFunc("/api/game/{id}/guess", gameController.SubmitGuess).Methods("POST")
	// Add more routes as needed

	h := middleware.Logger(middleware.ErrorHandler(router))
	ts := httptest.NewServer(h)
	cleanup := func() { ts.Close() }
	return ts, gameRepo, playerRepo, cleanup
}

func TestGameAPI_GetGameByID(t *testing.T) {
	ts, gameRepo, playerRepo, cleanup := setupGameTestServer(t)
	defer cleanup()
	// Add a player and a game with valid UUIDs
	p1ID := uuid.NewString()
	p2ID := uuid.NewString()
	gID := uuid.NewString()
	p1 := &models.Player{ID: p1ID, Name: "Alice"}
	p2 := &models.Player{ID: p2ID, Name: "Bob"}
	playerRepo.CreatePlayer(context.Background(), p1)
	playerRepo.CreatePlayer(context.Background(), p2)
	game := &models.Game{ID: gID, FirstPlayer: p1ID, SecondPlayer: p2ID, CurrentPlayer: p1ID, Guesses: []string{}, Solution: "APPLE"}
	gameRepo.CreateGame(context.Background(), game)
	resp, err := http.Get(ts.URL + "/api/game/" + gID)
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}
	var got dto.GameDTO
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if got.ID != gID {
		t.Errorf("Expected game ID %s, got %s", gID, got.ID)
	}
}

func TestGameAPI_CreateGame(t *testing.T) {
	ts, _, playerRepo, cleanup := setupGameTestServer(t)
	defer cleanup()
	p1ID := uuid.NewString()
	p2ID := uuid.NewString()
	playerRepo.CreatePlayer(context.Background(), &models.Player{ID: p1ID, Name: "Alice"})
	playerRepo.CreatePlayer(context.Background(), &models.Player{ID: p2ID, Name: "Bob"})
	body := strings.NewReader(`{"player_one":"` + p1ID + `","player_two":"` + p2ID + `"}`)
	resp, err := http.Post(ts.URL+"/api/game", "application/json", body)
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}
	var got dto.GameDTO
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if got.FirstPlayer.ID != p1ID || got.SecondPlayer.ID != p2ID {
		t.Errorf("Expected players %s and %s, got %+v", p1ID, p2ID, got)
	}
}

func TestGameAPI_GetGamesByPlayer(t *testing.T) {
	ts, gameRepo, playerRepo, cleanup := setupGameTestServer(t)
	defer cleanup()
	p1ID := uuid.NewString()
	p2ID := uuid.NewString()
	playerRepo.CreatePlayer(context.Background(), &models.Player{ID: p1ID, Name: "Alice"})
	playerRepo.CreatePlayer(context.Background(), &models.Player{ID: p2ID, Name: "Bob"})
	gID := uuid.NewString()
	game := &models.Game{ID: gID, FirstPlayer: p1ID, SecondPlayer: p2ID, CurrentPlayer: p1ID, Guesses: []string{}, Solution: "APPLE"}
	gameRepo.CreateGame(context.Background(), game)
	resp, err := http.Get(ts.URL + "/api/player/" + p1ID + "/games")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}
	var got []dto.GameDTO
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if len(got) == 0 || got[0].ID != gID {
		t.Errorf("Expected game ID %s, got %+v", gID, got)
	}
}

func TestGameAPI_SubmitGuess(t *testing.T) {
	ts, gameRepo, playerRepo, cleanup := setupGameTestServer(t)
	defer cleanup()
	p1ID := uuid.NewString()
	p2ID := uuid.NewString()
	playerRepo.CreatePlayer(context.Background(), &models.Player{ID: p1ID, Name: "Alice"})
	playerRepo.CreatePlayer(context.Background(), &models.Player{ID: p2ID, Name: "Bob"})
	gID := uuid.NewString()
	game := &models.Game{ID: gID, FirstPlayer: p1ID, SecondPlayer: p2ID, CurrentPlayer: p1ID, Guesses: []string{}, Solution: "APPLE"}
	gameRepo.CreateGame(context.Background(), game)
	body := strings.NewReader(`{"guess":"APPLE","player_id":"` + p1ID + `"}`)
	req, err := http.NewRequest("POST", ts.URL+"/api/game/"+gID+"/guess", body)
	if err != nil {
		t.Fatalf("NewRequest failed: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("Expected 200, got %d", resp.StatusCode)
	}
	var got dto.GameDTO
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if len(got.Guesses) == 0 || got.Guesses[0] != "APPLE" {
		t.Errorf("Expected guess APPLE, got %+v", got.Guesses)
	}
}
