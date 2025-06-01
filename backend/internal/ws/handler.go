package ws

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/mehrbod2002/fxtrader/interfaces"
	"github.com/mehrbod2002/fxtrader/internal/models"
	"github.com/mehrbod2002/fxtrader/internal/repository"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512
)

var Upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type WebSocketHandler struct {
	hub            *Hub
	tradeService   interfaces.TradeService
	userRepository repository.UserRepository
}

func NewWebSocketHandler(hub *Hub, tradeService interfaces.TradeService, user_repository repository.UserRepository) *WebSocketHandler {
	return &WebSocketHandler{
		hub:            hub,
		tradeService:   tradeService,
		userRepository: user_repository,
	}
}

func (h *WebSocketHandler) HandleConnection(c *gin.Context) {
	conn, err := Upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	client := h.hub.RegisterClient(conn)

	go h.readPump(client)
	go h.writePump(client)
}

func (h *WebSocketHandler) readPump(client *models.Client) {
	defer func() {
		h.hub.UnregisterClient(client)
	}()

	client.Conn.SetReadLimit(maxMessageSize)
	if err := client.Conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		return
	}
	client.Conn.SetPongHandler(func(string) error {
		if err := client.Conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
			return err
		}
		return nil
	})

	for {
		_, message, err := client.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket read error: %v", err)
			}
			break
		}

		var socketMsg struct {
			Action      string `json:"action"`
			Symbol      string `json:"symbol"`
			AccountType string `json:"account_type"`
			UserID      string `json:"user_id"`
		}

		if err := json.Unmarshal(message, &socketMsg); err != nil {
			response := models.ErrorResponse{Error: "Invalid message format"}
			if err := client.Conn.WriteJSON(response); err != nil {
				break
			}
			continue
		}

		switch socketMsg.Action {
		case "subscribe":
			client.Subscribe(socketMsg.Symbol)
			var symbols []string
			for symbol := range client.Symbols {
				symbols = append(symbols, symbol)
			}
			response := models.SubscriptionResponse{
				Status:  "success",
				Message: "Subscribed to " + socketMsg.Symbol,
				Symbols: symbols,
			}
			if err := client.Conn.WriteJSON(response); err != nil {
				continue
			}

		case "subscribe_trades":
			if socketMsg.AccountType != "DEMO" && socketMsg.AccountType != "REAL" {
				response := models.ErrorResponse{Error: "Invalid account type"}
				if err := client.Conn.WriteJSON(response); err != nil {
					log.Printf("Error sending error response: %v", err)
				}
				continue
			}
			user, err := h.userRepository.GetUserByTelegramID(socketMsg.UserID)
			if err != nil {
				response := models.ErrorResponse{Error: "Invalid user ID"}
				if err := client.Conn.WriteJSON(response); err != nil {
					log.Printf("Error sending error response: %v", err)
				}
				continue
			}

			subscriptionKey := socketMsg.UserID + ":" + socketMsg.AccountType
			client.Subscribe(subscriptionKey)

			streamChan, err := h.tradeService.StreamTrades(user.ID.Hex(), socketMsg.AccountType)
			if err != nil {
				response := models.ErrorResponse{Error: fmt.Sprintf("Failed to start trade stream: %v", err)}
				if err := client.Conn.WriteJSON(response); err != nil {
					log.Printf("Error sending error response: %v", err)
				}
				client.Unsubscribe(subscriptionKey)
				continue
			}

			go func() {
				for response := range streamChan {
					select {
					case client.SendOrders <- response:
					default:
						log.Printf("Client %s order stream buffer full, skipping message", client.ID)
					}
				}
			}()

			response := models.SubscriptionResponse{
				Status:      "success",
				Message:     fmt.Sprintf("Subscribed to trade stream for user %s (%s)", socketMsg.UserID, socketMsg.AccountType),
				UserID:      socketMsg.UserID,
				AccountType: socketMsg.AccountType,
			}
			if err := client.Conn.WriteJSON(response); err != nil {
				continue
			}

			if err := client.Conn.WriteJSON(map[string]string{
				"status":       "trade_stream_started",
				"user_id":      socketMsg.UserID,
				"account_type": socketMsg.AccountType,
			}); err != nil {
				continue
			}

		case "unsubscribe":
			client.Unsubscribe(socketMsg.Symbol)
			var symbols []string
			for symbol := range client.Symbols {
				symbols = append(symbols, symbol)
			}
			response := models.SubscriptionResponse{
				Status:  "success",
				Message: "Unsubscribed from " + socketMsg.Symbol,
				Symbols: symbols,
			}
			if err := client.Conn.WriteJSON(response); err != nil {
				continue
			}

		default:
			response := models.ErrorResponse{Error: "Unknown action"}
			if err := client.Conn.WriteJSON(response); err != nil {
				continue
			}
		}
	}
}

func (h *WebSocketHandler) writePump(client *models.Client) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		client.Conn.Close()
	}()

	for {
		select {
		case price, ok := <-client.Send:
			if err := client.Conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				return
			}
			if !ok {
				if err := client.Conn.WriteMessage(websocket.CloseMessage, []byte{}); err != nil {
					log.Printf("Error sending close message: %v", err)
				}
				return
			}
			if err := client.Conn.WriteJSON(price); err != nil {
				return
			}

		case trade, ok := <-client.SendTrade:
			if err := client.Conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				return
			}
			if !ok {
				if err := client.Conn.WriteMessage(websocket.CloseMessage, []byte{}); err != nil {
					log.Printf("Error sending close message: %v", err)
				}
				return
			}
			if err := client.Conn.WriteJSON(trade); err != nil {
				return
			}

		case balance, ok := <-client.SendBalance:
			if err := client.Conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				return
			}
			if !ok {
				if err := client.Conn.WriteMessage(websocket.CloseMessage, []byte{}); err != nil {
					log.Printf("Error sending close message: %v", err)
				}
				return
			}
			if err := client.Conn.WriteJSON(balance); err != nil {
				return
			}

		case orderStream, ok := <-client.SendOrders:
			if err := client.Conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				return
			}
			if !ok {
				if err := client.Conn.WriteMessage(websocket.CloseMessage, []byte{}); err != nil {
					log.Printf("Error sending close message: %v", err)
				}
				return
			}
			if err := client.Conn.WriteJSON(orderStream); err != nil {
				return
			}

		case <-ticker.C:
			if err := client.Conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				continue
			}
			if err := client.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
