package models

type Player struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Registered   bool    `json:"registered"`
	PasswordHash *string `json:"-"`
	Elo          *int    `json:"elo,omitempty"`
	CreatedAt    string  `json:"created_at"`
}
