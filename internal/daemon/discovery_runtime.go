package daemon

import (
	"ardents/internal/diagnostics"
	"ardents/internal/discovery"
	"fmt"
	"strings"
)

func DegradeDiscoveryPrivacy(
	diag *diagnostics.Recorder,
	resource string,
	code string,
	rejected int,
	setDiscoveryDegraded func(string),
	adoptPrimary func(domain string, state string, reason *diagnostics.Reason),
	moveLifecycle func(string),
) {
	detail := fmt.Sprintf("private discovery channel rejected %d envelope(s)", rejected)
	if rejected == 0 {
		detail = "private discovery channel is unavailable"
	}
	reason := &diagnostics.Reason{
		Code: code, Domain: "discovery", Summary: "private discovery is degraded",
		Detail: detail, Impact: "remote discovery may be incomplete; retained records remain usable",
		Recovery: "operator", OperatorActionRequired: true, Resource: resource,
	}
	setDiscoveryDegraded(code)
	diag.SetSubsystem("discovery", diagnostics.HealthDegraded, reason)
	if diag.Health().State != diagnostics.HealthFailed {
		adoptPrimary("discovery", diagnostics.HealthDegraded, reason)
		moveLifecycle("degraded")
	}
	diag.RecordEvent("discovery", "privacy_degraded", resource, reason.Summary, code, map[string]any{"rejected": rejected})
}

func (m *RuntimeManager) setDiscoveryDegradedLocked(reason string) {
	if err := m.disco.Degrade(reason); err != nil {
		m.recordDiscoveryPersistenceFailureLocked(err)
	}
}

func (m *RuntimeManager) setDiscoveryReadyLocked() {
	if err := m.disco.Ready(); err != nil {
		m.recordDiscoveryPersistenceFailureLocked(err)
	}
}

func (m *RuntimeManager) recordDiscoveryPersistenceFailureLocked(err error) {
	if m.diag == nil {
		return
	}
	m.diag.RecordEvent("discovery", "state_persistence_failed", m.cfgName, "discovery state persistence failed", "discovery.persistence.failed", map[string]any{"detail": err.Error()})
}

const trustSubsystem = "trust"

func (m *RuntimeManager) SyncDiscoveryTrustDiagnosticsLocked() {
	issue, ok := m.discoveryTrustIssueLocked()
	if !ok || issue == nil {
		m.clearDiscoveryTrustDiagnosticsLocked()
		return
	}
	if issue.Code == "trust.record.untrusted" {
		m.clearDiscoveryTrustDiagnosticsLocked()
		m.diag.RecordEvent(trustSubsystem, "catalog_untrusted", issue.Resource, issue.Summary, issue.Code, map[string]any{
			"detail": issue.Detail,
		})
		return
	}

	m.diag.SetSubsystem(trustSubsystem, diagnostics.HealthDegraded, issue)
	if m.diag.Health().State != diagnostics.HealthFailed {
		m.adoptPrimaryReasonLocked(trustSubsystem, diagnostics.HealthDegraded, issue)
		m.moveLifecycleLocked(diagnostics.Degraded)
	}
	m.diag.RecordEvent(trustSubsystem, "catalog_degraded", issue.Resource, issue.Summary, issue.Code, map[string]any{
		"detail": issue.Detail,
	})
}

func (m *RuntimeManager) clearDiscoveryTrustDiagnosticsLocked() {
	if !trustReasonActive(m.diag.Health()) {
		return
	}
	m.diag.ClearSubsystem(trustSubsystem)
	m.restorePrimaryReasonLocked(trustSubsystem)
	m.diag.RecordEvent(trustSubsystem, "catalog_ready", m.cfgName, "discovery trust catalog is usable", "", map[string]any{
		"records": len(m.disco.Entries()),
	})
}

func (m *RuntimeManager) discoveryTrustIssueLocked() (*diagnostics.Reason, bool) {
	var (
		selected     *diagnostics.Reason
		selectedRank int
	)
	for _, entry := range m.disco.Entries() {
		result := m.trust.Evaluate(entry.Record)
		if result.Usable {
			continue
		}
		reason := discoveryTrustReason(entry, result)
		rank := discoveryTrustReasonRank(reason)
		if selected == nil || rank > selectedRank {
			selected = reason
			selectedRank = rank
		}
	}
	return selected, selected != nil
}

func discoveryTrustReason(entry discovery.Entry, result discovery.TrustResult) *diagnostics.Reason {
	reason := &diagnostics.Reason{
		Code:                   "trust.record.invalid",
		Domain:                 trustSubsystem,
		Summary:                "discovery catalog contains invalid record",
		Detail:                 result.Reason,
		Impact:                 "discovery routing truth may include records that cannot be used safely",
		Recovery:               "operator",
		OperatorActionRequired: true,
		Resource:               entry.Record.ID,
	}
	switch {
	case result.Outcome == "expired":
		reason.Code = "trust.record.expired"
		reason.Summary = "discovery catalog contains expired record"
		reason.Impact = "discovery catalog still carries records that are no longer usable"
	case result.Valid && !result.Trusted:
		reason.Code = "trust.record.untrusted"
		reason.Summary = "discovery catalog contains untrusted record"
		reason.Impact = "remote discovery matches may resolve but remain unusable for trusted routing"
	case strings.TrimSpace(result.Reason) == "":
		reason.Detail = "record is not usable"
	}
	return reason
}

func trustReasonActive(health diagnostics.HealthSummary) bool {
	for _, item := range health.Subsystems {
		if item.Domain != trustSubsystem || item.Reason == nil {
			continue
		}
		return true
	}
	return false
}

func discoveryTrustReasonRank(reason *diagnostics.Reason) int {
	if reason == nil {
		return 0
	}
	switch reason.Code {
	case "trust.record.invalid", "trust.record.expired":
		return 2
	case "trust.record.untrusted":
		return 1
	default:
		return 2
	}
}

func (n *Node) ResolveRecord(subject, kind string) (discovery.ResolutionResult, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.discoveryResolver.ResolveRecord(subject, kind)
}

func (n *Node) ResolveService(serviceType string) (discovery.ServiceResult, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.discoveryResolver.ResolveService(serviceType)
}

func (n *Node) ListRecords() ([]discovery.CatalogRecordSnapshot, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.discoveryCommands.ListRecords()
}

func (n *Node) ImportRecord(record discovery.CatalogRecordSnapshot) (discovery.RecordImportResult, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.discoveryCommands.ImportRecord(record)
}
