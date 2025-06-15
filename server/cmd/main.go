package main

import (
	"log"
	"net/http"

	"battle-wordle/server/internal/database"
	"battle-wordle/server/internal/game"
	"battle-wordle/server/internal/handlers"
)

func main() {
	// Initialize database
	db, err := database.New("./game.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Initialize services
	gameService, err := game.NewService(db.DB)
	if err != nil {
		log.Fatal(err)
	}

	matchmakingService := game.NewMatchmakingService(gameService)

	// Initialize handlers
	wsHandler := handlers.NewWebSocketHandler(gameService, matchmakingService)
	httpHandler := handlers.NewHTTPHandler(gameService)

	// Set up routes
	http.HandleFunc("/ws", wsHandler.HandleConnection)
	http.HandleFunc("/api/stats", httpHandler.HandleStats)
	http.HandleFunc("/api/set-player-name", httpHandler.HandleSetPlayerName)
	http.HandleFunc("/api/recent-games", httpHandler.HandleRecentGames)
	http.HandleFunc("/api/head-to-head-stats", httpHandler.HandleHeadToHeadStats)

	// Start server
	log.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
