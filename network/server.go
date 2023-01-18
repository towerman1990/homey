package network

import (
	"context"
	"encoding/base64"
	"time"

	log "github.com/homey/logger"
	"go.uber.org/zap"

	"github.com/gorilla/websocket"
	"github.com/homey/distribute"
	"github.com/homey/service"
	"github.com/homey/utils"
	"github.com/labstack/echo/v4"
)

var (
	upgrader = websocket.Upgrader{}
)

type Server interface {

	// get connection manager
	GetConnectionManager() ConnectionManager

	// set a function it would be called on a connection openning
	SetOnConnOpen(func(conn Connection))

	// set a function it would be called on a connection closing
	SetOnConnClose(func(conn Connection))

	// call function on connection openning
	CallOnConnOpen(conn Connection)

	// call function on connection closing
	CallOnConnClose(conn Connection)
}

type Homey struct {
	Context context.Context

	ConnectionManager

	MessageHandler

	ForwardMsgChan chan *[]byte
}

func (h *Homey) Stop() {
	h.ConnectionManager.Clear()
}

func (h *Homey) AddRouter(msgID uint32, router Router) error {
	return h.MessageHandler.AddRouter(msgID, router)
}

func (h *Homey) GetConnectionManager() ConnectionManager {
	return h.ConnectionManager
}

func (h *Homey) SetOnConnOpen(func(conn Connection)) {

}

func (h *Homey) SetOnConnClose(func(conn Connection)) {

}

func (h *Homey) CallOnConnOpen(conn Connection) {

}

func (h *Homey) CallOnConnClose(conn Connection) {

}

func (h *Homey) Subscribe() {
	rdb := service.GetRedisClient()
	pubsub := rdb.Subscribe(h.Context, distribute.WorldChannel)
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

			if conn, err := h.ConnectionManager.Get(msg.GetConnID()); err == nil {
				conn.SendMsg(*data)
			}
		}
	}
}

func (h *Homey) Distribute() {
	go h.Subscribe()
	go h.ForwardMsgHandler()
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

		h.MessageHandler.StartWorkPool()
		conn := NewEchoConnection(id, h, c, ws)
		defer conn.Close()
		go conn.Open()
		switchs := 0
		for {
			conn.SendMsg([]byte("hello world"))
			time.Sleep(time.Second)
			switchs++
			if switchs == 3 {
				conn.Close()
			}
		}
		return
	}
}
