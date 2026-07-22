package diagnostics

import (
	"context"

	ardentsv1 "ardents/internal/localapi/protocol"
	"ardents/internal/localapi/rpc"

	"connectrpc.com/connect"
)

func (h *Endpoint) GetHealthSummary(ctx context.Context, _ *connect.Request[ardentsv1.GetHealthSummaryRequest]) (*connect.Response[ardentsv1.HealthSummaryResponse], error) {
	return rpc.RespondContext(ctx, func(rpc.Call) (*ardentsv1.HealthSummaryResponse, *rpc.Error) {
		return &ardentsv1.HealthSummaryResponse{
			Status: operationStatus("completed", "health summary available", true),
			Health: toHealthSnapshot(h.service.GetHealthSummary()),
		}, nil
	})
}

func (h *Endpoint) ExplainFailure(ctx context.Context, req *connect.Request[ardentsv1.ExplainFailureRequest]) (*connect.Response[ardentsv1.FailureExplanationResponse], error) {
	return rpc.RespondContext(ctx, func(rpc.Call) (*ardentsv1.FailureExplanationResponse, *rpc.Error) {
		res := h.service.ExplainFailure(req.Msg.GetScope(), req.Msg.GetResourceId())
		return &ardentsv1.FailureExplanationResponse{
			Status:      operationStatus("completed", "failure explanation available", true),
			Explanation: toFailureExplanationSnapshot(res),
		}, nil
	})
}

func (h *Endpoint) ListRecentEvents(ctx context.Context, req *connect.Request[ardentsv1.ListRecentEventsRequest]) (*connect.Response[ardentsv1.ListEventsResponse], error) {
	return rpc.RespondContext(ctx, func(rpc.Call) (*ardentsv1.ListEventsResponse, *rpc.Error) {
		events, nextCursor := h.service.ListRecentEvents(int(req.Msg.GetLimit()), req.Msg.GetCursor())
		return &ardentsv1.ListEventsResponse{
			Status:     operationStatus("completed", "recent events available", true),
			Events:     toEventEnvelopes(events),
			NextCursor: nextCursor,
		}, nil
	})
}
