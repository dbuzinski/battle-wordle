package controllers

import (
	"battle-wordle/server/services"
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

type StatsController struct {
	service *services.StatsService
}

func NewStatsController(service *services.StatsService) *StatsController {
	return &StatsController{service: service}
}

func (c *StatsController) GetHeadToHeadStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	firstPlayerID := vars["first_player"]
	secondPlayerID := vars["second_player"]

	if firstPlayerID == "" || secondPlayerID == "" {
		http.Error(w, "Missing required parameters: first_player and second_player", http.StatusBadRequest)
		return
	}

	stats, err := c.service.GetHeadToHeadStats(ctx, firstPlayerID, secondPlayerID)
	if err != nil {
		log.Printf("error fetching head to head stats for players %q and %q: %v", firstPlayerID, secondPlayerID, err)
		http.Error(w, "Stats not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}
