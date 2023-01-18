package network

import (
	"context"
	"fmt"

	"github.com/gorilla/websocket"
	"github.com/homey/config"
	"github.com/homey/distribute"
	log "github.com/homey/logger"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

type Connection interface {

	// get connecton ID
	GetID() uint64

	// establish a connection between server and client
	Open()

	// close a connection
	Close()

	// open connection reader
	OpenReader()

	// open connection writer
	OpenWriter()

	// server send message to client by connection
	SendMsg(data []byte) error
}

type connection struct {
	ID uint64

	server Server

	Conn *websocket.Conn

	messageType int

	messageHandler MessageHandler

	sendMsgChan chan *[]byte

	exitChan chan bool

	isClosed bool

	context context.Context
}

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
	c.isClosed = true

	log.Logger.Info("close connection", zap.Uint64("connection", c.ID), zap.String("remote address", c.Conn.RemoteAddr().String()))

	c.exitChan <- true

	close(c.sendMsgChan)
	close(c.exitChan)

	c.Conn.Close()

	c.server.GetConnectionManager().Remove(c)
}

func (c *connection) OpenReader() {
	defer log.Logger.Info("close connection reader", zap.Uint64("id", c.ID))

	for {
		messageType, binaryMessage, err := c.Conn.ReadMessage()
		if err != nil {
			log.Logger.Error("failed to read message", zap.Uint64("connection", c.ID), zap.String("error", err.Error()))
			return
		}

		if messageType == websocket.TextMessage && config.GlobalConfig.Message.Format != "text" {
			log.Logger.Error("invalid message type", zap.Uint64("connection", c.ID), zap.String("error", "unsupport text message type"))
			return
		}

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
			c.messageHandler.SendMsgToTaskQueue(req)
		} else {
			go c.messageHandler.ExecHandler(req)
		}
	}
}

func (c *connection) OpenWriter() {
	defer log.Logger.Info("close connection writer", zap.Uint64("id", c.ID))

	for {
		select {
		case data := <-c.sendMsgChan:
			if err := c.Conn.WriteMessage(c.messageType, *data); err != nil {
				log.Logger.Error("failed to write message", zap.Uint64("connection", c.ID), zap.String("error", err.Error()))
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

	err = distribute.PublishForwardMsg(c.context, data)

	return
}

func (c *connection) GetStatus() bool {
	return c.isClosed
}

func NewEchoConnection(id uint64, server Server, context echo.Context, conn *websocket.Conn) Connection {
	messageType := websocket.TextMessage
	if config.GlobalConfig.Message.Format == "binary" {
		messageType = websocket.BinaryMessage
	}
	log.Logger.Info(config.GlobalConfig.Message.Format)
	log.Logger.Info(config.GlobalConfig.Message.Endian)
	echoConn := &connection{
		ID:             id,
		server:         server,
		Conn:           conn,
		messageType:    messageType,
		messageHandler: NewMessageHandler(),
		sendMsgChan:    make(chan *[]byte),
		exitChan:       make(chan bool),
		isClosed:       false,
		context:        context.Request().Context(),
	}

	echoConn.server.GetConnectionManager().Add(echoConn)

	return echoConn
}
