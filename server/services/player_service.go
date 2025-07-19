package services

import (
	"battle-wordle/server/models"
	"battle-wordle/server/repositories"
	"context"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// PlayerService provides business logic for managing players.
type PlayerService struct {
	repo      *repositories.PlayerRepository
	jwtSecret string
}

// NewPlayerService creates a new PlayerService.
func NewPlayerService(repo *repositories.PlayerRepository, jwtSecret string) *PlayerService {
	return &PlayerService{repo: repo, jwtSecret: jwtSecret}
}

func (s *PlayerService) GetByID(ctx context.Context, playerID string) (*models.Player, error) {
	return s.repo.GetByID(ctx, playerID)
}

func (s *PlayerService) CreatePlayer(ctx context.Context, name string, password *string, id *string) (*models.Player, *string, error) {
	var player *models.Player
	var jwtToken *string
	if password == nil {
		// Guest account
		player = &models.Player{
			ID:         uuid.NewString(),
			Name:       name,
			Registered: false,
			Elo:        nil,
			CreatedAt:  time.Now().UTC(),
		}
		if err := s.repo.CreatePlayer(ctx, player); err != nil {
			if err.Error() == "username_taken" {
				return nil, nil, fmt.Errorf("username_taken")
			}
			return nil, nil, err
		}
		return player, nil, nil
	} else {
		// Logged-in account (new or upgrade)
		var playerID string
		if id != nil {
			playerID = *id
			// Check if this id exists and is a guest
			existing, err := s.repo.GetByID(ctx, playerID)
			if err != nil {
				return nil, nil, err
			}
			if existing != nil && !existing.Registered {
				// Upgrade guest to registered
				// Check if new name is taken by another player
				hash, err := bcrypt.GenerateFromPassword([]byte(*password), bcrypt.DefaultCost)
				if err != nil {
					return nil, nil, err
				}
				if err := s.repo.UpdateGuestToRegistered(ctx, playerID, name, string(hash)); err != nil {
					if err.Error() == "username_taken" {
						return nil, nil, fmt.Errorf("username_taken")
					}
					return nil, nil, err
				}
				// Fetch updated player
				player, err = s.repo.GetByID(ctx, playerID)
				if err != nil {
					return nil, nil, err
				}
				token, err := s.GenerateJWT(playerID)
				if err != nil {
					return nil, nil, err
				}
				jwtToken = &token
				return player, jwtToken, nil
			}
		}
		// If id is not a guest, treat as new registration
		// Check if username is already taken
		existingByName, err := s.repo.GetByName(ctx, name)
		if err != nil {
			return nil, nil, err
		}
		if existingByName != nil && existingByName.Registered {
			return nil, nil, fmt.Errorf("username_taken")
		}
		playerID = uuid.NewString()
		hash, err := bcrypt.GenerateFromPassword([]byte(*password), bcrypt.DefaultCost)
		if err != nil {
			return nil, nil, err
		}
		player = &models.Player{
			ID:           playerID,
			Name:         name,
			Registered:   true,
			PasswordHash: ptr(string(hash)),
			Elo:          nil,
			CreatedAt:    time.Now().UTC(),
		}
		token, err := s.GenerateJWT(playerID)
		if err != nil {
			return nil, nil, err
		}
		jwtToken = &token
		if err := s.repo.CreatePlayer(ctx, player); err != nil {
			if err.Error() == "username_taken" {
				return nil, nil, fmt.Errorf("username_taken")
			}
			return nil, nil, err
		}
		return player, jwtToken, nil
	}
}

func (s *PlayerService) Login(ctx context.Context, name string, password string) (*models.Player, *string, error) {
	// Find player by name (must be registered)
	player, err := s.repo.GetByName(ctx, name)
	if err != nil || player == nil || !player.Registered || player.PasswordHash == nil {
		return nil, nil, err
	}
	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(*player.PasswordHash), []byte(password)); err != nil {
		return nil, nil, err
	}
	token, err := s.GenerateJWT(player.ID)
	if err != nil {
		return nil, nil, err
	}
	return player, &token, nil
}

// SearchByName returns players whose names contain the substring (case-insensitive)
func (s *PlayerService) SearchByName(ctx context.Context, name string) ([]*models.Player, error) {
	return s.repo.SearchByName(ctx, name)
}

func (s *PlayerService) GenerateJWT(playerID string) (string, error) {
	claims := jwt.MapClaims{
		"player_id": playerID,
		"exp":       time.Now().Add(24 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.jwtSecret))
}

func ptr[T any](v T) *T { return &v }
