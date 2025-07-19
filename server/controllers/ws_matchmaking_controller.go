package controllers

import (
	"battle-wordle/server/services"
	"encoding/json"
	"net/http"

	"github.com/gorilla/websocket"
)

// WSMatchmakingController handles matchmaking WebSocket connections.
type WSMatchmakingController struct {
	matchmakingService *services.MatchmakingService
	upgrader           websocket.Upgrader
}

func NewWSMatchmakingController(matchmakingService *services.MatchmakingService) *WSMatchmakingController {
	return &WSMatchmakingController{
		matchmakingService: matchmakingService,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin:     func(r *http.Request) bool { return true },
		},
	}
}

// HandleWebSocket handles the matchmaking WebSocket connection.
func (c *WSMatchmakingController) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := c.upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, "Could not upgrade to WebSocket", http.StatusBadRequest)
		return
	}
	defer conn.Close()

	// Read player ID from first message
	_, msg, err := conn.ReadMessage()
	if err != nil {
		return
	}
	var joinMsg struct {
		Type     string `json:"type"`
		PlayerID string `json:"player_id"`
	}
	if err := json.Unmarshal(msg, &joinMsg); err != nil || joinMsg.Type != "join" || joinMsg.PlayerID == "" {
		return
	}

	playerID := joinMsg.PlayerID
	c.matchmakingService.JoinQueue(playerID, conn)

	// Try to match
	if gameID, conns, ok := c.matchmakingService.TryMatch(r.Context()); ok {
		resp := struct {
			Type   string `json:"type"`
			GameID string `json:"game_id"`
		}{Type: "match_found", GameID: gameID}
		b, _ := json.Marshal(resp)
		for _, c := range conns {
			if wsConn, ok := c.(*websocket.Conn); ok {
				wsConn.WriteMessage(websocket.TextMessage, b)
				wsConn.Close()
			}
		}
		return
	}

	// Wait for match or disconnect
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			// Remove from queue if still present
			c.matchmakingService.LeaveQueue(playerID)
			return
		}
	}
}
