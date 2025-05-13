package ws

import (
	"log"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/mehrbod2002/fxtrader/internal/models"
)

type Hub struct {
	clients        map[string]*models.Client
	register       chan *models.Client
	unregister     chan *models.Client
	broadcast      chan *models.PriceData
	tradeBroadcast chan *models.TradeHistory
	mu             sync.RWMutex
}

func NewHub() *Hub {
	return &Hub{
		clients:        make(map[string]*models.Client),
		register:       make(chan *models.Client),
		unregister:     make(chan *models.Client),
		broadcast:      make(chan *models.PriceData),
		tradeBroadcast: make(chan *models.TradeHistory),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client.ID] = client
			h.mu.Unlock()
		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client.ID]; ok {
				delete(h.clients, client.ID)
				client.Close()
			}
			h.mu.Unlock()
		case price := <-h.broadcast:
			h.mu.RLock()
			for _, client := range h.clients {
				if client.IsSubscribed(price.Symbol) {
					select {
					case client.Send <- price:
					default:
						log.Printf("Client %s buffer full, skipping message", client.ID)
					}
				}
			}
			h.mu.RUnlock()
		case trade := <-h.tradeBroadcast:
			h.mu.RLock()
			for _, client := range h.clients {
				subscriptionKey := trade.UserID.Hex() + ":" + trade.AccountType
				if client.IsSubscribed(subscriptionKey) {
					select {
					case client.SendTrade <- trade:
					default:
						log.Printf("Client %s trade buffer full, skipping message", client.ID)
					}
				}
			}
			h.mu.RUnlock()
		}
	}
}

func (h *Hub) RegisterClient(conn *websocket.Conn) *models.Client {
	clientID := uuid.New().String()
	client := models.NewClient(clientID, conn)
	h.register <- client
	return client
}

func (h *Hub) UnregisterClient(client *models.Client) {
	h.unregister <- client
}

func (h *Hub) BroadcastPrice(data *models.PriceData) {
	h.broadcast <- data
}

func (h *Hub) BroadcastTrade(trade *models.TradeHistory) {
	h.tradeBroadcast <- trade
}

func (h *Hub) GetClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}
