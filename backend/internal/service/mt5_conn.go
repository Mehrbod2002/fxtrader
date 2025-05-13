package service

import (
	"log"
	"net"
	"sync"
)

type MT5ConnManager struct {
	conn   *net.TCPConn
	connMu sync.Mutex
}

var mt5ConnManager = &MT5ConnManager{}

func GetMT5Conn() *net.TCPConn {
	mt5ConnManager.connMu.Lock()
	defer mt5ConnManager.connMu.Unlock()
	return mt5ConnManager.conn
}

func SetMT5Conn(conn *net.TCPConn) {
	mt5ConnManager.connMu.Lock()
	defer mt5ConnManager.connMu.Unlock()
	if mt5ConnManager.conn != nil {
		if err := mt5ConnManager.conn.Close(); err != nil {
			log.Printf("Failed to close existing MT5 connection: %v", err)
		}
	}
	mt5ConnManager.conn = conn
	log.Printf("Set new MT5 connection from %s", conn.RemoteAddr().String())
}

func CloseMT5Conn() error {
	mt5ConnManager.connMu.Lock()
	defer mt5ConnManager.connMu.Unlock()
	if mt5ConnManager.conn != nil {
		err := mt5ConnManager.conn.Close()
		mt5ConnManager.conn = nil
		if err != nil {
			return err
		}
		log.Println("MT5 connection closed")
	}
	return nil
}
