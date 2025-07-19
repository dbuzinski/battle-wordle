package models

type GameMode string

const (
	AntiWordleBattle  GameMode = "anti_wordle_battle"
	SpeedWordleBattle GameMode = "speed_wordle_battle"
	SpeedWordleTrial  GameMode = "speed_wordle_trial"
)

type LobbyState string

const (
	LobbyStateWaiting LobbyState = "waiting"
	LobbyStatePlaying LobbyState = "playing"
)

type Lobby struct {
	ID       string     `json:"id"`
	Players  []Player   `json:"players"`
	GameMode GameMode   `json:"game_mode"`
	State    LobbyState `json:"state"`
}
