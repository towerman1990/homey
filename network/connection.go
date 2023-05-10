package network

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/towerman1990/homey/config"
	"github.com/towerman1990/homey/distribute"
	log "github.com/towerman1990/homey/logger"
	"go.uber.org/zap"
)

var (
	// Time allowed to write a message to the peer.
	WriteWait = 10 * time.Second
	// Time allowed to read the next pong message from the peer.
	PongWait = 60 * time.Second
	// Send pings to peer with this period. Must be less than pongWait.
	PingPeriod = (PongWait * 9) / 10
	// Maximum message size allowed from peer.
	MaxMessageSize int64 = 64 * 1024
)

type (
	Connection interface {

		// get connecton ID
		GetID() uint64

		// establish a connection between server and client
		Open()

		// close a connection
		Close()

		// reading message from websocket connection
		StartReader()

		// prepare for writing message into websocket connection
		StartWriter()

		// server send message to client by connection
		SendMsg(data []byte) error

		Context() context.Context
	}

	connection struct {
		ID uint64

		server Server

		Conn *websocket.Conn

		sendMsgChan chan *[]byte

		ctx context.Context

		cancel context.CancelFunc

		properties map[string]interface{}

		sync.RWMutex

		propertyLock sync.RWMutex

		isClosed bool
	}
)

func (c *connection) GetID() uint64 {
	return c.ID
}

func (c *connection) Open() {
	c.ctx, c.cancel = context.WithCancel(context.Background())

	if err := c.server.CallOnConnOpen(c); err != nil {
		log.Logger.Warn("connection [%d] open failed, error: %v", zap.Uint64("connection", c.ID), zap.String("error", err.Error()))
		return
	}

	go c.StartReader()
	go c.StartWriter()

	select {
	case <-c.ctx.Done():
		c.finalizer()
		return
	}
}

func (c *connection) Close() {
	c.cancel()
}

func (c *connection) finalizer() {
	c.server.CallOnConnClose(c)

	c.Lock()
	defer c.Unlock()

	if c.isClosed == true {
		return
	}

	log.Logger.Info("ready to close connection", zap.Uint64("connection", c.ID), zap.String("remote addr", c.Conn.RemoteAddr().String()))
	close(c.sendMsgChan)
	err := c.Conn.Close()
	if err != nil {
		log.Logger.Error("close connection [%d] failed, error: %v", zap.Uint64("connection", c.ID), zap.String("error", err.Error()))
	}
	c.isClosed = true

	c.server.ConnectionManager().Remove(c)
}

func (c *connection) StartReader() {
	defer c.Close()
	defer log.Logger.Info("close connection reader", zap.Uint64("id", c.ID))

	for {
		messageType, binaryMessage, err := c.Conn.ReadMessage()
		if err != nil {
			log.Logger.Error("failed to read message", zap.Uint64("connection", c.ID), zap.String("error", err.Error()))
			return
		}

		log.Logger.Info("received message", zap.Int("message type", messageType))

		msg, err := UnPack(binaryMessage, false)
		if err != nil {
			log.Logger.Error("failed to unpack message", zap.Uint64("connection", c.ID), zap.String("error", err.Error()))
			break
		}

		req := &request{
			conn: c,
			msg:  msg,
		}

		if config.Global.WorkerPoolSize > 0 {
			c.server.MessageHandler().SendMsgToTaskQueue(req)
		} else {
			go c.server.MessageHandler().ExecHandler(req)
		}
	}
}

func (c *connection) StartWriter() {
	defer log.Logger.Info("close connection writer", zap.Uint64("id", c.ID))
	ticker := time.NewTicker(PingPeriod)
	defer ticker.Stop()

	for {
		select {
		case data := <-c.sendMsgChan:
			if err := c.Conn.WriteMessage(c.server.GetMsgType(), *data); err != nil {
				log.Logger.Error("failed to write message", zap.Uint64("connection", c.ID), zap.String("error", err.Error()))
				c.Close()
				return
			}
		case <-ticker.C:
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Logger.Error("failed to ping client", zap.Uint64("connection", c.ID), zap.String("error", err.Error()))
				return
			}
		case <-c.ctx.Done():
			return
		}
	}
}

func (c *connection) SendMsg(data []byte) (err error) {
	if c.isClosed {
		return fmt.Errorf("connection [%d] has closed", c.ID)
	}

	c.sendMsgChan <- &data
	return
}

func (c *connection) SendForwardMsg(data []byte) (err error) {
	if c.isClosed {
		return fmt.Errorf("connection [%d] has closed", c.ID)
	}

	err = distribute.PublishForwardMsg(c.ctx, data)
	return
}

func (c *connection) GetStatus() bool {
	return c.isClosed
}

func (c *connection) Context() context.Context {
	return c.ctx
}

func (c *connection) SetProperty(key string, value interface{}) {
	c.propertyLock.RLock()
	defer c.propertyLock.RUnlock()

	c.properties[key] = value
}

func (c *connection) GetProperty(key string) (value interface{}, ok bool) {
	c.propertyLock.RLock()
	defer c.propertyLock.RUnlock()

	value, ok = c.properties[key]
	return
}

func NewEchoConnection(id uint64, server Server, conn *websocket.Conn) Connection {
	echoConn := &connection{
		ID:          id,
		server:      server,
		Conn:        conn,
		sendMsgChan: make(chan *[]byte),
	}
	echoConn.server.ConnectionManager().Add(echoConn)

	return echoConn
}
