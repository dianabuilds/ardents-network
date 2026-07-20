package connectrpc

import (
	"context"
	"strings"

	nodeapi "ardents/internal/node/api"
	ardents "ardents/proto/ardents/v1"

	"connectrpc.com/connect"
)

func (s *Server) GetNodeStatus(ctx context.Context, req *connect.Request[ardents.GetNodeStatusRequest]) (*connect.Response[ardents.NodeStatusResponse], error) {
	return respond(s, req.Header(), func(call callContext) (*ardents.NodeStatusResponse, *rpcError) {
		if err := requireRead(call, "node", "node.status"); err != nil {
			return nil, err
		}
		return &ardents.NodeStatusResponse{
			Status:       statusProto("completed", "snapshot available", true),
			Snapshot:     toSnapshot(s.node.Snapshot()),
			Capabilities: toCapabilitiesSnapshot(s.node.Capabilities()),
		}, nil
	})
}

func (s *Server) GetNodeCapabilities(ctx context.Context, req *connect.Request[ardents.GetNodeCapabilitiesRequest]) (*connect.Response[ardents.CapabilitiesResponse], error) {
	return respond(s, req.Header(), func(call callContext) (*ardents.CapabilitiesResponse, *rpcError) {
		if err := requireRead(call, "node", "node.capabilities"); err != nil {
			return nil, err
		}
		return &ardents.CapabilitiesResponse{Capabilities: toCapabilitiesSnapshot(s.node.Capabilities())}, nil
	})
}

func (s *Server) StreamNodeEvents(ctx context.Context, req *connect.Request[ardents.StreamNodeEventsRequest], stream *connect.ServerStream[ardents.EventEnvelope]) error {
	call, err := s.auth.callContext(req.Header())
	if err != nil {
		return err
	}
	if apiErr := requireRead(call, "node", "node.events"); apiErr != nil {
		return toConnectError(apiErr)
	}
	for evt := range bridgeRuntimeEvents(ctx, s.node.Subscribe(ctx)) {
		if err := stream.Send(evt); err != nil {
			return err
		}
	}
	return nil
}

func bridgeRuntimeEvents(ctx context.Context, src <-chan nodeapi.Event) <-chan *ardents.EventEnvelope {
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
					Time:     ts(evt.Time),
					Domain:   eventDomain(evt.Topic),
					Type:     eventType(evt.Topic),
					Resource: eventResource(evt.Data),
					Payload:  toStruct(cloneEventPayload(evt.Data)),
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
	for k, v := range in {
		out[k] = v
	}
	return out
}
