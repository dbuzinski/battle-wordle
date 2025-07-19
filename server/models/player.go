package models

import "time"

// Player represents a user in the system.
type Player struct {
	// ID is the unique identifier for the player.
	ID string `json:"id"`
	// Name is the player's display name.
	Name string `json:"name"`
	// Registered indicates if the player has a registered account.
	Registered bool `json:"registered"`
	// PasswordHash is the bcrypt hash of the player's password (if registered).
	PasswordHash *string `json:"-"`
	// Elo is the player's rating (optional).
	Elo *int `json:"elo,omitempty"`
	// CreatedAt is the timestamp when the player was created.
	CreatedAt time.Time `json:"created_at"`
}
