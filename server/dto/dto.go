package dto

import (
	"battle-wordle/server/models"
)

// PlayerDTO is the API-safe representation of a player.
type PlayerDTO struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Registered bool   `json:"registered"`
	Elo        *int   `json:"elo,omitempty"`
	CreatedAt  string `json:"created_at"`
}

// GameDTO is the API-safe representation of a game.
type GameDTO struct {
	ID            string           `json:"id"`
	CreatedAt     string           `json:"created_at"`
	UpdatedAt     string           `json:"updated_at"`
	FirstPlayer   PlayerSummaryDTO `json:"first_player"`
	SecondPlayer  PlayerSummaryDTO `json:"second_player"`
	CurrentPlayer string           `json:"current_player"`
	Result        string           `json:"result"`
	Guesses       []string         `json:"guesses"`
	Feedback      [][]string       `json:"feedback"`
	Solution      *string          `json:"solution,omitempty"`
}

type PlayerSummaryDTO struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// MapPlayer maps a Player model to a PlayerDTO.
func MapPlayer(p *models.Player) *PlayerDTO {
	if p == nil {
		return nil
	}
	return &PlayerDTO{
		ID:         p.ID,
		Name:       p.Name,
		Registered: p.Registered,
		Elo:        p.Elo,
		CreatedAt:  p.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

// MapGame maps a Game model to a GameDTO. Requires player lookup for names.
func MapGame(game *models.Game, firstPlayer *models.Player, secondPlayer *models.Player, feedback [][]string, solution *string) *GameDTO {
	return &GameDTO{
		ID:            game.ID,
		CreatedAt:     game.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:     game.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		FirstPlayer:   PlayerSummaryDTO{ID: game.FirstPlayer, Name: getName(firstPlayer)},
		SecondPlayer:  PlayerSummaryDTO{ID: game.SecondPlayer, Name: getName(secondPlayer)},
		CurrentPlayer: game.CurrentPlayer,
		Result:        game.Result,
		Guesses:       game.Guesses,
		Feedback:      feedback,
		Solution:      solution,
	}
}

func getName(p *models.Player) string {
	if p != nil {
		return p.Name
	}
	return "Unknown"
}
