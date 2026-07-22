package workload

import (
	"errors"

	hostingapi "ardents/internal/hosting"
	identityaccess "ardents/internal/identity/access"
	protocol "ardents/internal/localapi/protocol"
	"ardents/internal/localapi/protocol/ardentsv1connect"
	workloadapi "ardents/internal/workload"
)

var ErrInvalidResourceTarget = errors.New("workload resource target is invalid")

func CanonicalizeResource(procedure string, message any, kind identityaccess.ResourceKind) (identityaccess.ResourceTarget, error) {
	target := identityaccess.ResourceTarget{Kind: kind}
	var id string
	var err error
	valid := true
	switch procedure {
	case ardentsv1connect.WorkloadServiceRegisterWorkloadProcedure:
		request, ok := message.(*protocol.RegisterWorkloadRequest)
		if !ok || request.GetSpec() == nil {
			valid = false
			break
		}
		id, err = workloadapi.AccessResourceID(request.GetSpec().GetId())
	case ardentsv1connect.WorkloadServiceStartWorkloadProcedure:
		request, ok := message.(*protocol.StartWorkloadRequest)
		if !ok {
			valid = false
			break
		}
		id, err = workloadapi.AccessResourceID(request.GetId())
	case ardentsv1connect.WorkloadServiceStopWorkloadProcedure:
		request, ok := message.(*protocol.StopWorkloadRequest)
		if !ok {
			valid = false
			break
		}
		id, err = workloadapi.AccessResourceID(request.GetId())
	case ardentsv1connect.WorkloadServiceRestartWorkloadProcedure:
		request, ok := message.(*protocol.RestartWorkloadRequest)
		if !ok {
			valid = false
			break
		}
		id, err = workloadapi.AccessResourceID(request.GetId())
	case ardentsv1connect.WorkloadServiceGetWorkloadStatusProcedure:
		request, ok := message.(*protocol.GetWorkloadStatusRequest)
		if !ok {
			valid = false
			break
		}
		id, err = workloadapi.AccessResourceID(request.GetId())
	case ardentsv1connect.WorkloadServiceListWorkloadsProcedure:
		_, valid = message.(*protocol.ListWorkloadsRequest)
	case ardentsv1connect.WorkloadServiceGetHostedServiceProcedure:
		request, ok := message.(*protocol.GetHostedServiceRequest)
		if !ok {
			valid = false
			break
		}
		id, err = hostingapi.ServiceAccessResourceID(request.GetId())
	case ardentsv1connect.WorkloadServiceListHostedServicesProcedure:
		_, valid = message.(*protocol.ListHostedServicesRequest)
	case ardentsv1connect.WorkloadServiceGetServicePublicationStatusProcedure:
		request, ok := message.(*protocol.GetServicePublicationStatusRequest)
		if !ok {
			valid = false
			break
		}
		id, err = hostingapi.ServiceAccessResourceID(request.GetId())
	default:
		valid = false
	}
	if !valid || err != nil {
		return identityaccess.ResourceTarget{}, ErrInvalidResourceTarget
	}
	target.ID = id
	return target, nil
}
