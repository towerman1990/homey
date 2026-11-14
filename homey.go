package homey

import (
	"github.com/gorilla/websocket"
	"github.com/towerman1990/homey/config"
	"github.com/towerman1990/homey/network"
)

func New() *network.Homey {
	return newServer(config.Global)
}

func NewWithConfig(cfg config.GlobalConfig) (*network.Homey, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return newServer(cfg), nil
}

func newServer(cfg config.GlobalConfig) *network.Homey {
	messageType := websocket.BinaryMessage
	if cfg.Message.Format == "text" {
		messageType = websocket.TextMessage
	}
	return network.NewHomeyWithConfig(messageType, cfg)
}
