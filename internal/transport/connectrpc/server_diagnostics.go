package connectrpc

import (
	"context"

	ardentsv1 "ardents/proto/ardents/v1"

	"connectrpc.com/connect"
)

func (s *Server) GetHealthSummary(ctx context.Context, req *connect.Request[ardentsv1.GetHealthSummaryRequest]) (*connect.Response[ardentsv1.HealthSummaryResponse], error) {
	return respond(s, req.Header(), func(call callContext) (*ardentsv1.HealthSummaryResponse, *rpcError) {
		if err := requireRead(call, "diagnostics", "diagnostics.health_summary"); err != nil {
			return nil, err
		}
		return &ardentsv1.HealthSummaryResponse{
			Status: statusProto("completed", "health summary available", true),
			Health: toHealthSnapshot(s.diagnostics.GetHealthSummary()),
		}, nil
	})
}

func (s *Server) ExplainFailure(ctx context.Context, req *connect.Request[ardentsv1.ExplainFailureRequest]) (*connect.Response[ardentsv1.FailureExplanationResponse], error) {
	return respond(s, req.Header(), func(call callContext) (*ardentsv1.FailureExplanationResponse, *rpcError) {
		if err := requireRead(call, "diagnostics", "diagnostics.explain_failure"); err != nil {
			return nil, err
		}
		res := s.diagnostics.ExplainFailure(req.Msg.GetScope(), req.Msg.GetResourceId())
		return &ardentsv1.FailureExplanationResponse{
			Status:      statusProto("completed", "failure explanation available", true),
			Explanation: toFailureExplanationSnapshot(res),
		}, nil
	})
}

func (s *Server) ListRecentEvents(ctx context.Context, req *connect.Request[ardentsv1.ListRecentEventsRequest]) (*connect.Response[ardentsv1.ListEventsResponse], error) {
	return respond(s, req.Header(), func(call callContext) (*ardentsv1.ListEventsResponse, *rpcError) {
		if err := requireRead(call, "diagnostics", "diagnostics.recent_events"); err != nil {
			return nil, err
		}
		events, nextCursor := s.diagnostics.ListRecentEvents(int(req.Msg.GetLimit()), req.Msg.GetCursor())
		return &ardentsv1.ListEventsResponse{
			Status:     statusProto("completed", "recent events available", true),
			Events:     toEventEnvelopes(events),
			NextCursor: nextCursor,
		}, nil
	})
}
