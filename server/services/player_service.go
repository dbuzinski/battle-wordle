package services

import (
	"context"

	"battle-wordle/server/models"
	"battle-wordle/server/repositories"

	"github.com/google/uuid"
)

type PlayerService struct {
	repo *repositories.PlayerRepository
}

func NewPlayerService(repo *repositories.PlayerRepository) *PlayerService {
	return &PlayerService{repo: repo}
}

func (s *PlayerService) GetByID(ctx context.Context, playerID string) (*models.Player, error) {
	return s.repo.GetByID(ctx, playerID)
}

func (s *PlayerService) CreatePlayer(ctx context.Context, name string) (*models.Player, error) {
	player := &models.Player{
		ID:   uuid.NewString(),
		Name: name,
	}

	if err := s.repo.CreatePlayer(ctx, player); err != nil {
		return nil, err
	}
	return player, nil
}
