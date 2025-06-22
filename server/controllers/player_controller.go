package controllers

import (
	"encoding/json"
	"log"
	"net/http"

	"battle-wordle/server/services"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

type PlayerController struct {
	service *services.PlayerService
}

func NewPlayerController(service *services.PlayerService) *PlayerController {
	return &PlayerController{service: service}
}

func (c *PlayerController) GetPlayerById(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	playerID := vars["id"]
	ctx := r.Context()

	// Validate uuid
	if err := uuid.Validate(playerID); err != nil {
		log.Printf("invalid player id %s: %v", playerID, err)
		http.Error(w, "Games not found", http.StatusNotFound)
		return
	}

	game, err := c.service.GetByID(ctx, playerID)
	if err != nil {
		log.Printf("error fetching player %q: %v", playerID, err)
		http.Error(w, "Player not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(game)
}

func (c *PlayerController) Register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	player, err := c.service.CreatePlayer(ctx, req.Name)
	if err != nil {
		log.Printf("error creating player: %v", err)
		http.Error(w, "Failed to create player", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(player)
}
