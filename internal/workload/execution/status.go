package execution

import (
	"ardents/internal/workload/registry"
	"time"
)

const (
	ObservedAccepted  = "accepted"
	ObservedPreparing = "preparing"
	ObservedRunning   = "running"
	ObservedStopping  = "stopping"
	ObservedStopped   = "stopped"
	ObservedFailed    = "failed"
	ObservedDegraded  = "degraded"
	ObservedRemoved   = "removed"
)

type PublishedServiceStatus struct {
	ID             string   `json:"id"`
	Type           string   `json:"type"`
	Owner          string   `json:"owner"`
	Mode           string   `json:"mode"`
	Published      bool     `json:"published"`
	Endpoints      []string `json:"endpoints,omitempty"`
	ProbeEndpoints []string `json:"probe_endpoints,omitempty"`
	Reason         string   `json:"reason,omitempty"`
}

type Status struct {
	Spec                registry.Spec            `json:"spec"`
	Observed            string                   `json:"observed"`
	Reason              string                   `json:"reason,omitempty"`
	LastTransitionAt    time.Time                `json:"last_transition_at"`
	NeedsOperatorAction bool                     `json:"needs_operator_action"`
	RestartCount        int                      `json:"restart_count"`
	PublishedServices   []PublishedServiceStatus `json:"published_services,omitempty"`
	Instance            Instance                 `json:"instance"`
}

func NormalizeStatus(item Status) Status {
	item.Spec = registry.NormalizeSpec(item.Spec)
	item.PublishedServices = append([]PublishedServiceStatus(nil), item.PublishedServices...)
	if item.PublishedServices == nil {
		item.PublishedServices = ServiceStatuses(item.Spec, item.Observed == ObservedRunning, item.Reason)
	}
	return item
}

func NormalizeItems(items map[string]Status) map[string]Status {
	if items == nil {
		return map[string]Status{}
	}
	out := make(map[string]Status, len(items))
	for id, item := range items {
		out[id] = NormalizeStatus(item)
	}
	return out
}

func SnapshotStatuses(items map[string]Status) []Status {
	out := make([]Status, 0, len(items))
	for _, item := range items {
		out = append(out, CloneStatus(item))
	}
	return out
}

func ServiceStatuses(spec registry.Spec, published bool, reason string) []PublishedServiceStatus {
	if len(spec.Services) == 0 {
		return []PublishedServiceStatus{}
	}
	out := make([]PublishedServiceStatus, 0, len(spec.Services))
	for _, item := range spec.Services {
		out = append(out, PublishedServiceStatus{
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

func CloneStatus(item Status) Status {
	item.Spec.Services = append([]registry.ServiceSpec(nil), item.Spec.Services...)
	for i := range item.Spec.Services {
		item.Spec.Services[i].Endpoints = append([]string(nil), item.Spec.Services[i].Endpoints...)
		item.Spec.Services[i].ProbeEndpoints = append([]string(nil), item.Spec.Services[i].ProbeEndpoints...)
	}
	item.Spec.Requirements = append([]registry.WorkloadRequirement(nil), item.Spec.Requirements...)
	item.PublishedServices = append([]PublishedServiceStatus(nil), item.PublishedServices...)
	for i := range item.PublishedServices {
		item.PublishedServices[i].Endpoints = append([]string(nil), item.PublishedServices[i].Endpoints...)
		item.PublishedServices[i].ProbeEndpoints = append([]string(nil), item.PublishedServices[i].ProbeEndpoints...)
	}
	return item
}
