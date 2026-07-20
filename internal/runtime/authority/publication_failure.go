package authority

import (
	"ardents/internal/diagnostics"
	networkprivacy "ardents/internal/network/privacy"
	nodelifecycle "ardents/internal/node/lifecycle"
)

func (c *Controller) handlePublicationErrorLocked(err error) error {
	if !networkprivacy.IsCapabilityFailure(err) {
		return err
	}
	code := networkprivacy.CodeOf(err)
	if persistErr := c.disco.Degrade(code); persistErr != nil {
		return persistErr
	}
	reason := &diagnostics.Reason{
		Code: code, Domain: "discovery", Summary: "private discovery publication is unavailable",
		Detail: err.Error(), Impact: "local state is retained but is not published to the private network",
		Recovery: "operator", OperatorActionRequired: true, Resource: c.cfgName,
	}
	c.diag.SetSubsystem("discovery", diagnostics.HealthDegraded, reason)
	if c.life.State() == nodelifecycle.Ready || c.life.State() == nodelifecycle.Initializing {
		_ = c.life.Move(nodelifecycle.Degraded)
	}
	c.publish("discovery.privacy_degraded", map[string]any{"id": c.cfgName, "reason": code})
	c.diag.RecordEvent("discovery", "privacy_degraded", c.cfgName, reason.Summary, code, nil)
	return nil
}
