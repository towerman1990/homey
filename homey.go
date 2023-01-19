package homey

import (
	"github.com/gorilla/websocket"
	"github.com/homey/config"
	"github.com/homey/network"
)

func New() (homey *network.Homey) {
	messageType := websocket.BinaryMessage
	if config.GlobalConfig.Message.Format == "text" {
		messageType = websocket.TextMessage
	}

	homey = network.NewHomey(messageType)

	return
}
