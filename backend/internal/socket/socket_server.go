package socket

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/mehrbod2002/fxtrader/interfaces"
	"github.com/mehrbod2002/fxtrader/internal/models"
)

const (
	pingInterval            = 30 * time.Second
	readTimeout             = 120 * time.Second
	writeTimeout            = 10 * time.Second
	maxMessageSize          = 1024 * 1024
	reconnectBackoffInitial = 2 * time.Second
	reconnectBackoffMax     = 30 * time.Second
	maxRetries              = 10
	retryDelay              = 10 * time.Second
	maxMissedPongs          = 5
)

type HandlerFunc func(message map[string]interface{}, client *Client) error

type WebSocketServer struct {
	listenAddr   string
	handlers     map[string]HandlerFunc
	handlersMu   sync.RWMutex
	clients      map[string]*Client
	clientsMu    sync.RWMutex
	tradeService interfaces.TradeService
	ctx          context.Context
	cancel       context.CancelFunc
	upgrader     websocket.Upgrader
}

type Client struct {
	conn       *websocket.Conn
	cancelPing context.CancelFunc
	clientID   string
	writeMu    sync.Mutex
}

func NewWebSocketServer(listenPort int) (*WebSocketServer, error) {
	ctx, cancel := context.WithCancel(context.Background())
	return &WebSocketServer{
		listenAddr: fmt.Sprintf(":%d", listenPort),
		handlers:   make(map[string]HandlerFunc),
		clients:    make(map[string]*Client),
		ctx:        ctx,
		cancel:     cancel,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  8192,
			WriteBufferSize: 8192,
			CheckOrigin:     func(r *http.Request) bool { return true },
		},
	}, nil
}

func (s *WebSocketServer) RegisterHandler(msgType string, handler HandlerFunc) {
	s.handlersMu.Lock()
	defer s.handlersMu.Unlock()
	s.handlers[msgType] = handler
}

func (s *WebSocketServer) Start(tradeService interfaces.TradeService) error {
	s.tradeService = tradeService

	s.RegisterHandler("handshake", s.handleHandshake)
	s.RegisterHandler("ping", s.handlePing)
	s.RegisterHandler("pong", s.handlePong)
	s.RegisterHandler("disconnect", s.handleDisconnect)
	s.RegisterHandler("close_trade_response", s.handleCloseTradeResponse)
	s.RegisterHandler("order_stream_response", s.handleOrderStreamResponse)
	s.RegisterHandler("trade_response", s.handleTradeResponse)
	s.RegisterHandler("balance_response", s.handleBalanceResponse)
	s.RegisterHandler("balance_stream_response", s.handleBalanceStreamResponse)

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := s.upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		go s.handleConnection(conn, s.ctx)
	})

	go func() {
		if err := http.ListenAndServe(s.listenAddr, nil); err != nil {
			log.Printf("WebSocket server failed: %v", err)
		}
	}()
	return nil
}

func (s *WebSocketServer) handleTradeResponse(msg map[string]interface{}, client *Client) error {
	var response interfaces.TradeResponse
	data, err := json.Marshal(msg)

	if err != nil {
		return fmt.Errorf("failed to marshal trade response: %v", err)
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return fmt.Errorf("failed to unmarshal trade response: %v", err)
	}

	return s.tradeService.HandleTradeResponse(response)
}

func (s *WebSocketServer) handleCloseTradeResponse(msg map[string]interface{}, client *Client) error {
	var response interfaces.TradeResponse
	data, err := json.Marshal(msg)

	if err != nil {
		return fmt.Errorf("failed to marshal close trade response: %v", err)
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return fmt.Errorf("failed to unmarshal close trade response: %v", err)
	}

	return s.tradeService.HandleCloseTradeResponse(response)
}

func (s *WebSocketServer) handleOrderStreamResponse(msg map[string]interface{}, client *Client) error {
	var response models.OrderStreamResponse
	data, err := json.Marshal(msg)

	if err != nil {
		return fmt.Errorf("failed to marshal order stream response: %v", err)
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return fmt.Errorf("failed to unmarshal order stream response: %v", err)
	}

	if err := s.tradeService.HandleOrderStreamResponse(response); err != nil {
		errResponse := models.ErrorResponse{Error: fmt.Sprintf("Failed to process order stream: %v", err)}
		if err := client.conn.WriteJSON(errResponse); err != nil {
			return fmt.Errorf("Error sending error response to client: %v", err)
		}
		return err
	}

	return nil
}

func (s *WebSocketServer) handleBalanceResponse(msg map[string]interface{}, client *Client) error {
	var response interfaces.BalanceResponse
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal balance response: %v", err)
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return fmt.Errorf("failed to unmarshal balance response: %v", err)
	}
	log.Printf("Forwarding balance response to TradeService: %+v", response)
	return nil
}

func (s *WebSocketServer) addClient(clientID string, conn *websocket.Conn, cancelPing context.CancelFunc) {
	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()

	if oldClient, exists := s.clients[clientID]; exists {
		log.Printf("Replacing existing connection for client %s", clientID)
		oldClient.cancelPing()
		oldClient.writeMu.Lock()
		oldClient.conn.Close()
		oldClient.writeMu.Unlock()

		disconnectMsg := map[string]interface{}{
			"type":      "disconnect",
			"reason":    "New connection established",
			"timestamp": time.Now().Unix(),
		}
		if err := s.sendJSONMessage(oldClient, disconnectMsg); err != nil {
			log.Printf("Error sending disconnect message to client %s: %v", clientID, err)
		}
	}

	s.clients[clientID] = &Client{
		conn:       conn,
		cancelPing: cancelPing,
		clientID:   clientID,
		writeMu:    sync.Mutex{},
	}
	log.Printf("Added client %s to connection pool", clientID)

	if s.tradeService != nil {
		s.tradeService.RegisterMT5Connection(conn)
	}
}

func (s *WebSocketServer) removeClient(clientID string) {
	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()

	if client, exists := s.clients[clientID]; exists {
		client.writeMu.Lock()
		client.cancelPing()
		if err := client.conn.Close(); err != nil {
			log.Printf("Error closing connection for client %s: %v", clientID, err)
		}
		client.writeMu.Unlock()
		delete(s.clients, clientID)
		log.Printf("Removed client %s from connection pool", clientID)
	}
}

func (s *WebSocketServer) isClientConnected(clientID string) bool {
	s.clientsMu.RLock()
	defer s.clientsMu.RUnlock()
	_, exists := s.clients[clientID]
	return exists
}

func (s *WebSocketServer) startPingMonitor(client *Client, ctx context.Context) {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	missedPongs := 0

	for {
		select {
		case <-ctx.Done():
			log.Printf("Ping monitor stopped for client %s", client.clientID)
			return
		case <-ticker.C:
			if !s.isClientConnected(client.clientID) {
				log.Printf("Client %s not found in connection pool, stopping ping monitor", client.clientID)
				return
			}

			client.writeMu.Lock()
			if client.conn == nil {
				client.writeMu.Unlock()
				log.Printf("Connection closed for client %s, stopping ping monitor", client.clientID)
				return
			}
			client.writeMu.Unlock()

			pingMsg := map[string]interface{}{
				"type":      "ping",
				"timestamp": time.Now().Unix(),
			}
			if err := s.sendJSONMessage(client, pingMsg); err != nil {
				log.Printf("Failed to send ping to client %s: %v", client.clientID, err)
				missedPongs++
				if missedPongs >= maxMissedPongs {
					log.Printf("Client %s missed %d pongs, closing connection", client.clientID, maxMissedPongs)
					s.removeClient(client.clientID)
					return
				}
				continue
			}
			log.Printf("Sent ping to client %s", client.clientID)
		}
	}
}

func (s *WebSocketServer) handlePong(msg map[string]interface{}, client *Client) error {
	return nil
}

func (s *WebSocketServer) handlePing(msg map[string]interface{}, client *Client) error {
	if client == nil || client.conn == nil {
		return fmt.Errorf("nil client or connection")
	}

	pongMsg := map[string]interface{}{
		"type":      "pong",
		"timestamp": time.Now().Unix(),
	}
	if err := s.sendJSONMessage(client, pongMsg); err != nil {
		log.Printf("Failed to send pong to client %s: %v", client.clientID, err)
		return fmt.Errorf("failed to send pong: %v", err)
	}
	log.Printf("Sent pong to client %s", client.clientID)
	return nil
}

func (s *WebSocketServer) handleConnection(conn *websocket.Conn, ctx context.Context) {
	defer conn.Close()

	conn.SetReadLimit(maxMessageSize)
	tempClientID := uuid.New().String()
	retryCount := 0

	for {
		select {
		case <-ctx.Done():
			log.Printf("Connection closed for %s due to server shutdown", tempClientID)
			return
		default:
			if err := conn.SetReadDeadline(time.Now().Add(readTimeout)); err != nil {
				log.Printf("Failed to set read deadline for %s: %v", tempClientID, err)
				return
			}

			_, message, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					log.Printf("WebSocket closed normally for %s", tempClientID)
					return
				}
				retryCount++
				log.Printf("Read error from %s, retry %d/%d: %v", tempClientID, retryCount, maxRetries, err)
				time.Sleep(retryDelay)
				if retryCount >= maxRetries {
					log.Printf("Max retries (%d) reached for %s", maxRetries, tempClientID)
					return

				}
				continue
			}
			retryCount = 0

			if err := s.processMessage(message, conn, &tempClientID); err != nil {
				log.Printf("Error processing message from %s: %v", tempClientID, err)
			}
		}
	}
}

func (s *WebSocketServer) processMessage(message []byte, conn *websocket.Conn, tempClientID *string) error {
	var msg map[string]interface{}
	if err := json.Unmarshal(message, &msg); err != nil {
		return fmt.Errorf("failed to decode JSON: %v", err)
	}

	msgType, ok := msg["type"].(string)
	if !ok {
		return fmt.Errorf("missing or invalid 'type' field in message")
	}

	if msgType == "handshake" {
		clientID, ok := msg["client_id"].(string)
		if !ok || clientID == "" {
			return fmt.Errorf("missing or invalid 'client_id' in handshake")
		}
		*tempClientID = clientID
		ctx, cancel := context.WithCancel(s.ctx)
		client := &Client{
			conn:       conn,
			cancelPing: cancel,
			clientID:   clientID,
			writeMu:    sync.Mutex{},
		}
		s.addClient(clientID, conn, cancel)
		go s.startPingMonitor(client, ctx)
		log.Printf("Handshake successful for client %s", clientID)
		return nil
	}

	s.clientsMu.RLock()
	client, exists := s.clients[*tempClientID]
	s.clientsMu.RUnlock()
	if !exists {
		return fmt.Errorf("client %s not found", *tempClientID)
	}

	s.handlersMu.RLock()
	handler, exists := s.handlers[msgType]
	s.handlersMu.RUnlock()

	if !exists {
		log.Printf("No handler registered for message type: %s", msgType)
		return fmt.Errorf("unknown message type: %s", msgType)
	}

	return handler(msg, client)
}

func (s *WebSocketServer) sendJSONMessage(client *Client, msg interface{}) error {
	client.writeMu.Lock()
	defer client.writeMu.Unlock()

	if err := client.conn.SetWriteDeadline(time.Now().Add(writeTimeout)); err != nil {
		return fmt.Errorf("failed to set write deadline: %v", err)
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %v", err)
	}

	return client.conn.WriteMessage(websocket.TextMessage, data)
}

func (s *WebSocketServer) handleHandshake(msg map[string]interface{}, client *Client) error {
	response := map[string]interface{}{
		"type":      "handshake_response",
		"status":    "success",
		"server":    "FXTrader_Server",
		"version":   "1.0",
		"timestamp": time.Now().Unix(),
	}

	return s.sendJSONMessage(client, response)
}

func (s *WebSocketServer) handleDisconnect(msg map[string]interface{}, client *Client) error {
	reason, _ := msg["reason"].(string)
	clientID := client.clientID
	log.Printf("Client %s initiated disconnect. Reason: %s", clientID, reason)

	client.writeMu.Lock()
	defer client.writeMu.Unlock()
	client.cancelPing()
	if err := client.conn.Close(); err != nil {
		log.Printf("Error closing connection for client %s: %v", clientID, err)
	}
	s.removeClient(clientID)
	return nil
}

func (s *WebSocketServer) SendTradeRequest(tradeRequest map[string]interface{}) error {
	s.clientsMu.RLock()
	defer s.clientsMu.RUnlock()
	if len(s.clients) == 0 {
		return fmt.Errorf("no active MT5 connections available")
	}
	var lastErr error
	for clientID, client := range s.clients {
		if err := s.sendJSONMessage(client, tradeRequest); err != nil {
			log.Printf("Failed to send trade request to client %s: %v", clientID, err)
			lastErr = err
		} else {
			log.Printf("Trade request sent to client %s (account_type: %v)", clientID, tradeRequest["account_type"])
			return nil
		}
	}
	return lastErr
}

func (s *WebSocketServer) SendCloseTradeRequest(closeRequest map[string]interface{}) error {
	s.clientsMu.RLock()
	defer s.clientsMu.RUnlock()
	if len(s.clients) == 0 {
		return fmt.Errorf("no active MT5 connections available")
	}
	var lastErr error
	for clientID, client := range s.clients {
		if err := s.sendJSONMessage(client, closeRequest); err != nil {
			log.Printf("Failed to send close trade request to client %s: %v", clientID, err)
			lastErr = err
		} else {
			log.Printf("Close trade request sent to client %s (account_type: %v)", clientID, closeRequest["account_type"])
			return nil
		}
	}
	return lastErr
}

func (s *WebSocketServer) SendOrderStreamRequest(streamRequest map[string]interface{}) error {
	s.clientsMu.RLock()
	defer s.clientsMu.RUnlock()
	if len(s.clients) == 0 {
		return fmt.Errorf("no active MT5 connections available")
	}
	var lastErr error
	for clientID, client := range s.clients {
		if err := s.sendJSONMessage(client, streamRequest); err != nil {
			log.Printf("Failed to send order stream request to client %s: %v", clientID, err)
			lastErr = err
		} else {
			log.Printf("Order stream request sent to client %s (account_type: %v)", clientID, streamRequest["account_type"])
			return nil
		}
	}
	return lastErr
}

func (s *WebSocketServer) SendBalanceRequest(balanceRequest map[string]interface{}) error {
	s.clientsMu.RLock()
	defer s.clientsMu.RUnlock()
	if len(s.clients) == 0 {
		return fmt.Errorf("no active MT5 connections available")
	}
	var lastErr error
	for _, client := range s.clients {
		if err := s.sendJSONMessage(client, balanceRequest); err != nil {
			lastErr = err
		} else {
			return nil
		}
	}
	return lastErr
}

func (s *WebSocketServer) handleBalanceStreamResponse(msg map[string]interface{}, client *Client) error {
	var response interfaces.BalanceResponse
	data, err := json.Marshal(msg)

	if err != nil {
		return fmt.Errorf("failed to marshal balance stream response: %v", err)
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return fmt.Errorf("failed to unmarshal balance stream response: %v", err)
	}
	return nil
}

func (s *WebSocketServer) SendBalanceStreamRequest(streamRequest map[string]interface{}) error {
	s.clientsMu.RLock()
	defer s.clientsMu.RUnlock()
	if len(s.clients) == 0 {
		return fmt.Errorf("no active MT5 connections available")
	}
	var lastErr error
	for _, client := range s.clients {
		if err := s.sendJSONMessage(client, streamRequest); err != nil {
			lastErr = err
		} else {
			return nil
		}
	}
	return lastErr
}

func (s *WebSocketServer) Stop() {
	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()

	for _, client := range s.clients {
		client.conn.Close()
	}

	for k := range s.clients {
		delete(s.clients, k)
	}
	s.cancel()
}
