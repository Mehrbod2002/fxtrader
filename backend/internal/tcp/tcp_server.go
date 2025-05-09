package tcp

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"sync"
)

type HandlerFunc func(message map[string]interface{}) error

type TCPServer struct {
	listenAddr   *net.TCPAddr
	handlers     map[string]HandlerFunc
	handlersMu   sync.RWMutex
	responseChan chan interface{}
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
	}, nil
}

func (s *TCPServer) RegisterHandler(msgType string, handler HandlerFunc) {
	s.handlersMu.Lock()
	defer s.handlersMu.Unlock()
	s.handlers[msgType] = handler
}

func (s *TCPServer) Start() error {
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
			go s.handleConnection(conn)
		}
	}()

	return nil
}

func (s *TCPServer) handleConnection(conn *net.TCPConn) {
	defer conn.Close()
	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		log.Printf("Failed to read from TCP connection: %v", err)
		return
	}

	var msg map[string]interface{}
	if err := json.Unmarshal(buf[:n], &msg); err != nil {
		log.Printf("Failed to unmarshal TCP message: %v", err)
		return
	}

	msgType, ok := msg["type"].(string)
	if !ok {
		log.Printf("Missing or invalid message type in TCP message")
		return
	}

	s.handlersMu.RLock()
	handler, exists := s.handlers[msgType]
	s.handlersMu.RUnlock()

	if !exists {
		log.Printf("No handler registered for message type: %s", msgType)
		return
	}

	if err := handler(msg); err != nil {
		log.Printf("Failed to handle message type %s: %v", msgType, err)
	}
}
