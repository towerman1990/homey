package network

import (
	"context"
	"encoding/base64"
	"os"

	"github.com/homey/config"
	log "github.com/homey/logger"
	"go.uber.org/zap"

	"github.com/gorilla/websocket"
	"github.com/homey/distribute"
	"github.com/homey/utils"
	"github.com/labstack/echo/v4"
)

var (
	upgrader = websocket.Upgrader{}
)

type (
	Server interface {
		Context() context.Context
		// get message type
		GetMsgType() int

		// get connection manager
		GetConnManager() ConnectionManager

		// set a function it would be called on http request arrive
		SetOnInit(func(context.Context))

		// set a function it would be called on a connection openning
		SetOnConnOpen(func(conn Connection))

		// set a function it would be called on a connection closing
		SetOnConnClose(func(conn Connection))

		// call this function on http request arrive
		CallOnInit(context.Context)

		// call this function on connection openning
		CallOnConnOpen(Connection)

		// call this function on connection closing
		CallOnConnClose(Connection)
	}

	Homey struct {
		ctx context.Context

		msgType int

		ConnManager ConnectionManager

		MsgHandler MessageHandler

		ForwardMsgChan chan *[]byte

		OnInit func(context.Context)

		OnConnOpen func(Connection)

		OnConnClose func(Connection)
	}
)

func (h *Homey) Context() context.Context {
	return h.ctx
}

func (h *Homey) GetMsgType() int {
	return h.msgType
}

func (h *Homey) GetConnManager() ConnectionManager {
	return h.ConnManager
}

func (h *Homey) SetOnInit(hookFunc func(context.Context)) {
	h.OnInit = hookFunc
}

func (h *Homey) SetOnConnOpen(hookFunc func(Connection)) {
	h.OnConnOpen = hookFunc
}

func (h *Homey) SetOnConnClose(hookFunc func(Connection)) {
	h.OnConnClose = hookFunc
}

func (h *Homey) CallOnInit(ctx context.Context) {
	if h.OnConnOpen != nil {
		h.OnInit(ctx)
	}
}

func (h *Homey) CallOnConnOpen(conn Connection) {
	if h.OnConnOpen != nil {
		h.OnConnOpen(conn)
	}
}

func (h *Homey) CallOnConnClose(conn Connection) {
	if h.OnConnClose != nil {
		h.OnConnClose(conn)
	}
}

func (h *Homey) Stop() {
	h.ConnManager.Clear()
}

func (h *Homey) AddRouter(msgID uint32, router Router) error {
	return h.MsgHandler.AddRouter(msgID, router)
}
func (h *Homey) Subscribe() {
	rdb := distribute.GetRedisClient()
	pubsub := rdb.Subscribe(h.Context(), distribute.WorldChannel)
	defer pubsub.Close()

	for msg := range pubsub.Channel() {
		data, err := base64.StdEncoding.DecodeString(msg.Payload)
		if err != nil {
			log.Logger.Error("failed to base64 decode message", zap.String("error", err.Error()))
		}

		h.ForwardMsgChan <- &data
	}
}

func (h *Homey) ForwardMsgHandler() {
	for {
		select {
		case data := <-h.ForwardMsgChan:
			msg, err := UnPack(*data, true)
			if err != nil {
				log.Logger.Error("failed to unpack forward msg", zap.String("error", err.Error()))
			}

			if conn, err := h.ConnManager.Get(msg.GetConnID()); err == nil {
				conn.SendMsg(*data)
			}
		}
	}
}

func (h *Homey) Distribute() {
	if !config.GlobalConfig.Distribute.Status {
		log.Logger.Error("distribute status is false, please set the value true and configurate redis")
		os.Exit(1)
	}

	go h.Subscribe()
	go h.ForwardMsgHandler()
}

func (h *Homey) Echo() echo.HandlerFunc {
	return func(c echo.Context) (err error) {
		ws, err := upgrader.Upgrade(c.Response(), c.Request(), c.Request().Header)
		if err != nil {
			return err
		}

		id, err := utils.GenID()
		if err != nil {
			return err
		}

		h.MsgHandler.StartWorkPool()
		conn := NewEchoConnection(id, h, c, ws)
		defer conn.Close()
		go conn.Open()

		return
	}
}

func NewHomey(messageType int) *Homey {
	return &Homey{
		ctx:            context.Background(),
		msgType:        messageType,
		ConnManager:    NewConnectionManager(),
		MsgHandler:     NewMessageHandler(),
		ForwardMsgChan: make(chan *[]byte),
	}
}
