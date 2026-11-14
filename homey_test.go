package homey

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
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
