package workload

import (
	"context"

	ardentsv1 "ardents/internal/localapi/protocol"
	"ardents/internal/localapi/rpc"

	"connectrpc.com/connect"
)

func (h *Service) GetHostedService(ctx context.Context, req *connect.Request[ardentsv1.GetHostedServiceRequest]) (*connect.Response[ardentsv1.GetHostedServiceResponse], error) {
	return rpc.RespondContext(ctx, func(rpc.Call) (*ardentsv1.GetHostedServiceResponse, *rpc.Error) {
		res, err := h.hosting.GetHostedService(req.Msg.GetId())
		if err != nil {
			return nil, rpc.MapError("workload", "workload.hosted_service", "failed", "hosted service lookup failed", false, err)
		}
		return &ardentsv1.GetHostedServiceResponse{
			Status:  statusProto("completed", "hosted service available", true),
			Service: toHostedServiceStatusSnapshot(res),
		}, nil
	})
}

func (h *Service) ListHostedServices(ctx context.Context, _ *connect.Request[ardentsv1.ListHostedServicesRequest]) (*connect.Response[ardentsv1.ListHostedServicesResponse], error) {
	return rpc.RespondContext(ctx, func(rpc.Call) (*ardentsv1.ListHostedServicesResponse, *rpc.Error) {
		res, err := h.hosting.ListHostedServices()
		if err != nil {
			return nil, rpc.MapError("workload", "workload.hosted_services", "failed", "hosted service listing failed", false, err)
		}
		return &ardentsv1.ListHostedServicesResponse{Services: toHostedServiceSnapshots(res)}, nil
	})
}

func (h *Service) GetServicePublicationStatus(ctx context.Context, req *connect.Request[ardentsv1.GetServicePublicationStatusRequest]) (*connect.Response[ardentsv1.ServicePublicationStatusResponse], error) {
	return rpc.RespondContext(ctx, func(rpc.Call) (*ardentsv1.ServicePublicationStatusResponse, *rpc.Error) {
		res, err := h.hosting.GetServicePublicationStatus(req.Msg.GetId())
		if err != nil {
			return nil, rpc.MapError("workload", "workload.service_publication", "failed", "service publication lookup failed", false, err)
		}
		return &ardentsv1.ServicePublicationStatusResponse{
			Status:      statusProto("completed", "service publication available", true),
			Publication: toPublicationStatusSnapshot(res),
		}, nil
	})
}
