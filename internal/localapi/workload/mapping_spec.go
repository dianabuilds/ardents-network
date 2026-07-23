package workload

import (
	"fmt"

	ardentsv1 "ardents/internal/localapi/protocol"
	workloadapi "ardents/internal/workload"
	workloadregistry "ardents/internal/workload/registry"
)

func fromWorkloadSpecSnapshot(in *ardentsv1.WorkloadSpecSnapshot) (workloadapi.SpecSnapshot, error) {
	if in == nil {
		return workloadapi.SpecSnapshot{}, fmt.Errorf("workload specification is required")
	}
	if len(in.ProtoReflect().GetUnknown()) != 0 {
		return workloadapi.SpecSnapshot{}, fmt.Errorf("workload specification contains unknown fields")
	}
	requirements, err := workloadRequirementsFromWire(in.GetRequirements())
	if err != nil {
		return workloadapi.SpecSnapshot{}, err
	}
	out := workloadapi.SpecSnapshot{
		ID:            in.GetId(),
		Kind:          in.GetKind(),
		Owner:         in.GetOwner(),
		Config:        in.GetConfig(),
		Desired:       in.GetDesired(),
		Requirements:  requirements,
		PolicyRef:     in.GetPolicyRef(),
		RestartPolicy: in.GetRestartPolicy(),
	}
	for _, item := range in.GetServices() {
		out.Services = append(out.Services, fromPublishedServiceSnapshot(item))
	}
	return out, nil
}

func toWorkloadSpecSnapshot(in workloadapi.SpecSnapshot) (*ardentsv1.WorkloadSpecSnapshot, error) {
	requirements, err := workloadRequirementsToWire(in.Requirements)
	if err != nil {
		return nil, err
	}
	out := &ardentsv1.WorkloadSpecSnapshot{
		Id:            in.ID,
		Kind:          in.Kind,
		Owner:         in.Owner,
		Config:        in.Config,
		Desired:       in.Desired,
		Requirements:  requirements,
		PolicyRef:     in.PolicyRef,
		RestartPolicy: in.RestartPolicy,
	}
	for _, item := range in.Services {
		out.Services = append(out.Services, toPublishedServiceSnapshot(item))
	}
	return out, nil
}

func workloadRequirementsFromWire(values []string) ([]workloadregistry.WorkloadRequirement, error) {
	out := make([]workloadregistry.WorkloadRequirement, len(values))
	for index, value := range values {
		requirement, err := workloadregistry.ParseWorkloadRequirement(value)
		if err != nil {
			return nil, fmt.Errorf("workload requirement is invalid")
		}
		out[index] = requirement
	}
	return out, nil
}

func workloadRequirementsToWire(values []workloadregistry.WorkloadRequirement) ([]string, error) {
	out := make([]string, len(values))
	for index, value := range values {
		if !value.Valid() {
			return nil, fmt.Errorf("workload requirement is invalid")
		}
		out[index] = value.String()
	}
	return out, nil
}

func toWorkloadInstanceSnapshot(in workloadapi.InstanceSnapshot) *ardentsv1.WorkloadInstanceSnapshot {
	out := &ardentsv1.WorkloadInstanceSnapshot{
		WorkloadId: in.WorkloadID,
		Generation: in.Generation,
		Running:    in.Running,
		StartedAt:  ts(in.StartedAt),
		FinishedAt: ts(in.FinishedAt),
		Reason:     in.Reason,
	}
	if in.ExitCode != nil {
		out.ExitCode = new(int32(*in.ExitCode))
	}
	return out
}
