package registry

import (
	"fmt"

	"ardents/internal/workload/desiredstate"
	"ardents/internal/workload/observedstate"
	domainworkload "ardents/internal/workload/workload"
)

func ValidateSpec(spec domainworkload.Spec) error {
	if spec.ID == "" {
		return fmt.Errorf("missing workload id")
	}
	if spec.Kind == "" {
		return fmt.Errorf("missing workload kind")
	}
	switch spec.Kind {
	case "service", "worker", "app", "adapter":
	default:
		return fmt.Errorf("unsupported workload kind %s", spec.Kind)
	}
	if spec.Config == "invalid" {
		return fmt.Errorf("invalid config reference")
	}
	return nil
}

func NormalizeSpec(spec domainworkload.Spec) domainworkload.Spec {
	if spec.Services == nil {
		spec.Services = []domainworkload.ServiceSpec{}
	}
	if spec.Capabilities == nil {
		spec.Capabilities = []string{}
	}
	spec.Desired = desiredstate.Normalize(spec.Desired)
	if spec.RestartPolicy == "" {
		spec.RestartPolicy = domainworkload.DefaultRestartPolicy
	}
	return spec
}

func NormalizeStatus(item observedstate.Status) observedstate.Status {
	item.Spec = NormalizeSpec(item.Spec)
	item.PublishedServices = append([]observedstate.PublishedServiceStatus(nil), item.PublishedServices...)
	if item.PublishedServices == nil {
		item.PublishedServices = ServiceStatuses(item.Spec, item.Observed == observedstate.Running, item.Reason)
	}
	return item
}

func NormalizeItems(items map[string]observedstate.Status) map[string]observedstate.Status {
	if items == nil {
		return map[string]observedstate.Status{}
	}
	out := make(map[string]observedstate.Status, len(items))
	for id, item := range items {
		out[id] = NormalizeStatus(item)
	}
	return out
}

func SnapshotStatuses(items map[string]observedstate.Status) []observedstate.Status {
	out := make([]observedstate.Status, 0, len(items))
	for _, item := range items {
		out = append(out, CloneStatus(item))
	}
	return out
}

func ServiceStatuses(spec domainworkload.Spec, published bool, reason string) []observedstate.PublishedServiceStatus {
	if len(spec.Services) == 0 {
		return []observedstate.PublishedServiceStatus{}
	}
	out := make([]observedstate.PublishedServiceStatus, 0, len(spec.Services))
	for _, item := range spec.Services {
		out = append(out, observedstate.PublishedServiceStatus{
			ID:             item.ID,
			Type:           item.Type,
			Owner:          spec.ID,
			Mode:           item.Mode,
			Published:      published,
			Endpoints:      append([]string(nil), item.Endpoints...),
			ProbeEndpoints: append([]string(nil), item.ProbeEndpoints...),
			Reason:         reason,
		})
	}
	return out
}

func CloneStatus(item observedstate.Status) observedstate.Status {
	item.Spec.Services = append([]domainworkload.ServiceSpec(nil), item.Spec.Services...)
	for i := range item.Spec.Services {
		item.Spec.Services[i].Endpoints = append([]string(nil), item.Spec.Services[i].Endpoints...)
		item.Spec.Services[i].ProbeEndpoints = append([]string(nil), item.Spec.Services[i].ProbeEndpoints...)
	}
	item.Spec.Capabilities = append([]string(nil), item.Spec.Capabilities...)
	item.PublishedServices = append([]observedstate.PublishedServiceStatus(nil), item.PublishedServices...)
	for i := range item.PublishedServices {
		item.PublishedServices[i].Endpoints = append([]string(nil), item.PublishedServices[i].Endpoints...)
		item.PublishedServices[i].ProbeEndpoints = append([]string(nil), item.PublishedServices[i].ProbeEndpoints...)
	}
	return item
}
