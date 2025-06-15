package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"

	"battle-wordle/server/internal/game"
)

// HTTPHandler handles HTTP endpoints
type HTTPHandler struct {
	gameService *game.Service
}

// NewHTTPHandler creates a new HTTP handler
func NewHTTPHandler(gameService *game.Service) *HTTPHandler {
	return &HTTPHandler{
		gameService: gameService,
	}
}

// HandleStats handles player stats requests
func (h *HTTPHandler) HandleStats(w http.ResponseWriter, r *http.Request) {
	// Add CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	// Handle preflight requests
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	playerId := r.URL.Query().Get("playerId")
	if playerId == "" {
		http.Error(w, "Player ID is required", http.StatusBadRequest)
		return
	}

	wins, losses, draws, err := h.gameService.GetPlayerStats(playerId)
	if err != nil {
		if err == sql.ErrNoRows {
			// Return zero stats for new players
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]int{
				"wins":   0,
				"losses": 0,
				"draws":  0,
			})
			return
		}
		http.Error(w, "Error fetching player stats", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{
		"wins":   wins,
		"losses": losses,
		"draws":  draws,
	})
}

// HandleSetPlayerName handles setting player names
func (h *HTTPHandler) HandleSetPlayerName(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		PlayerId   string `json:"playerId"`
		PlayerName string `json:"playerName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if err := h.gameService.SetPlayerName(req.PlayerId, req.PlayerName); err != nil {
		http.Error(w, "Error setting player name", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// HandleRecentGames handles recent games requests
func (h *HTTPHandler) HandleRecentGames(w http.ResponseWriter, r *http.Request) {
	// Add CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	// Handle preflight requests
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	playerId := r.URL.Query().Get("playerId")
	if playerId == "" {
		http.Error(w, "Player ID is required", http.StatusBadRequest)
		return
	}

	games, err := h.gameService.GetRecentGames(playerId)
	if err != nil {
		http.Error(w, "Error fetching recent games", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(games); err != nil {
		log.Printf("Error encoding games to JSON: %v", err)
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
		return
	}
}

// HandleHeadToHeadStats handles head-to-head stats requests
func (h *HTTPHandler) HandleHeadToHeadStats(w http.ResponseWriter, r *http.Request) {
	// Add CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	// Handle preflight requests
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	playerId := r.URL.Query().Get("playerId")
	opponentId := r.URL.Query().Get("opponentId")
	if playerId == "" || opponentId == "" {
		http.Error(w, "Player ID and Opponent ID are required", http.StatusBadRequest)
		return
	}

	wins, losses, draws, err := h.gameService.GetHeadToHeadStats(playerId, opponentId)
	if err != nil {
		http.Error(w, "Error fetching head-to-head stats", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{
		"wins":   wins,
		"losses": losses,
		"draws":  draws,
	})
}
