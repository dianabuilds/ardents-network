package projection

import (
	"context"
	"strings"
	"time"

	hostingapi "ardents/internal/hosting/api"
	hostingreadiness "ardents/internal/hosting/readiness"
	hostingregistry "ardents/internal/hosting/registry"
	"ardents/internal/workload/observedstate"
	domainworkload "ardents/internal/workload/workload"
)

func (s *QueryService) ListHostedServicesLocked() ([]hostingapi.HostedServiceSnapshot, error) {
	if err := s.workload.SyncObservedWorkloadsLocked(context.Background()); err != nil {
		return nil, err
	}
	statuses := s.reader.workload.List()
	now := time.Now().UTC()
	if err := s.reader.srv.Observe(context.Background(), hostedServiceBackings(statuses), now); err != nil {
		return nil, err
	}
	items := make([]hostingapi.HostedServiceSnapshot, 0)
	seen := map[string]struct{}{}
	for _, item := range statuses {
		for _, svc := range item.Spec.Services {
			probe, _ := s.reader.srv.Readiness(svc.ID, now)
			items = append(items, hostedServiceSnapshot(item, svc, probe))
			seen[svc.ID] = struct{}{}
		}
	}
	for _, spec := range s.reader.srv.List() {
		if _, ok := seen[spec.ID]; ok {
			continue
		}
		items = append(items, hostingapi.HostedServiceSnapshot{
			ID:                 spec.ID,
			Type:               spec.Type,
			Owner:              spec.Owner,
			Visibility:         spec.Mode,
			DesiredPublication: "registered",
			RuntimeBacking:     "unbound",
			Readiness:          hostingreadiness.StateInactive,
			Endpoints:          endpointSnapshots(spec.Endpoints, false, hostingreadiness.ReasonRuntimeInactive),
		})
	}
	return items, nil
}

func hostedServiceSnapshot(status observedstate.Status, svc domainworkload.ServiceSpec, probe hostingreadiness.Snapshot) hostingapi.HostedServiceSnapshot {
	runtimeBacking := "inactive"
	snapshot := workloadHostedServiceBase(status, svc, probe)
	for _, published := range status.PublishedServices {
		if published.ID == svc.ID {
			if published.Published {
				runtimeBacking = "active"
			}
			snapshot.RuntimeBacking = runtimeBacking
			snapshot.Endpoints = probedEndpointSnapshots(published.Endpoints, probe, published.Reason)
			return snapshot
		}
	}
	snapshot.Endpoints = probedEndpointSnapshots(svc.Endpoints, probe, "service is not published")
	return snapshot
}

func workloadHostedServiceBase(status observedstate.Status, svc domainworkload.ServiceSpec, probe hostingreadiness.Snapshot) hostingapi.HostedServiceSnapshot {
	return hostingapi.HostedServiceSnapshot{
		ID:                 svc.ID,
		Type:               svc.Type,
		Owner:              svc.Owner,
		WorkloadID:         status.Spec.ID,
		Visibility:         svc.Mode,
		DesiredPublication: status.Spec.Desired,
		RuntimeBacking:     "inactive",
		PolicyRef:          status.Spec.PolicyRef,
		Readiness:          probe.State,
		Ready:              probe.Ready,
		ExposureEligible:   probe.ExposureEligible,
		Generation:         probe.Generation,
		LastProbeAt:        probe.LastProbeAt,
	}
}

func hostedServiceBackings(items []observedstate.Status) []hostingregistry.Backing {
	out := make([]hostingregistry.Backing, 0)
	for _, item := range items {
		for _, svc := range item.Spec.Services {
			out = append(out, hostingregistry.Backing{Spec: svc, WorkloadID: item.Spec.ID,
				Generation: item.Instance.Generation, Running: item.Observed == observedstate.Running && item.Instance.Running,
				StartedAt: item.Instance.StartedAt})
		}
	}
	return out
}

func probedEndpointSnapshots(items []string, probe hostingreadiness.Snapshot, fallbackReason string) []hostingapi.ServiceEndpointSnapshot {
	out := make([]hostingapi.ServiceEndpointSnapshot, 0, len(items))
	for index, item := range items {
		var status hostingreadiness.EndpointStatus
		found := index < len(probe.Endpoints)
		if found {
			status = probe.Endpoints[index]
		}
		reason := fallbackReason
		if found {
			reason = status.Reason
		}
		protocol := endpointProtocol(item)
		out = append(out, hostingapi.ServiceEndpointSnapshot{Kind: protocol, Address: item, Protocol: protocol,
			Exposure: "network", Reachable: found && status.Reachable, Reason: reason})
	}
	return out
}

func endpointSnapshots(items []string, reachable bool, reason string) []hostingapi.ServiceEndpointSnapshot {
	if len(items) == 0 {
		return nil
	}
	out := make([]hostingapi.ServiceEndpointSnapshot, 0, len(items))
	for _, item := range items {
		protocol := endpointProtocol(item)
		out = append(out, hostingapi.ServiceEndpointSnapshot{
			Kind:      protocol,
			Address:   item,
			Protocol:  protocol,
			Exposure:  "network",
			Reachable: reachable,
			Reason:    reason,
		})
	}
	return out
}

func endpointProtocol(endpoint string) string {
	if strings.HasPrefix(endpoint, "/") {
		return "multiaddr"
	}
	if prefix, _, ok := strings.Cut(endpoint, "://"); ok {
		return prefix
	}
	return "unknown"
}
