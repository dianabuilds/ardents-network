package node

import (
	"context"
	"fmt"
	"maps"
	"strings"

	daemonruntime "ardents/internal/daemon"
	ardents "ardents/internal/localapi/protocol"
	"ardents/internal/localapi/rpc"

	"connectrpc.com/connect"
)

func (h *RuntimeHandler) GetNodeStatus(ctx context.Context, _ *connect.Request[ardents.GetNodeStatusRequest]) (*connect.Response[ardents.NodeStatusResponse], error) {
	return rpc.RespondContext(ctx, func(rpc.Call) (*ardents.NodeStatusResponse, *rpc.Error) {
		snapshot, err := toSnapshot(h.service.Snapshot())
		if err != nil {
			return nil, rpc.MapError("node", "node.status", "invalid_snapshot", "node status is invalid", false, err)
		}
		return &ardents.NodeStatusResponse{
			Status:   statusProto("completed", "snapshot available", true),
			Snapshot: snapshot,
			Features: toNodeFeaturesSnapshot(h.service.NodeFeatures()),
		}, nil
	})
}

func (h *RuntimeHandler) GetNodeFeatures(ctx context.Context, _ *connect.Request[ardents.GetNodeFeaturesRequest]) (*connect.Response[ardents.NodeFeaturesResponse], error) {
	return rpc.RespondContext(ctx, func(rpc.Call) (*ardents.NodeFeaturesResponse, *rpc.Error) {
		return &ardents.NodeFeaturesResponse{Features: toNodeFeaturesSnapshot(h.service.NodeFeatures())}, nil
	})
}

func (h *RuntimeHandler) StreamNodeEvents(ctx context.Context, _ *connect.Request[ardents.StreamNodeEventsRequest], stream *connect.ServerStream[ardents.EventEnvelope]) error {
	if _, ok := rpc.CallFromContext(ctx); !ok {
		return connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("authenticated call context is required"))
	}
	for evt := range bridgeRuntimeEvents(ctx, h.service.Subscribe(ctx)) {
		if err := stream.Send(evt); err != nil {
			return err
		}
	}
	return nil
}

func bridgeRuntimeEvents(ctx context.Context, src <-chan daemonruntime.Event) <-chan *ardents.EventEnvelope {
	dst := make(chan *ardents.EventEnvelope, 16)
	go func() {
		defer close(dst)
		for {
			select {
			case <-ctx.Done():
				return
			case evt, ok := <-src:
				if !ok {
					return
				}
				envelope := &ardents.EventEnvelope{
					Seq:      evt.Seq,
					Time:     rpc.Timestamp(evt.Time),
					Domain:   eventDomain(evt.Topic),
					Type:     eventType(evt.Topic),
					Resource: eventResource(evt.Data),
					Payload:  rpc.Struct(cloneEventPayload(evt.Data)),
				}
				select {
				case dst <- envelope:
				case <-ctx.Done():
					return
				default:
				}
			}
		}
	}()
	return dst
}

func eventDomain(topic string) string {
	if prefix, _, ok := strings.Cut(topic, "."); ok {
		return prefix
	}
	return "node"
}

func eventType(topic string) string {
	if _, suffix, ok := strings.Cut(topic, "."); ok {
		return suffix
	}
	return topic
}

func eventResource(data map[string]any) string {
	if id, ok := data["id"].(string); ok {
		return id
	}
	if subject, ok := data["subject"].(string); ok {
		return subject
	}
	return ""
}

func cloneEventPayload(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	maps.Copy(out, in)
	return out
}
