package ws

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/mehrbod2002/fxtrader/internal/models"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type WebSocketHandler struct {
	hub *Hub
}

func NewWebSocketHandler(hub *Hub) *WebSocketHandler {
	return &WebSocketHandler{hub: hub}
}

func (h *WebSocketHandler) HandleConnection(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
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
		log.Printf("error setting read deadline: %v", err)
		return
	}
	client.Conn.SetPongHandler(func(string) error {
		if err := client.Conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
			log.Printf("error setting read deadline in pong handler: %v", err)
			return err
		}
		return nil
	})

	for {
		_, message, err := client.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}

		var socketMsg struct {
			Action      string `json:"action"`
			Symbol      string `json:"symbol"`
			AccountType string `json:"account_type"`
		}

		if err := json.Unmarshal(message, &socketMsg); err != nil {
			response := models.ErrorResponse{Error: "Invalid message format"}
			if err := client.Conn.WriteJSON(response); err != nil {
				log.Printf("error: %v", err)
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
				log.Printf("error: %v", err)
				continue
			}

		case "subscribe_trades":
			if socketMsg.AccountType != "DEMO" && socketMsg.AccountType != "REAL" {
				response := models.ErrorResponse{Error: "Invalid account type"}
				if err := client.Conn.WriteJSON(response); err != nil {
					log.Printf("error: %v", err)
				}
				continue
			}
			subscriptionKey := socketMsg.Symbol + ":" + socketMsg.AccountType
			client.Subscribe(subscriptionKey)
			var symbols []string
			for symbol := range client.Symbols {
				symbols = append(symbols, symbol)
			}
			response := models.SubscriptionResponse{
				Status:  "success",
				Message: "Subscribed to trade stream for user " + socketMsg.Symbol + " (" + socketMsg.AccountType + ")",
				Symbols: symbols,
			}
			if err := client.Conn.WriteJSON(response); err != nil {
				log.Printf("error: %v", err)
				continue
			}
			if err := client.Conn.WriteJSON(map[string]string{"status": "trade_stream_started", "user_id": socketMsg.Symbol, "account_type": socketMsg.AccountType}); err != nil {
				log.Printf("error: %v", err)
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
				log.Printf("error: %v", err)
				continue
			}

		default:
			response := models.ErrorResponse{Error: "Unknown action"}
			if err := client.Conn.WriteJSON(response); err != nil {
				log.Printf("error: %v", err)
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
				log.Printf("error: %v", err)
				return
			}
			if !ok {
				if err := client.Conn.WriteMessage(websocket.CloseMessage, []byte{}); err != nil {
					log.Printf("error: %v", err)
					return
				}
				return
			}

			err := client.Conn.WriteJSON(price)
			if err != nil {
				return
			}

		case trade, ok := <-client.SendTrade:
			if err := client.Conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				log.Printf("error: %v", err)
				return
			}
			if !ok {
				if err := client.Conn.WriteMessage(websocket.CloseMessage, []byte{}); err != nil {
					log.Printf("error: %v", err)
					return
				}
				return
			}
			err := client.Conn.WriteJSON(trade)
			if err != nil {
				return
			}

		case <-ticker.C:
			if err := client.Conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				log.Printf("error: %v", err)
				continue
			}
			if err := client.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
