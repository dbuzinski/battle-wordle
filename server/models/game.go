package models

import "time"

// Game represents a two-player Wordle game session.
type Game struct {
	// ID is the unique identifier for the game.
	ID string `json:"id"`
	// Solution is the word to be guessed.
	Solution string `json:"solution"`
	// CreatedAt is the timestamp when the game was created.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt is the timestamp when the game was last updated.
	UpdatedAt time.Time `json:"updated_at"`
	// FirstPlayer is the ID of the first player.
	FirstPlayer string `json:"first_player"`
	// SecondPlayer is the ID of the second player.
	SecondPlayer string `json:"second_player"`
	// CurrentPlayer is the ID of the player whose turn it is.
	CurrentPlayer string `json:"current_player"`
	// Result is the outcome of the game (e.g., "draw", "lose:<playerID>").
	Result string `json:"result"`
	// Guesses is the list of guesses made in the game.
	Guesses []string `json:"guesses"`
}
