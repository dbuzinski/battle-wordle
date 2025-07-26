package services

import (
	"context"

	"battle-wordle/server/repositories"
)

// StatsService provides logic for player and game statistics.
type StatsService struct {
	gameRepo   *repositories.GameRepository
	playerRepo *repositories.PlayerRepository
}

type HeadToHeadStats struct {
	FirstPlayerWins  int `json:"first_player_wins"`
	SecondPlayerWins int `json:"second_player_wins"`
	Draws            int `json:"draws"`
}

// NewStatsService creates a new StatsService.
func NewStatsService(gameRepo *repositories.GameRepository, playerRepo *repositories.PlayerRepository) *StatsService {
	return &StatsService{gameRepo: gameRepo, playerRepo: playerRepo}
}

func (s *StatsService) GetHeadToHeadStats(ctx context.Context, firstPlayerID string, secondPlayerID string) (HeadToHeadStats, error) {
	// TODO: Implement StatsService logic or remove if not needed. Currently a stub.
	return HeadToHeadStats{FirstPlayerWins: 0, SecondPlayerWins: 0, Draws: 0}, nil
}
