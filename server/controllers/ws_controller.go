package controllers

import (
	"battle-wordle/server/models"
	"battle-wordle/server/services"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

const (
	JOIN_MSG  = "join"
	GUESS_MSG = "guess"
)

type WSMessage struct {
	Type     string `json:"type"`
	Guess    string `json:"guess,omitempty"`
	PlayerID string `json:"player_id,omitempty"`
}

type PlayerSummary struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type GameStateMessage struct {
	Type          string        `json:"type"`
	ID            string        `json:"id"`
	CreatedAt     string        `json:"created_at"`
	UpdatedAt     string        `json:"updated_at"`
	FirstPlayer   PlayerSummary `json:"first_player"`
	SecondPlayer  PlayerSummary `json:"second_player"`
	CurrentPlayer string        `json:"current_player"`
	Result        string        `json:"result"`
	Guesses       []string      `json:"guesses"`
	Feedback      [][]string    `json:"feedback"`
	Solution      *string       `json:"solution,omitempty"`
}

type WSController struct {
	gameService        *services.GameService
	playerService      *services.PlayerService
	matchmakingService *services.MatchmakingService
	upgrader           websocket.Upgrader

	connections map[string]map[*websocket.Conn]bool
	mu          sync.RWMutex
}

func NewWSController(gameService *services.GameService, playerService *services.PlayerService, matchmakingService *services.MatchmakingService) *WSController {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     func(r *http.Request) bool { return true },
	}

	return &WSController{
		gameService:        gameService,
		playerService:      playerService,
		matchmakingService: matchmakingService,
		upgrader:           upgrader,
		connections:        make(map[string]map[*websocket.Conn]bool),
	}
}

func (ws *WSController) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	gameID := vars["id"]
	ctx := r.Context()

	conn, err := ws.upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, "Could not upgrade to WebSocket", http.StatusBadRequest)
		return
	}

	// Register connection (no log)
	ws.mu.Lock()
	if _, ok := ws.connections[gameID]; !ok {
		ws.connections[gameID] = make(map[*websocket.Conn]bool)
	}
	ws.connections[gameID][conn] = true
	ws.mu.Unlock()

	// Do NOT register game socket for notifications

	defer func() {
		// No log for normal close
		conn.Close()
		ws.mu.Lock()
		delete(ws.connections[gameID], conn)
		if len(ws.connections[gameID]) == 0 {
			delete(ws.connections, gameID)
		}
		ws.mu.Unlock()
		// Do NOT unregister notification connection here
	}()

	for {
		_, msgData, err := conn.ReadMessage()
		if err != nil {
			// Only log unexpected errors (not normal close 1000 or 1006)
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket unexpected close for game %s: %v", gameID, err)
			}
			break
		}

		var msg WSMessage
		if err := json.Unmarshal(msgData, &msg); err != nil {
			log.Printf("Invalid JSON: %v", err)
			continue
		}

		// --- Challenge and Rematch Handling ---
		switch msg.Type {
		case "challenge_invite":
			var invite ChallengeInviteMessage
			if err := json.Unmarshal(msgData, &invite); err != nil || invite.From == "" || invite.To == "" {
				continue
			}
			delivered := ws.matchmakingService.SendChallengeInvite(invite.From, invite.To)
			if delivered {
				// Get challenger display name
				challengerName := invite.From
				if p, err := ws.playerService.GetByID(ctx, invite.From); err == nil && p != nil {
					challengerName = p.Name
				}
				invite := ChallengeInviteMessage{Type: "challenge_invite", From: invite.From, FromName: challengerName, Rematch: invite.Rematch, To: invite.To}
				b, _ := json.Marshal(invite)
				if challengedConn, ok := ws.matchmakingService.OnlineConnection(invite.To); ok {
					log.Printf("[debug] challengedConn type: %T", challengedConn)
					if wsConn, ok := challengedConn.(*websocket.Conn); ok {
						wsConn.WriteMessage(websocket.TextMessage, b)
					} else {
						log.Printf("[debug] challengedConn is not a *websocket.Conn")
					}
				} else {
					log.Printf("[challenge] No notification connection found for player %s", invite.To)
				}
			}
		case "challenge_response":
			var resp ChallengeResponseMessage
			if err := json.Unmarshal(msgData, &resp); err != nil || resp.From == "" || resp.To == "" {
				continue
			}
			gameID, _, _, accepted := ws.matchmakingService.AcceptChallenge(ctx, resp.From, resp.Accepted)
			result := struct {
				Type     string  `json:"type"`
				Accepted bool    `json:"accepted"`
				GameID   *string `json:"game_id,omitempty"`
				From     string  `json:"from"`
				To       string  `json:"to"`
				Rematch  bool    `json:"rematch"`
			}{
				Type:     "challenge_result",
				Accepted: accepted,
				GameID:   nil,
				From:     resp.To,   // challenger
				To:       resp.From, // challenged
				Rematch:  false,
			}
			if accepted && gameID != "" {
				result.GameID = &gameID
			}
			b, _ := json.Marshal(result)
			// Notify both players via their notification WebSocket
			for _, pid := range []string{resp.From, resp.To} {
				if notifConn, ok := ws.matchmakingService.OnlineConnection(pid); ok {
					log.Printf("[challenge] Sending challenge_result to %s (game_id=%v, accepted=%v)", pid, gameID, accepted)
					if wsConn, ok := notifConn.(*websocket.Conn); ok {
						wsConn.WriteMessage(websocket.TextMessage, b)
					}
				}
			}
		default:
			// No log for every message received

			switch msg.Type {
			case JOIN_MSG:
				game, err := ws.gameService.GetByID(ctx, gameID)
				if err != nil {
					log.Printf("Failed to get game for JOIN: %v", err)
					continue
				}
				ws.sendGameState(conn, game)

			case GUESS_MSG:
				game, err := ws.gameService.GetByID(ctx, gameID)
				if err != nil || game == nil {
					log.Printf("Failed to get game for GUESS: %v", err)
					continue
				}
				if msg.PlayerID != game.CurrentPlayer {
					// Not this player's turn, ignore
					continue
				}
				updatedGame, err := ws.gameService.SubmitGuess(ctx, gameID, msg.Guess, msg.PlayerID)
				if err != nil {
					log.Printf("Failed to submit guess: %v", err)
					continue
				}
				ws.broadcastGameState(gameID, updatedGame)

			default:
				log.Printf("Unknown message type: %s", msg.Type)
			}
		}
	}
}

// sendGameState sends the game state to a single connection, omitting the solution unless the game is over, and including feedback.
func (ws *WSController) sendGameState(conn *websocket.Conn, game *models.Game) {
	ctx := context.Background()
	firstPlayer, err1 := ws.playerService.GetByID(ctx, game.FirstPlayer)
	secondPlayer, err2 := ws.playerService.GetByID(ctx, game.SecondPlayer)
	getName := func(p *models.Player, err error) string {
		if err == nil && p != nil {
			return p.Name
		}
		return "Unknown"
	}
	feedbacks := ws.gameService.GetFeedbacks(game)
	feedbackStrings := make([][]string, len(feedbacks))
	for i, fb := range feedbacks {
		feedbackStrings[i] = make([]string, len(fb))
		for j, f := range fb {
			feedbackStrings[i][j] = string(f)
		}
	}
	var solutionPtr *string
	if game.Result != "" {
		solutionPtr = &game.Solution
	}
	msg := GameStateMessage{
		Type:          "game_state",
		ID:            game.ID,
		CreatedAt:     game.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:     game.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		FirstPlayer:   PlayerSummary{ID: game.FirstPlayer, Name: getName(firstPlayer, err1)},
		SecondPlayer:  PlayerSummary{ID: game.SecondPlayer, Name: getName(secondPlayer, err2)},
		CurrentPlayer: game.CurrentPlayer,
		Result:        game.Result,
		Guesses:       game.Guesses,
		Feedback:      feedbackStrings,
		Solution:      solutionPtr,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Failed to marshal game state: %v", err)
		return
	}
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		log.Printf("Failed to send game state to client: %v", err)
	}
}

// broadcastGameState sends the game state to all connections for a game, omitting the solution unless the game is over, and including feedback.
func (ws *WSController) broadcastGameState(gameID string, game *models.Game) {
	ctx := context.Background()
	firstPlayer, err1 := ws.playerService.GetByID(ctx, game.FirstPlayer)
	secondPlayer, err2 := ws.playerService.GetByID(ctx, game.SecondPlayer)
	getName := func(p *models.Player, err error) string {
		if err == nil && p != nil {
			return p.Name
		}
		return "Unknown"
	}
	feedbacks := ws.gameService.GetFeedbacks(game)
	feedbackStrings := make([][]string, len(feedbacks))
	for i, fb := range feedbacks {
		feedbackStrings[i] = make([]string, len(fb))
		for j, f := range fb {
			feedbackStrings[i][j] = string(f)
		}
	}
	var solutionPtr *string
	if game.Result != "" {
		solutionPtr = &game.Solution
	}
	msg := GameStateMessage{
		Type:          "game_state",
		ID:            game.ID,
		CreatedAt:     game.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:     game.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		FirstPlayer:   PlayerSummary{ID: game.FirstPlayer, Name: getName(firstPlayer, err1)},
		SecondPlayer:  PlayerSummary{ID: game.SecondPlayer, Name: getName(secondPlayer, err2)},
		CurrentPlayer: game.CurrentPlayer,
		Result:        game.Result,
		Guesses:       game.Guesses,
		Feedback:      feedbackStrings,
		Solution:      solutionPtr,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Failed to marshal game state: %v", err)
		return
	}

	ws.mu.RLock()
	conns := ws.connections[gameID]
	ws.mu.RUnlock()

	for conn := range conns {
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			log.Printf("Broadcast failed, cleaning up connection: %v", err)
			conn.Close()
			ws.mu.Lock()
			delete(ws.connections[gameID], conn)
			if len(ws.connections[gameID]) == 0 {
				delete(ws.connections, gameID)
			}
			ws.mu.Unlock()
		}
	}
}

func (ws *WSController) HandleMatchmakingWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := ws.upgrader.Upgrade(w, r, nil)
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
	ws.matchmakingService.JoinQueue(playerID, conn)

	// Try to match
	if gameID, conns, ok := ws.matchmakingService.TryMatch(r.Context()); ok {
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
			ws.matchmakingService.LeaveQueue(playerID)
			return
		}
	}
}

// HandleNotificationsWebSocket handles the global notification WebSocket for a player
func (ws *WSController) HandleNotificationsWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := ws.upgrader.Upgrade(w, r, nil)
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
	ws.matchmakingService.RegisterConnection(playerID, conn)
	log.Printf("[debug] HandleNotificationsWebSocket: Registered playerID=%s, conn type=%T", playerID, conn)
	defer ws.matchmakingService.UnregisterConnection(playerID)

	// Keep the connection open for notifications
	for {
		_, msgData, err := conn.ReadMessage()
		if err != nil {
			// On disconnect, cleanup is handled by defer
			return
		}
		// Parse message type
		var msg struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(msgData, &msg); err != nil {
			continue
		}
		if msg.Type == "rematch_offer" {
			var offer struct {
				Type     string `json:"type"`
				From     string `json:"from"`
				To       string `json:"to"`
				FromName string `json:"from_name"`
			}
			if err := json.Unmarshal(msgData, &offer); err != nil || offer.From == "" || offer.To == "" || offer.FromName == "" {
				log.Printf("[notif-ws] Invalid rematch_offer: %v, offer: %+v", err, offer)
				continue
			}
			log.Printf("[notif-ws] Received rematch_offer from %s to %s", offer.From, offer.To)
			if targetConn, ok := ws.matchmakingService.OnlineConnection(offer.To); ok {
				log.Printf("[notif-ws] Forwarding rematch_offer to %s", offer.To)
				b, _ := json.Marshal(offer)
				if wsConn, ok := targetConn.(*websocket.Conn); ok {
					wsConn.WriteMessage(websocket.TextMessage, b)
				}
			}
		} else if msg.Type == "rematch_response" {
			var resp struct {
				Type     string `json:"type"`
				From     string `json:"from"`
				To       string `json:"to"`
				Accepted bool   `json:"accepted"`
				FromName string `json:"from_name"`
			}
			if err := json.Unmarshal(msgData, &resp); err != nil || resp.From == "" || resp.To == "" || resp.FromName == "" {
				log.Printf("[notif-ws] Invalid rematch_response: %v, resp: %+v", err, resp)
				continue
			}
			log.Printf("[notif-ws] Received rematch_response from %s to %s (accepted=%v)", resp.From, resp.To, resp.Accepted)
			var result struct {
				Type     string  `json:"type"`
				Accepted bool    `json:"accepted"`
				GameID   *string `json:"game_id,omitempty"`
				From     string  `json:"from"`
				To       string  `json:"to"`
				Rematch  bool    `json:"rematch"`
			}
			result = struct {
				Type     string  `json:"type"`
				Accepted bool    `json:"accepted"`
				GameID   *string `json:"game_id,omitempty"`
				From     string  `json:"from"`
				To       string  `json:"to"`
				Rematch  bool    `json:"rematch"`
			}{
				Type:     "rematch_result",
				Accepted: resp.Accepted,
				GameID:   nil,
				From:     resp.To,
				To:       resp.From,
				Rematch:  true,
			}
			if resp.Accepted {
				// Create new game
				game, err := ws.gameService.CreateGame(r.Context(), resp.From, resp.To)
				if err == nil {
					result.GameID = &game.ID
				}
			}
			b, _ := json.Marshal(result)
			for _, pid := range []string{resp.From, resp.To} {
				if notifConn, ok := ws.matchmakingService.OnlineConnection(pid); ok {
					log.Printf("[notif-ws] Sending rematch_result to %s (game_id=%v, accepted=%v)", pid, result.GameID, resp.Accepted)
					if wsConn, ok := notifConn.(*websocket.Conn); ok {
						wsConn.WriteMessage(websocket.TextMessage, b)
					}
				}
			}
		} else if msg.Type == "challenge_invite" {
			var invite ChallengeInviteMessage
			if err := json.Unmarshal(msgData, &invite); err != nil || invite.From == "" || invite.To == "" {
				log.Printf("[notif-ws] Invalid challenge_invite: %v, invite: %+v", err, invite)
				continue
			}
			log.Printf("[notif-ws] Received challenge_invite from %s to %s", invite.From, invite.To)
			delivered := ws.matchmakingService.SendChallengeInvite(invite.From, invite.To)
			if delivered {
				// Get challenger display name
				challengerName := invite.From
				if p, err := ws.playerService.GetByID(r.Context(), invite.From); err == nil && p != nil {
					challengerName = p.Name
				}
				invite := ChallengeInviteMessage{Type: "challenge_invite", From: invite.From, FromName: challengerName, Rematch: invite.Rematch, To: invite.To}
				if challengedConn, ok := ws.matchmakingService.OnlineConnection(invite.To); ok {
					log.Printf("[notif-ws] Sending challenge_invite from %s to %s", invite.From, invite.To)
					b, _ := json.Marshal(invite)
					if wsConn, ok := challengedConn.(*websocket.Conn); ok {
						wsConn.WriteMessage(websocket.TextMessage, b)
					}
				} else {
					log.Printf("[notif-ws] No notification connection found for player %s", invite.To)
				}
			} else {
				log.Printf("[notif-ws] SendChallengeInvite returned false for %s to %s", invite.From, invite.To)
			}
		} else if msg.Type == "challenge_cancel" {
			var req struct {
				Type string `json:"type"`
				From string `json:"from"`
				To   string `json:"to"`
			}
			if err := json.Unmarshal(msgData, &req); err != nil || req.From == "" || req.To == "" {
				log.Printf("[notif-ws] Invalid challenge_cancel: %v, req: %+v", err, req)
				continue
			}
			log.Printf("[notif-ws] Received challenge_cancel from %s to %s", req.From, req.To)
			ws.matchmakingService.CancelChallenge(req.From, req.To)
			// Optionally notify the challenged player
			if challengedConn, ok := ws.matchmakingService.OnlineConnection(req.To); ok {
				cancelMsg := struct {
					Type string `json:"type"`
					From string `json:"from"`
				}{"challenge_cancelled", req.From}
				b, _ := json.Marshal(cancelMsg)
				if wsConn, ok := challengedConn.(*websocket.Conn); ok {
					wsConn.WriteMessage(websocket.TextMessage, b)
				}
			}
		} else if msg.Type == "challenge_response" {
			var resp ChallengeResponseMessage
			if err := json.Unmarshal(msgData, &resp); err != nil || resp.From == "" || resp.To == "" {
				log.Printf("[notif-ws] Invalid challenge_response: %v, resp: %+v", err, resp)
				continue
			}
			log.Printf("[notif-ws] Received challenge_response from %s to %s (accepted=%v)", resp.From, resp.To, resp.Accepted)
			gameID, _, _, accepted := ws.matchmakingService.AcceptChallenge(r.Context(), resp.From, resp.Accepted)
			result := struct {
				Type     string  `json:"type"`
				Accepted bool    `json:"accepted"`
				GameID   *string `json:"game_id,omitempty"`
				From     string  `json:"from"`
				To       string  `json:"to"`
				Rematch  bool    `json:"rematch"`
			}{
				Type:     "challenge_result",
				Accepted: accepted,
				GameID:   nil,
				From:     resp.To,   // challenger
				To:       resp.From, // challenged
				Rematch:  false,
			}
			if accepted && gameID != "" {
				result.GameID = &gameID
			}
			b, _ := json.Marshal(result)
			// Notify both players via their notification WebSocket
			for _, pid := range []string{resp.From, resp.To} {
				if notifConn, ok := ws.matchmakingService.OnlineConnection(pid); ok {
					log.Printf("[notif-ws] Sending challenge_result to %s (game_id=%v, accepted=%v)", pid, gameID, accepted)
					if wsConn, ok := notifConn.(*websocket.Conn); ok {
						wsConn.WriteMessage(websocket.TextMessage, b)
					}
				}
			}
		}
	}
}

// --- Challenge and Rematch Message Types ---
type ChallengeInviteMessage struct {
	Type     string `json:"type"`
	From     string `json:"from"`
	To       string `json:"to"`
	FromName string `json:"from_name"`
	Rematch  bool   `json:"rematch"`
}

type ChallengeResponseMessage struct {
	Type     string `json:"type"`
	From     string `json:"from"`
	To       string `json:"to"`
	Accepted bool   `json:"accepted"`
	Rematch  bool   `json:"rematch"`
}

type ChallengeResultMessage struct {
	Type     string  `json:"type"`
	Accepted bool    `json:"accepted"`
	GameID   *string `json:"game_id,omitempty"`
	From     string  `json:"from"`
	To       string  `json:"to"`
	Rematch  bool    `json:"rematch"`
}
