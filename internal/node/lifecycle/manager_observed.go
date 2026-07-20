package lifecycle

import (
	"context"
	"strings"

	transport "ardents/internal/network/api"
	networkprivacy "ardents/internal/network/privacy"
	nodereadiness "ardents/internal/node/readiness"
)

func (m *Manager) SyncObservedTruthLocked() {
	if m.life == nil || m.diag == nil || m.boot == nil || m.trans == nil {
		return
	}
	if !nodereadiness.AllowsObservedSync(m.life.State(), Ready, Degraded) {
		return
	}

	transportBoot := m.trans.BootstrapStatus()
	m.authority.SyncDiscoveryTrustDiagnosticsLocked()
	m.setTransportHealthLocked()
	m.syncBootHealthLocked(transportBoot)
	m.syncPrimaryReasonLocked()
	m.syncLifecycleStateLocked()
}

func (m *Manager) syncBootHealthLocked(transportBoot transport.BootstrapStatus) {
	nodereadiness.SyncBootHealth(m.diag, m.boot, transportBoot)
}

func (m *Manager) syncPrimaryReasonLocked() {
	nodereadiness.SyncPrimaryReason(m.diag)
}

func (m *Manager) syncLifecycleStateLocked() {
	nodereadiness.SyncLifecycleState(m.life, m.diag, m.moveLifecycleLocked)
}

func (m *Manager) RefreshDiscoveryPublicationLocked(ctx context.Context) {
	if err := m.authority.SyncObservedWorkloadsLocked(ctx); err != nil {
		m.recordDiscoveryRefreshFailureLocked(err)
		return
	}
	if err := m.publication.RefreshNetworkPublicationLocked(ctx); err != nil {
		if networkprivacy.IsCapabilityFailure(err) {
			current := nodereadiness.SubsystemReasonCode(m.diag.Health(), "discovery")
			if current == "" || strings.HasPrefix(current, "privacy.capability.") {
				m.degradeDiscoveryPrivacyLocked(networkprivacy.CodeOf(err), 0)
			}
			return
		}
		m.recordDiscoveryRefreshFailureLocked(err)
		return
	}
	if err := m.refreshPrivateDiscoveryLocked(ctx); err != nil {
		m.recordDiscoveryRefreshFailureLocked(err)
		return
	}
	m.clearDiscoveryRefreshFailureLocked()
}

func (m *Manager) recordDiscoveryRefreshFailureLocked(err error) {
	nodereadiness.RecordDiscoveryRefreshFailure(
		m.diag,
		m.cfgName,
		err,
		m.setDiscoveryDegradedLocked,
		m.adoptPrimaryReasonLocked,
		m.moveLifecycleLocked,
		m.publish,
	)
}

func (m *Manager) clearDiscoveryRefreshFailureLocked() {
	nodereadiness.ClearDiscoveryRefreshFailure(
		m.diag,
		m.setDiscoveryReadyLocked,
		m.restorePrimaryReasonLocked,
		m.moveLifecycleLocked,
	)
}
