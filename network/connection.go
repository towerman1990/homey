package network

import (
	"fmt"

	"github.com/gorilla/websocket"
	"github.com/homey/config"
	"github.com/labstack/echo/v4"
	"github.com/labstack/gommon/log"
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

	msgChan chan []byte

	exitChan chan bool

	isClosed bool

	context interface{}
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

	log.Infof("close connection [%d], remote addr [%s]", c.ID, c.Conn.RemoteAddr().String())

	c.exitChan <- true

	close(c.msgChan)
	close(c.exitChan)

	c.Conn.Close()

	c.server.GetConnectionManager().Remove(c)
}

func (c *connection) OpenReader() {
	defer log.Info("close reader")
	for {
		messageType, binaryMessage, err := c.Conn.ReadMessage()
		if err != nil {
			log.Errorf("failed to read message, error: %v", err)
			return
		}
		if messageType == websocket.TextMessage {
			log.Infof("recevie message: %s", binaryMessage)
		}

		msg, err := UnPack(binaryMessage)
		if err != nil {
			log.Errorf("connection [%d] unpack message failed, error: %v", c.ID, err)
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
	log.Info("writer goroutine opened")
	defer log.Info("close writer")
	for {
		select {
		case data := <-c.msgChan:
			if err := c.Conn.WriteMessage(c.messageType, data); err != nil {
				log.Errorf("failed to write message, error: %v", err)
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

	c.msgChan <- data

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
	log.Info(config.GlobalConfig.Message.Format)
	log.Info(config.GlobalConfig.Message.Endian)
	echoConn := &connection{
		ID:             id,
		server:         server,
		Conn:           conn,
		messageType:    messageType,
		messageHandler: NewMessageHandler(),
		msgChan:        make(chan []byte),
		exitChan:       make(chan bool),
		isClosed:       false,
		context:        context,
	}

	echoConn.server.GetConnectionManager().Add(echoConn)

	return echoConn
}
