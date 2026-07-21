package workload

import "ardents/internal/workload/registry"

func SpecFromSnapshot(spec SpecSnapshot) registry.Spec {
	return registry.Spec{
		ID: spec.ID, Kind: spec.Kind, Owner: spec.Owner, Config: spec.Config,
		Desired: spec.Desired, Services: serviceSpecsFromSnapshots(spec.Services),
		Capabilities: append([]string(nil), spec.Capabilities...), PolicyRef: spec.PolicyRef,
		RestartPolicy: spec.RestartPolicy,
	}
}

func serviceSpecsFromSnapshots(items []PublishedServiceSnapshot) []registry.ServiceSpec {
	if len(items) == 0 {
		return nil
	}
	out := make([]registry.ServiceSpec, 0, len(items))
	for _, item := range items {
		if item.ID == "" || item.Type == "" {
			continue
		}
		out = append(out, registry.ServiceSpec{
			ID: item.ID, Type: item.Type, Owner: item.Owner, Mode: item.Mode,
			Endpoints:      append([]string(nil), item.Endpoints...),
			ProbeEndpoints: append([]string(nil), item.ProbeEndpoints...),
		})
	}
	return out
}
