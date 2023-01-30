package main

import (
	"log"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/towerman1990/homey"
	"github.com/towerman1990/homey/network"
)

type DefaultHandler struct {
	network.BaseRouter
}

func (dh *DefaultHandler) Handle(request network.Request) (err error) {
	msg := string(request.GetMsgData())
	log.Printf("receive request: %s", msg)
	request.GetConnection().SendMsg([]byte("server received message: " + msg))

	return
}

func main() {
	// Echo instance
	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	h := homey.New()
	h.SetOnConnOpen(OnConnectionAdd)

	h.AddRouter(0, &DefaultHandler{})
	h.MsgHandler.String()

	// Routes
	e.Static("/", "./public")
	e.GET("/ws", h.Echo())

	// Start server
	e.Logger.Fatal(e.Start(":8080"))
}

func OnConnectionAdd(conn network.Connection) {
	log.Printf("call OnConnectionAdd function, call connection ID: %d", conn.GetID())
}
