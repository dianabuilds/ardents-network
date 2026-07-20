package lifecycle

func (m *Manager) setDiscoveryDegradedLocked(reason string) {
	if err := m.disco.Degrade(reason); err != nil {
		m.recordDiscoveryPersistenceFailureLocked(err)
	}
}

func (m *Manager) setDiscoveryReadyLocked() {
	if err := m.disco.Ready(); err != nil {
		m.recordDiscoveryPersistenceFailureLocked(err)
	}
}

func (m *Manager) recordDiscoveryPersistenceFailureLocked(err error) {
	if m.diag == nil {
		return
	}
	m.diag.RecordEvent("discovery", "state_persistence_failed", m.cfgName, "discovery state persistence failed", "discovery.persistence.failed", map[string]any{"detail": err.Error()})
}
