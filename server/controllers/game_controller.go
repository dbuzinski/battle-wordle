package controllers

import (
	"encoding/json"
	"log"
	"net/http"

	"battle-wordle/server/services"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

type GameController struct {
	service *services.GameService
}

func NewGameController(service *services.GameService) *GameController {
	return &GameController{service: service}
}

func (c *GameController) GetGameByID(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	gameID := vars["id"]
	ctx := r.Context()

	// Validate uuid
	if err := uuid.Validate(gameID); err != nil {
		log.Printf("invalid game id %s: %v", gameID, err)
		http.Error(w, "Game not found", http.StatusNotFound)
		return
	}

	game, err := c.service.GetByID(ctx, gameID)

	if err != nil {
		log.Printf("error fetching game %q: %v", gameID, err)
		http.Error(w, "Game not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(game)
}

func (c *GameController) GetGamesByPlayer(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	playerID := vars["id"]
	ctx := r.Context()

	// Validate uuid
	if err := uuid.Validate(playerID); err != nil {
		log.Printf("invalid player id %s: %v", playerID, err)
		http.Error(w, "Games not found", http.StatusNotFound)
		return
	}

	games, err := c.service.GetByPlayer(ctx, playerID)

	if err != nil {
		log.Printf("error fetching game for player %q: %v", playerID, err)
		http.Error(w, "Games not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(games)
}
