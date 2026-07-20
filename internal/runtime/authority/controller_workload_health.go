package authority

import (
	"ardents/internal/diagnostics"
	nodelifecycle "ardents/internal/node/lifecycle"
	"ardents/internal/workload/observedstate"
)

const workloadRefreshFailedCode = "workload.observation.refresh_failed"

func (c *Controller) recordWorkloadRefreshFailureLocked(err error) {
	reason := &diagnostics.Reason{
		Code:                   workloadRefreshFailedCode,
		Domain:                 "workload",
		Summary:                "workload observation refresh failed",
		Detail:                 err.Error(),
		Impact:                 "workload and hosted service truth may be stale on operator surfaces",
		Recovery:               "operator",
		OperatorActionRequired: true,
		Resource:               c.cfgName,
	}
	c.diag.SetSubsystem("workload", diagnostics.HealthDegraded, reason)
	if c.diag.Health().State != diagnostics.HealthFailed {
		c.adoptPrimaryReasonLocked("workload", diagnostics.HealthDegraded, reason)
		switch c.life.State() {
		case nodelifecycle.Ready, nodelifecycle.Degraded:
			c.moveLifecycleLocked(nodelifecycle.Degraded)
		}
	}
	c.publish("workload.refresh_failed", map[string]any{"id": c.cfgName, "error": err.Error()})
	c.diag.RecordEvent("workload", "refresh_failed", c.cfgName, "workload observation refresh failed", workloadRefreshFailedCode, map[string]any{"detail": err.Error()})
}

func (c *Controller) clearWorkloadRefreshFailureLocked() {
	if workloadReasonCode(c.diag.Health()) != workloadRefreshFailedCode {
		return
	}
	c.diag.ClearSubsystem("workload")
	c.restorePrimaryReasonLocked("workload")
}

func (c *Controller) evaluateWorkloadHealthLocked() {
	var reason *diagnostics.Reason
	for _, item := range c.workload.List() {
		if !workloadImpactsNode(item) {
			continue
		}
		if item.Observed == observedstate.Running || item.Observed == observedstate.Accepted {
			continue
		}
		reason = &diagnostics.Reason{
			Code:                   "workload.hosted_service.degraded",
			Domain:                 "workload",
			Summary:                "node-owned hosted service is impaired",
			Detail:                 item.Reason,
			Impact:                 "hosted service publication is unavailable",
			Recovery:               "operator",
			OperatorActionRequired: true,
			Resource:               item.Spec.ID,
		}
		break
	}
	if reason == nil {
		c.diag.ClearSubsystem("workload")
		c.restorePrimaryReasonLocked("workload")
		return
	}
	c.diag.SetSubsystem("workload", diagnostics.HealthDegraded, reason)
	if c.diag.Health().State != diagnostics.HealthFailed {
		c.adoptPrimaryReasonLocked("workload", diagnostics.HealthDegraded, reason)
		c.moveLifecycleLocked(nodelifecycle.Degraded)
	}
}

func workloadReasonCode(health diagnostics.HealthSummary) string {
	for _, item := range health.Subsystems {
		if item.Domain != "workload" || item.Reason == nil {
			continue
		}
		return item.Reason.Code
	}
	return ""
}

func (c *Controller) adoptPrimaryReasonLocked(domain string, state string, reason *diagnostics.Reason) {
	health := c.diag.Health()
	if health.PrimaryReason != nil && health.PrimaryReason.Domain != domain {
		return
	}
	c.diag.SetPrimary(state, reason)
}

func (c *Controller) restorePrimaryReasonLocked(domain string) {
	health := c.diag.Health()
	if health.PrimaryReason == nil || health.PrimaryReason.Domain != domain {
		return
	}
	c.diag.ClearPrimary()
	for _, item := range c.diag.Health().Subsystems {
		if item.Reason == nil {
			continue
		}
		c.diag.SetPrimary(item.State, item.Reason)
		return
	}
}

func workloadImpactsNode(item observedstate.Status) bool {
	if item.Spec.Owner != "node" {
		return false
	}
	return len(item.Spec.Services) > 0
}
