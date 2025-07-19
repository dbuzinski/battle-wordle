package main

import (
	"bufio"
	"fmt"
	"log"
	"net/http"
	"os"

	"battle-wordle/server/config"
	"battle-wordle/server/controllers"
	"battle-wordle/server/repositories"
	"battle-wordle/server/services"
	"battle-wordle/server/ws"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

func loadWordList(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	wordList := make([]string, 0)
	for scanner.Scan() {
		word := scanner.Text()
		if word != "" {
			wordList = append(wordList, word)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(wordList) == 0 {
		return nil, fmt.Errorf("word list is empty")
	}
	return wordList, nil
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Wire up services using cfg.DBPath, cfg.JWTSecret, cfg.Port
	gameRepository, err := repositories.NewGameRepository(cfg.DBPath)
	if err != nil {
		log.Fatalf("Failed to create game repository: %v", err)
	}
	playerRepository, err := repositories.NewPlayerRepository(cfg.DBPath)
	if err != nil {
		log.Fatalf("Failed to create player repository: %v", err)
	}

	// Load word list
	wordList, err := loadWordList("word_list.txt")
	if err != nil {
		log.Fatalf("Failed to load word list: %v", err)
	}

	// Pass cfg.JWTSecret to player service (update player_service.go to accept it)
	gameService := services.NewGameService(gameRepository, wordList)
	playerService := services.NewPlayerService(playerRepository, cfg.JWTSecret)
	statsService := services.NewStatsService(gameRepository, playerRepository)
	matchmakingService := services.NewMatchmakingService(gameService)

	gameController := controllers.NewGameController(gameService)
	playerController := controllers.NewPlayerController(playerService)
	statsController := controllers.NewStatsController(statsService)
	gameHub := ws.NewHub()
	wsGameController := controllers.NewWSGameController(gameService, playerService, gameHub)
	wsMatchmakingController := controllers.NewWSMatchmakingController(matchmakingService)
	wsNotificationController := controllers.NewWSNotificationController(gameService, playerService, matchmakingService)

	// Set up routes
	r := mux.NewRouter()

	r.HandleFunc("/api/player/register", playerController.Register)
	r.HandleFunc("/api/player/login", playerController.Login)
	r.HandleFunc("/api/player/search", playerController.SearchPlayers)
	r.HandleFunc("/api/player/{id}", playerController.GetPlayerByID)
	r.HandleFunc("/api/player/{id}/games", gameController.GetGamesByPlayer)
	r.HandleFunc("/api/stats/h2h/{first_player}/{second_player}", statsController.GetHeadToHeadStats)
	r.HandleFunc("/ws/game/{id}", wsGameController.HandleWebSocket)
	r.HandleFunc("/ws/matchmaking", wsMatchmakingController.HandleWebSocket)
	r.HandleFunc("/ws/notifications", wsNotificationController.HandleWebSocket)

	// Middleware
	// Define allowed CORS options
	corsHandler := handlers.CORS(
		handlers.AllowedOrigins([]string{"*"}), // Or use your frontend's origin
		handlers.AllowedMethods([]string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}),
		handlers.AllowedHeaders([]string{"Content-Type", "Authorization"}),
	)

	// Start server
	log.Printf("Server is running on port %s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, corsHandler(r)); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
