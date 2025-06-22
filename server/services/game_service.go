package services

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"battle-wordle/server/models"
	"battle-wordle/server/repositories"

	"github.com/google/uuid"
)

type GameService struct {
	repo     *repositories.GameRepository
	wordList []string
}

// NewGameService creates a new GameService with the provided word list.
func NewGameService(repo *repositories.GameRepository, wordList []string) *GameService {
	return &GameService{repo: repo, wordList: wordList}
}

func (s *GameService) GetByID(ctx context.Context, gameID string) (*models.Game, error) {
	return s.repo.GetByID(ctx, gameID)
}

func (s *GameService) GetByPlayer(ctx context.Context, playerID string) ([]*models.Game, error) {
	return s.repo.GetByPlayer(ctx, playerID)
}

func (s *GameService) CreateGame(ctx context.Context, PlayerOne string, PlayerTwo string) (*models.Game, error) {
	if len(s.wordList) == 0 {
		return nil, fmt.Errorf("word list is empty")
	}
	solution := s.wordList[rand.Intn(len(s.wordList))]

	game := &models.Game{
		ID:            uuid.NewString(),
		Solution:      solution,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		FirstPlayer:   PlayerOne,
		SecondPlayer:  PlayerTwo,
		CurrentPlayer: PlayerOne,
		Result:        "",
		Guesses:       []string{},
	}
	if err := s.repo.CreateGame(ctx, game); err != nil {
		return nil, err
	}
	return game, nil
}

func (s *GameService) SubmitGuess(ctx context.Context, gameID string, guess string) (*models.Game, error) {
	game, err := s.repo.GetByID(ctx, gameID)
	if err != nil {
		return nil, err
	}
	if game == nil {
		return nil, fmt.Errorf("game not found")
	}
	game.Guesses = append(game.Guesses, guess)
	game.UpdatedAt = time.Now()
	if err := s.repo.UpdateGame(ctx, game); err != nil {
		return nil, err
	}
	return game, nil
}
