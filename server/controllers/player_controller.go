package controllers

import (
	"encoding/json"
	"log"
	"net/http"

	"battle-wordle/server/dto"
	"battle-wordle/server/services"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

// PlayerController handles HTTP requests related to players.
type PlayerController struct {
	service *services.PlayerService
}

// NewPlayerController creates a new PlayerController.
func NewPlayerController(service *services.PlayerService) *PlayerController {
	return &PlayerController{service: service}
}

func (c *PlayerController) GetPlayerByID(w http.ResponseWriter, r *http.Request) {
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
	json.NewEncoder(w).Encode(dto.MapPlayer(game))
}

func (c *PlayerController) Register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     string  `json:"name"`
		Password *string `json:"password,omitempty"`
		ID       *string `json:"id,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	player, jwtToken, err := c.service.CreatePlayer(ctx, req.Name, req.Password, req.ID)
	if err != nil {
		if err.Error() == "username_taken" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(map[string]string{"error": "Username is already taken."})
			return
		}
		log.Printf("error creating player: %v", err)
		http.Error(w, "Failed to create player", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if player.Registered && jwtToken != nil {
		json.NewEncoder(w).Encode(struct {
			Player *dto.PlayerDTO `json:"player"`
			Token  string         `json:"token"`
		}{
			Player: dto.MapPlayer(player),
			Token:  *jwtToken,
		})
	} else {
		json.NewEncoder(w).Encode(dto.MapPlayer(player))
	}
}

func (c *PlayerController) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     string `json:"name"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" || req.Password == "" {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	player, jwtToken, err := c.service.Login(ctx, req.Name, req.Password)
	if err != nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(struct {
		Player *dto.PlayerDTO `json:"player"`
		Token  string         `json:"token"`
	}{
		Player: dto.MapPlayer(player),
		Token:  *jwtToken,
	})
}

// SearchPlayers allows searching for players by (partial) name, case-insensitive
func (c *PlayerController) SearchPlayers(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" {
		http.Error(w, "Missing name query parameter", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	players, err := c.service.SearchByName(ctx, name)
	if err != nil {
		log.Printf("error searching players by name %q: %v", name, err)
		http.Error(w, "Failed to search players", http.StatusInternalServerError)
		return
	}
	// Return only id and name
	type playerResult struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	results := make([]dto.PlayerDTO, 0, len(players))
	for _, p := range players {
		mapped := dto.MapPlayer(p)
		if mapped != nil {
			results = append(results, *mapped)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}
