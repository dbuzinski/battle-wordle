package services

import (
	"context"

	"battle-wordle/server/repositories"
)

type StatsService struct {
	gameRepo   *repositories.GameRepository
	playerRepo *repositories.PlayerRepository
}

type HeadToHeadStats struct {
	FirstPlayerWins  int `json:"first_player_wins"`
	SecondPlayerWins int `json:"second_player_wins"`
	Draws            int `json:"draws"`
}

func NewStatsService(gameRepo *repositories.GameRepository, playerRepo *repositories.PlayerRepository) *StatsService {
	return &StatsService{gameRepo: gameRepo, playerRepo: playerRepo}
}

func (s *StatsService) GetHeadToHeadStats(ctx context.Context, firstPlayerID string, secondPlayerID string) (HeadToHeadStats, error) {
	return HeadToHeadStats{FirstPlayerWins: 0, SecondPlayerWins: 0, Draws: 0}, nil
}
