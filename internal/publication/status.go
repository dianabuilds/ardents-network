package publication

import (
	"context"
	"errors"
	"time"

	"ardents/internal/discovery"
	"ardents/internal/hosting"
	"ardents/internal/workload/execution"
	hostingreadiness "ardents/internal/workload/readiness"
	"ardents/internal/workload/registry"
)

func (m *Manager) LocalPresenceSnapshotLocked() LocalPresenceSnapshot {
	entry, ok := m.localNodePresenceLocked()
	ready := m.life.State() == "ready"
	if !ok {
		return LocalPresenceSnapshot{
			State:                  "unpublished",
			Reason:                 "local node presence is not published",
			OperatorActionRequired: ready,
		}
	}
	published := len(entry.Record.Endpoints) > 0
	state := "published"
	reason := ""
	if !published {
		state = "withdrawn"
		reason = "local node presence has no active endpoints"
	}
	return LocalPresenceSnapshot{
		Published:              published,
		State:                  state,
		Reason:                 reason,
		RecordID:               entry.Record.ID,
		PublishedAt:            entry.Record.IssuedAt,
		ExpiresAt:              entry.Record.ExpiresAt,
		OperatorActionRequired: !published && ready,
	}
}

func (m *Manager) ServicePublicationStatusLocked(id string) hosting.PublicationSnapshot {
	status, err := m.serviceRuntimeStatusLocked(id)
	if err != nil {
		if entry, ok := m.localServiceEntryLocked(id); ok {
			published := len(entry.Record.Endpoints) > 0
			state := "published"
			reason := ""
			if !published {
				state = "withdrawn"
				reason = "service publication has no active endpoints"
			}
			return hosting.PublicationSnapshot{
				State:                  state,
				Reason:                 reason,
				Published:              published,
				PublishedAt:            entry.Record.IssuedAt,
				ExpiresAt:              entry.Record.ExpiresAt,
				OperatorActionRequired: !published,
			}
		}
		return hosting.PublicationSnapshot{}
	}
	return m.servicePublicationSnapshotLocked(id, m.servicePublicationReasonLocked(id, status.Reason))
}

func (m *Manager) HostedServiceSnapshotLocked(id string) (hosting.ServiceStatusSnapshot, error) {
	status, err := m.serviceRuntimeStatusLocked(id)
	if err != nil {
		return hosting.ServiceStatusSnapshot{}, err
	}
	publication := m.servicePublicationSnapshotLocked(id, m.servicePublicationReasonLocked(id, status.Reason))
	return hosting.ServiceStatusSnapshot{
		ServiceID:              status.ID,
		State:                  status.State,
		Reason:                 hostedReadinessReason(status),
		Published:              publication.Published,
		RuntimeBacking:         status.Source,
		Ready:                  status.Ready,
		ExposureEligible:       status.ExposureEligible,
		Generation:             status.Generation,
		LastProbeAt:            status.LastProbeAt,
		Publication:            publication,
		OperatorActionRequired: !status.Ready,
	}, nil
}

func (m *Manager) servicePublicationSnapshotLocked(id string, reason string) hosting.PublicationSnapshot {
	entry, ok := m.localServiceEntryLocked(id)
	if !ok {
		return hosting.PublicationSnapshot{
			State:                  "unpublished",
			Reason:                 reason,
			OperatorActionRequired: true,
		}
	}
	published := len(entry.Record.Endpoints) > 0
	state := "published"
	publicationReason := reason
	if !published {
		state = "withdrawn"
		if publicationReason == "" {
			publicationReason = "service publication has no active endpoints"
		}
	}
	return hosting.PublicationSnapshot{
		State:                  state,
		Reason:                 publicationReason,
		Published:              published,
		PublishedAt:            entry.Record.IssuedAt,
		ExpiresAt:              entry.Record.ExpiresAt,
		OperatorActionRequired: !published,
	}
}

func (m *Manager) servicePublicationReasonLocked(id, fallback string) string {
	if m.srv == nil || m.trans == nil {
		return fallback
	}
	var allow PolicyFunc
	if m.policy != nil {
		allow = m.policy.AllowServicePublication
	}
	for _, item := range m.srv.ServiceStatuses(time.Now().UTC()) {
		if item.Spec.ID != id {
			continue
		}
		if item.Spec.Mode != "NetworkPublished" {
			return "service mode is not network-published"
		}
		if allow != nil {
			if err := allow(item.Spec); err != nil {
				return err.Error()
			}
		}
		if !item.Readiness.Ready && fallback != "" {
			return fallback
		}
		if err := publicationEligibilityError(item, m.trans.ReachabilitySnapshot(), nil); err != nil {
			return err.Error()
		}
		return fallback
	}
	return fallback
}

func (m *Manager) serviceRuntimeStatusLocked(id string) (hostingreadiness.Status, error) {
	if err := m.observeHostingReadinessLocked(context.Background()); err != nil {
		return hostingreadiness.Status{}, err
	}
	for _, item := range m.workloadStatusesLocked() {
		if status, ok := m.workloadHostedServiceStatus(item, id); ok {
			return status, nil
		}
	}
	var specs []registry.ServiceSpec
	if m.srv != nil {
		specs = m.srv.List()
	}
	for _, spec := range specs {
		if spec.ID == id {
			return staticHostedServiceStatus(spec), nil
		}
	}
	return hostingreadiness.Status{}, errors.New("hosted service not found")
}

func (m *Manager) workloadStatusesLocked() []execution.Status {
	if m.workload == nil {
		return nil
	}
	return m.workload.List()
}

func (m *Manager) workloadHostedServiceStatus(item execution.Status, id string) (hostingreadiness.Status, bool) {
	effective := effectiveWorkloadStatus(item, m.policy)
	for _, svc := range effective.PublishedServices {
		if svc.ID == id {
			return m.publishedHostedServiceStatus(svc), true
		}
	}
	for _, svc := range item.Spec.Services {
		if svc.ID == id {
			return m.unpublishedHostedServiceStatus(svc, m.unpublishedServiceReason(svc)), true
		}
	}
	return hostingreadiness.Status{}, false
}

func (m *Manager) publishedHostedServiceStatus(svc execution.PublishedServiceStatus) hostingreadiness.Status {
	status := hostingreadiness.Status{
		ID:        svc.ID,
		Type:      svc.Type,
		Owner:     svc.Owner,
		Mode:      svc.Mode,
		Published: svc.Published,
		Reason:    svc.Reason,
		Source:    "local",
		Endpoints: append([]string(nil), svc.Endpoints...),
	}
	return m.withProbeStatus(status)
}

func (m *Manager) unpublishedHostedServiceStatus(svc registry.ServiceSpec, reason string) hostingreadiness.Status {
	return m.withProbeStatus(hostingreadiness.Status{
		ID:        svc.ID,
		Type:      svc.Type,
		Owner:     svc.Owner,
		Mode:      svc.Mode,
		Published: false,
		Reason:    reason,
		Source:    "local",
		Endpoints: append([]string(nil), svc.Endpoints...),
	})
}

func (m *Manager) withProbeStatus(status hostingreadiness.Status) hostingreadiness.Status {
	if m.srv == nil {
		return status
	}
	probe, ok := m.srv.Readiness(status.ID, time.Now().UTC())
	if !ok {
		return status
	}
	status.State = probe.State
	status.Ready = probe.Ready
	status.ExposureEligible = probe.ExposureEligible
	status.Generation = probe.Generation
	status.LastProbeAt = probe.LastProbeAt
	status.EndpointStatuses = append([]hostingreadiness.EndpointStatus(nil), probe.Endpoints...)
	status.ReadinessReason = probe.Reason
	return status
}

func hostedReadinessReason(status hostingreadiness.Status) string {
	if status.ReadinessReason != "" {
		return status.ReadinessReason
	}
	return status.Reason
}

func (m *Manager) observeHostingReadinessLocked(ctx context.Context) error {
	if m.srv == nil {
		return nil
	}
	items := m.workloadStatusesLocked()
	backings := make([]registry.Backing, 0)
	for _, item := range items {
		for _, svc := range item.Spec.Services {
			backings = append(backings, registry.Backing{Spec: svc, WorkloadID: item.Spec.ID,
				Generation: item.Instance.Generation, Running: item.Observed == execution.ObservedRunning && item.Instance.Running,
				StartedAt: item.Instance.StartedAt})
		}
	}
	return m.srv.Observe(ctx, backings, time.Now().UTC())
}

func (m *Manager) unpublishedServiceReason(svc registry.ServiceSpec) string {
	if m.policy == nil {
		return "service has no active runtime backing"
	}
	if err := m.policy.AllowServicePublication(svc); err != nil {
		return err.Error()
	}
	return "service has no active runtime backing"
}

func (m *Manager) localNodePresenceLocked() (entry discovery.Entry, ok bool) {
	principal := m.ident.NodeSummary().Principal
	for _, item := range m.disco.Entries() {
		if item.Source == "local" && item.Record.Kind == "node" && item.Record.Subject == principal {
			return item, true
		}
	}
	return discovery.Entry{}, false
}

func (m *Manager) localServiceEntryLocked(id string) (entry discovery.Entry, ok bool) {
	for _, item := range m.disco.Entries() {
		if item.Source == "local" && item.Record.Kind == "service" && item.Record.ID == id {
			return item, true
		}
	}
	return discovery.Entry{}, false
}
