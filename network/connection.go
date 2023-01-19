package network

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
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
		OpenReader()

		// prepare for writing message into websocket connection
		OpenWriter()

		// server send message to client by connection
		SendMsg(data []byte) error

		Context() context.Context
	}

	connection struct {
		ID uint64

		server Server

		Conn *websocket.Conn

		sendMsgChan chan *[]byte

		exitChan chan bool

		isClosed bool

		ctx context.Context

		properties map[string]interface{}

		propertyLock sync.RWMutex
	}
)

func (c *connection) GetID() uint64 {
	return c.ID
}

func (c *connection) Open() {
	go c.OpenReader()
	go c.OpenWriter()

	c.server.CallOnConnOpen(c)
	select {}
}

func (c *connection) Close() {
	if c.isClosed {
		return
	}

	log.Logger.Info("close connection", zap.Uint64("connection", c.ID), zap.String("remote address", c.Conn.RemoteAddr().String()))

	c.isClosed = true
	c.exitChan <- true

	close(c.sendMsgChan)
	close(c.exitChan)

	err := c.Conn.Close()
	if err != nil {
		log.Logger.Error("close connection failed", zap.Uint64("connection", c.ID), zap.String("error", err.Error()))
	}

	c.server.ConnectionManager().Remove(c)
}

func (c *connection) OpenReader() {
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

		if config.GlobalConfig.WorkerPoolSize > 0 {
			c.server.MessageHandler().SendMsgToTaskQueue(req)
		} else {
			go c.server.MessageHandler().ExecHandler(req)
		}
	}
}

func (c *connection) OpenWriter() {
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
		case <-c.exitChan:
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

func NewEchoConnection(id uint64, server Server, ctx echo.Context, conn *websocket.Conn) Connection {
	echoConn := &connection{
		ID:          id,
		server:      server,
		Conn:        conn,
		sendMsgChan: make(chan *[]byte),
		exitChan:    make(chan bool),
		isClosed:    false,
		ctx:         ctx.Request().Context(),
	}

	echoConn.server.ConnectionManager().Add(echoConn)

	return echoConn
}
