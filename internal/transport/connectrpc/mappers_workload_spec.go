package connectrpc

import (
	workloadapi "ardents/internal/workload/api"
	ardentsv1 "ardents/proto/ardents/v1"
)

func fromWorkloadSpecSnapshot(in *ardentsv1.WorkloadSpecSnapshot) workloadapi.WorkloadSpecSnapshot {
	if in == nil {
		return workloadapi.WorkloadSpecSnapshot{}
	}
	out := workloadapi.WorkloadSpecSnapshot{
		ID:            in.GetId(),
		Kind:          in.GetKind(),
		Owner:         in.GetOwner(),
		Config:        in.GetConfig(),
		Desired:       in.GetDesired(),
		Capabilities:  append([]string(nil), in.GetCapabilities()...),
		PolicyRef:     in.GetPolicyRef(),
		RestartPolicy: in.GetRestartPolicy(),
	}
	for _, item := range in.GetServices() {
		out.Services = append(out.Services, fromPublishedServiceSnapshot(item))
	}
	return out
}

func toWorkloadSpecSnapshot(in workloadapi.WorkloadSpecSnapshot) *ardentsv1.WorkloadSpecSnapshot {
	out := &ardentsv1.WorkloadSpecSnapshot{
		Id:            in.ID,
		Kind:          in.Kind,
		Owner:         in.Owner,
		Config:        in.Config,
		Desired:       in.Desired,
		Capabilities:  append([]string(nil), in.Capabilities...),
		PolicyRef:     in.PolicyRef,
		RestartPolicy: in.RestartPolicy,
	}
	for _, item := range in.Services {
		out.Services = append(out.Services, toPublishedServiceSnapshot(item))
	}
	return out
}

func toWorkloadInstanceSnapshot(in workloadapi.WorkloadInstanceSnapshot) *ardentsv1.WorkloadInstanceSnapshot {
	out := &ardentsv1.WorkloadInstanceSnapshot{
		WorkloadId: in.WorkloadID,
		Generation: in.Generation,
		Running:    in.Running,
		StartedAt:  ts(in.StartedAt),
		FinishedAt: ts(in.FinishedAt),
		Reason:     in.Reason,
	}
	if in.ExitCode != nil {
		code := int32(*in.ExitCode)
		out.ExitCode = &code
	}
	return out
}
