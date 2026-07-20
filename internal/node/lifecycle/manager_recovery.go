package lifecycle

import (
	"context"

	"ardents/internal/diagnostics"
	discovery "ardents/internal/discovery"
	noderecovery "ardents/internal/node/recovery"
)

func (m *Manager) bootstrapDiscoveryLocked(ctx context.Context) error {
	sources := noderecovery.NetworkBootstrapSources(m.bootSources)
	if len(sources) == 0 {
		m.diag.SetSubsystem("transport", diagnostics.HealthReady, nil)
		return nil
	}
	result, err := discovery.FetchPrivateRecords(ctx, sources, m.privacy, m.trans)
	if err != nil {
		m.degradeTransportBootstrapLocked(
			"transport.bootstrap.fetch_failed",
			err.Error(),
			"bootstrap peer records could not be retrieved",
			"node remains controllable but remote discovery is incomplete",
		)
		return nil
	}
	if m.stopAfterPrivateBootstrapFailureLocked(result) {
		return nil
	}
	if len(result.Entries) == 0 && result.Replayed == 0 && len(m.disco.Entries()) == 0 {
		m.degradeTransportBootstrapLocked(
			"transport.bootstrap.empty",
			"no discovery records returned by bootstrap peers",
			"bootstrap peers did not provide discovery records",
			"node remains controllable but remote discovery is incomplete",
		)
		return nil
	}
	hadImportErrors := m.importBootstrapEntriesLocked(result.Entries)
	if !hadImportErrors && result.Reason == "" {
		m.setDiscoveryReadyLocked()
	}
	m.diag.SetSubsystem("transport", diagnostics.HealthReady, nil)
	m.diag.RecordEvent("transport", "bootstrap_synced", m.cfgName, "bootstrap peer records synchronized", "", map[string]any{
		"records": len(result.Entries), "rejected": result.Rejected, "replayed": result.Replayed,
	})
	return nil
}

func (m *Manager) refreshPrivateDiscoveryLocked(ctx context.Context) error {
	sources := noderecovery.NetworkBootstrapSources(m.bootSources)
	if len(sources) == 0 {
		return nil
	}
	result, err := discovery.FetchPrivateRecords(ctx, sources, m.privacy, m.trans)
	if err != nil {
		return err
	}
	if result.Reason != "" {
		m.degradeDiscoveryPrivacyLocked(result.Reason, result.Rejected)
		if len(result.Entries) == 0 {
			return nil
		}
	}
	m.importBootstrapEntriesLocked(result.Entries)
	m.diag.RecordEvent("discovery", "remote_refreshed", m.cfgName, "remote discovery records refreshed", "", map[string]any{
		"records": len(result.Entries), "rejected": result.Rejected, "replayed": result.Replayed,
	})
	return nil
}

func (m *Manager) stopAfterPrivateBootstrapFailureLocked(result discovery.PrivateFetchResult) bool {
	if result.Reason == "" {
		return false
	}
	m.degradeDiscoveryPrivacyLocked(result.Reason, result.Rejected)
	if len(result.Entries) > 0 {
		return false
	}
	m.diag.SetSubsystem("transport", diagnostics.HealthReady, nil)
	return true
}

func (m *Manager) degradeDiscoveryPrivacyLocked(code string, rejected int) {
	noderecovery.DegradeDiscoveryPrivacy(
		m.diag, m.cfgName, code, rejected,
		m.setDiscoveryDegradedLocked,
		m.adoptPrimaryReasonLocked,
		m.moveLifecycleLocked,
	)
}

func (m *Manager) importBootstrapEntriesLocked(entries []discovery.Entry) bool {
	return noderecovery.ImportBootstrapEntries(
		m.ident.NodeSummary().Principal,
		entries,
		func(record discovery.Record) (bool, error) {
			result, err := m.disco.Import(record, "bootstrap")
			if err != nil {
				return false, err
			}
			return result.Applied, nil
		},
		m.degradeDiscoveryImportLocked,
		m.authority.SyncDiscoveryTrustDiagnosticsLocked,
	)
}

func (m *Manager) degradeTransportBootstrapLocked(code, detail, summary, impact string) {
	snapshot := m.trans.ProfileSnapshot()
	payload := m.transportProfilePayloadLocked()
	payload["detail"] = detail
	qualifiedDetail := "profile " + string(snapshot.Profile) + ", mode " + string(snapshot.Mode) + ": " + detail
	noderecovery.DegradeTransportBootstrap(
		m.diag,
		m.cfgName,
		code,
		summary,
		qualifiedDetail,
		impact,
		payload,
		m.adoptPrimaryReasonLocked,
		m.moveLifecycleLocked,
	)
}

func (m *Manager) degradeDiscoveryImportLocked(recordID, detail string) {
	noderecovery.DegradeDiscoveryImport(
		m.diag,
		recordID,
		detail,
		m.setDiscoveryDegradedLocked,
		m.adoptPrimaryReasonLocked,
		m.moveLifecycleLocked,
	)
}
