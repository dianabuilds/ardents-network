package node

import (
	"context"
	"maps"
	"strings"

	daemonruntime "ardents/internal/daemon"
	ardents "ardents/internal/localapi/protocol"
	"ardents/internal/localapi/rpc"

	"connectrpc.com/connect"
)

func (h *RuntimeHandler) GetNodeStatus(_ context.Context, req *connect.Request[ardents.GetNodeStatusRequest]) (*connect.Response[ardents.NodeStatusResponse], error) {
	return rpc.Respond(h.auth, req.Header(), func(call rpc.CallContext) (*ardents.NodeStatusResponse, *rpc.Error) {
		if err := rpc.RequireRead(call, "node", "node.status"); err != nil {
			return nil, err
		}
		return &ardents.NodeStatusResponse{
			Status:       statusProto("completed", "snapshot available", true),
			Snapshot:     toSnapshot(h.service.Snapshot()),
			Capabilities: toCapabilitiesSnapshot(h.service.Capabilities()),
		}, nil
	})
}

func (h *RuntimeHandler) GetNodeCapabilities(_ context.Context, req *connect.Request[ardents.GetNodeCapabilitiesRequest]) (*connect.Response[ardents.CapabilitiesResponse], error) {
	return rpc.Respond(h.auth, req.Header(), func(call rpc.CallContext) (*ardents.CapabilitiesResponse, *rpc.Error) {
		if err := rpc.RequireRead(call, "node", "node.capabilities"); err != nil {
			return nil, err
		}
		return &ardents.CapabilitiesResponse{Capabilities: toCapabilitiesSnapshot(h.service.Capabilities())}, nil
	})
}

func (h *RuntimeHandler) StreamNodeEvents(ctx context.Context, req *connect.Request[ardents.StreamNodeEventsRequest], stream *connect.ServerStream[ardents.EventEnvelope]) error {
	call, err := h.auth.CallContext(req.Header())
	if err != nil {
		return err
	}
	if apiErr := rpc.RequireRead(call, "node", "node.events"); apiErr != nil {
		return rpc.ToConnectError(apiErr)
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
