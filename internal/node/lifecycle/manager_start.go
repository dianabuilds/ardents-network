package lifecycle

import (
	"context"
	"crypto/ed25519"

	identityapi "ardents/internal/identity/api"
	networkprivacy "ardents/internal/network/privacy"
	nodereadiness "ardents/internal/node/readiness"
	noderecovery "ardents/internal/node/recovery"
)

func (m *Manager) StartLocked(ctx context.Context) error {
	if m.life.State() == Ready || m.life.State() == Degraded {
		return nil
	}

	m.moveLifecycleLocked(Starting)
	if !m.loadDiagnosticsLocked() {
		return m.runtimeFailureLocked("start")
	}
	m.publish("node.starting", map[string]any{"id": m.cfgName, "state": Starting})
	m.diag.RecordEvent("node", "starting", m.cfgName, "node startup started", "", map[string]any{"id": m.cfgName})
	m.diag.MarkRecoveringExcept("", "operation recovered after restart")
	m.moveLifecycleLocked(Initializing)

	if !m.startupStateLoadLocked(ctx) {
		return m.runtimeFailureLocked("start")
	}

	var identPrivateKey ed25519.PrivateKey
	if !m.initializeIdentityLocked(ctx, &identPrivateKey) {
		return m.runtimeFailureLocked("start")
	}

	if err := m.trans.Start(ctx); err != nil {
		m.FailLocked("node.transport.start_failed", "transport", "transport start failed", err.Error(), "network plane unavailable", "restart_required")
		return m.runtimeFailureLocked("start")
	}
	m.setTransportHealthLocked()

	if !m.publishDiscoveryLocked(ctx, identPrivateKey) {
		return m.runtimeFailureLocked("start")
	}

	if !m.startupWorkloadsLocked(ctx) {
		return m.runtimeFailureLocked("start")
	}

	m.finishBootLocked(ctx)
	return m.runtimeFailureLocked("start")
}

func (m *Manager) runStartupStepLocked(_ context.Context, kind, domain, resource string, recoverable bool, recoveryAction string, fn func() error) bool {
	return noderecovery.RunStartupStep(
		m.diag,
		kind,
		domain,
		resource,
		recoverable,
		recoveryAction,
		m.FailLocked,
		fn,
	)
}

func (m *Manager) loadDiagnosticsLocked() bool {
	return noderecovery.LoadDiagnosticsForStartup(m.diag, m.FailLocked)
}

func (m *Manager) startupStateLoadLocked(ctx context.Context) bool {
	return m.runStartupStepLocked(ctx, StartupPhaseStateLoad, "node", "state", false, "", func() error {
		return noderecovery.LoadStartupState(
			m.state.Load,
			m.disco.Load,
			m.authority.LoadData,
			m.authority.LoadWorkloads,
		)
	})
}

func (m *Manager) initializeIdentityLocked(ctx context.Context, out *ed25519.PrivateKey) bool {
	return m.runStartupStepLocked(ctx, StartupPhaseIdentity, "identity", "local", false, "", func() error {
		return noderecovery.InitializeIdentityForStartup(
			func() (identityapi.Summary, ed25519.PrivateKey, error) {
				summary, privateKey, err := m.ident.EnsureNode(m.state, m.keys)
				if err == nil {
					*out = privateKey
				}
				return summary, privateKey, err
			},
			m.setPrivate,
			m.authority.SetLocalDataNodeID,
			m.trust.Trust,
			m.authority.SyncDiscoveryTrustDiagnosticsLocked,
		)
	})
}

func (m *Manager) publishDiscoveryLocked(ctx context.Context, privateKey ed25519.PrivateKey) bool {
	return m.runStartupStepLocked(ctx, StartupPhaseDiscovery, "discovery", "records", false, "", func() error {
		if err := m.publication.RefreshNetworkPublicationLocked(ctx); err != nil {
			if !networkprivacy.IsCapabilityFailure(err) {
				return err
			}
			m.degradeDiscoveryPrivacyLocked(networkprivacy.CodeOf(err), 0)
		}
		return noderecovery.PublishDiscoveryForStartup(
			ctx,
			privateKey,
			func(context.Context) error { return nil },
			m.bootstrapDiscoveryLocked,
		)
	})
}

func (m *Manager) startupWorkloadsLocked(ctx context.Context) bool {
	return m.runStartupStepLocked(ctx, StartupPhaseWorkloads, "workload", "workloads", true, "restart node", func() error {
		return noderecovery.StartWorkloadsForStartup(
			ctx,
			m.workloadSpecs,
			m.authority.SeedWorkloadsAndReconcileLocked,
		)
	})
}

func (m *Manager) finishBootLocked(_ context.Context) {
	transportBoot := m.trans.BootstrapStatus()
	m.setTransportHealthLocked()
	m.diag.RecordEvent("transport", "profile_active", m.cfgName, "transport profile is active", "", m.transportProfilePayloadLocked())
	m.syncBootHealthLocked(transportBoot)
	noderecovery.CompleteBoot(
		m.diag,
		transportBoot,
		m.setDiscoveryDegradedLocked,
		m.moveLifecycleLocked,
		m.diag.RetainCurrentHealth,
	)
	m.publish("node.started", map[string]any{"id": m.cfgName, "state": m.life.State()})
	m.diag.RecordEvent("node", "started", m.cfgName, "node startup completed", nodereadiness.CurrentPrimaryReasonCode(m.diag), map[string]any{"id": m.cfgName, "state": m.life.State()})
}

func (m *Manager) setTransportHealthLocked() {
	nodereadiness.ApplyTransportHealth(m.diag, m.trans.State(), m.trans.Reason(), m.trans.ProfileSnapshot())
}
