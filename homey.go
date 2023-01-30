package homey

import (
	"github.com/gorilla/websocket"
	"github.com/towerman1990/homey/config"
	"github.com/towerman1990/homey/network"
)

func New() (homey *network.Homey) {
	messageType := websocket.BinaryMessage
	if config.Global.Message.Format == "text" {
		messageType = websocket.TextMessage
	}

	homey = network.NewHomey(messageType)
	homey.MsgHandler.StartWorkPool()

	return
}
