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

// GameController handles HTTP requests related to games.
type GameController struct {
	service       *services.GameService
	playerService *services.PlayerService
}

// NewGameController creates a new GameController.
func NewGameController(service *services.GameService, playerService *services.PlayerService) *GameController {
	return &GameController{service: service, playerService: playerService}
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

	firstPlayer, _ := c.playerService.GetByID(ctx, game.FirstPlayer)
	secondPlayer, _ := c.playerService.GetByID(ctx, game.SecondPlayer)
	feedbacks := c.service.GetFeedbacks(game)
	feedbackStrings := make([][]string, len(feedbacks))
	for i, fb := range feedbacks {
		feedbackStrings[i] = make([]string, len(fb))
		for j, f := range fb {
			feedbackStrings[i][j] = string(f)
		}
	}
	var solutionPtr *string
	if game.Result != "" {
		solutionPtr = &game.Solution
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dto.MapGame(game, firstPlayer, secondPlayer, feedbackStrings, solutionPtr))
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

	var dtos []dto.GameDTO
	for _, game := range games {
		firstPlayer, _ := c.playerService.GetByID(ctx, game.FirstPlayer)
		secondPlayer, _ := c.playerService.GetByID(ctx, game.SecondPlayer)
		feedbacks := c.service.GetFeedbacks(game)
		feedbackStrings := make([][]string, len(feedbacks))
		for i, fb := range feedbacks {
			feedbackStrings[i] = make([]string, len(fb))
			for j, f := range fb {
				feedbackStrings[i][j] = string(f)
			}
		}
		var solutionPtr *string
		if game.Result != "" {
			solutionPtr = &game.Solution
		}
		dtos = append(dtos, *dto.MapGame(game, firstPlayer, secondPlayer, feedbackStrings, solutionPtr))
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dtos)
}

func (c *GameController) CreateGame(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PlayerOne string `json:"player_one"`
		PlayerTwo string `json:"player_two"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.PlayerOne == "" || req.PlayerTwo == "" {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	game, err := c.service.CreateGame(ctx, req.PlayerOne, req.PlayerTwo)
	if err != nil {
		log.Printf("error creating game: %v", err)
		http.Error(w, "Failed to create game", http.StatusInternalServerError)
		return
	}
	firstPlayer, _ := c.playerService.GetByID(ctx, game.FirstPlayer)
	secondPlayer, _ := c.playerService.GetByID(ctx, game.SecondPlayer)
	feedbacks := c.service.GetFeedbacks(game)
	feedbackStrings := make([][]string, len(feedbacks))
	for i, fb := range feedbacks {
		feedbackStrings[i] = make([]string, len(fb))
		for j, f := range fb {
			feedbackStrings[i][j] = string(f)
		}
	}
	var solutionPtr *string
	if game.Result != "" {
		solutionPtr = &game.Solution
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dto.MapGame(game, firstPlayer, secondPlayer, feedbackStrings, solutionPtr))
}

func (c *GameController) SubmitGuess(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	gameID := vars["id"]
	ctx := r.Context()
	var req struct {
		Guess    string `json:"guess"`
		PlayerID string `json:"player_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Guess == "" || req.PlayerID == "" {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	game, err := c.service.SubmitGuess(ctx, gameID, req.Guess, req.PlayerID)
	if err != nil {
		log.Printf("error submitting guess: %v", err)
		http.Error(w, "Failed to submit guess", http.StatusInternalServerError)
		return
	}
	firstPlayer, _ := c.playerService.GetByID(ctx, game.FirstPlayer)
	secondPlayer, _ := c.playerService.GetByID(ctx, game.SecondPlayer)
	feedbacks := c.service.GetFeedbacks(game)
	feedbackStrings := make([][]string, len(feedbacks))
	for i, fb := range feedbacks {
		feedbackStrings[i] = make([]string, len(fb))
		for j, f := range fb {
			feedbackStrings[i][j] = string(f)
		}
	}
	var solutionPtr *string
	if game.Result != "" {
		solutionPtr = &game.Solution
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dto.MapGame(game, firstPlayer, secondPlayer, feedbackStrings, solutionPtr))
}
