package network

import (
	"fmt"
	"log"
	"sync"
)

type ConnectionManager interface {

	// add a connection into connection manager
	Add(Connection)

	// remove a connection from connection manager
	Remove(Connection)

	// get a connection by connection ID
	Get(int64) (Connection, error)

	// count how many connections in total
	Count() int

	// disconnect all connections
	Clear()
}

type connectionManager struct {
	// all connections map
	connections map[int64]Connection

	lock sync.RWMutex
}

func (cm *connectionManager) Add(conn Connection) {
	cm.lock.Lock()
	defer cm.lock.Unlock()

	cm.connections[conn.GetID()] = conn
	log.Printf("add connection to connection_manager successfully, connID = [%d]", conn.GetID())
}

func (cm *connectionManager) Remove(conn Connection) {
	cm.lock.Lock()
	defer cm.lock.Unlock()
	conn.Close()
	delete(cm.connections, conn.GetID())
}

func (cm *connectionManager) Get(connID int64) (conn Connection, err error) {
	cm.lock.RLock()
	defer cm.lock.RUnlock()
	if conn, ok := cm.connections[conn.GetID()]; ok {
		return conn, err
	}

	return conn, fmt.Errorf("connection [%d] added failed", connID)
}

func (cm *connectionManager) Count() int {
	return len(cm.connections)
}

func (cm *connectionManager) Clear() {
	cm.lock.Lock()
	defer cm.lock.Unlock()

	for _, conn := range cm.connections {
		cm.Remove(conn)
	}
}

func NewConnectionManager() ConnectionManager {
	return &connectionManager{
		connections: make(map[int64]Connection),
		lock:        sync.RWMutex{},
	}
}
