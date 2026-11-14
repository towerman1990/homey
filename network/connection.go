package network

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/towerman1990/homey/config"
	"github.com/towerman1990/homey/distribute"
	log "github.com/towerman1990/homey/logger"
	"go.uber.org/zap"
)

var (
	ErrConnectionClosed = errors.New("homey: connection closed")
	ErrSendQueueFull    = errors.New("homey: send queue full")

	WriteWait            = 10 * time.Second
	PongWait             = 60 * time.Second
	PingPeriod           = (PongWait * 9) / 10
	MaxMessageSize int64 = 64 * 1024
)

type Connection interface {
	GetID() uint64
	Open()
	Close()
	StartReader()
	StartWriter()
	SendMsg(data []byte) error
	Context() context.Context
}

type connection struct {
	ID           uint64
	server       Server
	Conn         *websocket.Conn
	sendMsgChan  chan []byte
	ctx          context.Context
	cancel       context.CancelFunc
	settings     config.GlobalConfig
	openOnce     sync.Once
	closeOnce    sync.Once
	readerOnce   sync.Once
	writerOnce   sync.Once
	propertyLock sync.RWMutex
	properties   map[string]interface{}
}

func (c *connection) GetID() uint64 {
	return c.ID
}

func (c *connection) Open() {
	c.openOnce.Do(c.run)
}

func (c *connection) run() {
	defer c.Close()
	if c.ctx.Err() != nil {
		return
	}
	if err := c.server.CallOnConnOpen(c); err != nil {
		log.Logger.Warn("connection open hook rejected connection", zap.Uint64("connection", c.ID), zap.Error(err))
		return
	}

	defer func() {
		c.Close()
		c.server.CallOnConnClose(c)
	}()
	go c.StartWriter()
	c.StartReader()
}

func (c *connection) Close() {
	closed := false
	c.closeOnce.Do(func() {
		closed = true
		c.cancel()
		_ = c.Conn.Close()
		if manager, ok := c.server.ConnectionManager().(*connectionManager); ok {
			manager.unregister(c)
			closed = false
		}
	})
	if closed {
		c.server.ConnectionManager().Remove(c)
	}
}

func (c *connection) StartReader() {
	c.readerOnce.Do(c.readLoop)
}

func (c *connection) readLoop() {
	defer c.Close()
	readLimit := MaxMessageSize
	if c.settings.MaxPackageSize > 0 {
		packetLimit := int64(c.settings.MaxPackageSize)
		if c.settings.TLV.Type {
			packetLimit += 4
		}
		if c.settings.TLV.Length {
			packetLimit += 4
		}
		if readLimit <= 0 || packetLimit < readLimit {
			readLimit = packetLimit
		}
	}
	c.Conn.SetReadLimit(readLimit)
	if err := c.Conn.SetReadDeadline(time.Now().Add(PongWait)); err != nil {
		return
	}
	c.Conn.SetPongHandler(func(string) error {
		return c.Conn.SetReadDeadline(time.Now().Add(PongWait))
	})

	codec := newMessageCodec(c.settings)
	for c.ctx.Err() == nil {
		messageType, data, err := c.Conn.ReadMessage()
		if err != nil {
			return
		}
		if messageType != c.server.GetMsgType() {
			return
		}
		msg, err := codec.unpack(data, false)
		if err != nil {
			log.Logger.Warn("failed to unpack message", zap.Uint64("connection", c.ID), zap.Error(err))
			return
		}
		req := &request{conn: c, msg: msg}
		c.server.MessageHandler().SendMsgToTaskQueue(req)
	}
}

func (c *connection) StartWriter() {
	c.writerOnce.Do(c.writeLoop)
}

func (c *connection) writeLoop() {
	defer c.Close()
	ticker := time.NewTicker(PingPeriod)
	defer ticker.Stop()

	for c.ctx.Err() == nil {
		select {
		case data := <-c.sendMsgChan:
			if err := c.Conn.SetWriteDeadline(time.Now().Add(WriteWait)); err != nil {
				return
			}
			if err := c.Conn.WriteMessage(c.server.GetMsgType(), data); err != nil {
				return
			}
		case <-ticker.C:
			if err := c.Conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(WriteWait)); err != nil {
				return
			}
		case <-c.ctx.Done():
			return
		}
	}
}

// SendMsg queues a copy of data for the writer.
func (c *connection) SendMsg(data []byte) error {
	if c.ctx.Err() != nil {
		return ErrConnectionClosed
	}
	select {
	case <-c.ctx.Done():
		return ErrConnectionClosed
	case c.sendMsgChan <- append([]byte(nil), data...):
		return nil
	default:
		return ErrSendQueueFull
	}
}

func (c *connection) SendForwardMsg(data []byte) error {
	if c.ctx.Err() != nil {
		return ErrConnectionClosed
	}
	if h, ok := c.server.(*Homey); ok {
		return h.publishForward(c.ctx, data)
	}
	return distribute.PublishForwardMsg(c.ctx, data)
}

func (c *connection) GetStatus() bool {
	return c.ctx.Err() != nil
}

func (c *connection) Context() context.Context {
	return c.ctx
}

func (c *connection) SetProperty(key string, value interface{}) {
	c.propertyLock.Lock()
	defer c.propertyLock.Unlock()
	c.properties[key] = value
}

func (c *connection) GetProperty(key string) (interface{}, bool) {
	c.propertyLock.RLock()
	defer c.propertyLock.RUnlock()
	value, ok := c.properties[key]
	return value, ok
}

func NewEchoConnection(id uint64, server Server, conn *websocket.Conn) Connection {
	settings := config.Global
	if h, ok := server.(*Homey); ok {
		settings = h.settings
	}
	ctx, cancel := context.WithCancel(server.Context())
	client := &connection{
		ID:          id,
		server:      server,
		Conn:        conn,
		sendMsgChan: make(chan []byte, 64),
		ctx:         ctx,
		cancel:      cancel,
		settings:    settings,
		properties:  make(map[string]interface{}),
	}
	server.ConnectionManager().Add(client)
	if ctx.Err() != nil {
		client.Close()
	}
	return client
}
