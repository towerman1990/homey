package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/towerman1990/homey"
	"github.com/towerman1990/homey/config"
	"github.com/towerman1990/homey/network"
)

type DefaultHandler struct {
	network.BaseRouter
}

func (dh *DefaultHandler) Handle(request network.Request) (err error) {
	msg := string(request.GetMsgData())
	log.Printf("receive request: %s", msg)
	return request.GetConnection().SendMsg([]byte("server received message: " + msg))
}

func main() {
	// Echo instance
	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	cfg, err := config.LoadFile("conf/homey.yaml")
	if err != nil {
		log.Fatal(err)
	}
	h, err := homey.NewWithConfig(cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer h.Stop()
	if cfg.Distribute.Status {
		if err := h.Distribute(); err != nil {
			log.Printf("start Redis distribution: %v", err)
			return
		}
	}
	h.SetOnConnOpen(OnConnectionAdd)

	h.AddRouter(0, &DefaultHandler{})
	h.MsgHandler.String()

	// Routes
	e.Static("/", "./public")
	e.GET("/ws", h.Echo())

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	server := &http.Server{Addr: ":8080", Handler: e, ReadHeaderTimeout: 5 * time.Second}
	serverErr := make(chan error, 1)
	go func() { serverErr <- e.StartServer(server) }()
	select {
	case err := <-serverErr:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Printf("start HTTP server: %v", err)
		}
	case <-ctx.Done():
		h.Stop()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("shut down HTTP server: %v", err)
		}
	}
}

func OnConnectionAdd(conn network.Connection) (err error) {
	log.Printf("call OnConnectionAdd function, call connection ID: %d", conn.GetID())
	return
}
