package controllers

import (
	"battle-wordle/server/services"
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

// WSNotificationController handles notification WebSocket connections (challenges, rematches).
type WSNotificationController struct {
	gameService        *services.GameService
	playerService      *services.PlayerService
	matchmakingService *services.MatchmakingService
	upgrader           websocket.Upgrader
}

func NewWSNotificationController(gameService *services.GameService, playerService *services.PlayerService, matchmakingService *services.MatchmakingService) *WSNotificationController {
	return &WSNotificationController{
		gameService:        gameService,
		playerService:      playerService,
		matchmakingService: matchmakingService,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin:     func(r *http.Request) bool { return true },
		},
	}
}

// HandleWebSocket handles the notification WebSocket connection.
func (c *WSNotificationController) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
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
	c.matchmakingService.RegisterConnection(playerID, conn)
	log.Printf("[notif-ws] Registered playerID=%s", playerID)
	defer c.matchmakingService.UnregisterConnection(playerID)

	for {
		_, msgData, err := conn.ReadMessage()
		if err != nil {
			return
		}
		// Parse message type
		var baseMsg struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(msgData, &baseMsg); err != nil {
			continue
		}
		switch baseMsg.Type {
		case "challenge_invite":
			var invite struct {
				Type     string `json:"type"`
				From     string `json:"from"`
				To       string `json:"to"`
				FromName string `json:"from_name"`
				Rematch  bool   `json:"rematch"`
			}
			if err := json.Unmarshal(msgData, &invite); err != nil || invite.From == "" || invite.To == "" {
				continue
			}
			log.Printf("[notif-ws] challenge_invite: from=%s to=%s from_name=%s rematch=%v", invite.From, invite.To, invite.FromName, invite.Rematch)
			// Forward to target player if online
			if targetConn, ok := c.matchmakingService.OnlineConnection(invite.To); ok {
				b, _ := json.Marshal(invite)
				log.Printf("[notif-ws] Forwarding challenge_invite to %s: %s", invite.To, string(b))
				if wsConn, ok := targetConn.(*websocket.Conn); ok {
					wsConn.WriteMessage(websocket.TextMessage, b)
				}
			} else {
				log.Printf("[notif-ws] Target player %s not online for challenge_invite", invite.To)
			}
		case "challenge_response":
			var resp struct {
				Type     string `json:"type"`
				From     string `json:"from"`
				To       string `json:"to"`
				Accepted bool   `json:"accepted"`
				Rematch  bool   `json:"rematch"`
			}
			if err := json.Unmarshal(msgData, &resp); err != nil || resp.From == "" || resp.To == "" {
				continue
			}
			log.Printf("[notif-ws] challenge_response: from=%s to=%s accepted=%v rematch=%v", resp.From, resp.To, resp.Accepted, resp.Rematch)
			var gameID *string
			if resp.Accepted {
				// Create a new game if not a rematch (for rematch, frontend may handle differently)
				game, err := c.gameService.CreateGame(r.Context(), resp.From, resp.To)
				if err == nil && game != nil {
					gid := game.ID
					gameID = &gid
					log.Printf("[notif-ws] Created game for challenge: id=%s", game.ID)
				} else {
					log.Printf("[notif-ws] Failed to create game for challenge: %v", err)
				}
			}
			// Send challenge_result to both players
			result := struct {
				Type     string  `json:"type"`
				Accepted bool    `json:"accepted"`
				GameID   *string `json:"game_id,omitempty"`
				From     string  `json:"from"`
				To       string  `json:"to"`
				Rematch  bool    `json:"rematch"`
			}{
				Type:     "challenge_result",
				Accepted: resp.Accepted,
				GameID:   gameID,
				From:     resp.From,
				To:       resp.To,
				Rematch:  resp.Rematch,
			}
			b, _ := json.Marshal(result)
			for _, pid := range []string{resp.From, resp.To} {
				if targetConn, ok := c.matchmakingService.OnlineConnection(pid); ok {
					log.Printf("[notif-ws] Sending challenge_result to %s: %s", pid, string(b))
					if wsConn, ok := targetConn.(*websocket.Conn); ok {
						wsConn.WriteMessage(websocket.TextMessage, b)
					}
				} else {
					log.Printf("[notif-ws] Target player %s not online for challenge_result", pid)
				}
			}
		case "rematch_offer":
			var offer struct {
				Type     string `json:"type"`
				From     string `json:"from"`
				To       string `json:"to"`
				FromName string `json:"from_name"`
			}
			if err := json.Unmarshal(msgData, &offer); err != nil || offer.From == "" || offer.To == "" {
				continue
			}
			if targetConn, ok := c.matchmakingService.OnlineConnection(offer.To); ok {
				b, _ := json.Marshal(offer)
				if wsConn, ok := targetConn.(*websocket.Conn); ok {
					wsConn.WriteMessage(websocket.TextMessage, b)
				}
			}
		case "rematch_response":
			var resp struct {
				Type     string `json:"type"`
				From     string `json:"from"`
				To       string `json:"to"`
				Accepted bool   `json:"accepted"`
				FromName string `json:"from_name"`
			}
			if err := json.Unmarshal(msgData, &resp); err != nil || resp.From == "" || resp.To == "" {
				continue
			}
			if targetConn, ok := c.matchmakingService.OnlineConnection(resp.To); ok {
				b, _ := json.Marshal(resp)
				if wsConn, ok := targetConn.(*websocket.Conn); ok {
					wsConn.WriteMessage(websocket.TextMessage, b)
				}
			}
		}
	}
}
