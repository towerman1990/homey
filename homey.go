package homey

import "github.com/homey/network"

func New() *network.Homey {
	return &network.Homey{
		ConnectionManager: network.NewConnectionManager(),
		MessageHandler:    network.NewMessageHandler(),
	}
}
