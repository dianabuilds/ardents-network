// Package hosting owns operator-facing hosted-service inventory and publication/readiness composition.
// It does not own workload transitions, probes, or advertisements.
package hosting

import (
	"strings"
	"sync"
	"time"

	"ardents/internal/workload"
	"ardents/internal/workload/readiness"
	"ardents/internal/workload/registry"
)

type Publication interface {
	HostedServiceSnapshotLocked(string) (ServiceStatusSnapshot, error)
	ServicePublicationStatusLocked(string) PublicationSnapshot
}

type Service struct {
	lock        sync.Locker
	workloads   *workload.Runtime
	registry    *registry.Registry
	publication Publication
}

func NewService(lock sync.Locker, workloads *workload.Runtime, services *registry.Registry, publication Publication) *Service {
	return &Service{lock: lock, workloads: workloads, registry: services, publication: publication}
}

func (s *Service) ListHostedServices() ([]ServiceSnapshot, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	items, err := s.workloads.List()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	out := make([]ServiceSnapshot, 0)
	seen := map[string]struct{}{}
	for _, item := range items {
		for _, spec := range item.Spec.Services {
			probe, _ := s.registry.Readiness(spec.ID, now)
			out = append(out, workloadServiceSnapshot(item, spec, probe))
			seen[spec.ID] = struct{}{}
		}
	}
	for _, spec := range s.registry.List() {
		if _, ok := seen[spec.ID]; ok {
			continue
		}
		out = append(out, ServiceSnapshot{
			ID: spec.ID, Type: spec.Type, Owner: spec.Owner, Visibility: spec.Mode,
			DesiredPublication: "registered", RuntimeBacking: "unbound", Readiness: readiness.StateInactive,
			Endpoints: endpointSnapshots(spec.Endpoints, false, readiness.ReasonRuntimeInactive),
		})
	}
	return out, nil
}

func (s *Service) GetHostedService(id string) (ServiceStatusSnapshot, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.publication.HostedServiceSnapshotLocked(id)
}

func (s *Service) GetServicePublicationStatus(id string) (PublicationSnapshot, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.publication.ServicePublicationStatusLocked(id), nil
}

func workloadServiceSnapshot(status workload.StatusSnapshot, spec workload.PublishedServiceSnapshot, probe readiness.Snapshot) ServiceSnapshot {
	snapshot := ServiceSnapshot{
		ID: spec.ID, Type: spec.Type, Owner: spec.Owner, WorkloadID: status.Spec.ID, Visibility: spec.Mode,
		DesiredPublication: status.Spec.Desired, RuntimeBacking: "inactive", PolicyRef: status.Spec.PolicyRef,
		Readiness: probe.State, Ready: probe.Ready, ExposureEligible: probe.ExposureEligible,
		Generation: probe.Generation, LastProbeAt: probe.LastProbeAt,
	}
	for _, published := range status.PublishedServices {
		if published.ID == spec.ID {
			if published.Published {
				snapshot.RuntimeBacking = "active"
			}
			snapshot.Endpoints = probedEndpoints(published.Endpoints, probe, published.Reason)
			return snapshot
		}
	}
	snapshot.Endpoints = probedEndpoints(spec.Endpoints, probe, "service is not published")
	return snapshot
}

func probedEndpoints(items []string, probe readiness.Snapshot, fallback string) []EndpointSnapshot {
	out := make([]EndpointSnapshot, 0, len(items))
	for index, item := range items {
		status := readiness.EndpointStatus{}
		found := index < len(probe.Endpoints)
		if found {
			status = probe.Endpoints[index]
		}
		reason := fallback
		if found {
			reason = status.Reason
		}
		protocol := endpointProtocol(item)
		out = append(out, EndpointSnapshot{Kind: protocol, Address: item, Protocol: protocol, Exposure: "network", Reachable: found && status.Reachable, Reason: reason})
	}
	return out
}

func endpointSnapshots(items []string, reachable bool, reason string) []EndpointSnapshot {
	out := make([]EndpointSnapshot, 0, len(items))
	for _, item := range items {
		protocol := endpointProtocol(item)
		out = append(out, EndpointSnapshot{Kind: protocol, Address: item, Protocol: protocol, Exposure: "network", Reachable: reachable, Reason: reason})
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
