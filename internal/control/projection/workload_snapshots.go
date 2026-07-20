package projection

import (
	workloadapi "ardents/internal/workload/api"
	"ardents/internal/workload/observedstate"
	domainworkload "ardents/internal/workload/workload"
)

func WorkloadSnapshot(in observedstate.Status) workloadapi.WorkloadStatusSnapshot {
	return workloadapi.WorkloadStatusSnapshot{
		Spec: workloadapi.WorkloadSpecSnapshot{
			ID: in.Spec.ID, Kind: in.Spec.Kind, Owner: in.Spec.Owner, Desired: in.Spec.Desired,
			Services:     publishedServiceSnapshots(in.Spec.Services, false, ""),
			Capabilities: cloneStrings(in.Spec.Capabilities), PolicyRef: in.Spec.PolicyRef,
			RestartPolicy: in.Spec.RestartPolicy,
		},
		Observed: in.Observed, Reason: in.Reason, LastTransitionAt: in.LastTransitionAt,
		NeedsOperatorAction: in.NeedsOperatorAction, RestartCount: in.RestartCount,
		PublishedServices: publishedStatuses(in.PublishedServices),
		Instance: workloadapi.WorkloadInstanceSnapshot{
			WorkloadID: in.Instance.WorkloadID, Generation: in.Instance.Generation, Running: in.Instance.Running,
			StartedAt: in.Instance.StartedAt, FinishedAt: in.Instance.FinishedAt, ExitCode: in.Instance.ExitCode,
			OOMKilled: in.Instance.OOMKilled, Restarts: in.Instance.Restarts,
			MemoryLimitBytes: in.Instance.MemoryLimitBytes, NanoCPUs: in.Instance.NanoCPUs,
			PIDsLimit: in.Instance.PIDsLimit, Reason: in.Instance.Reason,
		},
	}
}

func publishedServiceSnapshots(in []domainworkload.ServiceSpec, published bool, reason string) []workloadapi.PublishedServiceSnapshot {
	out := make([]workloadapi.PublishedServiceSnapshot, 0, len(in))
	for _, item := range in {
		out = append(out, workloadapi.PublishedServiceSnapshot{
			ID: item.ID, Type: item.Type, Owner: item.Owner, Mode: item.Mode, Published: published,
			Endpoints: cloneStrings(item.Endpoints), ProbeEndpoints: cloneStrings(item.ProbeEndpoints), Reason: reason,
		})
	}
	return out
}

func publishedStatuses(in []observedstate.PublishedServiceStatus) []workloadapi.PublishedServiceSnapshot {
	out := make([]workloadapi.PublishedServiceSnapshot, 0, len(in))
	for _, item := range in {
		out = append(out, workloadapi.PublishedServiceSnapshot{
			ID: item.ID, Type: item.Type, Owner: item.Owner, Mode: item.Mode, Published: item.Published,
			Endpoints: cloneStrings(item.Endpoints), ProbeEndpoints: cloneStrings(item.ProbeEndpoints), Reason: item.Reason,
		})
	}
	return out
}
