package network

import (
	"time"

	"github.com/gorilla/websocket"
	"github.com/homey/utils"
	"github.com/labstack/echo/v4"
)

var (
	upgrader = websocket.Upgrader{}
)

type Server interface {

	// get connection manager
	GetConnectionManager() ConnectionManager

	// set a function whitch could be called on a connection openning
	SetOnConnOpen(func(conn Connection))

	// set a function whitch could be called on a connection closing
	SetOnConnClose(func(conn Connection))

	// call function on connection openning
	CallOnConnOpen(conn Connection)

	// call function on connection closing
	CallOnConnClose(conn Connection)
}

type Homey struct {
	ConnectionManager
	MessageHandler
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
