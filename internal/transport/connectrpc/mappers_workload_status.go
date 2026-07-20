package connectrpc

import (
	workloadapi "ardents/internal/workload/api"
	ardentsv1 "ardents/proto/ardents/v1"
)

func toWorkloadStatusSnapshot(in workloadapi.WorkloadStatusSnapshot) *ardentsv1.WorkloadStatusSnapshot {
	out := &ardentsv1.WorkloadStatusSnapshot{
		Spec:                toWorkloadSpecSnapshot(in.Spec),
		Observed:            in.Observed,
		Reason:              in.Reason,
		LastTransitionAt:    ts(in.LastTransitionAt),
		NeedsOperatorAction: in.NeedsOperatorAction,
		RestartCount:        int32(in.RestartCount),
		Instance:            toWorkloadInstanceSnapshot(in.Instance),
	}
	for _, item := range in.PublishedServices {
		out.PublishedServices = append(out.PublishedServices, toPublishedServiceSnapshot(item))
	}
	return out
}

func toPublishedServiceSnapshot(in workloadapi.PublishedServiceSnapshot) *ardentsv1.PublishedServiceSnapshot {
	return &ardentsv1.PublishedServiceSnapshot{
		Id:             in.ID,
		Type:           in.Type,
		Owner:          in.Owner,
		Mode:           in.Mode,
		Published:      in.Published,
		Endpoints:      append([]string(nil), in.Endpoints...),
		ProbeEndpoints: append([]string(nil), in.ProbeEndpoints...),
		Reason:         in.Reason,
	}
}

func fromPublishedServiceSnapshot(in *ardentsv1.PublishedServiceSnapshot) workloadapi.PublishedServiceSnapshot {
	if in == nil {
		return workloadapi.PublishedServiceSnapshot{}
	}
	return workloadapi.PublishedServiceSnapshot{
		ID:             in.GetId(),
		Type:           in.GetType(),
		Owner:          in.GetOwner(),
		Mode:           in.GetMode(),
		Published:      in.GetPublished(),
		Endpoints:      append([]string(nil), in.GetEndpoints()...),
		ProbeEndpoints: append([]string(nil), in.GetProbeEndpoints()...),
		Reason:         in.GetReason(),
	}
}
