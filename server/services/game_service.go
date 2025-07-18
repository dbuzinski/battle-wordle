package services

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"strings"
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

	log.Printf("New game created: solution=%s, PlayerOne=%s, PlayerTwo=%s", solution, PlayerOne, PlayerTwo)

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

func (s *GameService) SubmitGuess(ctx context.Context, gameID string, guess string, playerID string) (*models.Game, error) {
	game, err := s.repo.GetByID(ctx, gameID)
	if err != nil {
		return nil, err
	}
	if game == nil {
		return nil, fmt.Errorf("game not found")
	}
	if playerID != game.CurrentPlayer {
		return game, nil // Not this player's turn, ignore
	}
	game.Guesses = append(game.Guesses, guess)
	game.UpdatedAt = time.Now()
	// Check if guess is correct
	if strings.ToUpper(guess) == strings.ToUpper(game.Solution) {
		game.Result = "lose:" + playerID
		// Do not switch turn
	} else {
		// Switch turn
		if game.CurrentPlayer == game.FirstPlayer {
			game.CurrentPlayer = game.SecondPlayer
		} else {
			game.CurrentPlayer = game.FirstPlayer
		}
		// If max guesses reached and not solved, it's a draw
		maxGuesses := 6
		if len(game.Guesses) >= maxGuesses {
			game.Result = "draw"
		}
	}
	if err := s.repo.UpdateGame(ctx, game); err != nil {
		return nil, err
	}
	return game, nil
}

// FeedbackType is a string: "correct", "present", or "absent"
type FeedbackType string

const (
	FeedbackCorrect FeedbackType = "correct"
	FeedbackPresent FeedbackType = "present"
	FeedbackAbsent  FeedbackType = "absent"
)

// getFeedback returns feedback for a single guess
func getFeedback(guess, solution string) []FeedbackType {
	guess = strings.ToUpper(guess)
	solution = strings.ToUpper(solution)
	feedback := make([]FeedbackType, len(solution))
	targetArr := []rune(solution)
	guessArr := []rune(guess)
	used := make([]bool, len(solution))

	// First pass: correct
	for i := 0; i < len(solution); i++ {
		if guessArr[i] == targetArr[i] {
			feedback[i] = FeedbackCorrect
			used[i] = true
		}
	}
	// Second pass: present
	for i := 0; i < len(solution); i++ {
		if feedback[i] == FeedbackCorrect {
			continue
		}
		found := false
		for j := 0; j < len(solution); j++ {
			if !used[j] && guessArr[i] == targetArr[j] {
				feedback[i] = FeedbackPresent
				used[j] = true
				found = true
				break
			}
		}
		if !found {
			feedback[i] = FeedbackAbsent
		}
	}
	return feedback
}

// GetFeedbacks returns feedback for all guesses in a game
func (s *GameService) GetFeedbacks(game *models.Game) [][]FeedbackType {
	feedbacks := make([][]FeedbackType, len(game.Guesses))
	for i, guess := range game.Guesses {
		feedbacks[i] = getFeedback(guess, game.Solution)
	}
	return feedbacks
}
