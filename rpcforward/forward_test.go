package rpcforward_test

import (
	"context"
	"net"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/towerman1990/homey"
	"github.com/towerman1990/homey/config"
	"github.com/towerman1990/homey/network"
	"github.com/towerman1990/homey/rpcforward"
	forwardv1 "github.com/towerman1990/homey/rpcforward/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

type fullConnection struct {
	network.Connection
}

func (fullConnection) GetID() uint64        { return 100 }
func (fullConnection) Close()               {}
func (fullConnection) SendMsg([]byte) error { return network.ErrSendQueueFull }

func TestDeliverAcrossInstances(t *testing.T) {
	newHomey := func() *network.Homey {
		t.Helper()
		h, err := homey.NewWithConfig(config.Default())
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(h.Stop)
		return h
	}
	sender, receiver := newHomey(), newHomey()
	opened := make(chan uint64, 1)
	receiver.SetOnConnOpen(func(conn network.Connection) error {
		opened <- conn.GetID()
		return nil
	})
	ws := httptest.NewServer(receiver)
	t.Cleanup(ws.Close)
	client, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(ws.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	var id uint64
	select {
	case id = <-opened:
	case <-time.After(3 * time.Second):
		t.Fatal("connection did not register")
	}

	target, err := rpcforward.New(receiver, "node-b", nil, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	server := grpc.NewServer()
	forwardv1.RegisterForwarderServer(server, target)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)
	peer, err := rpcforward.DialPeer(listener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = peer.Close() })
	source, err := rpcforward.New(sender, "node-a", func(_ context.Context, _ string) (grpc.ClientConnInterface, error) {
		return peer, nil
	}, 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	receiver.ConnectionManager().Add(fullConnection{})
	cases := []struct {
		id   uint64
		want forwardv1.DeliveryResult
	}{
		{id, forwardv1.DeliveryResult_DELIVERY_RESULT_QUEUED},
		{id + 1, forwardv1.DeliveryResult_DELIVERY_RESULT_CONNECTION_NOT_FOUND},
		{100, forwardv1.DeliveryResult_DELIVERY_RESULT_QUEUE_FULL},
	}
	for _, tc := range cases {
		got, err := source.Send(context.Background(), rpcforward.Target{NodeID: "node-b", ConnectionID: tc.id}, []byte("hello"))
		if err != nil || got != tc.want {
			t.Fatalf("connection %d: got %s, %v, want %s", tc.id, got, err, tc.want)
		}
	}
	if err := client.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, payload, err := client.ReadMessage(); err != nil || string(payload) != "hello" {
		t.Fatalf("WebSocket read: %q, %v", payload, err)
	}
	got, err := target.Send(context.Background(), rpcforward.Target{NodeID: "node-b", ConnectionID: id}, []byte("local"))
	if err != nil || got != forwardv1.DeliveryResult_DELIVERY_RESULT_QUEUED {
		t.Fatalf("local send: got %s, %v", got, err)
	}
	if _, payload, err := client.ReadMessage(); err != nil || string(payload) != "local" {
		t.Fatalf("local WebSocket read: %q, %v", payload, err)
	}
	got, err = source.Send(context.Background(), rpcforward.Target{NodeID: "wrong-node", ConnectionID: id}, nil)
	if status.Code(err) != codes.FailedPrecondition || got != forwardv1.DeliveryResult_DELIVERY_RESULT_UNSPECIFIED {
		t.Fatalf("wrong node: got %s, %v", got, err)
	}
}
