package network

import (
	"context"
	"encoding/base64"
	"os"

	"github.com/towerman1990/homey/config"
	log "github.com/towerman1990/homey/logger"
	"go.uber.org/zap"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"github.com/towerman1990/homey/distribute"
	"github.com/towerman1990/homey/utils"
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
		ConnectionManager() ConnectionManager

		MessageHandler() MessageHandler

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

		RedirectMsgChan chan *[]byte

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

func (h *Homey) ConnectionManager() ConnectionManager {
	return h.ConnManager
}

func (h *Homey) MessageHandler() MessageHandler {
	return h.MsgHandler
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

func (h *Homey) AddRouter(msgID uint32, router Router) {
	h.MsgHandler.AddRouter(msgID, router)
}

func (h *Homey) SubscribeWorldChannel() {
	rdb := distribute.GetRedisClient()
	pubsub := rdb.Subscribe(h.ctx, distribute.WorldChannel)
	defer pubsub.Close()

	for msg := range pubsub.Channel() {
		data, err := base64.StdEncoding.DecodeString(msg.Payload)
		if err != nil {
			log.Logger.Error("failed to base64 decode message", zap.String("error", err.Error()))
		}

		h.RedirectMsgChan <- &data
	}
}

func (h *Homey) RedirectMsgHandler() {
	for {
		select {
		case data := <-h.RedirectMsgChan:
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
	if !config.Global.Distribute.Status {
		log.Logger.Error("distribute status is false, please set the value true and configurate redis")
		os.Exit(1)
	}

	go h.SubscribeWorldChannel()
	go h.RedirectMsgHandler()
}

func (h *Homey) Echo() echo.HandlerFunc {
	return func(c echo.Context) (err error) {
		ws, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
		if err != nil {
			return err
		}

		id, err := utils.GenID()
		if err != nil {
			return err
		}

		conn := NewEchoConnection(id, h, c, ws)
		defer conn.Close()
		conn.Open()

		return
	}
}

func NewHomey(messageType int) *Homey {
	return &Homey{
		ctx:             context.Background(),
		msgType:         messageType,
		ConnManager:     NewConnectionManager(),
		MsgHandler:      NewMessageHandler(),
		RedirectMsgChan: make(chan *[]byte),
	}
}
