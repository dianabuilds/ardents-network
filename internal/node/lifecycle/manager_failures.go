package lifecycle

import (
	"ardents/internal/diagnostics"
	nodereadiness "ardents/internal/node/readiness"
)

func (m *Manager) runtimeFailureLocked(action string) error {
	detail := ""
	if health := m.diag.Health(); health.PrimaryReason != nil {
		detail = health.PrimaryReason.Detail
	}
	return nodereadiness.RuntimeFailure(action, m.life.State() == Failed, detail)
}

func (m *Manager) moveLifecycleLocked(next string) {
	if err := m.life.Move(next); err != nil {
		m.diag.RecordEvent("node", "lifecycle_transition_rejected", m.cfgName, "lifecycle transition rejected", "node.lifecycle.transition_rejected", map[string]any{
			"from":  m.life.State(),
			"to":    next,
			"error": err.Error(),
		})
	}
}

func (m *Manager) FailLocked(code, domain, summary, detail, impact, recovery string) {
	m.diag.SetPrimary(diagnostics.HealthFailed, &diagnostics.Reason{
		Code:                   code,
		Domain:                 domain,
		Summary:                summary,
		Detail:                 detail,
		Impact:                 impact,
		Recovery:               recovery,
		OperatorActionRequired: true,
		Resource:               m.cfgName,
	})
	m.diag.SetSubsystem(domain, diagnostics.HealthFailed, &diagnostics.Reason{
		Code:                   code,
		Domain:                 domain,
		Summary:                summary,
		Detail:                 detail,
		Impact:                 impact,
		Recovery:               recovery,
		OperatorActionRequired: true,
		Resource:               m.cfgName,
	})
	if m.life.State() == Starting {
		m.moveLifecycleLocked(Initializing)
	}
	m.moveLifecycleLocked(Failed)
	m.publish("node.failed", map[string]any{"id": m.cfgName, "reason": detail, "code": code})
	m.diag.RecordEvent("node", "failed", m.cfgName, summary, code, map[string]any{"detail": detail})
}

func (m *Manager) adoptPrimaryReasonLocked(domain string, state string, reason *diagnostics.Reason) {
	nodereadiness.AdoptPrimaryReason(m.diag, domain, state, reason)
}

func (m *Manager) restorePrimaryReasonLocked(domain string) {
	nodereadiness.RestorePrimaryReason(m.diag, domain)
}

func (m *Manager) promoteSubsystemPrimaryLocked(domain string) {
	nodereadiness.PromoteSubsystemPrimary(m.diag, domain)
}
