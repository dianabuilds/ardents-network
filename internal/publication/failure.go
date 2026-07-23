package publication

import (
	"ardents/internal/diagnostics"
	privacy "ardents/internal/messaging"
)

func (m *Manager) HandleSyncError(err error) error {
	if !privacy.IsChannelGrantFailure(err) {
		return err
	}
	code := privacy.CodeOf(err)
	if persistErr := m.disco.Degrade(code); persistErr != nil {
		return persistErr
	}
	reason := &diagnostics.Reason{
		Code: code, Domain: "discovery", Summary: "private discovery publication is unavailable",
		Detail: err.Error(), Impact: "local state is retained but is not published to the private network",
		Recovery: "operator", OperatorActionRequired: true, Resource: m.cfgName,
	}
	m.diag.SetSubsystem("discovery", diagnostics.HealthDegraded, reason)
	if m.life.State() == diagnostics.Ready || m.life.State() == diagnostics.Initializing {
		if err := m.life.Move(diagnostics.Degraded); err != nil {
			return err
		}
	}
	m.publish("discovery.privacy_degraded", map[string]any{"id": m.cfgName, "reason": code})
	m.diag.RecordEvent("discovery", "privacy_degraded", m.cfgName, reason.Summary, code, nil)
	return nil
}
