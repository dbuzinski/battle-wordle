package models

import "errors"

var (
	ErrGameNotFound       = errors.New("game not found")
	ErrGameOver           = errors.New("game is over")
	ErrNotYourTurn        = errors.New("not your turn")
	ErrWaitingForOpponent = errors.New("waiting for opponent")
)
