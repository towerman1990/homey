package network

import (
	"fmt"
	"sync"

	log "github.com/towerman1990/homey/logger"
	"go.uber.org/zap"
)

type (
	ConnectionManager interface {

		// add a connection into connection manager
		Add(Connection)

		// remove a connection from connection manager
		Remove(Connection)

		// get a connection by connection ID
		Get(uint64) (Connection, error)

		// count how many connections in total
		Count() int

		// disconnect all connections
		Clear()
	}

	connectionManager struct {
		// all alive connections collection
		connections map[uint64]Connection

		lock sync.RWMutex
	}
)

func (cm *connectionManager) Add(conn Connection) {
	id := conn.GetID()
	cm.lock.Lock()
	cm.connections[id] = conn
	cm.lock.Unlock()
	log.Logger.Info("connection was added into connection manager successfully", zap.Uint64("connection", id))
}

func (cm *connectionManager) Remove(conn Connection) {
	if client, ok := conn.(*connection); ok {
		cm.unregister(client)
	} else {
		id := conn.GetID()
		cm.lock.Lock()
		delete(cm.connections, id)
		cm.lock.Unlock()
	}
	conn.Close()
}

func (cm *connectionManager) unregister(conn *connection) {
	cm.lock.Lock()
	if cm.connections[conn.ID] == conn {
		delete(cm.connections, conn.ID)
	}
	cm.lock.Unlock()
}

func (cm *connectionManager) Get(connID uint64) (conn Connection, err error) {
	cm.lock.RLock()
	defer cm.lock.RUnlock()
	if conn, ok := cm.connections[connID]; ok {
		return conn, err
	}

	return conn, fmt.Errorf("connection [%d] not found", connID)
}

func (cm *connectionManager) Count() int {
	cm.lock.RLock()
	defer cm.lock.RUnlock()
	return len(cm.connections)
}

func (cm *connectionManager) Clear() {
	cm.lock.Lock()
	connections := cm.connections
	cm.connections = make(map[uint64]Connection)
	cm.lock.Unlock()
	for _, conn := range connections {
		conn.Close()
	}
}

func NewConnectionManager() ConnectionManager {
	return &connectionManager{
		connections: make(map[uint64]Connection),
		lock:        sync.RWMutex{},
	}
}
