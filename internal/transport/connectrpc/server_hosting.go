package connectrpc

import (
	"context"

	ardentsv1 "ardents/proto/ardents/v1"

	"connectrpc.com/connect"
)

func (s *Server) GetHostedService(ctx context.Context, req *connect.Request[ardentsv1.GetHostedServiceRequest]) (*connect.Response[ardentsv1.GetHostedServiceResponse], error) {
	return respond(s, req.Header(), func(call callContext) (*ardentsv1.GetHostedServiceResponse, *rpcError) {
		if err := requireRead(call, "workload", "workload.hosted_service"); err != nil {
			return nil, err
		}
		res, err := s.hosting.GetHostedService(req.Msg.GetId())
		if err != nil {
			return nil, mapAPIError("workload", "workload.hosted_service", "failed", "hosted service lookup failed", false, err)
		}
		return &ardentsv1.GetHostedServiceResponse{
			Status:  statusProto("completed", "hosted service available", true),
			Service: toHostedServiceStatusSnapshot(res),
		}, nil
	})
}

func (s *Server) ListHostedServices(ctx context.Context, req *connect.Request[ardentsv1.ListHostedServicesRequest]) (*connect.Response[ardentsv1.ListHostedServicesResponse], error) {
	return respond(s, req.Header(), func(call callContext) (*ardentsv1.ListHostedServicesResponse, *rpcError) {
		if err := requireRead(call, "workload", "workload.hosted_services"); err != nil {
			return nil, err
		}
		res, err := s.hosting.ListHostedServices()
		if err != nil {
			return nil, mapAPIError("workload", "workload.hosted_services", "failed", "hosted service listing failed", false, err)
		}
		return &ardentsv1.ListHostedServicesResponse{Services: toHostedServiceSnapshots(res)}, nil
	})
}

func (s *Server) GetServicePublicationStatus(ctx context.Context, req *connect.Request[ardentsv1.GetServicePublicationStatusRequest]) (*connect.Response[ardentsv1.ServicePublicationStatusResponse], error) {
	return respond(s, req.Header(), func(call callContext) (*ardentsv1.ServicePublicationStatusResponse, *rpcError) {
		if err := requireRead(call, "workload", "workload.service_publication"); err != nil {
			return nil, err
		}
		res, err := s.hosting.GetServicePublicationStatus(req.Msg.GetId())
		if err != nil {
			return nil, mapAPIError("workload", "workload.service_publication", "failed", "service publication lookup failed", false, err)
		}
		return &ardentsv1.ServicePublicationStatusResponse{
			Status:      statusProto("completed", "service publication available", true),
			Publication: toPublicationStatusSnapshot(res),
		}, nil
	})
}
