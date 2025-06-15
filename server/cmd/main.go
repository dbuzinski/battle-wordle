package main

import (
	"log"
	"net/http"

	"battle-wordle/server/internal/config"
	"battle-wordle/server/internal/database"
	"battle-wordle/server/internal/game"
	"battle-wordle/server/internal/handlers"
)

func main() {
	// Load configuration
	if err := config.Load(); err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}
	cfg := config.Get()

	// Initialize database
	db, err := database.New(cfg.Server.DbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Initialize services
	gameService, err := game.NewService(db.DB)
	if err != nil {
		log.Fatalf("Failed to initialize game service: %v", err)
	}

	matchmakingService := game.NewMatchmakingService(gameService)

	// Initialize handlers
	httpHandler := handlers.NewHTTPHandler(gameService, cfg)
	wsHandler := handlers.NewWebSocketHandler(gameService, matchmakingService, cfg)

	// Set up routes
	http.HandleFunc("/api/set-player-name", httpHandler.HandleSetPlayerName)
	http.HandleFunc("/api/recent-games", httpHandler.HandleRecentGames)
	http.HandleFunc("/api/stats", httpHandler.HandleStats)
	http.HandleFunc("/ws", wsHandler.HandleConnection)

	// Start server
	log.Printf("Starting server on port %d", cfg.Server.Port)
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
