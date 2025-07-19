package services

import (
	"context"
	"testing"
	"time"

	"battle-wordle/server/models"
)

type mockGameRepo struct {
	created *models.Game
	updated *models.Game
}

func (m *mockGameRepo) CreateGame(ctx context.Context, game *models.Game) error {
	m.created = game
	return nil
}
func (m *mockGameRepo) GetByID(ctx context.Context, id string) (*models.Game, error) {
	if m.created != nil && m.created.ID == id {
		return m.created, nil
	}
	if m.updated != nil && m.updated.ID == id {
		return m.updated, nil
	}
	return nil, nil
}
func (m *mockGameRepo) GetByPlayer(ctx context.Context, playerID string) ([]*models.Game, error) {
	return nil, nil
}
func (m *mockGameRepo) UpdateGame(ctx context.Context, game *models.Game) error {
	m.updated = game
	return nil
}

func TestCreateGame(t *testing.T) {
	repo := &mockGameRepo{}
	wordList := []string{"APPLE", "BANJO"}
	service := NewGameService(repo, wordList)
	ctx := context.Background()
	game, err := service.CreateGame(ctx, "p1", "p2")
	if err != nil {
		t.Fatalf("CreateGame failed: %v", err)
	}
	if game.ID == "" || game.FirstPlayer != "p1" || game.SecondPlayer != "p2" {
		t.Errorf("Game fields not set correctly: %+v", game)
	}
	if game.Solution != "APPLE" && game.Solution != "BANJO" {
		t.Errorf("Unexpected solution: %s", game.Solution)
	}
}

func TestCreateGame_EmptyWordList(t *testing.T) {
	repo := &mockGameRepo{}
	service := NewGameService(repo, []string{})
	ctx := context.Background()
	_, err := service.CreateGame(ctx, "p1", "p2")
	if err == nil {
		t.Errorf("Expected error for empty word list")
	}
}

func TestGetFeedbacks(t *testing.T) {
	repo := &mockGameRepo{}
	service := NewGameService(repo, []string{"APPLE"})
	game := &models.Game{Solution: "APPLE", Guesses: []string{"APPLE", "GRAPE", "MANGO"}}
	feedbacks := service.GetFeedbacks(game)
	if len(feedbacks) != 3 {
		t.Errorf("Expected 3 feedbacks, got %d", len(feedbacks))
	}
	if feedbacks[0][0] != FeedbackCorrect {
		t.Errorf("Expected first guess to be correct")
	}
}

func TestSubmitGuess(t *testing.T) {
	repo := &mockGameRepo{}
	wordList := []string{"APPLE"}
	service := NewGameService(repo, wordList)
	ctx := context.Background()
	// Create a game
	game, err := service.CreateGame(ctx, "p1", "p2")
	if err != nil {
		t.Fatalf("CreateGame failed: %v", err)
	}
	game.Solution = "APPLE"
	game.CurrentPlayer = "p1"
	game.FirstPlayer = "p1"
	game.SecondPlayer = "p2"
	game.CreatedAt = time.Now()
	game.UpdatedAt = time.Now()
	repo.created = game

	// Wrong guess by p1
	updated, err := service.SubmitGuess(ctx, game.ID, "GRAPE", "p1")
	if err != nil {
		t.Fatalf("SubmitGuess failed: %v", err)
	}
	if updated.CurrentPlayer != "p2" {
		t.Errorf("Expected turn to switch to p2, got %s", updated.CurrentPlayer)
	}
	if updated.Result != "" {
		t.Errorf("Expected no result yet, got %s", updated.Result)
	}

	// Wrong guess by p2
	updated, err = service.SubmitGuess(ctx, game.ID, "MANGO", "p2")
	if err != nil {
		t.Fatalf("SubmitGuess failed: %v", err)
	}
	if updated.CurrentPlayer != "p1" {
		t.Errorf("Expected turn to switch to p1, got %s", updated.CurrentPlayer)
	}

	// Correct guess by p1
	updated, err = service.SubmitGuess(ctx, game.ID, "APPLE", "p1")
	if err != nil {
		t.Fatalf("SubmitGuess failed: %v", err)
	}
	if updated.Result != "lose:p1" {
		t.Errorf("Expected result 'lose:p1', got %s", updated.Result)
	}

	// Test draw (6 wrong guesses)
	game2, _ := service.CreateGame(ctx, "p1", "p2")
	game2.Solution = "APPLE"
	game2.CurrentPlayer = "p1"
	game2.FirstPlayer = "p1"
	game2.SecondPlayer = "p2"
	game2.CreatedAt = time.Now()
	game2.UpdatedAt = time.Now()
	repo.created = game2
	for i := 0; i < 6; i++ {
		player := "p1"
		if i%2 == 1 {
			player = "p2"
		}
		_, err := service.SubmitGuess(ctx, game2.ID, "WRONG", player)
		if err != nil {
			t.Fatalf("SubmitGuess failed: %v", err)
		}
	}
	if repo.updated.Result != "draw" {
		t.Errorf("Expected result 'draw', got %s", repo.updated.Result)
	}
}

func TestSubmitGuess_InvalidPlayer(t *testing.T) {
	repo := &mockGameRepo{}
	wordList := []string{"APPLE"}
	service := NewGameService(repo, wordList)
	ctx := context.Background()
	game, _ := service.CreateGame(ctx, "p1", "p2")
	game.CurrentPlayer = "p1"
	repo.created = game
	updated, err := service.SubmitGuess(ctx, game.ID, "GRAPE", "p2")
	if err != nil {
		t.Fatalf("SubmitGuess failed: %v", err)
	}
	if updated.CurrentPlayer != "p1" {
		t.Errorf("Expected turn to remain p1, got %s", updated.CurrentPlayer)
	}
	if len(updated.Guesses) != 0 {
		t.Errorf("Expected no guesses recorded, got %v", updated.Guesses)
	}
}
