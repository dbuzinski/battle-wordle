package main

import (
	"bufio"
	"fmt"
	"log"
	"net/http"
	"os"

	"battle-wordle/server/controllers"
	"battle-wordle/server/repositories"
	"battle-wordle/server/services"

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
	dbPath := "./battlewordle.db"

	// Wire up services
	gameRepository, err := repositories.NewGameRepository(dbPath)
	if err != nil {
		log.Fatalf("Failed to create game repository: %v", err)
	}
	playerRepository, err := repositories.NewPlayerRepository(dbPath)
	if err != nil {
		log.Fatalf("Failed to create player repository: %v", err)
	}

	// Load word list
	wordList, err := loadWordList("word_list.txt")
	if err != nil {
		log.Fatalf("Failed to load word list: %v", err)
	}

	gameService := services.NewGameService(gameRepository, wordList)
	playerService := services.NewPlayerService(playerRepository)
	statsService := services.NewStatsService(gameRepository, playerRepository)

	gameController := controllers.NewGameController(gameService)
	playerController := controllers.NewPlayerController(playerService)
	statsController := controllers.NewStatsController(statsService)
	wsController := controllers.NewWSController(gameService)

	// Set up routes
	r := mux.NewRouter()

	r.HandleFunc("/api/player/register", playerController.Register)
	r.HandleFunc("/api/player/login", playerController.Login)
	r.HandleFunc("/api/player/{id}", playerController.GetPlayerById)
	r.HandleFunc("/api/player/{id}/games", gameController.GetGamesByPlayer)
	r.HandleFunc("/api/stats/h2h/{first_player}/{second_player}", statsController.GetHeadToHeadStats)
	r.HandleFunc("/ws/game/{id}", wsController.HandleWebSocket)
	r.HandleFunc("/ws/matchmaking", wsController.HandleMatchmakingWebSocket)

	// Middleware
	// Define allowed CORS options
	corsHandler := handlers.CORS(
		handlers.AllowedOrigins([]string{"*"}), // Or use your frontend's origin
		handlers.AllowedMethods([]string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}),
		handlers.AllowedHeaders([]string{"Content-Type", "Authorization"}),
	)

	// Start server
	log.Println("Server is running on port 8080")
	if err := http.ListenAndServe(":8080", corsHandler(r)); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
