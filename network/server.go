package network

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-redis/redis/v9"

	"github.com/towerman1990/homey/config"
	log "github.com/towerman1990/homey/logger"
	"go.uber.org/zap"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"github.com/sony/sonyflake"
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
		SetOnConnOpen(func(conn Connection) error)

		// set a function it would be called on a connection closing
		SetOnConnClose(func(conn Connection))

		// call this function on http request arrive
		CallOnInit(context.Context)

		// call this function on connection openning
		CallOnConnOpen(Connection) error

		// call this function on connection closing
		CallOnConnClose(Connection)
	}

	Homey struct {
		ctx         context.Context
		cancel      context.CancelFunc
		settings    config.GlobalConfig
		nextID      atomic.Uint64
		idGenerator *sonyflake.Sonyflake
		redisMu     sync.Mutex
		redisClient *redis.Client

		msgType int

		ConnManager ConnectionManager

		MsgHandler MessageHandler

		RedirectMsgChan chan *[]byte

		OnInit func(context.Context)

		OnConnOpen func(Connection) error

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

func (h *Homey) SetOnConnOpen(hookFunc func(Connection) error) {
	h.OnConnOpen = hookFunc
}

func (h *Homey) SetOnConnClose(hookFunc func(Connection)) {
	h.OnConnClose = hookFunc
}

func (h *Homey) CallOnInit(ctx context.Context) {
	if h.OnInit != nil {
		h.OnInit(ctx)
	}
}

func (h *Homey) CallOnConnOpen(conn Connection) (err error) {
	if h.OnConnOpen != nil {
		return h.OnConnOpen(conn)
	}

	return
}

func (h *Homey) CallOnConnClose(conn Connection) {
	if h.OnConnClose != nil {
		h.OnConnClose(conn)
	}
}

func (h *Homey) Stop() {
	h.cancel()
	h.ConnManager.Clear()
	h.redisMu.Lock()
	defer h.redisMu.Unlock()
	if h.redisClient != nil {
		_ = h.redisClient.Close()
		h.redisClient = nil
	}
}

func (h *Homey) AddRouter(msgID uint32, router Router) {
	h.MsgHandler.AddRouter(msgID, router)
}

func (h *Homey) SubscribeWorldChannel() {
	h.redisMu.Lock()
	client := h.redisClient
	h.redisMu.Unlock()
	if client != nil {
		h.subscribe(client, h.settings.Redis.WorldChannel)
	}
}

func (h *Homey) subscribe(client *redis.Client, channels ...string) {
	pubsub := client.Subscribe(h.ctx)
	defer pubsub.Close()
	if err := pubsub.Subscribe(h.ctx, channels...); err != nil {
		log.Logger.Warn("failed to subscribe to Redis", zap.Error(err))
		return
	}
	h.readSubscription(pubsub)
}

func (h *Homey) readSubscription(pubsub *redis.PubSub) {
	messages := pubsub.Channel()
	for {
		select {
		case <-h.ctx.Done():
			return
		case msg, ok := <-messages:
			if !ok {
				return
			}
			data, err := base64.StdEncoding.DecodeString(msg.Payload)
			if err != nil {
				log.Logger.Warn("failed to decode forwarded message", zap.Error(err))
				continue
			}
			select {
			case h.RedirectMsgChan <- &data:
			case <-h.ctx.Done():
				return
			}
		}
	}
}

func (h *Homey) RedirectMsgHandler() {
	codec := newMessageCodec(h.settings)
	for {
		select {
		case <-h.ctx.Done():
			return
		case data, ok := <-h.RedirectMsgChan:
			if !ok {
				return
			}
			if data == nil {
				continue
			}
			msg, err := codec.unpack(*data, true)
			if err != nil {
				log.Logger.Warn("failed to unpack forwarded message", zap.Error(err))
				continue
			}

			if conn, err := h.ConnManager.Get(msg.GetConnID()); err == nil {
				// The connection ID belongs to the Redis envelope.
				if err := conn.SendMsg((*data)[8:]); err != nil {
					log.Logger.Warn("failed to forward message", zap.Error(err))
				}
			}
		}
	}
}

func (h *Homey) Distribute() error {
	h.redisMu.Lock()
	defer h.redisMu.Unlock()

	if err := h.ctx.Err(); err != nil {
		return err
	}
	if !h.settings.Distribute.Status {
		return errors.New("homey: distribution is disabled")
	}
	if err := h.settings.Validate(); err != nil {
		return err
	}
	if h.redisClient != nil {
		return nil
	}
	client := distribute.NewClient(h.settings.Redis)
	ctx, cancel := context.WithTimeout(h.ctx, 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return err
	}

	pubsub := client.Subscribe(ctx)
	if err := pubsub.Subscribe(ctx, h.settings.Redis.WorldChannel, h.settings.Redis.ForwardChannel); err != nil {
		_ = pubsub.Close()
		_ = client.Close()
		return err
	}
	h.redisClient = client
	go func() {
		defer pubsub.Close()
		h.readSubscription(pubsub)
	}()
	go h.RedirectMsgHandler()
	return nil
}

func (h *Homey) publishForward(ctx context.Context, data []byte) error {
	h.redisMu.Lock()
	client := h.redisClient
	h.redisMu.Unlock()
	if client == nil {
		return errors.New("homey: distribution is not running")
	}
	return distribute.Publish(ctx, client, h.settings.Redis.ForwardChannel, data)
}

func (h *Homey) Echo() echo.HandlerFunc {
	return func(c echo.Context) (err error) {
		if h.ctx.Err() != nil {
			return echo.NewHTTPError(http.StatusServiceUnavailable)
		}
		h.CallOnInit(c.Request().Context())
		var id uint64
		if h.settings.Distribute.Status {
			if h.idGenerator == nil {
				return echo.NewHTTPError(http.StatusServiceUnavailable, "distributed connection IDs are unavailable")
			}
			id, err = h.idGenerator.NextID()
			if err != nil {
				return err
			}
		} else {
			id = h.nextID.Add(1)
		}
		ws, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
		if err != nil {
			return err
		}

		conn := NewEchoConnection(id, h, ws)
		conn.Open()

		return
	}
}

func NewHomey(messageType int) *Homey {
	return NewHomeyWithConfig(messageType, config.Global)
}

func NewHomeyWithConfig(messageType int, cfg config.GlobalConfig) *Homey {
	ctx, cancel := context.WithCancel(context.Background())
	h := &Homey{
		ctx:             ctx,
		cancel:          cancel,
		settings:        cfg,
		msgType:         messageType,
		ConnManager:     NewConnectionManager(),
		MsgHandler:      newMessageHandler(ctx, cfg),
		RedirectMsgChan: make(chan *[]byte),
	}
	if cfg.Distribute.Status && cfg.Distribute.MachineID != 0 {
		h.idGenerator = utils.NewIDGenerator(cfg.Distribute.MachineID)
	}
	return h
}

func (h *Homey) Pack(msg Message) ([]byte, error) {
	return newMessageCodec(h.settings).pack(msg)
}

func (h *Homey) UnPack(data []byte, isForward bool) (Message, error) {
	return newMessageCodec(h.settings).unpack(data, isForward)

}
