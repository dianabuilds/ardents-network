package workload

import (
	"ardents/internal/hosting"
	ardentsv1 "ardents/internal/localapi/protocol"
)

func toPublicationStatusSnapshot(in hosting.PublicationSnapshot) *ardentsv1.PublicationStatusSnapshot {
	return &ardentsv1.PublicationStatusSnapshot{
		State:                  in.State,
		Reason:                 in.Reason,
		Published:              in.Published,
		PublishedAt:            ts(in.PublishedAt),
		ExpiresAt:              ts(in.ExpiresAt),
		WithdrawnAt:            tsp(in.WithdrawnAt),
		OperatorActionRequired: in.OperatorActionRequired,
	}
}

func toHostedServiceSnapshot(in hosting.ServiceSnapshot) *ardentsv1.HostedServiceSnapshot {
	out := &ardentsv1.HostedServiceSnapshot{
		Id:                 in.ID,
		Type:               in.Type,
		Owner:              in.Owner,
		WorkloadId:         in.WorkloadID,
		Visibility:         in.Visibility,
		DesiredPublication: in.DesiredPublication,
		RuntimeBacking:     in.RuntimeBacking,
		PolicyRef:          in.PolicyRef,
		Readiness:          in.Readiness,
		Ready:              in.Ready,
		ExposureEligible:   in.ExposureEligible,
		Generation:         in.Generation,
		LastProbeAt:        ts(in.LastProbeAt),
	}
	for _, item := range in.Endpoints {
		out.Endpoints = append(out.Endpoints, &ardentsv1.ServiceEndpointSnapshot{
			Kind:      item.Kind,
			Address:   item.Address,
			Protocol:  item.Protocol,
			Port:      int32(item.Port),
			Exposure:  item.Exposure,
			Reachable: item.Reachable,
			Reason:    item.Reason,
		})
	}
	return out
}

func toHostedServiceSnapshots(items []hosting.ServiceSnapshot) []*ardentsv1.HostedServiceSnapshot {
	out := make([]*ardentsv1.HostedServiceSnapshot, 0, len(items))
	for _, item := range items {
		out = append(out, toHostedServiceSnapshot(item))
	}
	return out
}

func toHostedServiceStatusSnapshot(in hosting.ServiceStatusSnapshot) *ardentsv1.HostedServiceStatusSnapshot {
	return &ardentsv1.HostedServiceStatusSnapshot{
		ServiceId:              in.ServiceID,
		State:                  in.State,
		Reason:                 in.Reason,
		Published:              in.Published,
		RuntimeBacking:         in.RuntimeBacking,
		Ready:                  in.Ready,
		ExposureEligible:       in.ExposureEligible,
		Generation:             in.Generation,
		LastProbeAt:            ts(in.LastProbeAt),
		Publication:            toPublicationStatusSnapshot(in.Publication),
		LastTransitionAt:       ts(in.LastTransitionAt),
		OperatorActionRequired: in.OperatorActionRequired,
	}
}
