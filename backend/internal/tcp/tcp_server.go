package tcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"github.com/mehrbod2002/fxtrader/internal/service"
)

const (
	pingInterval   = 20 * time.Second
	readTimeout    = 30 * time.Second
	writeTimeout   = 10 * time.Second
	maxMessageSize = 1024 * 1024
)

type HandlerFunc func(message map[string]interface{}, conn *net.TCPConn) error

type TCPServer struct {
	listenAddr   *net.TCPAddr
	handlers     map[string]HandlerFunc
	handlersMu   sync.RWMutex
	responseChan chan interface{}
	clients      map[string]*net.TCPConn
	clientsMu    sync.RWMutex
	tradeService service.TradeService
}

func NewTCPServer(listenPort int) (*TCPServer, error) {
	listenAddr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf(":%d", listenPort))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve listen address: %v", err)
	}
	return &TCPServer{
		listenAddr:   listenAddr,
		handlers:     make(map[string]HandlerFunc),
		responseChan: make(chan interface{}, 100),
		clients:      make(map[string]*net.TCPConn),
	}, nil
}

func (s *TCPServer) RegisterHandler(msgType string, handler HandlerFunc) {
	s.handlersMu.Lock()
	defer s.handlersMu.Unlock()
	s.handlers[msgType] = handler
}

func (s *TCPServer) Start(tradeService service.TradeService) error {
	s.tradeService = tradeService

	s.RegisterHandler("handshake", s.handleHandshake)
	s.RegisterHandler("pong", s.handlePong)
	s.RegisterHandler("disconnect", s.handleDisconnect)

	listener, err := net.ListenTCP("tcp", s.listenAddr)
	if err != nil {
		return fmt.Errorf("failed to start TCP server: %v", err)
	}
	log.Printf("TCP server listening on %s", s.listenAddr.String())

	go func() {
		defer listener.Close()
		for {
			conn, err := listener.AcceptTCP()
			if err != nil {
				log.Printf("Failed to accept TCP connection: %v", err)
				continue
			}

			conn.SetKeepAlive(true)
			conn.SetKeepAlivePeriod(30 * time.Second)
			conn.SetReadBuffer(8192)
			conn.SetWriteBuffer(8192)

			clientID := conn.RemoteAddr().String()
			s.addClient(clientID, conn)

			log.Printf("New connection from %s", clientID)

			if s.tradeService != nil {
				s.tradeService.RegisterMT5Connection(conn)
			}

			go s.handleConnection(conn, clientID)
			go s.startPingMonitor(conn, clientID)
		}
	}()
	return nil
}

func (s *TCPServer) addClient(clientID string, conn *net.TCPConn) {
	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()

	if oldConn, exists := s.clients[clientID]; exists {
		log.Printf("Replacing existing connection for client %s", clientID)
		oldConn.Close()
	}

	s.clients[clientID] = conn
}

func (s *TCPServer) removeClient(clientID string) {
	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()
	delete(s.clients, clientID)
	log.Printf("Removed client %s from connection pool", clientID)
}

func (s *TCPServer) startPingMonitor(conn *net.TCPConn, clientID string) {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			pingMsg := map[string]interface{}{
				"type":      "ping",
				"timestamp": time.Now().Unix(),
			}

			if err := s.sendJSONMessage(conn, pingMsg); err != nil {
				log.Printf("Failed to send ping to client %s: %v", clientID, err)
				s.removeClient(clientID)
				conn.Close()
				return
			}
		}
	}
}

func (s *TCPServer) handleConnection(conn *net.TCPConn, clientID string) {
	defer func() {
		conn.Close()
		s.removeClient(clientID)
		log.Printf("Connection closed for client %s", clientID)
	}()

	reader := bufio.NewReaderSize(conn, 8192)

	for {
		if err := conn.SetReadDeadline(time.Now().Add(readTimeout)); err != nil {
			log.Printf("Failed to set read deadline: %v", err)
			return
		}

		message, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				log.Printf("Client %s closed connection", clientID)
			} else if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				log.Printf("Read timeout for client %s, continuing", clientID)
				continue
			} else {
				log.Printf("Error reading from client %s: %v", clientID, err)
			}
			return
		}

		if err := s.processMessage(message, conn); err != nil {
			log.Printf("Error processing message from client %s: %v", clientID, err)
		}
	}
}

func (s *TCPServer) processMessage(message string, conn *net.TCPConn) error {
	var msg map[string]interface{}
	if err := json.Unmarshal([]byte(message), &msg); err != nil {
		return fmt.Errorf("failed to decode JSON: %v", err)
	}

	msgType, ok := msg["type"].(string)
	if !ok {
		return fmt.Errorf("missing or invalid 'type' field in message")
	}

	s.handlersMu.RLock()
	handler, exists := s.handlers[msgType]
	s.handlersMu.RUnlock()

	if !exists {
		log.Printf("No handler registered for message type: %s", msgType)
		return fmt.Errorf("unknown message type: %s", msgType)
	}

	return handler(msg, conn)
}

func (s *TCPServer) sendJSONMessage(conn *net.TCPConn, message interface{}) error {
	if err := conn.SetWriteDeadline(time.Now().Add(writeTimeout)); err != nil {
		return fmt.Errorf("failed to set write deadline: %v", err)
	}

	// Marshal the message
	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %v", err)
	}

	// Add newline for message framing
	data = append(data, '\n')

	// Send the message
	_, err = conn.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write message: %v", err)
	}

	return nil
}

func (s *TCPServer) handleHandshake(msg map[string]interface{}, conn *net.TCPConn) error {
	log.Printf("Received handshake from client: %v", msg)

	response := map[string]interface{}{
		"type":      "handshake_response",
		"status":    "success",
		"server":    "FXTrader_Server",
		"version":   "1.0",
		"timestamp": time.Now().Unix(),
	}

	return s.sendJSONMessage(conn, response)
}

func (s *TCPServer) handlePong(msg map[string]interface{}, conn *net.TCPConn) error {
	// Just log pong responses
	log.Printf("Received pong from client")
	return nil
}

func (s *TCPServer) handleDisconnect(msg map[string]interface{}, conn *net.TCPConn) error {
	reason, _ := msg["reason"].(string)
	log.Printf("Client initiated disconnect. Reason: %s", reason)
	return nil
}

// SendTradeRequest sends a trade request to MetaTrader
func (s *TCPServer) SendTradeRequest(tradeRequest map[string]interface{}) error {
	// Broadcast to all MT5 clients
	s.clientsMu.RLock()
	defer s.clientsMu.RUnlock()

	if len(s.clients) == 0 {
		return fmt.Errorf("no active MT5 connections available")
	}

	var lastErr error
	for clientID, conn := range s.clients {
		if err := s.sendJSONMessage(conn, tradeRequest); err != nil {
			log.Printf("Failed to send trade request to client %s: %v", clientID, err)
			lastErr = err
		} else {
			// Successfully sent to at least one client
			log.Printf("Trade request sent to client %s", clientID)
			return nil
		}
	}

	return lastErr
}

func (s *TCPServer) Stop() {
	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()

	for clientID, conn := range s.clients {
		log.Printf("Closing connection for client %s", clientID)
		conn.Close()
	}

	s.clients = make(map[string]*net.TCPConn)
}
