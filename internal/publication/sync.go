package publication

import (
	"context"
	"crypto/ed25519"
	"fmt"

	discovery "ardents/internal/discovery"
	discoverysource "ardents/internal/discovery/source"
	hostingexposure "ardents/internal/hosting/exposure"
	hostingservice "ardents/internal/hosting/service"
	identityapi "ardents/internal/identity/api"
	"time"
)

func (m *Manager) SyncDesiredLocked(ctx context.Context) error {
	id := m.ident.NodeSummary()
	private := m.privateKey()
	if id.Principal == "" || len(private) == 0 {
		return nil
	}
	if _, err := m.workload.RefreshObserved(ctx); err != nil {
		return err
	}
	previousEntries := m.localDiscoveryEntriesLocked()
	if err := m.syncLocalDesiredLocked(ctx, id, private); err != nil {
		return err
	}
	m.publicationAttempted = true
	if err := m.publishDiscoveryEntriesWithCompensationLocked(ctx, previousEntries); err != nil {
		return err
	}
	m.networkPublished = true
	return nil
}

func (m *Manager) SyncLocalDesiredLocked() error {
	id := m.ident.NodeSummary()
	private := m.privateKey()
	if id.Principal == "" || len(private) == 0 {
		return nil
	}
	return m.syncLocalDesiredLocked(context.Background(), id, private)
}

func (m *Manager) localDiscoveryEntriesLocked() []discovery.Entry {
	return discoverysource.LocalEntries(m.disco.Entries())
}

func (m *Manager) syncLocalDesiredLocked(ctx context.Context, id identityapi.Summary, private ed25519.PrivateKey) error {
	if err := WithdrawAllLocalServices(m.disco, id, private); err != nil {
		return err
	}
	if err := m.publishDesiredServicesLocked(ctx, id, private); err != nil {
		return err
	}
	return nil
}

func (m *Manager) publishDesiredServicesLocked(ctx context.Context, id identityapi.Summary, private ed25519.PrivateKey) error {
	services, denied, err := m.publicationPlanLocked(ctx)
	if err != nil {
		return err
	}
	for _, item := range denied {
		m.publish("policy.denied", deniedPayload(item.ID, "service.publish", item.Err))
	}
	for _, svc := range services {
		if err := m.publishDesiredServiceLocked(id, private, svc); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) publicationPlanLocked(ctx context.Context) ([]hostingservice.Spec, []hostingexposure.Denial, error) {
	if err := m.observeHostingReadinessLocked(ctx); err != nil {
		return nil, nil, err
	}
	if m.srv == nil || m.trans == nil {
		return nil, nil, nil
	}
	allowed, denied := publicationGatePlan(m.srv.ServiceStatuses(time.Now().UTC()), m.trans.ReachabilitySnapshot(), m.policy.AllowServicePublication)
	return allowed, denied, nil
}

func (m *Manager) publishDesiredServiceLocked(id identityapi.Summary, private ed25519.PrivateKey, svc hostingservice.Spec) error {
	return PublishLocalService(m.disco, id, private, LocalServiceSpec{
		ID:        svc.ID,
		Type:      svc.Type,
		Owner:     svc.Owner,
		Mode:      svc.Mode,
		Endpoints: cloneStrings(svc.Endpoints),
	})
}

func (m *Manager) publishDiscoveryEntriesWithCompensationLocked(ctx context.Context, previousEntries []discovery.Entry) error {
	if err := m.publishDiscoveryEntries(ctx, m.localDiscoveryEntriesLocked()); err != nil {
		if requiresNetworkCompensation(err) {
			compensateCtx, cancel := rollbackContext(ctx)
			defer cancel()
			if compensateErr := m.compensateNetworkLocked(compensateCtx, previousEntries); compensateErr != nil {
				return fmt.Errorf("%w; compensation failed: %v", err, compensateErr)
			}
		}
		return err
	}
	m.ClearRollbackLocked()
	return nil
}

func cloneStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	return append([]string(nil), in...)
}
