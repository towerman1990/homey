package homey

import (
	"context"

	"github.com/homey/network"
)

func New() *network.Homey {
	return &network.Homey{
		Context:           context.Background(),
		ConnectionManager: network.NewConnectionManager(),
		MessageHandler:    network.NewMessageHandler(),
		ForwardMsgChan:    make(chan *[]byte),
	}
}
