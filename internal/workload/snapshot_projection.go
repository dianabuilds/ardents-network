package workload

import (
	"ardents/internal/workload/execution"
	domain "ardents/internal/workload/registry"
)

func ProjectStatus(in execution.Status) StatusSnapshot {
	return StatusSnapshot{
		Spec: SpecSnapshot{
			ID: in.Spec.ID, Kind: in.Spec.Kind, Owner: in.Spec.Owner, Desired: in.Spec.Desired,
			Services:     projectServices(in.Spec.Services, false, ""),
			Capabilities: append([]string(nil), in.Spec.Capabilities...), PolicyRef: in.Spec.PolicyRef,
			RestartPolicy: in.Spec.RestartPolicy,
		},
		Observed: in.Observed, Reason: in.Reason, LastTransitionAt: in.LastTransitionAt,
		NeedsOperatorAction: in.NeedsOperatorAction, RestartCount: in.RestartCount,
		PublishedServices: projectPublished(in.PublishedServices),
		Instance: InstanceSnapshot{
			WorkloadID: in.Instance.WorkloadID, Generation: in.Instance.Generation, Running: in.Instance.Running,
			StartedAt: in.Instance.StartedAt, FinishedAt: in.Instance.FinishedAt, ExitCode: in.Instance.ExitCode,
			OOMKilled: in.Instance.OOMKilled, Restarts: in.Instance.Restarts,
			MemoryLimitBytes: in.Instance.MemoryLimitBytes, NanoCPUs: in.Instance.NanoCPUs,
			PIDsLimit: in.Instance.PIDsLimit, Reason: in.Instance.Reason,
		},
	}
}

func projectServices(items []domain.ServiceSpec, published bool, reason string) []PublishedServiceSnapshot {
	out := make([]PublishedServiceSnapshot, 0, len(items))
	for _, item := range items {
		out = append(out, PublishedServiceSnapshot{ID: item.ID, Type: item.Type, Owner: item.Owner, Mode: item.Mode,
			Published: published, Endpoints: append([]string(nil), item.Endpoints...),
			ProbeEndpoints: append([]string(nil), item.ProbeEndpoints...), Reason: reason})
	}
	return out
}

func projectPublished(items []execution.PublishedServiceStatus) []PublishedServiceSnapshot {
	out := make([]PublishedServiceSnapshot, 0, len(items))
	for _, item := range items {
		out = append(out, PublishedServiceSnapshot{ID: item.ID, Type: item.Type, Owner: item.Owner, Mode: item.Mode,
			Published: item.Published, Endpoints: append([]string(nil), item.Endpoints...),
			ProbeEndpoints: append([]string(nil), item.ProbeEndpoints...), Reason: item.Reason})
	}
	return out
}
