package models

import "time"

type Game struct {
	ID            string    `json:"id"`
	Solution      string    `json:"solution"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	FirstPlayer   string    `json:"first_player"`
	SecondPlayer  string    `json:"second_player"`
	CurrentPlayer string    `json:"current_player"`
	Result        string    `json:"result"`
	Guesses       []string  `json:"guesses"`
}
