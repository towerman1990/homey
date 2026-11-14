// Package rpcforward provides optional gRPC unicast between Homey instances.
package rpcforward

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/towerman1990/homey/network"
	forwardv1 "github.com/towerman1990/homey/rpcforward/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Target identifies the connection and the node that accepted it.
type Target struct {
	NodeID       string
	ConnectionID uint64
}

// ResolvePeer returns a reusable client connection for a specific node.
type ResolvePeer func(ctx context.Context, nodeID string) (grpc.ClientConnInterface, error)

// Forwarder delivers to local connections or routes to a remote node.
type Forwarder struct {
	forwardv1.UnimplementedForwarderServer
	homey   *network.Homey
	nodeID  string
	resolve ResolvePeer
	timeout time.Duration
}

func New(h *network.Homey, nodeID string, resolve ResolvePeer, timeout time.Duration) (*Forwarder, error) {
	if h == nil || nodeID == "" || timeout <= 0 {
		return nil, errors.New("homey forwarding requires a server, node ID and positive timeout")
	}
	return &Forwarder{homey: h, nodeID: nodeID, resolve: resolve, timeout: timeout}, nil
}

// DialPeer creates a reusable client connection to a specific node. The caller
// owns the connection and supplies its transport credentials.
func DialPeer(address string, options ...grpc.DialOption) (*grpc.ClientConn, error) {
	return grpc.NewClient(address, append(options, grpc.WithDisableRetry(), grpc.WithDisableServiceConfig())...)
}

// Send returns a business result once the target answers. An RPC timeout or
// transport error can leave the outcome unknown.
func (f *Forwarder) Send(ctx context.Context, target Target, payload []byte) (forwardv1.DeliveryResult, error) {
	if target.NodeID == "" || target.ConnectionID == 0 {
		return forwardv1.DeliveryResult_DELIVERY_RESULT_UNSPECIFIED, status.Error(codes.InvalidArgument, "target node and connection ID are required")
	}
	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()
	req := &forwardv1.DeliverRequest{ConnectionId: target.ConnectionID, Payload: payload, NodeId: target.NodeID}
	var reply *forwardv1.DeliverResponse
	var err error
	if target.NodeID == f.nodeID {
		reply, err = f.Deliver(ctx, req)
	} else {
		if f.resolve == nil {
			return forwardv1.DeliveryResult_DELIVERY_RESULT_UNSPECIFIED, fmt.Errorf("homey: no resolver for node %q", target.NodeID)
		}
		peer, resolveErr := f.resolve(ctx, target.NodeID)
		if resolveErr != nil {
			return forwardv1.DeliveryResult_DELIVERY_RESULT_UNSPECIFIED, fmt.Errorf("resolve Homey node %q: %w", target.NodeID, resolveErr)
		}
		if peer == nil {
			return forwardv1.DeliveryResult_DELIVERY_RESULT_UNSPECIFIED, fmt.Errorf("homey: no peer connection for node %q", target.NodeID)
		}
		reply, err = forwardv1.NewForwarderClient(peer).Deliver(ctx, req)
	}
	if err != nil {
		return forwardv1.DeliveryResult_DELIVERY_RESULT_UNSPECIFIED, err
	}
	switch result := reply.GetResult(); result {
	case forwardv1.DeliveryResult_DELIVERY_RESULT_QUEUED,
		forwardv1.DeliveryResult_DELIVERY_RESULT_CONNECTION_NOT_FOUND,
		forwardv1.DeliveryResult_DELIVERY_RESULT_QUEUE_FULL:
		return result, nil
	default:
		return forwardv1.DeliveryResult_DELIVERY_RESULT_UNSPECIFIED, status.Error(codes.Internal, "unknown delivery result")
	}
}

func (f *Forwarder) Deliver(ctx context.Context, req *forwardv1.DeliverRequest) (*forwardv1.DeliverResponse, error) {
	if req.GetConnectionId() == 0 || req.GetNodeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "target node and connection ID are required")
	}
	if req.GetNodeId() != f.nodeID {
		return nil, status.Error(codes.FailedPrecondition, "request reached the wrong Homey node")
	}
	if err := ctx.Err(); err != nil {
		return nil, status.FromContextError(err).Err()
	}
	if f.homey.Context().Err() != nil {
		return nil, status.Error(codes.Unavailable, "Homey is shutting down")
	}
	conn, err := f.homey.ConnectionManager().Get(req.GetConnectionId())
	result := forwardv1.DeliveryResult_DELIVERY_RESULT_CONNECTION_NOT_FOUND
	if err == nil {
		switch err := conn.SendMsg(req.GetPayload()); {
		case err == nil:
			result = forwardv1.DeliveryResult_DELIVERY_RESULT_QUEUED
		case errors.Is(err, network.ErrSendQueueFull):
			result = forwardv1.DeliveryResult_DELIVERY_RESULT_QUEUE_FULL
		case errors.Is(err, network.ErrConnectionClosed):
		default:
			return nil, status.Error(codes.Internal, "failed to queue WebSocket message")
		}
	}
	return &forwardv1.DeliverResponse{Result: result}, nil
}
