package diagnostics

import (
	"context"

	ardentsv1 "ardents/internal/localapi/protocol"
	"ardents/internal/localapi/rpc"

	"connectrpc.com/connect"
)

func (h *Endpoint) GetHealthSummary(_ context.Context, req *connect.Request[ardentsv1.GetHealthSummaryRequest]) (*connect.Response[ardentsv1.HealthSummaryResponse], error) {
	return rpc.Respond(h.auth, req.Header(), func(call rpc.CallContext) (*ardentsv1.HealthSummaryResponse, *rpc.Error) {
		if err := rpc.RequireRead(call, "diagnostics", "diagnostics.health_summary"); err != nil {
			return nil, err
		}
		return &ardentsv1.HealthSummaryResponse{
			Status: operationStatus("completed", "health summary available", true),
			Health: toHealthSnapshot(h.service.GetHealthSummary()),
		}, nil
	})
}

func (h *Endpoint) ExplainFailure(_ context.Context, req *connect.Request[ardentsv1.ExplainFailureRequest]) (*connect.Response[ardentsv1.FailureExplanationResponse], error) {
	return rpc.Respond(h.auth, req.Header(), func(call rpc.CallContext) (*ardentsv1.FailureExplanationResponse, *rpc.Error) {
		if err := rpc.RequireRead(call, "diagnostics", "diagnostics.explain_failure"); err != nil {
			return nil, err
		}
		res := h.service.ExplainFailure(req.Msg.GetScope(), req.Msg.GetResourceId())
		return &ardentsv1.FailureExplanationResponse{
			Status:      operationStatus("completed", "failure explanation available", true),
			Explanation: toFailureExplanationSnapshot(res),
		}, nil
	})
}

func (h *Endpoint) ListRecentEvents(_ context.Context, req *connect.Request[ardentsv1.ListRecentEventsRequest]) (*connect.Response[ardentsv1.ListEventsResponse], error) {
	return rpc.Respond(h.auth, req.Header(), func(call rpc.CallContext) (*ardentsv1.ListEventsResponse, *rpc.Error) {
		if err := rpc.RequireRead(call, "diagnostics", "diagnostics.recent_events"); err != nil {
			return nil, err
		}
		events, nextCursor := h.service.ListRecentEvents(int(req.Msg.GetLimit()), req.Msg.GetCursor())
		return &ardentsv1.ListEventsResponse{
			Status:     operationStatus("completed", "recent events available", true),
			Events:     toEventEnvelopes(events),
			NextCursor: nextCursor,
		}, nil
	})
}
