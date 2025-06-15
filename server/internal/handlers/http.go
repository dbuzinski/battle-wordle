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

// allowedOrigins is a list of allowed origins for CORS
var allowedOrigins = []string{
	"https://battlewordle.app",
	"https://www.battlewordle.app",
}

// corsMiddleware adds CORS headers to the response
func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		for _, allowedOrigin := range allowedOrigins {
			if origin == allowedOrigin {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				break
			}
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next(w, r)
	}
}

// HandleStats handles player stats requests
func (h *HTTPHandler) HandleStats(w http.ResponseWriter, r *http.Request) {
	corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
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
	})(w, r)
}

// HandleSetPlayerName handles setting player names
func (h *HTTPHandler) HandleSetPlayerName(w http.ResponseWriter, r *http.Request) {
	corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
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
			log.Printf("Error setting name for player %s: %v", req.PlayerId, err)
			http.Error(w, "Error setting player name", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	})(w, r)
}

// HandleRecentGames handles recent games requests
func (h *HTTPHandler) HandleRecentGames(w http.ResponseWriter, r *http.Request) {
	corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
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
			log.Printf("Error fetching recent games for player %s: %v", playerId, err)
			http.Error(w, "Error fetching recent games", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(games); err != nil {
			log.Printf("Error encoding games to JSON: %v", err)
			http.Error(w, "Error encoding response", http.StatusInternalServerError)
			return
		}
	})(w, r)
}

// HandleHeadToHeadStats handles head-to-head stats requests
func (h *HTTPHandler) HandleHeadToHeadStats(w http.ResponseWriter, r *http.Request) {
	corsMiddleware(func(w http.ResponseWriter, r *http.Request) {
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
			log.Printf("Error fetching head-to-head stats for players %s vs %s: %v", playerId, opponentId, err)
			http.Error(w, "Error fetching head-to-head stats", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]int{
			"wins":   wins,
			"losses": losses,
			"draws":  draws,
		})
	})(w, r)
}
