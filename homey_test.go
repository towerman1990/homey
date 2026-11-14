package homey

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"github.com/towerman1990/homey/config"
	"github.com/towerman1990/homey/network"
)

type echoRouter struct {
	network.BaseRouter
}

func (*echoRouter) Handle(req network.Request) error {
	return req.GetConnection().SendMsg(req.GetMsgData())
}

func testWebSocketServer(t *testing.T, h *network.Homey) string {
	t.Helper()
	e := echo.New()
	e.GET("/ws", h.Echo())
	srv := httptest.NewServer(e)
	t.Cleanup(srv.Close)
	t.Cleanup(h.Stop)
	return "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
}

func dialWebSocket(t *testing.T, url string) *websocket.Conn {
	t.Helper()
	client, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	if err := client.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	return client
}

func TestNewHomey(t *testing.T) {
	cfg := config.Default()
	cfg.WorkerPoolSize = 3
	cfg.MaxWorkerTaskLen = 1
	h, err := NewWithConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	h.AddRouter(0, &echoRouter{})
	opened := make(chan network.Connection, 2)
	closed := make(chan struct{}, 2)
	h.SetOnConnOpen(func(conn network.Connection) error {
		opened <- conn
		return conn.SendMsg([]byte("welcome"))
	})
	h.SetOnConnClose(func(network.Connection) { closed <- struct{}{} })
	url := testWebSocketServer(t, h)
	ids := make(map[uint64]bool)
	for i := 0; i < 2; i++ {
		client := dialWebSocket(t, url)
		if _, data, err := client.ReadMessage(); err != nil || string(data) != "welcome" {
			t.Fatalf("welcome message = %q, err = %v", data, err)
		}
		conn := <-opened
		if ids[conn.GetID()] {
			t.Fatal("duplicate connection ID")
		}
		ids[conn.GetID()] = true
		if got, err := h.ConnectionManager().Get(conn.GetID()); err != nil || got != conn {
			t.Fatalf("lookup returned %v, %v", got, err)
		}
		if err := client.WriteMessage(websocket.TextMessage, []byte("hello")); err != nil {
			t.Fatal(err)
		}
		if kind, data, err := client.ReadMessage(); err != nil || kind != websocket.TextMessage || string(data) != "hello" {
			t.Fatalf("echo message type = %d, data = %q, err = %v", kind, data, err)
		}
	}

	h.Stop()
	if count := h.ConnectionManager().Count(); count != 0 {
		t.Fatalf("expected no connections after Stop, got %d", count)
	}
	for i := 0; i < 2; i++ {
		select {
		case <-closed:
		case <-time.After(3 * time.Second):
			t.Fatal("close hook was not called")
		}
	}
	_, resp, err := websocket.DefaultDialer.Dial(url, nil)
	if resp == nil || resp.StatusCode != http.StatusServiceUnavailable || err == nil {
		t.Fatalf("expected HTTP 503 after Stop, got %v, %v", resp, err)
	}
}

func TestBinaryRoutingAndForwarding(t *testing.T) {
	cfg := config.Default()
	cfg.Message.Format = "binary"
	cfg.Message.Endian = "big"
	cfg.TLV.Type = true
	h, err := NewWithConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Distribute(); err == nil {
		t.Fatal("expected disabled distribution to return an error")
	}
	h.AddRouter(7, &echoRouter{})
	opened := make(chan network.Connection, 1)
	h.SetOnConnOpen(func(conn network.Connection) error {
		opened <- conn
		return nil
	})
	client := dialWebSocket(t, testWebSocketServer(t, h))

	msg := network.NewMessage(7, []byte("hello"))
	wire, err := h.Pack(msg)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.WriteMessage(websocket.BinaryMessage, wire); err != nil {
		t.Fatal(err)
	}
	if kind, data, err := client.ReadMessage(); err != nil || kind != websocket.BinaryMessage || string(data) != "hello" {
		t.Fatalf("binary reply type = %d, data = %q, err = %v", kind, data, err)
	}

	msg.SetConnID((<-opened).GetID())
	forwarded, err := h.Pack(msg)
	if err != nil {
		t.Fatal(err)
	}
	go h.RedirectMsgHandler()
	select {
	case h.RedirectMsgChan <- &forwarded:
	case <-time.After(3 * time.Second):
		t.Fatal("redirect handler did not receive the message")
	}
	if kind, data, err := client.ReadMessage(); err != nil || kind != websocket.BinaryMessage || string(data) != string(wire) {
		t.Fatalf("forwarded message type = %d, data = %v, err = %v", kind, data, err)
	}
}

func TestRejectedConnectionIsRemoved(t *testing.T) {
	h := New()
	opened := make(chan network.Connection, 1)
	h.SetOnConnOpen(func(conn network.Connection) error {
		opened <- conn
		return errors.New("rejected")
	})
	client := dialWebSocket(t, testWebSocketServer(t, h))
	if _, _, err := client.ReadMessage(); err == nil {
		t.Fatal("rejected connection should be closed")
	} else if timeout, ok := err.(interface{ Timeout() bool }); ok && timeout.Timeout() {
		t.Fatalf("rejected connection remained open: %v", err)
	}
	select {
	case conn := <-opened:
		deadline := time.Now().Add(3 * time.Second)
		for h.ConnectionManager().Count() != 0 && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond)
		}
		if _, err := h.ConnectionManager().Get(conn.GetID()); err == nil {
			t.Fatal("rejected connection remains registered")
		}
		if err := conn.SendMsg([]byte("hello")); !errors.Is(err, network.ErrConnectionClosed) {
			t.Fatalf("send on rejected connection returned %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("open hook was not called")
	}
}

func TestDistributedConfigRequiresMachineID(t *testing.T) {
	cfg := config.Default()
	cfg.Distribute.Status = true
	if h, err := NewWithConfig(cfg); err == nil {
		h.Stop()
		t.Fatal("accepted distributed configuration without a machine ID")
	}
	cfg.Distribute.MachineID = 1
	h, err := NewWithConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(h.Stop)
}

type blockingRouter struct {
	network.BaseRouter
	started chan struct{}
	release chan struct{}
}

func (r *blockingRouter) Handle(network.Request) error {
	close(r.started)
	<-r.release
	return nil
}

func TestSlowHandlerAndShutdown(t *testing.T) {
	h, err := NewWithConfig(config.Default())
	if err != nil {
		t.Fatal(err)
	}
	router := &blockingRouter{started: make(chan struct{}), release: make(chan struct{})}
	h.AddRouter(0, router)
	server := httptest.NewServer(h)
	t.Cleanup(server.Close)
	t.Cleanup(h.Stop)
	released := false
	t.Cleanup(func() {
		if !released {
			close(router.release)
		}
	})
	client := dialWebSocket(t, "ws"+strings.TrimPrefix(server.URL, "http"))
	if err := client.WriteMessage(websocket.TextMessage, []byte("work")); err != nil {
		t.Fatal(err)
	}
	select {
	case <-router.started:
	case <-time.After(3 * time.Second):
		t.Fatal("handler did not start")
	}
	pong := make(chan struct{}, 1)
	client.SetPongHandler(func(string) error {
		pong <- struct{}{}
		return nil
	})
	readErr := make(chan error, 1)
	go func() {
		_, _, err := client.ReadMessage()
		readErr <- err
	}()
	if err := client.WriteControl(websocket.PingMessage, nil, time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	select {
	case <-pong:
	case <-time.After(time.Second):
		t.Fatal("slow handler blocked WebSocket control messages")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err := h.Shutdown(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Shutdown returned %v while the handler was running", err)
	}
	select {
	case err := <-readErr:
		if !websocket.IsCloseError(err, websocket.CloseGoingAway) {
			t.Fatalf("expected going-away close, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("client did not receive the close message")
	}
	close(router.release)
	released = true
	ctx, cancel = context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := h.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestRedisDistribution(t *testing.T) {
	addr := os.Getenv("HOMEY_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("set HOMEY_TEST_REDIS_ADDR to an isolated Redis server")
	}
	cfg := config.Default()
	cfg.Distribute.Status = true
	cfg.Redis.Addr = addr
	suffix := time.Now().Format("20060102150405.000000000")
	cfg.Redis.WorldChannel = "homey-test-world-" + suffix
	cfg.Redis.ForwardChannel = "homey-test-forward-" + suffix

	newInstance := func(machineID uint16) *network.Homey {
		t.Helper()
		cfg.Distribute.MachineID = machineID
		h, err := NewWithConfig(cfg)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(h.Stop)
		if err := h.Distribute(); err != nil {
			t.Fatal(err)
		}
		return h
	}
	sender, receiver := newInstance(1), newInstance(2)
	opened := make(chan uint64, 1)
	receiver.SetOnConnOpen(func(c network.Connection) error {
		opened <- c.GetID()
		return nil
	})
	client := dialWebSocket(t, testWebSocketServer(t, receiver))
	var id uint64
	select {
	case id = <-opened:
	case <-time.After(time.Second):
		t.Fatal("connection did not register")
	}
	msg := network.NewMessage(0, []byte("across instances"))
	msg.SetConnID(id)
	wire, err := sender.Pack(msg)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := sender.PublishForwardMsg(ctx, wire); err != nil {
		t.Fatal(err)
	}
	if _, data, err := client.ReadMessage(); err != nil || string(data) != "across instances" {
		t.Fatalf("received %q, err = %v", data, err)
	}

	admin := redis.NewClient(&redis.Options{Addr: addr})
	defer admin.Close()
	if err := admin.ClientKillByFilter(ctx, "TYPE", "pubsub").Err(); err != nil {
		t.Fatal(err)
	}
	for {
		counts, err := admin.PubSubNumSub(ctx, cfg.Redis.WorldChannel, cfg.Redis.ForwardChannel).Result()
		if err != nil {
			t.Fatal(err)
		}
		if counts[cfg.Redis.WorldChannel] == 2 && counts[cfg.Redis.ForwardChannel] == 2 {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("Redis subscriptions did not reconnect")
		case <-time.After(20 * time.Millisecond):
		}
	}
	if err := client.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := sender.PublishWorldMsg(ctx, wire); err != nil {
		t.Fatal(err)
	}
	if _, data, err := client.ReadMessage(); err != nil || string(data) != "across instances" {
		t.Fatalf("received %q after reconnect, err = %v", data, err)
	}
	for _, h := range []*network.Homey{sender, receiver} {
		if err := h.Shutdown(ctx); err != nil {
			t.Fatal(err)
		}
	}
}
