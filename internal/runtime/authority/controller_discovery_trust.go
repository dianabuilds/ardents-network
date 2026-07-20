package authority

import (
	"strings"

	"ardents/internal/diagnostics"
	discovery "ardents/internal/discovery"
	nodelifecycle "ardents/internal/node/lifecycle"
)

const trustSubsystem = "trust"

func (c *Controller) moveLifecycleLocked(next string) {
	if err := c.life.Move(next); err != nil {
		c.diag.RecordEvent("node", "lifecycle_transition_rejected", c.cfgName, "lifecycle transition rejected", "node.lifecycle.transition_rejected", map[string]any{
			"from":  c.life.State(),
			"to":    next,
			"error": err.Error(),
		})
	}
}

func (c *Controller) SyncDiscoveryTrustDiagnosticsLocked() {
	issue, ok := c.discoveryTrustIssueLocked()
	if !ok {
		c.clearDiscoveryTrustDiagnosticsLocked()
		return
	}
	if issue.Code == "trust.record.untrusted" {
		c.clearDiscoveryTrustDiagnosticsLocked()
		c.diag.RecordEvent(trustSubsystem, "catalog_untrusted", issue.Resource, issue.Summary, issue.Code, map[string]any{
			"detail": issue.Detail,
		})
		return
	}

	c.diag.SetSubsystem(trustSubsystem, diagnostics.HealthDegraded, issue)
	if c.diag.Health().State != diagnostics.HealthFailed {
		c.adoptPrimaryReasonLocked(trustSubsystem, diagnostics.HealthDegraded, issue)
		c.moveLifecycleLocked(nodelifecycle.Degraded)
	}
	c.diag.RecordEvent(trustSubsystem, "catalog_degraded", issue.Resource, issue.Summary, issue.Code, map[string]any{
		"detail": issue.Detail,
	})
}

func (c *Controller) clearDiscoveryTrustDiagnosticsLocked() {
	if !trustReasonActive(c.diag.Health()) {
		return
	}
	c.diag.ClearSubsystem(trustSubsystem)
	c.restorePrimaryReasonLocked(trustSubsystem)
	c.diag.RecordEvent(trustSubsystem, "catalog_ready", c.cfgName, "discovery trust catalog is usable", "", map[string]any{
		"records": len(c.disco.Entries()),
	})
}

func (c *Controller) discoveryTrustIssueLocked() (*diagnostics.Reason, bool) {
	var (
		selected     *diagnostics.Reason
		selectedRank int
	)
	for _, entry := range c.disco.Entries() {
		result := c.trust.Evaluate(entry.Record)
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
