package authority

import (
	"ardents/internal/diagnostics"
	networkprivacy "ardents/internal/network/privacy"
	nodelifecycle "ardents/internal/node/lifecycle"
)

func (c *Controller) handleDataPrivacyFailureLocked(err error) bool {
	if !networkprivacy.IsCapabilityFailure(err) {
		return false
	}
	code := networkprivacy.CodeOf(err)
	reason := &diagnostics.Reason{
		Code: code, Domain: "data", Summary: "private data exchange is unavailable",
		Detail: err.Error(), Impact: "local data remains available but remote blob exchange is disabled",
		Recovery: "operator", OperatorActionRequired: true, Resource: c.cfgName,
	}
	c.diag.SetSubsystem("data", diagnostics.HealthDegraded, reason)
	if c.life.State() == nodelifecycle.Ready || c.life.State() == nodelifecycle.Initializing {
		_ = c.life.Move(nodelifecycle.Degraded)
	}
	c.publish("data.privacy_degraded", map[string]any{"id": c.cfgName, "reason": code})
	c.diag.RecordEvent("data", "privacy_degraded", c.cfgName, reason.Summary, code, nil)
	return true
}
