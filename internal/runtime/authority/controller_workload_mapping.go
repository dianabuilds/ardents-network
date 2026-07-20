package authority

import (
	workloadapi "ardents/internal/workload/api"
	domainworkload "ardents/internal/workload/workload"
)

func workloadSpecFromAPI(spec workloadapi.WorkloadSpecSnapshot) domainworkload.Spec {
	return domainworkload.Spec{
		ID:            spec.ID,
		Kind:          spec.Kind,
		Owner:         spec.Owner,
		Config:        spec.Config,
		Desired:       spec.Desired,
		Services:      toWorkloadServiceSpecs(spec.Services),
		Capabilities:  cloneStrings(spec.Capabilities),
		PolicyRef:     spec.PolicyRef,
		RestartPolicy: spec.RestartPolicy,
	}
}

func toWorkloadServiceSpecs(items []workloadapi.PublishedServiceSnapshot) []domainworkload.ServiceSpec {
	if len(items) == 0 {
		return nil
	}
	out := make([]domainworkload.ServiceSpec, 0, len(items))
	for _, item := range items {
		if item.ID == "" || item.Type == "" {
			continue
		}
		out = append(out, domainworkload.ServiceSpec{
			ID:             item.ID,
			Type:           item.Type,
			Owner:          item.Owner,
			Mode:           item.Mode,
			Endpoints:      cloneStrings(item.Endpoints),
			ProbeEndpoints: cloneStrings(item.ProbeEndpoints),
		})
	}
	return out
}
