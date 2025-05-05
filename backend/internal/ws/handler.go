package ws

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/mehrbod2002/fxtrader/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

const (
	writeWait = 10 * time.Second

	pongWait = 60 * time.Second

	pingPeriod = (pongWait * 9) / 10

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
	client.Conn.SetReadDeadline(time.Now().Add(pongWait))
	client.Conn.SetPongHandler(func(string) error {
		client.Conn.SetReadDeadline(time.Now().Add(pongWait))
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

		var socketMsg models.SocketMessage
		if err := json.Unmarshal(message, &socketMsg); err != nil {
			response := models.ErrorResponse{Error: "Invalid message format"}
			client.Conn.WriteJSON(response)
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
			client.Conn.WriteJSON(response)

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
			client.Conn.WriteJSON(response)

		default:
			response := models.ErrorResponse{Error: "Unknown action"}
			client.Conn.WriteJSON(response)
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
			client.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				client.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			err := client.Conn.WriteJSON(price)
			if err != nil {
				return
			}

		case <-ticker.C:
			client.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := client.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
